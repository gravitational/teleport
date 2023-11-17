package main

import (
	"github.com/sirupsen/logrus"
)

// This maps the s3sync logging functions to logrus so that they match the
// rest of the logging framework's settings.
// Their docs outline the custom logger configuration here:
// https://github.com/seqsense/s3sync#sets-the-custom-logger
type s3logger struct{}

// Maps the s3sync log function to logrus.
func (s3logger) Log(v ...interface{}) {
	logrus.Debugln(v...)
}

// Maps the s3sync logf function to logrus.
func (s3logger) Logf(format string, v ...interface{}) {
	logrus.Debugf(format, v...)
}
