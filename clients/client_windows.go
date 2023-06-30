package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"./db"

	"github.com/AlecAivazis/survey"
	"github.com/briandowns/spinner"
	"github.com/gosuri/uiprogress"
)

var VERSION = "1.0.0"
var BANNER = `    _____________
   /____________/
   ||     ______
   ||    |_____/!
   ||          ||
   ||   _||_   ||
   ||  |_||_|  ||  
   ||__________||
   |/__________|/` + "  by Siriil v" + VERSION + "\n\n"
var BATCH_MAX_SIZE = 300
var BAR *uiprogress.Bar

func main() {
	time_start := time.Now()

	logger := db.NewCustomLogger()
	fmt.Println(BANNER)

	if runtime.GOOS != "windows" {
		logger.Error("The program must be run on Windows S.O.")
	}

	if !isSuperUserWindows() {
		logger.Error("The program must be run with administrator privileges")
	}

	selectedDisk, numCPUs, path := askOptions(logger)
	fmt.Println()

	database, dbfilename, err := db.CreateDatabase()
	if err != nil {
		logger.Error("Error creating the database:", err)
	}
	logger.Info("Database saved as", "'"+dbfilename+"'")

	s := spinner.New(spinner.CharSets[26], 100*time.Millisecond)
	s.Prefix = "Getting files "
	s.Start()
	err = getFiles(database, selectedDisk+path)
	if err != nil {
		logger.Error("Error get files in disk selected:", err)
	}
	s.Stop()
	nrows, _ := database.LenData()
	database.Close()
	logger.Info("Got", nrows, "files")

	var wg sync.WaitGroup
	wg.Add(numCPUs)

	BAR = uiprogress.AddBar(nrows).AppendCompleted().PrependElapsed()
	BAR.PrependFunc(func(b *uiprogress.Bar) string {
		return fmt.Sprintf("[+] Process Files (%d/%d)", b.Current(), nrows)
	})
	uiprogress.Start()
	for i := 1; i <= numCPUs; i++ {
		go processData(logger, dbfilename, nrows, i, numCPUs, &wg)
	}
	wg.Wait()
	uiprogress.Stop()

	logger.Info("Update metadata table")
	processMetadata(logger, dbfilename, time_start)

	time_elapsed := time.Since(time_start)
	logger.Info("Time elapsed:", time_elapsed)
	fmt.Println()

	fmt.Printf("Press ENTER key to close...")
	fmt.Scanln()
	os.Exit(0)
}

func askOptions(logger *db.CustomLogger) (string, int, string) {
	disks, err := getDisksWindows()
	if err != nil {
		logger.Error("Error getting the list of disks:", err)
	}
	var selectedDisk string
	prompt_select := &survey.Select{
		Message: "Select an disk:",
		Options: disks,
	}
	survey.AskOne(prompt_select, &selectedDisk, nil)

	numAvailableCPUs := runtime.NumCPU()
	var cpus []string
	var strNumCPUs string
	for i := 1; i <= numAvailableCPUs; i++ {
		cpus = append(cpus, fmt.Sprintf("%d", i))
	}
	prompt_select = &survey.Select{
		Message: "Select a number of cpu cores:",
		Options: cpus,
	}
	survey.AskOne(prompt_select, &strNumCPUs, nil)
	numCPUs, _ := strconv.Atoi(strNumCPUs)
	runtime.GOMAXPROCS(numCPUs)

	path := ""
	prompt_input := &survey.Input{
		Message: `Specifies path in disk (Leave blank for all) (Example: \Users\User1\):`,
	}
	survey.AskOne(prompt_input, &path)
	if path != "" {
		_, err = os.Stat(selectedDisk + path)
		if err != nil {
			path = ""
			logger.Warning("The path entered is wrong, so the whole disk will be scanned")
		}
	}

	if (selectedDisk == "") || (numCPUs < 1) {
		logger.Error("Error parameter in menu selection")
	}

	return selectedDisk, numCPUs, path
}

