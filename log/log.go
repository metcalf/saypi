package log

import (
	"bytes"
	"fmt"
	"log"
	"os"
)

// TODO: Report metrics from logging

var logger Logger

func init() {
	logger = Logger{log.New(os.Stderr, "", log.LstdFlags)}
}

// Print outputs in a structured format to the standard logger
func Print(event, msg string, data map[string]interface{}) {
	logger.Print(event, msg, data)
}

// Fatal is equivalent to Print followed by os.Exit(1)
func Fatal(event, msg string, data map[string]interface{}) {
	logger.Fatal(event, msg, data)
}

// Logger wraps a stdlib log.Logger to provide structured logging.
type Logger struct {
	Logger *log.Logger
}

// Print outputs in a structured format to the underlying stdlib log.Logger.
func (l *Logger) Print(event, msg string, data map[string]interface{}) {
	var buf bytes.Buffer

	if data != nil && len(data) > 0 {
		first := true

		if len(msg) > 0 {
			buf.WriteRune(' ')
		}

		buf.WriteRune('(')

		for key, value := range data {
			if !first {
				buf.WriteRune(' ')
				first = false
			}
			_, err := buf.WriteString(fmt.Sprintf("%s=%q", key, value))
			if err != nil {
				panic(err)
			}
		}

		buf.WriteRune(')')
	}

	l.Logger.Printf("%s: %s%s", event, msg, buf.String())
}

// Fatal is equivalent to l.Print followed by os.Exit(1)
func (l *Logger) Fatal(event, msg string, data map[string]interface{}) {
	l.Print(event, msg, data)
	os.Exit(1)
}
