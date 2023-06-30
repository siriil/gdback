package db

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	conn *sql.DB
}
type Data struct {
	ID                   int
	FullPath             string
	FileName             string
	FileExtension        string
	HashMD5              string
	SizeBytes            int
	DateCreation         string
	DateLastModification string
}
type Metadata struct {
	ID             int
	SignatureMD5   string
	Challenge      string
	SO             string
	Architecture   string
	DateDBCreation string
}

func (db *Database) setup() error {
	metadataTable := `
		CREATE TABLE IF NOT EXISTS metadata (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			signature_md5 TEXT,
			challenge TEXT,
			so TEXT,
			architecture TEXT,
			date_db_creation TEXT
		);
	`

	dataTable := `
		CREATE TABLE IF NOT EXISTS data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			full_path TEXT,
			file_name TEXT,
			file_extension TEXT,
			hash_md5 TEXT,
			size_bytes INTEGER,
			date_creation TEXT,
			date_last_modification TEXT
		);
	`

	_, err := db.conn.Exec(metadataTable)
	if err != nil {
		return fmt.Errorf("Failed to create metadata table: %v", err)
	}

	_, err = db.conn.Exec(dataTable)
	if err != nil {
		return fmt.Errorf("Failed to create data table: %v", err)
	}

	return nil
}

func CreateDatabase() (*Database, string, error) {
	db := &Database{}
	dbFileName := strconv.FormatInt(time.Now().Unix(), 10) + ".sqlite"
	conn, err := sql.Open("sqlite3", dbFileName)
	if err != nil {
		return nil, dbFileName, fmt.Errorf("Failed to create database: %v", err)
	}
	db.conn = conn

	err = db.setup()
	if err != nil {
		return nil, dbFileName, fmt.Errorf("Failed to setup database: %v", err)
	}

	return db, dbFileName, nil
}

func Connect(dbpath string) (*Database, error) {
	db := &Database{}
	conn, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		return nil, fmt.Errorf("Failed to open database connection: %v", err)
	}

	db.conn = conn

	return db, nil
}

func (db *Database) Close() error {
	return db.conn.Close()
}

func (db *Database) GetDatas(startID, endID int) ([]*Data, error) {
	query := "SELECT * FROM data WHERE id >= ? AND id <= ?"
	rows, err := db.query(query, startID, endID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var datas []*Data
	for rows.Next() {
		data := &Data{}
		err := rows.Scan(&data.ID, &data.FullPath, &data.FileName, &data.FileExtension, &data.HashMD5, &data.SizeBytes, &data.DateCreation, &data.DateLastModification)
		if err != nil {
			return nil, err
		}
		datas = append(datas, data)
	}

	return datas, nil
}

func (db *Database) InsertDatas(datas []*Data) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("Failed to start transaction: %v", err)
	}

	query := `
		INSERT INTO data (full_path, file_name, file_extension, hash_md5, size_bytes, date_creation, date_last_modification)
		VALUES (?, ?, ?, ?, ?, ?, ?);
	`

	stmt, err := tx.Prepare(query)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("Failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	for _, data := range datas {
		_, err = stmt.Exec(data.FullPath, data.FileName, data.FileExtension, data.HashMD5, data.SizeBytes, data.DateCreation, data.DateLastModification)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("Failed to insert data: %v", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("Failed to commit transaction: %v", err)
	}

	return nil
}

func (db *Database) UpdateDatas(datas []*Data) error {
	if len(datas) == 0 {
		return nil
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("Error initiating transaction: %v", err)
	}

	query := `
		UPDATE data
		SET full_path = ?,
			file_name = ?,
			file_extension = ?,
			hash_md5 = ?,
			size_bytes = ?,
			date_creation = ?,
			date_last_modification = ?
		WHERE id = ?;
	`

	for _, data := range datas {
		_, err := tx.Exec(query, data.FullPath, data.FileName, data.FileExtension, data.HashMD5, data.SizeBytes, data.DateCreation, data.DateLastModification, data.ID)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("Error updating table 'data': %v", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("Error while committing transaction: %v", err)
	}

	return nil
}

func (db *Database) InsertMetadatas(metadatas []*Metadata) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("Failed to start transaction: %v", err)
	}

	query := `
		INSERT INTO metadata (signature_md5, challenge, so, architecture, date_db_creation)
		VALUES (?, ?, ?, ?, ?);
	`

	stmt, err := tx.Prepare(query)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("Failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	for _, metadata := range metadatas {
		_, err = stmt.Exec(metadata.SignatureMD5, metadata.Challenge, metadata.SO, metadata.Architecture, metadata.DateDBCreation)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("Failed to insert metadata: %v", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("Failed to commit transaction: %v", err)
	}

	return nil
}

func (db *Database) LenData() (int, error) {
	query := `
		SELECT COUNT(*)
		FROM data;
	`

	var count int
	err := db.conn.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("Error when obtaining the number of rows in table 'data'.: %v", err)
	}

	return count, nil
}

func (db *Database) query(query string, args ...interface{}) (*sql.Rows, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (db *Database) GetTableMD5(tableName string, salt string) (string, error) {
	query := fmt.Sprintf("SELECT * FROM %s", tableName)

	rows, err := db.query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	hashes := make([]byte, 0)

	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}

	values := make([]interface{}, len(columns))
	for i := range values {
		var value interface{}
		values[i] = &value
	}

	for rows.Next() {
		err := rows.Scan(values...)
		if err != nil {
			return "", err
		}

		for _, value := range values {
			hashInput := fmt.Sprintf("%s%v", salt, *value.(*interface{}))
			hash := md5.Sum([]byte(hashInput))
			hashes = append(hashes, hash[:]...)
		}
	}

	finalHash := md5.Sum(hashes)

	return hex.EncodeToString(finalHash[:]), nil
}

type CustomLogger struct {
	infoLogger    *log.Logger
	errorLogger   *log.Logger
	warningLogger *log.Logger
}

func NewCustomLogger() *CustomLogger {
	return &CustomLogger{
		infoLogger:    log.New(os.Stdout, "[+] ", log.LstdFlags),
		errorLogger:   log.New(os.Stderr, "[x] ", log.LstdFlags),
		warningLogger: log.New(os.Stdout, "[!] ", log.LstdFlags),
	}
}

func (c *CustomLogger) Write(p []byte) (n int, err error) {
	c.infoLogger.Output(2, string(p))
	return len(p), nil
}

func (c *CustomLogger) Info(v ...interface{}) {
	c.infoLogger.Println(v...)
}

func (c *CustomLogger) Error(v ...interface{}) {
	c.errorLogger.Println(v...)
	fmt.Println()
	os.Exit(0)
}

func (c *CustomLogger) Warning(v ...interface{}) {
	c.warningLogger.Println(v...)
}
