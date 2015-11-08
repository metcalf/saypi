package log

import (
	"bytes"
	"fmt"
	"log"
	"os"
)

// TODO: Report metrics from logging

var logger *log.Logger

func init() {
	logger = log.New(os.Stderr, "", log.LstdFlags)
}

func Print(event, msg string, data map[string]interface{}) {
	var buf bytes.Buffer

	if data != nil && len(data) > 0 {
		first := true

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

	logger.Printf("%s: %s%s", event, msg, buf.String())
}

func Fatal(event, msg string, data map[string]interface{}) {
	Print(event, msg, data)
	os.Exit(1)
}
