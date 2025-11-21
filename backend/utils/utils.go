package utils

import (
	"log"
	"os"
)

var Logger *log.Logger

func init() {
	f, err := os.OpenFile("log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}

	Logger = log.New(f, "FES_LOG: ", log.Ldate|log.Ltime|log.Lshortfile)
}
