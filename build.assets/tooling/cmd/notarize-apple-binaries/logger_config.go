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
	"os"

	"github.com/sirupsen/logrus"
)

type LoggerConfig struct {
	logLevel logrus.Level
	logJSON  bool
}

func NewLoggerConfig(logLevel logrus.Level, logJSON bool) *LoggerConfig {
	return &LoggerConfig{
		logLevel: logLevel,
		logJSON:  logJSON,
	}
}

func (lc *LoggerConfig) setupLogger() {
	if lc.logJSON {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{})
	}
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(lc.logLevel)
	logrus.Debugf("Setup logger with config: %+v", lc)
}
