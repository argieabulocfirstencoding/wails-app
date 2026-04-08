package service_classes

import "fmt"

type LoggerService struct {
}

func NewLoggerService() *LoggerService {
	return &LoggerService{}
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

func (l *LoggerService) Print(a ...any) {
	fmt.Print(a)
}

func (l *LoggerService) Println(a ...any) {
	fmt.Println(a)
}

func (l *LoggerService) Printf(format string, a ...any) {
	fmt.Printf(format, a)
}

func (l *LoggerService) Printlnf(message string) {

}
