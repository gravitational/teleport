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

package srv

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/host"
)

// HostUsersProvisioningBackend is used to implement HostUsersBackend
type HostUsersProvisioningBackend struct {
}

// HostSudoersProvisioningBackend is used to implement HostSudoersBackend
type HostSudoersProvisioningBackend struct {
	// HostUUID is the UUID of the running host
	HostUUID string
	// SudoersPath is the path to write sudoers files to.
	SudoersPath string
}

// newHostUsersBackend initializes a new OS specific HostUsersBackend
func newHostUsersBackend() (HostUsersBackend, error) {
	var missing []string
	for _, requiredBin := range []string{"usermod", "useradd", "getent", "groupadd", "visudo"} {
		if _, err := exec.LookPath(requiredBin); err != nil {
			missing = append(missing, requiredBin)
		}
	}
	if len(missing) != 0 {
		return nil, trace.NotFound("missing required binaries: %s", strings.Join(missing, ","))
	}

	return &HostUsersProvisioningBackend{}, nil
}

// newHostUsersBackend initializes a new OS specific HostUsersBackend
func newHostSudoersBackend(uuid string) (HostSudoersBackend, error) {
	return &HostSudoersProvisioningBackend{
		SudoersPath: "/etc/sudoers.d/",
		HostUUID:    uuid,
	}, nil
}

// Lookup implements host user information lookup
func (*HostUsersProvisioningBackend) Lookup(username string) (*user.User, error) {
	return user.Lookup(username)
}

// UserGIDs returns the list of group IDs for a user
func (*HostUsersProvisioningBackend) UserGIDs(u *user.User) ([]string, error) {
	return u.GroupIds()
}

// LookupGroup host group information lookup
func (*HostUsersProvisioningBackend) LookupGroup(name string) (*user.Group, error) {
	return user.LookupGroup(name)
}

// LookupGroup host group information lookup by GID
func (*HostUsersProvisioningBackend) LookupGroupByID(gid string) (*user.Group, error) {
	return user.LookupGroupId(gid)
}

// GetAllUsers returns a full list of users present on a system
func (*HostUsersProvisioningBackend) GetAllUsers() ([]string, error) {
	users, _, err := host.GetAllUsers()
	return users, err
}

// CreateGroup creates a group on a host
func (*HostUsersProvisioningBackend) CreateGroup(name string, gid string) error {
	_, err := host.GroupAdd(name, gid)
	return trace.Wrap(err)
}

// CreateUser creates a user on a host
func (*HostUsersProvisioningBackend) CreateUser(name string, groups []string, home, uid, gid string) error {
	_, err := host.UserAdd(name, groups, home, uid, gid)
	return trace.Wrap(err)
}

// DeleteUser deletes a user on a host.
// The user must not be logged in.
func (*HostUsersProvisioningBackend) DeleteUser(name string) error {
	code, err := host.UserDel(name)
	if code == host.UserLoggedInExit {
		return trace.Wrap(ErrUserLoggedIn)
	}
	return trace.Wrap(err)
}

// CheckSudoers ensures that a sudoers file to be written is valid
func (*HostSudoersProvisioningBackend) CheckSudoers(contents []byte) error {
	err := host.CheckSudoers(contents)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func writeSudoersFile(root, name string, data []byte) (string, error) {
	// as per sudoers(8), the includedir directive will ignore
	// (temporary) files with a "." in their name
	f, err := os.CreateTemp(root, fmt.Sprintf("tmp.%s.%s", "teleport", name))
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer f.Close()

	if _, err = f.Write(data); err != nil {
		return f.Name(), trace.Wrap(err)
	}
	if err = f.Chmod(0640); err != nil {
		return f.Name(), trace.Wrap(err)
	}
	return f.Name(), nil
}

// WriteSudoersFile creates the user's sudoers file.
func (u *HostSudoersProvisioningBackend) WriteSudoersFile(username string, contents []byte) error {
	if err := u.CheckSudoers(contents); err != nil {
		return trace.Wrap(err)
	}
	fileUsername := sanitizeSudoersName(username)
	sudoersFilePath := filepath.Join(u.SudoersPath, fmt.Sprintf("teleport-%s-%s", u.HostUUID, fileUsername))
	tmpSudoers, err := writeSudoersFile(u.SudoersPath, username, contents)
	if err != nil {
		if tmpSudoers != "" {
			rmErr := os.Remove(tmpSudoers)
			return trace.NewAggregate(rmErr, err)
		}
		return trace.Wrap(err)
	}

	err = os.Rename(tmpSudoers, sudoersFilePath)
	return trace.Wrap(err)
}

// RemoveSudoersFile deletes a user's sudoers file.
func (u *HostSudoersProvisioningBackend) RemoveSudoersFile(username string) error {
	fileUsername := sanitizeSudoersName(username)
	sudoersFilePath := filepath.Join(u.SudoersPath, fmt.Sprintf("teleport-%s-%s", u.HostUUID, fileUsername))
	if _, err := os.Stat(sudoersFilePath); os.IsNotExist(err) {
		log.Debugf("User %q, did not have sudoers file as it did not exist at path %q",
			username,
			sudoersFilePath)
		return nil
	}
	return trace.Wrap(os.Remove(sudoersFilePath))
}

// readDefaultKey reads /etc/default/useradd and returns the key if
// its found, if its not found it'll return the provided defaultValue
func readDefaultKey(key string, defaultValue string) (string, error) {
	b, err := os.Open("/etc/default/useradd")
	if err != nil {
		if os.IsNotExist(err) {
			return defaultValue, nil
		}
		return "", err
	}

	scanner := bufio.NewScanner(b)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, key) {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			return defaultValue, nil
		}
		return strings.TrimSpace(kv[1]), nil
	}
	return defaultValue, nil
}

// readDefaultHome reads /etc/default/useradd for the HOME key,
// defaulting to "/home" and join it with the user for the user
// home directory
func readDefaultHome(user string) (string, error) {
	const defaultHome = "/home"
	home, err := readDefaultKey("HOME", defaultHome)
	return filepath.Join(home, user), trace.Wrap(err)
}

// readDefaultHome reads /etc/default/useradd for the SKEL key, defaulting to "/etc/skel"
func readDefaultSkel() (string, error) {
	const defaultSkel = "/etc/skel"
	skel, err := readDefaultKey("SKEL", defaultSkel)
	return skel, trace.Wrap(err)
}

func (u *HostUsersProvisioningBackend) CreateHomeDirectory(userHome, uidS, gidS string) error {
	uid, err := strconv.Atoi(uidS)
	if err != nil {
		return trace.Wrap(err)
	}
	gid, err := strconv.Atoi(gidS)
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.Mkdir(userHome, 0o700)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return trace.Wrap(err)
	}

	skelDir, err := readDefaultSkel()
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = os.Stat(skelDir)
	if err != nil && !os.IsNotExist(err) {
		return trace.Wrap(err)
	}

	if !os.IsNotExist(err) {
		if err := utils.RecursiveCopy(skelDir, userHome, func(src, dest string) (bool, error) {
			destInfo, err := os.Lstat(dest)
			if err != nil {
				if os.IsNotExist(err) {
					return false, nil
				}
				return true, trace.ConvertSystemError(err)
			}
			return destInfo.Mode().Type()&os.ModeSymlink != 0, nil
		}); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := utils.RecursiveChown(userHome, uid, gid); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
