/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package selinux

import (
	"bufio"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
	ocselinux "github.com/opencontainers/selinux/go-selinux"
)

const (
	selinuxConfig        = "/etc/selinux/config"
	selinuxTypeTag       = "SELINUXTYPE"
	permissiveModuleName = "permissive_" + domain
)

// copied from github.com/opencontainers/selinux/go-selinux/selinux-linux.go
func readConfig(target string) string {
	in, err := os.Open(selinuxConfig)
	if err != nil {
		return ""
	}
	defer in.Close()

	scanner := bufio.NewScanner(in)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			// Skip blank lines
			continue
		}
		if line[0] == ';' || line[0] == '#' {
			// Skip comments
			continue
		}
		fields := bytes.SplitN(line, []byte{'='}, 2)
		if len(fields) != 2 {
			continue
		}
		if bytes.Equal(fields[0], []byte(target)) {
			return string(bytes.Trim(fields[1], `"`))
		}
	}
	return ""
}

// CheckConfiguration returns an error if SELinux is not configured to
// enforce the SSH service correctly.
func CheckConfiguration() error {
	if !ocselinux.GetEnabled() {
		return trace.Errorf("SELinux is disabled or not present")
	}
	if ocselinux.EnforceMode() != ocselinux.Enforcing {
		return trace.Errorf("SELinux mode is not enforcing, SELinux will not constrain anything")
	}

	selinuxType := readConfig(selinuxTypeTag)
	if selinuxType == "" {
		return trace.NotFound("could not find SELinux type")
	}
	selinuxDir := filepath.Join("/var/lib/selinux", selinuxType, "active/modules")

	var moduleInstalled, moduleDisabled, modulePermissive bool
	err := filepath.WalkDir(selinuxDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		name := d.Name()
		if strings.Contains(name, moduleName) {
			moduleInstalled = true
			if name == permissiveModuleName {
				modulePermissive = true
				// if the module is permissive, we also know it's installed
				// so we can end the walk
				return filepath.SkipAll
			}
			if filepath.Base(filepath.Dir(path)) == "disabled" {
				moduleDisabled = true
				// if the module is disabled, we also know it's installed
				// so we can end the walk
				return filepath.SkipAll
			}
		}

		return nil
	})
	if err != nil {
		return trace.Wrap(err, "failed to find SSH SELinux module")
	}

	if !moduleInstalled {
		return trace.Errorf("the SSH SELinux module %s is not installed", moduleName)
	}
	if moduleDisabled {
		return trace.Errorf("the SSH SELinux module %s is disabled", moduleName)
	}
	if modulePermissive {
		return trace.Errorf("the SSH SELinux module %s is permissive, so policy denials will be logged but not enforced", moduleName)
	}

	return nil
}

// UserContext returns the SELinux context that should be used when
// creating processes as a certain user.
func UserContext(login string) (string, error) {
	seUser, level, err := ocselinux.GetSeUserByName(login)
	if err != nil {
		return "", trace.Wrap(err)
	}
	curLabel, err := ocselinux.CurrentLabel()
	if err != nil {
		return "", trace.Wrap(err)
	}
	seContext, err := ocselinux.GetDefaultContextWithLevel(seUser, level, curLabel)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return seContext, nil
}
