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
	"flag"
	"os"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

type LoggerConfig struct {
	logLevel uint
	logJSON  bool
}

func NewLoggerConfig() *LoggerConfig {
	lc := &LoggerConfig{}
	flag.UintVar(&lc.logLevel, "log-level", uint(logrus.InfoLevel), "Log level from 0 to 6, 6 being the most verbose")
	flag.BoolVar(&lc.logJSON, "log-json", false, "True if the log entries should use JSON format, false for text logging")

	return lc
}

func (lc *LoggerConfig) Check() error {
	if err := lc.validateLogLevel(); err != nil {
		return trace.Wrap(err, "failed to validate the log level flag")
	}

	return nil
}

func (lc *LoggerConfig) setupLogger() {
	if lc.logJSON {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{})
	}
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.Level(lc.logLevel))
	logrus.Debugf("Setup logger with config: %+v", lc)
}

func (lc *LoggerConfig) validateLogLevel() error {
	if lc.logLevel > 6 {
		return trace.BadParameter("the log-level flag should be between 0 and 6")
	}

	return nil
}
