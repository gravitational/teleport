// +build !windows

/*
Copyright 2018 Gravitational, Inc.

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

package utils

import (
	"io/ioutil"
	"log/syslog"
	"os"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	logrusSyslog "github.com/sirupsen/logrus/hooks/syslog"
)

// SwitchLoggingtoSyslog tells the logger to send the output to syslog. This
// code is behind a build flag because Windows does not support syslog.
func SwitchLoggingtoSyslog() error {
	log.StandardLogger().SetHooks(make(log.LevelHooks))
	hook, err := logrusSyslog.NewSyslogHook("", "", syslog.LOG_WARNING, "")
	if err != nil {
		// syslog is not available
		log.SetOutput(os.Stderr)
		return trace.Wrap(err)
	}
	log.AddHook(hook)
	// ... and disable stderr:
	log.SetOutput(ioutil.Discard)
	return nil
}
