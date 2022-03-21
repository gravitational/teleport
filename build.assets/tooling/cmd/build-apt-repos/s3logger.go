package main

import (
	"github.com/sirupsen/logrus"
)

type s3logger struct{}

// Maps the s3sync log function to logrus.
func (s3logger) Log(v ...interface{}) {
	logrus.Debugln(v...)
}

// Maps the s3sync logf function to logrus.
func (s3logger) Logf(format string, v ...interface{}) {
	logrus.Debugf(format, v...)
}
