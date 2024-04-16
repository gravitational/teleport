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

package shell

import (
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

const (
	DefaultShell = "/bin/sh"
)

// GetLoginShell determines the login shell for a given username.
func GetLoginShell(username string) (string, error) {
	var err error
	var shellcmd string

	shellcmd, err = getLoginShell(username)
	if err != nil {
		if trace.IsNotFound(err) {
			logrus.Warnf("No shell specified for %v, using default %v.", username, DefaultShell)
			return DefaultShell, nil
		}
		return "", trace.Wrap(err)
	}

	return shellcmd, nil
}
