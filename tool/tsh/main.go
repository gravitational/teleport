/*
Copyright 2015 Gravitational, Inc.

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
	"io/ioutil"
	"log/syslog"
	"os"

	log "github.com/Sirupsen/logrus"
	logrus_syslog "github.com/Sirupsen/logrus/hooks/syslog"
	"github.com/gravitational/teleport/tool/tsh/tsh"
)

func main() {
	initLogger()

	err := tsh.RunTSH(os.Args)
	if err != nil {
		log.Errorf(err.Error())
		os.Exit(-1)
	}
}

func initLogger() {
	// configure logrus to use syslog:
	hook, err := logrus_syslog.NewSyslogHook("", "", syslog.LOG_ERR, "")
	if err != nil {
		panic(err)
	}
	log.AddHook(hook)
	// ... and disable its own output:
	log.SetOutput(ioutil.Discard)
	log.SetLevel(log.InfoLevel)
}