func getFiles(database *db.Database, root string) error {
	files := make([]*db.Data, 0)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if (err == nil) && (!info.IsDir()) {
			fileData := &db.Data{
				FullPath:             path,
				FileName:             "",
				FileExtension:        "",
				HashMD5:              "",
				SizeBytes:            -1,
				DateCreation:         "",
				DateLastModification: "",
			}

			files = append(files, fileData)

			if len(files) >= BATCH_MAX_SIZE {
				err := database.InsertDatas(files)
				if err != nil {
					return fmt.Errorf("Failed to insert batch of files: %v", err)
				}
				files = files[:0]
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("Failed to walk through files: %v", err)
	}

	if len(files) > 0 {
		err := database.InsertDatas(files)
		if err != nil {
			return fmt.Errorf("Failed to insert remaining files: %v", err)
		}
	}

	return nil
}

func processData(logger *db.CustomLogger, dbpath string, nrows int, id int, nworkers int, wg *sync.WaitGroup) {
	defer wg.Done()
	total_files := nrows / nworkers
	index_start := 1
	index_end := -1
	for i := 1; i <= id; i++ {
		if i > 1 {
			index_start = index_start + total_files
		}
		if i == nworkers {
			index_end = nrows
		} else {
			index_end = index_start + total_files - 1
		}
	}

	database, err := db.Connect(dbpath)
	if err != nil {
		logger.Error("Error connecting the database in goroutine", id, ":", err)
	}
	defer database.Close()

	index_i := index_start
	index_j := index_start + BATCH_MAX_SIZE
	for index_i <= index_end {
		if index_j > index_end {
			index_j = index_end
		}
		rows, err := database.GetDatas(index_i, index_j)
		if err != nil {
			logger.Error("Error GetDatas in goroutine", id, ":", err)
		}
		nrows := len(rows)
		for _, row := range rows {
			path := row.FullPath
			fileInfo, err := os.Stat(path)
			if err == nil {
				row.FileName = fileInfo.Name()
				row.FileExtension = filepath.Ext(path)
				row.SizeBytes = int(fileInfo.Size())
				row.DateLastModification = fileInfo.ModTime().Format("2006-01-02 15:04:05")
			}
			row.DateCreation, _ = getFileCreationTimeWindows(path)
			row.HashMD5, _ = getFileMD5(path)
			BAR.Incr()
		}
		database.UpdateDatas(rows)

		index_i = index_i + nrows
		if (index_i + BATCH_MAX_SIZE) <= index_end {
			index_j = index_j + BATCH_MAX_SIZE
		} else {
			index_j = index_end
		}
	}
}

func processMetadata(logger *db.CustomLogger, dbpath string, time_start time.Time) {
	database, err := db.Connect(dbpath)
	if err != nil {
		logger.Error("Error connecting the database in processMetadata:", err)
	}
	defer database.Close()
	metadatas := make([]*db.Metadata, 0)

	randomString := "510D5B0B6245A77B40B52C60DF3E0F85480D323C184B3BF6673044E249E12B3F"
	datecreation := time_start.Format("2006-01-02 15:04:05")
	sigmd5, _ := database.GetTableMD5("data", randomString)

	metadata := &db.Metadata{
		SignatureMD5:   sigmd5,
		Challenge:      randomString,
		SO:             runtime.GOOS,
		Architecture:   runtime.GOARCH,
		DateDBCreation: datecreation,
	}
	metadatas = append(metadatas, metadata)
	err = database.InsertMetadatas(metadatas)
	if err != nil {
		logger.Info("Failed to insert batch of metadatas: %v", err)
	}
}

func isSuperUserWindows() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}

func getDisksWindows() ([]string, error) {
	var disks []string

	driveLetters := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	for _, letter := range driveLetters {
		path := string(letter) + ":\\"
		_, err := os.Open(path)
		if err == nil {
			disks = append(disks, path)
		}
	}

	return disks, nil
}

func getFileMD5(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	hashBytes := hash.Sum(nil)
	md5String := hex.EncodeToString(hashBytes)

	return md5String, nil
}

func getFileCreationTimeWindows(path string) (string, error) {
	var fileInfo os.FileInfo
	var err error

	if fileInfo, err = os.Stat(path); err != nil {
		return "", err
	}

	var creationTime time.Time
	winFileSys := fileInfo.Sys().(*syscall.Win32FileAttributeData)
	nsec := winFileSys.CreationTime.Nanoseconds()
	creationTime = time.Unix(0, nsec)

	formattedTime := creationTime.Format("2006-01-02 15:04:05")
	return formattedTime, nil
}
