package utils

import (
	"fmt"
	"log"
	"os"
	"strings"
)

type LoggerService struct {
	logger *log.Logger
}

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

var configuredLogLevel = loadLogLevel()

func loadLogLevel() LogLevel {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("FES_LOG_LEVEL"))) {
	case "debug":
		return LevelDebug
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

func levelLabel(level LogLevel) string {
	switch level {
	case LevelDebug:
		return "DEBUG"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "INFO"
	}
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
	l.log(LevelWarn, message)
}

func (l *LoggerService) Info(message string) {
	l.log(LevelInfo, message)
}

func (l *LoggerService) Debug(message string) {
	l.log(LevelDebug, message)
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

func (l *LoggerService) Infof(format string, a ...any) {
	l.log(LevelInfo, fmt.Sprintf(format, a...))
}

func (l *LoggerService) Warnf(format string, a ...any) {
	l.log(LevelWarn, fmt.Sprintf(format, a...))
}

func (l *LoggerService) ErrorLogf(format string, a ...any) {
	l.log(LevelError, fmt.Sprintf(format, a...))
}

func (l *LoggerService) Debugf(format string, a ...any) {
	l.log(LevelDebug, fmt.Sprintf(format, a...))
}

func (l *LoggerService) log(level LogLevel, message string) {
	if level < configuredLogLevel {
		return
	}
	l.getLogger().Printf("level=%s msg=%q", levelLabel(level), strings.TrimSpace(message))
}
