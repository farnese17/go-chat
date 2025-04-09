package storage

import (
	"log"
	"os"
)

type Logger struct {
	logger *log.Logger
	file   *os.File
}

func SetupLogger(path string) (*Logger, error) {
	if path == "" {
		path = "/var/log/storage"
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}

	logFile, err := os.OpenFile(path+"/log.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0664)
	if err != nil {
		return nil, err
	}

	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Llongfile)
	logger := log.New(logFile, "", log.Ldate|log.Ltime|log.Llongfile)

	return &Logger{logger: logger, file: logFile}, nil
}

func (l *Logger) Close() {
	l.file.Close()
}
