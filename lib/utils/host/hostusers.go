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

package host

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// man GROUPADD(8), exit codes section
const GroupExistExit = 9

// man USERADD(8), exit codes section
const UserExistExit = 9
const UserLoggedInExit = 8

// GroupAdd creates a group on a host using `groupadd` optionally
// specifying the GID to create the group with.
func GroupAdd(groupname string, gid string) (exitCode int, err error) {
	groupaddBin, err := exec.LookPath("groupadd")
	if err != nil {
		return -1, trace.Wrap(err, "cant find groupadd binary")
	}
	var args []string
	if gid != "" {
		args = append(args, "--gid", gid)
	}
	args = append(args, groupname)

	cmd := exec.Command(groupaddBin, args...)
	output, err := cmd.CombinedOutput()
	log.Debugf("%s output: %s", cmd.Path, string(output))
	if cmd.ProcessState.ExitCode() == GroupExistExit {
		return cmd.ProcessState.ExitCode(), trace.AlreadyExists("group already exists")
	}
	return cmd.ProcessState.ExitCode(), trace.Wrap(err)
}

// UserAdd creates a user on a host using `useradd`
func UserAdd(username string, groups []string, home, uid, gid string) (exitCode int, err error) {
	useraddBin, err := exec.LookPath("useradd")
	if err != nil {
		return -1, trace.Wrap(err, "cant find useradd binary")
	}

	if home == "" {
		// Users without a home directory should land at the root, to match OpenSSH behavior.
		home = string(os.PathSeparator)
	}

	// useradd ---no-create-home (username) (groups)...
	args := []string{"--no-create-home", "--home-dir", home, username}
	if len(groups) != 0 {
		args = append(args, "--groups", strings.Join(groups, ","))
	}
	if uid != "" {
		args = append(args, "--uid", uid)
	}
	if gid != "" {
		args = append(args, "--gid", gid)
	}

	cmd := exec.Command(useraddBin, args...)
	output, err := cmd.CombinedOutput()
	log.Debugf("%s output: %s", cmd.Path, string(output))
	if cmd.ProcessState.ExitCode() == UserExistExit {
		return cmd.ProcessState.ExitCode(), trace.AlreadyExists("user already exists")
	}
	return cmd.ProcessState.ExitCode(), trace.Wrap(err)
}

// AddUserToGroups adds a user to a list of specified groups on a host using `usermod`
func AddUserToGroups(username string, groups []string) (exitCode int, err error) {
	usermodBin, err := exec.LookPath("usermod")
	if err != nil {
		return -1, trace.Wrap(err, "cant find usermod binary")
	}
	args := []string{"-aG"}
	args = append(args, groups...)
	args = append(args, username)
	// usermod -aG (append groups) (username)
	cmd := exec.Command(usermodBin, args...)
	output, err := cmd.CombinedOutput()
	log.Debugf("%s output: %s", cmd.Path, string(output))
	return cmd.ProcessState.ExitCode(), trace.Wrap(err)
}

// UserDel deletes a user on a host using `userdel`.
func UserDel(username string) (exitCode int, err error) {
	userdelBin, err := exec.LookPath("userdel")
	if err != nil {
		return -1, trace.NotFound("cant find userdel binary: %s", err)
	}
	u, err := user.Lookup(username)
	if err != nil {
		return -1, trace.Wrap(err)
	}
	args := make([]string, 0, 2)
	// Only remove the home dir if it exists and isn't the root.
	if u.HomeDir != "" && u.HomeDir != string(os.PathSeparator) {
		args = append(args, "--remove")
	}
	args = append(args, username)
	// userdel --remove (remove home) username
	cmd := exec.Command(userdelBin, args...)
	output, err := cmd.CombinedOutput()
	log.Debugf("%s output: %s", cmd.Path, string(output))
	return cmd.ProcessState.ExitCode(), trace.Wrap(err)
}

func GetAllUsers() ([]string, int, error) {
	getentBin, err := exec.LookPath("getent")
	if err != nil {
		return nil, -1, trace.NotFound("cant find getent binary: %s", err)
	}
	// getent passwd
	cmd := exec.Command(getentBin, "passwd")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, -1, trace.Wrap(err)
	}
	var users []string
	for _, line := range bytes.Split(output, []byte("\n")) {
		line := string(line)
		passwdEnt := strings.SplitN(line, ":", 2)
		if passwdEnt[0] != "" {
			users = append(users, passwdEnt[0])
		}
	}
	return users, -1, nil
}

var ErrInvalidSudoers = errors.New("visudo: invalid sudoers file")

// CheckSudoers tests a suders file using `visudo`. The contents
// are written to the process via stdin pipe.
func CheckSudoers(contents []byte) error {
	visudoBin, err := exec.LookPath("visudo")
	if err != nil {
		return trace.Wrap(err, "cant find visudo binary")
	}
	cmd := exec.Command(visudoBin, "--check", "--file", "-")

	cmd.Stdin = bytes.NewBuffer(contents)
	output, err := cmd.Output()
	if cmd.ProcessState.ExitCode() != 0 {
		return trace.WrapWithMessage(ErrInvalidSudoers, string(output))
	}
	return trace.Wrap(err)
}
