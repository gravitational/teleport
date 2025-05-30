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
	"log/slog"
	"os"
	"path/filepath"
	"text/template"

	"github.com/gravitational/trace"
	ocselinux "github.com/opencontainers/selinux/go-selinux"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/versioncontrol"
)

const (
	selinuxConfig        = "/etc/selinux/config"
	selinuxRoot          = "/var/lib/selinux"
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

// FileContexts returns file contexts for the SELinux SSH module.
func FileContexts(dataDir, configPath string) (string, error) {
	fcTempl, err := template.New("selinux file contexts").Parse(fileContexts)
	if err != nil {
		return "", trace.Wrap(err, "failed to parse file contexts template")
	}

	execPath, err := os.Executable()
	if err != nil {
		return "", trace.Wrap(err, "failed to get the path of the executable")
	}
	binaryPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", trace.Wrap(err, "failed to expand symlinks for the executable")
	}

	// Generate a file specifying the locations of important dirs so SELinux
	// will allow Teleport SSH to be able to access them.
	var buf bytes.Buffer
	err = fcTempl.Execute(&buf, filePaths{
		BinaryPath:     binaryPath,
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
			slog.WarnContext(context.TODO(), "The SELinux mode is set to permissive SELinux will not enforce rules, only log denials")
		}
	}

	selinuxType, err := readConfig(selinuxTypeTag)
	if err != nil {
		return trace.Wrap(err, "failed to find SELinux type")
	}

	modulesDir := filepath.Join(selinuxRoot, selinuxType, "active/modules")
	installed, disabled, permissive, err := moduleStatus(modulesDir)
	if err != nil {
		return trace.Wrap(err)
	}

	if !installed {
		return trace.Errorf("the SSH SELinux module %s is not installed", moduleName)
	}
	if disabled {
		return trace.Errorf("the SSH SELinux module %s is disabled", moduleName)
	}
	if permissive {
		if ensureEnforced {
			return trace.Errorf("the SSH SELinux module %s is permissive, denials will be logged but not enforced", moduleName)
		} else {
			slog.WarnContext(context.TODO(), "the SSH SELinux module is permissive, denials will be logged but not enforced", "module_name", moduleName)
		}
	}

	return nil
}

func moduleStatus(modulesDir string) (installed bool, disabled bool, permissive bool, err error) {
	disabledModPath := filepath.Join(modulesDir, "disabled", moduleName)
	// SELinux modules can't be disabled and permissive, so we can
	// safely return here
	if utils.FileExists(disabledModPath) {
		disabled = true
		installed = true
		return
	}

	moduleDirs, err := os.ReadDir(modulesDir)
	if err != nil {
		return false, false, false, trace.Wrap(err, "failed to list modules directory")
	}

	for _, moduleDir := range moduleDirs {
		name := moduleDir.Name()
		if name == "disabled" {
			continue
		}
		path := filepath.Join(modulesDir, name)

		permModPath := filepath.Join(path, permissiveModuleName)
		if utils.FileExists(permModPath) {
			// if the module is permissive, we also know it's installed
			// so we can safely return
			permissive = true
			installed = true
			return
		}

		modPath := filepath.Join(path, moduleName)
		if utils.FileExists(modPath) {
			installed = true
		}
	}

	return
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
