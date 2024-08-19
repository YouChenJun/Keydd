package log

import (
	"fmt"
	"io"
	"log"
	"os"
)

var (
	Trace     *log.Logger
	Info      *log.Logger
	Warning   *log.Logger
	Error     *log.Logger
	MysalInfo *log.Logger
)

func Init() {
	err := os.MkdirAll("./log", os.ModePerm)
	if err != nil {
		fmt.Println("创建目录失败:", err)
		return
	}
	traceFile, err := os.OpenFile("./log/trace.log",
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Failed to open trace log file:", err)
	}

	infoFile, err := os.OpenFile("./log/info.log",
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Failed to open info log file:", err)
	}

	warningFile, err := os.OpenFile("./log/warning.log",
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Failed to open warning log file:", err)
	}

	errorFile, err := os.OpenFile("./log/error.log",
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Failed to open error log file:", err)
	}
	MysqlFile, err := os.OpenFile("./log/sqlerror.log",
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Failed to open error log file:", err)
	}

	Trace = log.New(io.MultiWriter(traceFile, os.Stdout), "TRACE ", log.Ldate|log.Ltime|log.Lshortfile)
	Info = log.New(io.MultiWriter(infoFile, os.Stdout), "INFO ", log.Ldate|log.Ltime|log.Lshortfile)
	Warning = log.New(io.MultiWriter(warningFile, os.Stdout), "WARNING ", log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(io.MultiWriter(errorFile, os.Stderr), "ERROR ", log.Ldate|log.Ltime|log.Lshortfile)
	MysalInfo = log.New(io.MultiWriter(MysqlFile, os.Stderr), "ERROR ", log.Ldate|log.Ltime|log.Lshortfile)
}
