<p align="center"><img src="images/main_logo.jpg"></p>

# Description
**GDBack** is a tool developed in Golang to extract relevant information about the system. This can be useful when performing an investigation on a computer. The collected data is stored in a sqlite file.
### Features
 - **Extracts parameters of files** from a disk or from a specific path with different parameters: their name, path, MD5 hash, size, modification and creation dates.
# Usage

## Windows
Download the executable file in [Client Windows v1.0.0](https://github.com/siriil/gdback/releases/download/v1.0.0/client_windows.exe) and run it as administrator, right click with the mouse and then Run as administrator.

# Screenshots
<p align="center"><img src="images/screenshot_windows_running.png"></p>
</br>
<p align="center"><img src="images/sqlite.png"></p>

# Build

## Dependency
```
go get github.com/AlecAivazis/survey
go get github.com/briandowns/spinner
go get github.com/gosuri/uiprogress
```

## Windows
```
go env -w GO111MODULE=off
$Env:CGO_ENABLED = 1
go build client_windows.go
```
