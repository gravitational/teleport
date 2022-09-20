//go:build go1.16
// +build go1.16

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package log

import (
	"fmt"
	"os"
	"time"
)

// Classification is used to group entries.  Each group can be toggled on or off
type Classification string

const (
	// Request entries contain information about HTTP requests.
	// This includes information like the URL, query parameters, and headers.
	Request Classification = "Request"

	// Response entries containe information about HTTP responses.
	// This includes information like the HTTP status code, headers, and request URL.
	Response Classification = "Response"

	// RetryPolicy entries contain information specific to the rety policy in use.
	RetryPolicy Classification = "RetryPolicy"

	// LongRunningOperation entries contian information specific to long-running operations.
	// This includes information like polling location, operation state, and sleep intervals.
	LongRunningOperation Classification = "LongRunningOperation"
)

// logger controls which classifications to log and writing to the underlying log.
type logger struct {
	cls []Classification
	lst func(Classification, string)
}

// SetClassifications is used to control which classifications are written to
// the log.  By default all log classifications are writen.
func SetClassifications(cls ...Classification) {
	log.cls = cls
}

// SetListener will set the Logger to write to the specified listener.
func SetListener(lst func(Classification, string)) {
	log.lst = lst
}

// Should returns true if the specified log classification should be written to the log.
// By default all log classifications will be logged.  Call SetClassification() to limit
// the log classifications for logging.
// If no listener has been set this will return false.
// Calling this method is useful when the message to log is computationally expensive
// and you want to avoid the overhead if its log classification is not enabled.
func Should(cls Classification) bool {
	if log.lst == nil {
		return false
	}
	if log.cls == nil || len(log.cls) == 0 {
		return true
	}
	for _, c := range log.cls {
		if c == cls {
			return true
		}
	}
	return false
}

// Write invokes the underlying listener with the specified classification and message.
// If the classification shouldn't be logged or there is no listener then Write does nothing.
func Write(cls Classification, message string) {
	if !Should(cls) {
		return
	}
	log.lst(cls, message)
}

// Writef invokes the underlying listener with the specified classification and formatted message.
// If the classification shouldn't be logged or there is no listener then Writef does nothing.
func Writef(cls Classification, format string, a ...interface{}) {
	if !Should(cls) {
		return
	}
	log.lst(cls, fmt.Sprintf(format, a...))
}

// TestResetClassifications is used for testing purposes only.
func TestResetClassifications() {
	log.cls = nil
}

// the process-wide logger
var log logger

func init() {
	initLogging()
}

// split out for testing purposes
func initLogging() {
	if cls := os.Getenv("AZURE_SDK_GO_LOGGING"); cls == "all" {
		// cls could be enhanced to support a comma-delimited list of log classifications
		log.lst = func(cls Classification, msg string) {
			// simple console logger, it writes to stderr in the following format:
			// [time-stamp] Classification: message
			fmt.Fprintf(os.Stderr, "[%s] %s: %s\n", time.Now().Format(time.StampMicro), cls, msg)
		}
	}
}
