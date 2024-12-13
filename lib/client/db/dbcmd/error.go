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
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
)

//var install = map[string]map[string]string{}
//
//func installCommand(tool string) (string, string, error) {
//	switch {
//	case runtime.GOOS == constants.DarwinOS && hasCommand("brew"):
//		return installDarwinBrew(tool)
//	case runtime.GOOS == constants.LinuxOS && hasCommand("apt"):
//		return installLinuxDebian(tool)
//	case runtime.GOOS == constants.LinuxOS && (hasCommand("yum") || hasCommand("dnf")):
//		return installLinuxRHEL(tool)
//	default:
//		return "", "", trace.BadParameter("unknown OS/tool: %v/%v", runtime.GOOS, tool)
//	}
//}
//
//
//type details struct {
//	url string
//	cmd string
//}
//
//func installDarwinBrew(tool string) (string, string, error) {
//	p, ok := brewmap[tool]
//	if !ok {
//		return "", "", trace.BadParameter("no package for %v available", tool)
//	}
//	return p[0], p[1]
//}
//
//func installLinuxDebian(tool string) (string, string, error) {
//	return "", "", trace.BadParameter("no package for %v available", tool)
//}
//
//func installLinuxRHEL(tool string) (string, string, error) {
//	return "", "", trace.BadParameter("no package for %v available", tool)
//}
//
//func hasCommand(command string) bool {
//	_, err := exec.LookPath(command)
//	return err == nil
//}

// ConvertCommandError translates some common errors to more user friendly
// messages.
//
// This helps in situations where the user does not have the full context to
// decipher errors when the database command is executed internally (e.g.
// command executed through "tsh db connect").
func ConvertCommandError(cb *CLICommandBuilder, cmd *exec.Cmd, err error, peakStderr string) error {
	var ee *exec.Error
	switch {
	case errors.As(err, &ee):
		// TODO(russjones): Can this never return an error? Can we be in a situation
		// where no install details are found?
		url, command := cb.getInstallDetails()
		var b strings.Builder
		b.WriteString(fmt.Sprintf("In order to connect to this database, tsh requires that the %q tool is installed.\n", cmd.Args[0]))
		b.WriteString(fmt.Sprintf("Install it from %v or via your system's package manager", url))
		if command != "" {
			b.WriteString(fmt.Sprintf(" (%v).\n", command))
		}
		fmt.Print(b.String())

		return trace.Wrap(err)

		// TODO(russjones): Capture all dbcommands: https://github.com/gravitational/teleport/blob/master/lib/client/db/dbcmd/dbcmd.go

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
		//fmt.Printf("--> %v\n", err)
	}

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
