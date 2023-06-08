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

package srv

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/utils/host"
)

// HostUsersProvisioningBackend is used to implement HostUsersBackend
type HostUsersProvisioningBackend struct {
	// SudoersPath is the path to write sudoers files to.
	SudoersPath string
	// HostUUID is the UUID of the running host
	HostUUID string
}

// newHostUsersBackend initializes a new OS specific HostUsersBackend
func newHostUsersBackend(uuid string) (HostUsersBackend, error) {
	return &HostUsersProvisioningBackend{
		SudoersPath: "/etc/sudoers.d",
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

// GetAllUsers returns a full list of users present on a system
func (*HostUsersProvisioningBackend) GetAllUsers() ([]string, error) {
	users, _, err := host.GetAllUsers()
	return users, err
}

// CreateGroup creates a group on a host
func (*HostUsersProvisioningBackend) CreateGroup(name string) error {
	_, err := host.GroupAdd(name)
	return trace.Wrap(err)
}

// CreateUser creates a user on a host
func (*HostUsersProvisioningBackend) CreateUser(name string, groups []string) error {
	_, err := host.UserAdd(name, groups)
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
func (*HostUsersProvisioningBackend) CheckSudoers(contents []byte) error {
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
func (u *HostUsersProvisioningBackend) WriteSudoersFile(username string, contents []byte) error {
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
func (u *HostUsersProvisioningBackend) RemoveSudoersFile(username string) error {
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
