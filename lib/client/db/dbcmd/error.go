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
)

// ConvertCommandError translates some common errors to more user friendly
// messages.
//
// This helps in situations where the user does not have the full context to
// decipher errors when the database command is executed internally (e.g.
// command executed through "tsh db connect").
func ConvertCommandError(cmd *exec.Cmd, err error, peakStderr string) error {
	switch filepath.Base(cmd.Path) {
	case redisBin:
		// This redis-cli "Unrecognized option ..." error can be confusing to
		// users that they may think it is the `tsh` binary that is not
		// recognizing the flag.
		if strings.HasPrefix(peakStderr, "Unrecognized option or bad number of args for") {
			// TLS support starting 6.0. "--insecure" flag starting 6.2.
			return trace.BadParameter(
				"'%s' has exited with the above error. Please make sure '%s' with version 6.2 or newer is installed.",
				cmd.Path,
				redisBin,
			)
		}
	}

	lowerCaseStderr := strings.ToLower(peakStderr)
	if strings.Contains(lowerCaseStderr, "access to db denied") {
		fmtString := "%v: '%s' exited with the above error. Use 'tsh db ls' to see your available logins, " +
			"or ask your Teleport administrator to grant you access." +
			"\nSee https://goteleport.com/docs/database-access/troubleshooting/#access-to-db-denied for more information."
		return trace.AccessDenied(fmtString, err, cmd.Path)
	}
	return trace.Wrap(err)
}

const (
	// PeakStderrSize is the recommended size for capturing stderr that is used
	// for ConvertCommandError.
	PeakStderrSize = 100
)
