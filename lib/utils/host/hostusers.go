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

package host

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// man GROUPADD(8), exit codes section
const GroupExistExit = 9

// man USERADD(8), exit codes section
const UserExistExit = 9
const UserLoggedInExit = 8

// GroupAdd creates a group on a host using `groupadd`
func GroupAdd(groupname string) (exitCode int, err error) {
	groupaddBin, err := exec.LookPath("groupadd")
	if err != nil {
		return -1, trace.Wrap(err, "cant find groupadd binary")
	}
	cmd := exec.Command(groupaddBin, groupname)
	output, err := cmd.Output()
	log.Debugf("%s output: %s", cmd.Path, string(output))
	if cmd.ProcessState.ExitCode() == GroupExistExit {
		return cmd.ProcessState.ExitCode(), trace.AlreadyExists("group already exists")
	}
	return cmd.ProcessState.ExitCode(), trace.Wrap(err)
}

// UserAdd creates a user on a host using `useradd`
func UserAdd(username string, groups []string) (exitCode int, err error) {
	useraddBin, err := exec.LookPath("useradd")
	if err != nil {
		return -1, trace.Wrap(err, "cant find useradd binary")
	}
	// useradd --create-home (username) (groups)...
	args := []string{"--create-home", username}
	if len(groups) != 0 {
		args = append(args, "--groups", strings.Join(groups, ","))
	}
	cmd := exec.Command(useraddBin, args...)
	output, err := cmd.Output()
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
	output, err := cmd.Output()
	log.Debugf("%s output: %s", cmd.Path, string(output))
	return cmd.ProcessState.ExitCode(), trace.Wrap(err)
}

// UserDel deletes a user on a host using `userdel`.
func UserDel(username string) (exitCode int, err error) {
	userdelBin, err := exec.LookPath("userdel")
	if err != nil {
		return -1, trace.Wrap(err, "cant find userdel binary")
	}
	// userdel --remove (remove home) username
	cmd := exec.Command(userdelBin, "--remove", username)
	output, err := cmd.Output()
	log.Debugf("%s output: %s", cmd.Path, string(output))
	return cmd.ProcessState.ExitCode(), trace.Wrap(err)
}

func GetAllUsers() ([]string, int, error) {
	getentBin, err := exec.LookPath("getent")
	if err != nil {
		return nil, -1, trace.Wrap(err, "cant find getent binary")
	}
	// getent passwd
	cmd := exec.Command(getentBin, "passwd")
	output, err := cmd.Output()
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
