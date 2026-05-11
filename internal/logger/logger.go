package logger

import (
	"context"
	"log"
	"strings"
)

type Logger struct {
	level string
}

func New(level string) *Logger {
	return &Logger{level: level}
}

func (l *Logger) Debug(ctx context.Context, msg string, keyvals ...interface{}) {
	l.log("DEBUG", msg, keyvals)
}

func (l *Logger) Info(ctx context.Context, msg string, keyvals ...interface{}) {
	l.log("INFO", msg, keyvals)
}

func (l *Logger) Warn(ctx context.Context, msg string, keyvals ...interface{}) {
	l.log("WARN", msg, keyvals)
}

func (l *Logger) Error(ctx context.Context, msg string, keyvals ...interface{}) {
	l.log("ERROR", msg, keyvals)
}

func (l *Logger) Fatal(ctx context.Context, msg string, keyvals ...interface{}) {
	l.log("FATAL", msg, keyvals)
}

func (l *Logger) log(level, msg string, keyvals []interface{}) {
	// Simple logger implementation for infrastructure skeleton
	// Level filtering
	if !l.shouldLog(level) {
		return
	}
	
	log.Printf("[%s] %s", level, msg)
}

func (l *Logger) shouldLog(level string) bool {
	levels := map[string]int{
		"DEBUG": 0,
		"INFO":  1,
		"WARN":  2,
		"ERROR": 3,
		"FATAL": 4,
	}
	
	currentLevel, ok := levels[strings.ToUpper(l.level)]
	if !ok {
		return true
	}
	
	msgLevel, ok := levels[level]
	if !ok {
		return true
	}
	
	return msgLevel >= currentLevel
}