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

package dbcmd

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

// ConvertCommandError translates some common errors to more user friendly
// messages.
//
// This helps in situations where the user does not have the full context to
// decipher errors when the database command is executed internally (e.g. tsh
// db connect).
func ConvertCommandError(cmd *exec.Cmd, err error, peakStderr string) error {
	switch filepath.Base(cmd.Path) {
	case redisBin:
		if strings.HasPrefix(peakStderr, "Unrecognized option or bad number of args for") {
			// TLS support starting 6.0. "--insecure" flag starting 6.2.
			minVersion := "6.0"
			if utils.SliceContainsStr(cmd.Args, "--insecure") {
				minVersion = "6.2"
			}
			return trace.BadParameter(
				"'%s' has exited with the above error. Please make sure '%s' with version %s or newer is installed.",
				cmd.Path,
				redisBin,
				minVersion,
			)
		}
	}

	return trace.Wrap(err)
}
