package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

var levelNames = map[Level]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
	LevelFatal: "FATAL",
}

var levelColors = map[Level]string{
	LevelDebug: "\033[36m",
	LevelInfo:  "\033[32m",
	LevelWarn:  "\033[33m",
	LevelError: "\033[31m",
	LevelFatal: "\033[35m",
}

const resetColor = "\033[0m"

type Logger struct {
	level   Level
	out     io.Writer
	errOut  io.Writer
	verbose bool
	prefix  string
}

func New(verbose bool) *Logger {
	return &Logger{
		level:   LevelInfo,
		out:     os.Stdout,
		errOut:  os.Stderr,
		verbose: verbose,
		prefix:  "staticgen",
	}
}

func (l *Logger) SetLevel(level Level) {
	l.level = level
}

func (l *Logger) SetVerbose(v bool) {
	l.verbose = v
	if v {
		l.level = LevelDebug
	}
}

func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	now := time.Now().Format("15:04:05")
	color := levelColors[level]
	name := levelNames[level]

	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("%s[%s] %s%s %s: %s\n", color, now, l.prefix, resetColor, name, msg)

	if level >= LevelError {
		fmt.Fprint(l.errOut, line)
	} else {
		fmt.Fprint(l.out, line)
	}

	if level == LevelFatal {
		os.Exit(1)
	}
}

func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(LevelFatal, format, args...)
}

func (l *Logger) Success(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	now := time.Now().Format("15:04:05")
	line := fmt.Sprintf("\033[32m[%s] %s\033[0m %s\n", now, l.prefix, msg)
	fmt.Fprint(l.out, line)
}

func StripANSI(s string) string {
	for _, color := range levelColors {
		s = strings.ReplaceAll(s, color, "")
	}
	s = strings.ReplaceAll(s, resetColor, "")
	return s
}
