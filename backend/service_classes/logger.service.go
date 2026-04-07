package serviceclasses

import "fmt"

type LoggerService struct {
}

func NewLoggerService() *LoggerService {
	return &LoggerService{}
}

func (l *LoggerService) Log(message string) {
	fmt.Print(message)
}

func (l *LoggerService) Error(message string) {

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

func (l *LoggerService) Print(message string) {

}

func (l *LoggerService) Println(message string) {

}

func (l *LoggerService) Printf(message string) {

}

func (l *LoggerService) Printlnf(message string) {

}