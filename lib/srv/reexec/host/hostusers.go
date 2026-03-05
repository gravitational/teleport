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
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
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
		return -1, fmt.Errorf("cant find groupadd binary: %w")
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
		return code, fmt.Errorf("group already exists")
	case GroupInvalidArg:
		errMsg := "bad parameter"
		if strings.Contains(string(output), "not a valid group name") {
			errMsg = "invalid group name"
		}
		return code, fmt.Errorf("%s", errMsg)
	default:
		return code, err
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
		return -1, fmt.Errorf("cant find useradd binary: %w")
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
		return cmd.ProcessState.ExitCode(), fmt.Errorf("user already exists")
	}
	return cmd.ProcessState.ExitCode(), err
}

// UserUpdate sets the groups and default shell for a host user using `usermod`,
// overriding any existing supplementary groups.
func UserUpdate(username string, groups []string, defaultShell string) (exitCode int, err error) {
	usermodBin, err := exec.LookPath("usermod")
	if err != nil {
		return -1, fmt.Errorf("cant find usermod binary: %w")
	}
	var args []string
	if groups != nil {
		args = append(args, "-G", strings.Join(groups, ","))
	}
	if defaultShell != "" {
		if shell, err := exec.LookPath(defaultShell); err != nil {
			slog.WarnContext(context.Background(), "configured shell not found, falling back to host default", "shell", defaultShell)
		} else {
			args = append(args, "--shell", shell)
		}
	}
	// usermod -G (replace groups) --shell (default shell) (username)
	cmd := exec.Command(usermodBin, append(args, username)...)
	output, err := cmd.CombinedOutput()
	slog.DebugContext(context.Background(), "usermod completed",
		"command_path", cmd.Path,
		"output", string(output),
	)
	return cmd.ProcessState.ExitCode(), err
}

// UserDel deletes a user on a host using `userdel`.
func UserDel(username string) (exitCode int, err error) {
	userdelBin, err := exec.LookPath("userdel")
	if err != nil {
		return -1, fmt.Errorf("cant find userdel binary: %w", err)
	}
	u, err := user.Lookup(username)
	if err != nil {
		return -1, err
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
	return cmd.ProcessState.ExitCode(), err
}

func GetAllUsers() ([]string, int, error) {
	getentBin, err := exec.LookPath("getent")
	if err != nil {
		return nil, -1, fmt.Errorf("cant find getent binary: %w", err)
	}
	// getent passwd
	cmd := exec.Command(getentBin, "passwd")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, -1, err
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
		return false, -1, fmt.Errorf("cannot find chage binary: %w", err)
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd := exec.Command(chageBin, "-l", username)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return false, cmd.ProcessState.ExitCode(), fmt.Errorf("running chage (%+q): %w", stderr.String(), err)
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
			return false, -1, fmt.Errorf("chage output invalid")
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
		return -1, fmt.Errorf("cannot find chage binary: %w", err)
	}

	usermodBin, err := exec.LookPath("usermod")
	if err != nil {
		return -1, fmt.Errorf("cannot find usermod binary: %w", err)
	}

	// remove all expirations from user
	// chage -E -1 -I -1 <username>
	cmd := exec.Command(chageBin, "-E", "-1", "-I", "-1", "-M", "-1", username)
	var errs []error
	if err := cmd.Run(); err != nil {
		errs = append(errs, fmt.Errorf("removing expirations with chage: %w", err))
	}

	// unlock user password if locked
	cmd = exec.Command(usermodBin, "-U", username)
	if err := cmd.Run(); err != nil {
		errs = append(errs, fmt.Errorf("removing lock with usermod, %w", err))
	}

	if len(errs) > 0 {
		return cmd.ProcessState.ExitCode(), errors.Join(errs...)
	}

	return cmd.ProcessState.ExitCode(), nil
}

var ErrInvalidSudoers = errors.New("visudo: invalid sudoers file")

// CheckSudoers tests a suders file using `visudo`. The contents
// are written to the process via stdin pipe.
func CheckSudoers(contents []byte) error {
	visudoBin, err := exec.LookPath("visudo")
	if err != nil {
		return fmt.Errorf("cant find visudo binary: %w")
	}
	cmd := exec.Command(visudoBin, "--check", "--file", "-")

	cmd.Stdin = bytes.NewBuffer(contents)
	output, err := cmd.Output()
	if cmd.ProcessState.ExitCode() != 0 {
		return fmt.Errorf("%w: %+q", ErrInvalidSudoers, output)
	}
	return err
}

// UserShell invokes the 'getent' binary in order to fetch the default shell for the
// given user.
func UserShell(username string) (string, error) {
	if username == "" {
		return "", fmt.Errorf("cannot lookup shell without username")
	}
	getentBin, err := exec.LookPath("getent")
	if err != nil {
		return "", fmt.Errorf("cannot find getent binary: %w", err)
	}
	cmd := exec.Command(getentBin, "passwd", username)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	// grab last element in passwd entry
	entry := bytes.TrimSpace(output)
	shellIdx := bytes.LastIndex(entry, []byte(":")) + 1
	if shellIdx >= len(entry) {
		return "", fmt.Errorf("invalid passwd entry for user %+q", username)
	}

	return string(entry), nil
}

// GetHostUserCredential parses the uid, gid, and groups of the given user intoAdd commentMore actions
// a credential object for a command to use.
func GetHostUserCredential(localUser *user.User) (*syscall.Credential, error) {
	uid, err := strconv.ParseUint(localUser.Uid, 10, 32)
	if err != nil {
		return nil, err
	}
	gid, err := strconv.ParseUint(localUser.Gid, 10, 32)
	if err != nil {
		return nil, err
	}

	if runtime.GOOS == "darwin" {
		// on macOS we should rely on the list of groups managed by the system
		// (the use of setgroups is "highly discouraged", as per the setgroups
		// man page in macOS 13.5)
		return &syscall.Credential{
			Uid:         uint32(uid),
			Gid:         uint32(gid),
			NoSetGroups: true,
		}, nil
	}

	// Lookup supplementary groups for the user.
	userGroups, err := localUser.GroupIds()
	if err != nil {
		return nil, err
	}
	groups := make([]uint32, 0)
	for _, sgid := range userGroups {
		igid, err := strconv.ParseUint(sgid, 10, 32)
		if err != nil {
			slog.WarnContext(context.Background(), "Cannot interpret user group", "user_group", sgid)
		} else {
			groups = append(groups, uint32(igid))
		}
	}
	if len(groups) == 0 {
		groups = append(groups, uint32(gid))
	}

	return &syscall.Credential{
		Uid:    uint32(uid),
		Gid:    uint32(gid),
		Groups: groups,
	}, nil
}

// MaybeSetCommandCredentialAsUser sets process credentials if the UID/GID of the
// requesting user are different from the process (Teleport).
func MaybeSetCommandCredentialAsUser(ctx context.Context, cmd *exec.Cmd, requestUser *user.User, logger *slog.Logger) error {
	credential, err := GetHostUserCredential(requestUser)
	if err != nil {
		return err
	}

	if os.Getuid() == int(credential.Uid) && os.Getgid() == int(credential.Gid) {
		logger.DebugContext(ctx, "Creating process with ambient credentials",
			"uid", credential.Uid,
			"gid", credential.Gid,
			"groups", credential.Groups,
		)
		return nil
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Credential = credential
	logger.DebugContext(ctx, "Creating process",
		"uid", credential.Uid,
		"gid", credential.Gid,
		"groups", credential.Groups,
	)
	return nil
}
