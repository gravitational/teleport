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
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/versioncontrol"
)

const (
	selinuxConfig        = "/etc/selinux/config"
	selinuxRoot          = "/var/lib/selinux"
	selinuxTypeTag       = "SELINUXTYPE"
	moduleName           = "teleport_ssh"
	execType             = "teleport_ssh_exec_t"
	domain               = "teleport_ssh_t"
	permissiveModuleName = "permissive_" + domain

	// the SELinux user and role that Systemd services run as by default
	selinuxSystemUser = "system_u"
	selinuxSystemRole = "system_r"
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

type selinuxContext struct {
	user               string
	role               string
	domain             string
	multiLevelSecurity string
}

// parseLabel parses an SELinux context string.
func parseLabel(label string) (*selinuxContext, error) {
	seCtx, err := ocselinux.NewContext(label)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse SELinux context")
	}

	return &selinuxContext{
		user:               seCtx["user"],
		role:               seCtx["role"],
		domain:             seCtx["type"],
		multiLevelSecurity: seCtx["level"],
	}, nil
}

func (c *selinuxContext) String() string {
	if c.multiLevelSecurity == "" {
		return c.user + ":" + c.role + ":" + c.domain
	}
	return c.user + ":" + c.role + ":" + c.domain + ":" + c.multiLevelSecurity
}

// CheckConfiguration returns an error if SELinux is not configured to
// enforce the SSH service correctly.
func CheckConfiguration(ensureEnforced bool, logger *slog.Logger) error {
	// ensure SELinux is enabled and running with the correct mode.
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

	// ensure we are running under the correct domain.
	label, err := ocselinux.CurrentLabel()
	if err != nil {
		return trace.Wrap(err, "failed to get SELinux context")
	}
	seCtx, err := parseLabel(label)
	if err != nil {
		return trace.Wrap(err)
	}
	if seCtx.domain != domain {
		return trace.Wrap(diagnoseWrongDomain(seCtx, logger))
	}

	// ensure the SELinux module is installed and enabled.
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

// this function attempts to diagnose why Teleport is not running under the correct SELinux domain.
func diagnoseWrongDomain(procCtx *selinuxContext, logger *slog.Logger) error {
	if procCtx.user != selinuxSystemUser || procCtx.role != selinuxSystemRole {
		logger.WarnContext(
			context.Background(),
			"Teleport is not running as the system_u:system_r SELinux user and role, running Teleport as a Systemd service is recommended when --enable-selinux is passed",
			"selinux_context", logutils.StringerAttr(procCtx),
		)
	}
	const fallbackErrMsg = "" +
		"Teleport is running under the wrong SELinux domain %q instead of %q, SELinux will not enforce Teleport correctly. " +
		"Refer to https://goteleport.com/docs/admin-guides/management/security/selinux/ for more information."
	fallbackErr := trace.Errorf(fallbackErrMsg, procCtx.domain, domain)

	execPath, err := os.Executable()
	if err != nil {
		return trace.NewAggregate(trace.Wrap(err, "failed to get executable path"), fallbackErr)
	}
	label, err := ocselinux.FileLabel(execPath)
	if err != nil {
		return trace.NewAggregate(trace.Wrap(err, "failed to get SELinux label of executable"), fallbackErr)
	}
	fileCtx, err := parseLabel(label)
	if err != nil {
		return trace.NewAggregate(trace.Wrap(err, "failed to parse SELinux context"), fallbackErr)
	}
	if fileCtx.domain != domain {
		const binaryLabelErrMsg = "" +
			"Teleport binary %q is labeled with SELinux type %q, it needs to be labeled with type %q to be enforced by SELinux correctly. " +
			"Refer to https://goteleport.com/docs/admin-guides/management/security/selinux/ for how to label the binary correctly."
		return trace.Errorf(binaryLabelErrMsg, execPath, fileCtx.domain, execType)
	}

	return fallbackErr
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
