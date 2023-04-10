/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
