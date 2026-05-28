package utils

import (
	"fmt"
	"log"
	"os"
)

type LoggerService struct {
	logger *log.Logger
}

func (l *LoggerService) getLogger() *log.Logger {
	if l.logger != nil {
		return l.logger
	}
	if Logger != nil {
		l.logger = Logger
		return l.logger
	}
	l.logger = log.New(os.Stdout, "FES_LOG: ", log.Ldate|log.Ltime|log.Lshortfile)
	return l.logger
}

func NewLoggerService() *LoggerService {
	if Logger == nil {
		return &LoggerService{
			logger: log.New(os.Stdout, "FES_LOG: ", log.Ldate|log.Ltime|log.Lshortfile),
		}
	}
	return &LoggerService{logger: Logger}
}

func (l *LoggerService) Error(a ...any) error {
	return fmt.Errorf("%v", fmt.Sprint(a...))
}

func (l *LoggerService) Errorf(format string, a ...any) error {
	return fmt.Errorf(format, a...)
}

func (l *LoggerService) Warn(message string) {

}

func (l *LoggerService) Info(message string) {

}

func (l *LoggerService) Debug(message string) {

}

func (l *LoggerService) Trace(message string) {

}

func (l *LoggerService) Fatal(message string) {

}

func (l *LoggerService) Panic(message string) {

}

func (l *LoggerService) Print(a ...any) {
	l.getLogger().Print(a...)
}

func (l *LoggerService) Println(a ...any) {
	l.getLogger().Println(a...)
}

func (l *LoggerService) Printf(format string, a ...any) {
	l.getLogger().Printf(format, a...)
}

func (l *LoggerService) Printlnf(format string, a ...any) {
	l.getLogger().Printf(format+"\n", a...)
}
