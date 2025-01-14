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
	"bufio"
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"slices"
	"strings"

	"github.com/gravitational/trace"
)

// man GROUPADD(8), exit codes section
const GroupExistExit = 9
const GroupInvalidArg = 3

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
	slog.DebugContext(context.Background(), "groupadd command completed",
		"command_path", cmd.Path,
		"output", string(output),
	)

	switch code := cmd.ProcessState.ExitCode(); code {
	case GroupExistExit:
		return code, trace.AlreadyExists("group already exists")
	case GroupInvalidArg:
		errMsg := "bad parameter"
		if strings.Contains(string(output), "not a valid group name") {
			errMsg = "invalid group name"
		}
		return code, trace.BadParameter(errMsg)
	default:
		return code, trace.Wrap(err)
	}
}

// UserOpts allow for customizing the resulting command for adding a new user.
type UserOpts struct {
	// UID a user should be created with. When empty, the UID is determined by the
	// useradd command.
	UID string
	// GID a user should be assigned to on creation. When empty, a group of the same name
	// as the user will be used.
	GID string
	// Home directory for a user. When empty, this will be the root directory to match
	// OpenSSH behavior.
	Home string
	// Shell that the user should use when logging in. When empty, the default shell
	// for the host is used (typically /usr/bin/sh).
	Shell string
}

// UserAdd creates a user on a host using `useradd`
func UserAdd(username string, groups []string, opts UserOpts) (exitCode int, err error) {
	useraddBin, err := exec.LookPath("useradd")
	if err != nil {
		return -1, trace.Wrap(err, "cant find useradd binary")
	}

	if opts.Home == "" {
		// Users without a home directory should land at the root, to match OpenSSH behavior.
		opts.Home = string(os.PathSeparator)
	}

	// user's without an explicit gid should be added to the group that shares their
	// login name if it's defined, otherwise user creation will fail because their primary
	// group already exists
	if slices.Contains(groups, username) && opts.GID == "" {
		opts.GID = username
	}

	// useradd ---no-create-home (username) (groups)...
	args := []string{"--no-create-home", "--home-dir", opts.Home, username}
	if len(groups) != 0 {
		args = append(args, "--groups", strings.Join(groups, ","))
	}

	if opts.UID != "" {
		args = append(args, "--uid", opts.UID)
	}

	if opts.GID != "" {
		args = append(args, "--gid", opts.GID)
	}

	if opts.Shell != "" {
		if shell, err := exec.LookPath(opts.Shell); err != nil {
			slog.WarnContext(context.Background(), "configured shell not found, falling back to host default", "shell", opts.Shell)
		} else {
			args = append(args, "--shell", shell)
		}
	}

	cmd := exec.Command(useraddBin, args...)
	output, err := cmd.CombinedOutput()
	slog.DebugContext(context.Background(), "useradd command completed",
		"command_path", cmd.Path,
		"output", string(output),
	)
	if cmd.ProcessState.ExitCode() == UserExistExit {
		return cmd.ProcessState.ExitCode(), trace.AlreadyExists("user already exists")
	}
	return cmd.ProcessState.ExitCode(), trace.Wrap(err)
}

// SetUserGroups adds a user to a list of specified groups on a host using `usermod`,
// overriding any existing supplementary groups.
func SetUserGroups(username string, groups []string) (exitCode int, err error) {
	usermodBin, err := exec.LookPath("usermod")
	if err != nil {
		return -1, trace.Wrap(err, "cant find usermod binary")
	}
	// usermod -G (replace groups) (username)
	cmd := exec.Command(usermodBin, "-G", strings.Join(groups, ","), username)
	output, err := cmd.CombinedOutput()
	slog.DebugContext(context.Background(), "usermod completed",
		"command_path", cmd.Path,
		"output", string(output),
	)
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
	slog.DebugContext(context.Background(), "userdel command completed",
		"command_path", cmd.Path,
		"output", string(output),
	)
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

// UserHasExpirations determines if the given username has an expired password, inactive password, or expired account
// by parsing the output of 'chage -l <username>'.
func UserHasExpirations(username string) (bool bool, exitCode int, err error) {
	chageBin, err := exec.LookPath("chage")
	if err != nil {
		return false, -1, trace.NotFound("cannot find chage binary: %s", err)
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd := exec.Command(chageBin, "-l", username)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return false, cmd.ProcessState.ExitCode(), trace.WrapWithMessage(err, "running chage: %s", stderr.String())
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// ignore empty lines
			continue
		}

		key, value, validLine := strings.Cut(line, ":")
		if !validLine {
			return false, -1, trace.Errorf("chage output invalid")
		}

		if strings.TrimSpace(value) == "never" {
			continue
		}

		switch strings.TrimSpace(key) {
		case "Password expires", "Password inactive", "Account expires":
			return true, 0, nil
		}
	}

	return false, cmd.ProcessState.ExitCode(), nil
}

// RemoveUserExpirations uses chage to remove any future or past expirations associated with the given username. It also uses usermod to remove any account locks that may have been placed.
func RemoveUserExpirations(username string) (exitCode int, err error) {
	chageBin, err := exec.LookPath("chage")
	if err != nil {
		return -1, trace.NotFound("cannot find chage binary: %s", err)
	}

	usermodBin, err := exec.LookPath("usermod")
	if err != nil {
		return -1, trace.NotFound("cannot find usermod binary: %s", err)
	}

	// remove all expirations from user
	// chage -E -1 -I -1 <username>
	cmd := exec.Command(chageBin, "-E", "-1", "-I", "-1", "-M", "-1", username)
	var errs []error
	if err := cmd.Run(); err != nil {
		errs = append(errs, trace.Wrap(err, "removing expirations with chage"))
	}

	// unlock user password if locked
	cmd = exec.Command(usermodBin, "-U", username)
	if err := cmd.Run(); err != nil {
		errs = append(errs, trace.Wrap(err, "removing lock with usermod"))
	}

	if len(errs) > 0 {
		return cmd.ProcessState.ExitCode(), trace.NewAggregate(errs...)
	}

	return cmd.ProcessState.ExitCode(), nil
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
