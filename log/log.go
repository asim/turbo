package log

import (
	"os"

	log "github.com/sirupsen/logrus"
)

type Fields = log.Fields

func Init() {
	// Set up a file logger
	f, err := os.OpenFile("turbo.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(f)
	log.SetFormatter(&log.TextFormatter{})
}

func Fatal(v ...interface{}) {
	log.Println(v...)
	os.Exit(1)
}

func Print(v ...interface{}) {
	log.Println(v...)
}

func Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func WithFields(v Fields) *log.Entry {
	return log.WithFields(v)
}
