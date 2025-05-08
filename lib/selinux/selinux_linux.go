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
	"bytes"
	"context"
	_ "embed"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gravitational/trace"
	ocselinux "github.com/opencontainers/selinux/go-selinux"

	"github.com/gravitational/teleport/lib/versioncontrol"
)

const (
	selinuxConfig        = "/etc/selinux/config"
	selinuxTypeTag       = "SELINUXTYPE"
	moduleName           = "teleport_ssh"
	domain               = "teleport_ssh_t"
	permissiveModuleName = "permissive_" + domain
)

//go:embed teleport_ssh.te
var module string

//go:embed teleport_ssh.fc.tmpl
var fileContexts string

// ModuleSource returns the source of the SELinux SSH module.
func ModuleSource() string {
	return module
}

type filePaths struct {
	InstallDir     string
	DataDir        string
	ConfigPath     string
	UpgradeUnitDir string
}

// FileContexts returns file contexts for the SELinux SSH module.
func FileContexts(installDir, dataDir, configPath string) (string, error) {
	fcTempl, err := template.New("selinux file contexts").Parse(fileContexts)
	if err != nil {
		return "", trace.Wrap(err, "failed to parse file contexts template")
	}

	// Generate a file specifying the locations of important dirs so SELinux
	// will allow Teleport SSH to be able to access them.
	var buf bytes.Buffer
	err = fcTempl.Execute(&buf, filePaths{
		InstallDir:     installDir,
		DataDir:        dataDir,
		ConfigPath:     configPath,
		UpgradeUnitDir: versioncontrol.UnitConfigDir,
	})
	if err != nil {
		return "", trace.Wrap(err, "failed to expand file contexts template")
	}

	return buf.String(), nil
}

// CheckConfiguration returns an error if SELinux is not configured to
// enforce the SSH service correctly.
func CheckConfiguration(ensureEnforced bool, logger *slog.Logger) error {
	if !ocselinux.GetEnabled() {
		return trace.Errorf("SELinux is disabled or not present")
	}
	if ocselinux.EnforceMode() == ocselinux.Disabled {
		return trace.Errorf("SELinux mode is disabled, SELinux will not enforce or log anything")
	}
	if ocselinux.EnforceMode() == ocselinux.Permissive {
		if ensureEnforced {
			return trace.Errorf("SELinux mode is permissive, SELinux will not enforce rules only log denials")
		} else {
			slog.WarnContext(context.TODO(), "SELinux mode is permissive, SELinux will not enforce rules only log denials")
		}
	}

	selinuxType, err := readConfig(selinuxTypeTag)
	if err != nil {
		return trace.Wrap(err, "failed to find SELinux type")
	}
	selinuxDir := filepath.Join("/var/lib/selinux", selinuxType, "active/modules")

	var moduleInstalled, moduleDisabled, modulePermissive bool
	err = filepath.WalkDir(selinuxDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return trace.Wrap(err, "failed to access %q", path)
		}

		name := d.Name()
		if !strings.Contains(name, moduleName) {
			return nil
		}

		if d.IsDir() {
			moduleInstalled = true
			if name == permissiveModuleName {
				modulePermissive = true
				// if the module is permissive, we also know it's installed
				// so we can end the walk
				return filepath.SkipAll
			}
			// if the module is disabled, an empty file with the module's
			// name will exist in the "disabled" dir
		} else if filepath.Base(filepath.Dir(path)) == "disabled" {
			moduleDisabled = true
			// if the module is disabled, we also know it's installed
			// so we can end the walk
			return filepath.SkipAll
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
		if ensureEnforced {
			return trace.Errorf("the SSH SELinux module %s is permissive, denials will be logged but not enforced", moduleName)
		} else {
			slog.WarnContext(context.TODO(), "the SSH SELinux module is permissive, denials will be logged but not enforced", "module_name", moduleName)
		}
	}

	return nil
}

// UserContext returns the SELinux context that should be used when
// creating processes as a certain Linux user.
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
