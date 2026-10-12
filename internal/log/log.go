// Copyright 2026 dywoq - Apache License 2.0
// https://github.com/dywoq/scintilla

package log

import (
	"fmt"
	"log"
)

func Info(l *log.Logger, format string, v ...any) {
	if l == nil {
		return
	}
	l.Println("[info]", fmt.Sprintf(format, v...))
}

func Err(l *log.Logger, format string, v ...any) {
	if l == nil {
		return
	}
	l.Println("[error]", fmt.Sprintf(format, v...))
}

func Warn(l *log.Logger, format string, v ...any) {
	if l == nil {
		return
	}
	l.Println("[warn]", fmt.Sprintf(format, v...))
}
