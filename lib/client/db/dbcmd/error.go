/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package dbcmd

import (
	"fmt"
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

	// ./tsh.sh -d db connect --db-user=rjones --db-name=foo postgres
	// 	ERROR REPORT:
	// Original Error: *exec.Error exec: &#34;psql&#34;: executable file not found in $PATH
	// Stack Trace:
	// 		github.com/gravitational/teleport/lib/client/db/dbcmd/error.go:58 github.com/gravitational/teleport/lib/client/db/dbcmd.ConvertCommandError
	// 		github.com/gravitational/teleport/tool/tsh/common/db.go:811 github.com/gravitational/teleport/tool/tsh/common.onDatabaseConnect
	// 		github.com/gravitational/teleport/tool/tsh/common/tsh.go:1578 github.com/gravitational/teleport/tool/tsh/common.Run
	// 		github.com/gravitational/teleport/tool/tsh/common/tsh.go:627 github.com/gravitational/teleport/tool/tsh/common.Main
	// 		github.com/gravitational/teleport/tool/tsh/main.go:26 main.main
	// 		runtime/proc.go:272 runtime.main
	// 		runtime/asm_arm64.s:1223 runtime.goexit
	// User Message: exec: &#34;psql&#34;: executable file not found in $PATH
	fmt.Printf("--> %v\n", err)

	return trace.Wrap(err)
}

const (
	// PeakStderrSize is the recommended size for capturing stderr that is used
	// for ConvertCommandError.
	PeakStderrSize = 100
)
