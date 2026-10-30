// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	template "github.com/DataDog/datadog-agent/pkg/template/text"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/autoupdate"
	"github.com/gravitational/teleport/lib/config/systemd"
	"github.com/gravitational/teleport/lib/defaults"
	tbotsystemd "github.com/gravitational/teleport/lib/tbot/config/systemd"
	libutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/versioncontrol"
	"github.com/gravitational/teleport/session/selinux"
)

const (
	defaultInstallDir = "/opt/teleport"
	defaultPathDir    = "/usr/local/bin"
)

// Setup installs service and timer files for the teleport-update binary.
// Afterwords, Setup reloads systemd and enables the timer with --now.
func (ns *Namespace) Setup(ctx context.Context, path string, rev Revision, f SetupFeatures) error {
	if f.SELinuxSSH {
		if err := ns.installSELinux(ctx, rev); err != nil {
			ns.log.WarnContext(ctx, "Failed to install SELinux module.", errorKey, err)
		}
	} else {
		if err := ns.removeSELinux(ctx); err != nil {
			ns.log.WarnContext(ctx, "Failed to remove SELinux module.", errorKey, err)
		}
	}

	if ok, err := hasServiceManager(); err == nil && !ok {
		ns.log.WarnContext(ctx, "Systemd is not running, skipping updater installation.")
		return nil
	}

	err := ns.writeConfigFiles(ctx, path, rev, f)
	if err != nil {
		return trace.Wrap(err, "failed to write teleport-update systemd config files")
	}
	timer := &SystemdService{
		ServiceName: filepath.Base(ns.updaterTimerFile),
		Log:         ns.log,
	}
	if err := timer.Sync(ctx); err != nil {
		return trace.Wrap(err, "failed to sync systemd config")
	}
	if err := timer.Enable(ctx, true); err != nil {
		return trace.Wrap(err, "failed to enable teleport-update systemd timer")
	}
	if ns.name == "" {
		oldTimer := &SystemdService{
			ServiceName: deprecatedTimerName,
			Log:         ns.log,
		}
		// If the old teleport-upgrade script is detected, disable it to ensure they do not interfere.
		// Note that the schedule is also set to nop by the Teleport agent -- this just prevents restarts.
		present, err := oldTimer.IsPresent(ctx)
		if errors.Is(err, ErrNotAvailable) { // systemd too old
			if err := oldTimer.Disable(ctx, true); err != nil {
				ns.log.DebugContext(ctx, "The deprecated teleport-ent-updater package is either missing, or could not be disabled.", errorKey, err)
			}
			return nil
		}
		if err != nil {
			return trace.Wrap(err, "failed to determine if deprecated teleport-upgrade systemd timer is present")
		}
		if present {
			if err := oldTimer.Disable(ctx, true); err != nil {
				ns.log.ErrorContext(ctx, "The deprecated teleport-ent-updater package is installed on this server and cannot be disabled due to an error.", errorKey, err)
				ns.log.ErrorContext(ctx, "You must remove the teleport-ent-updater package after verifying that teleport-update is working.", errorKey, err)
			} else {
				ns.log.WarnContext(ctx, "The deprecated teleport-ent-updater package is installed on this server.")
				ns.log.WarnContext(ctx, "The systemd timer included in this package has been disabled to prevent conflicts.", "timer", deprecatedTimerName)
				ns.log.WarnContext(ctx, "The systemd service included in this package will no longer perform updates.", "service", deprecatedServiceName)
				ns.log.WarnContext(ctx, "Please remove the teleport-ent-updater package after verifying that teleport-update is working.")
			}
		}
	}

	return nil
}

func (ns *Namespace) writeConfigFiles(ctx context.Context, path string, rev Revision, f SetupFeatures) error {
	teleportService := filepath.Base(ns.teleportServiceFile)
	tbotService := filepath.Base(ns.tbotServiceFile)
	params := confParams{
		TeleportService:   teleportService,
		UpdaterBinary:     filepath.Join(path, BinaryName),
		InstallSuffix:     ns.name,
		InstallDir:        ns.installDir,
		Path:              path,
		UpdaterConfigFile: filepath.Join(ns.Dir(), UpdateConfigName),
	}

	for _, v := range []struct {
		path, tmpl string
	}{
		{ns.updaterServiceFile, updateServiceTemplate},
		{ns.updaterTimerFile, updateTimerTemplate},
		{ns.teleportDropInFile, teleportDropInTemplate},
		{ns.deprecatedDropInFile, deprecatedDropInTemplate},
	} {
		if v.path == "" {
			continue
		}
		err := writeSystemTemplate(v.path, genHeader(rev), v.tmpl, params)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Needrestart config is non-critical for updater functionality.

	nrServices := []string{teleportService}
	if f.Tbot {
		nrServices = append(nrServices, tbotService)
	}

	_, err := os.Stat(filepath.Dir(ns.needrestartConfigFile))
	if os.IsNotExist(err) {
		return nil // needrestart is not present
	}
	if err != nil {
		ns.log.ErrorContext(ctx, "Unable to disable needrestart.", errorKey, err)
		return nil
	}
	ns.log.InfoContext(ctx, "Disabling needrestart.")
	err = writeSystemTemplate(ns.needrestartConfigFile, "", needrestartConfTemplate, confParams{
		Services: nrServices,
	})
	if err != nil {
		ns.log.ErrorContext(ctx, "Unable to disable needrestart.", errorKey, err)
		return nil
	}
	return nil
}

// writeSystemTemplate atomically writes a template to a system file, creating any needed directories.
// Temporarily files are stored in the target path to ensure the file has needed SELinux contexts.
func writeSystemTemplate(path, header, t string, values any) error {
	dir, file := filepath.Split(path)
	if err := os.MkdirAll(dir, systemDirMode); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(writeAtomicWithinDir(path, configFileMode, func(w io.Writer) error {
		if header != "" {
			_, err := fmt.Fprint(w, header)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		tmpl, err := template.New(file).Funcs(template.FuncMap{
			"replace": func(s, old, new string) string {
				return strings.ReplaceAll(s, old, new)
			},
			// escape is a best-effort function for escaping quotes in systemd service templates.
			// Paths that are escaped with this method should not be advertised to the user as
			// configurable until a more robust escaping mechanism is shipped.
			// See: https://www.freedesktop.org/software/systemd/man/latest/systemd.syntax.html
			"escape": func(s string) string {
				replacer := strings.NewReplacer(
					`"`, `\"`,
					`\`, `\\`,
				)
				return replacer.Replace(s)
			},
		}).Parse(t)
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(tmpl.Execute(w, values))
	}))
}

func (ns *Namespace) installSELinux(ctx context.Context, rev Revision) error {
	ns.log.InfoContext(ctx, "Installing SELinux module.")

	if err := ns.checkSELinux(ctx, true); err != nil {
		return trace.Wrap(err)
	}

	binaryPath := filepath.Join(ns.Dir(), versionsDirName, rev.Dir(), "bin", "teleport")
	//nolint:staticcheck // SA4023. FileContexts on non-Linux platforms always returns err.
	fileCtxs, err := selinux.FileContexts(ns.dataDir, ns.teleportConfigFile, binaryPath)
	//nolint:staticcheck // SA4023. The check appears always-true because FileContexts on non-Linux platforms always returns err.
	if err != nil {
		return trace.Wrap(err)
	}

	tempDir, err := os.MkdirTemp("", "selinux-*")
	if err != nil {
		return trace.Wrap(err, "failed to create temporary directory")
	}
	defer os.RemoveAll(tempDir)

	err = os.WriteFile(filepath.Join(tempDir, "teleport_ssh.fc"), []byte(fileCtxs), 0440)
	if err != nil {
		return trace.Wrap(err, "failed to write module source file")
	}

	err = os.WriteFile(filepath.Join(tempDir, "teleport_ssh.te"), []byte(selinux.ModuleSource()), 0440)
	if err != nil {
		return trace.Wrap(err, "failed to write module source file")
	}

	cmd := localExec{
		Dir:      tempDir,
		Log:      ns.log,
		ErrLevel: slog.LevelDebug,
		OutLevel: slog.LevelDebug,
	}

	// Build and install the module, then ensure files are properly labeled.
	_, err = cmd.Run(ctx, "make", "-f", selinuxMakefile, "teleport_ssh.pp")
	if err != nil {
		return trace.Wrap(err, "failed to build module")
	}

	_, err = cmd.Run(ctx, "semodule", "-i", "teleport_ssh.pp")
	if err != nil {
		return trace.Wrap(err, "failed to install module")
	}

	if err := ns.createAndLabelDirs(ctx, cmd); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (ns *Namespace) createAndLabelDirs(ctx context.Context, cmd localExec) error {
	// Create the teleport-upgrade and data dirs ahead of time so we
	// can ensure they exist when we label them.
	if err := os.MkdirAll(versioncontrol.UnitConfigDir, defaults.DirectoryPermissions); err != nil {
		ns.log.WarnContext(ctx, "Failed to create teleport-upgrade directory.", errorKey, err)
	}
	if err := os.MkdirAll(ns.dataDir, teleport.PrivateDirMode); err != nil {
		ns.log.WarnContext(ctx, "Failed to create teleport data directory.", errorKey, err)
	}

	dirsToLabel := []string{
		filepath.Clean(ns.installDir),
		ns.dataDir,
		versioncontrol.UnitConfigDir,
	}
	// Create an empty teleport.yaml config file if it doesn't exist, and
	// only attempt to label it if it exists.
	if libutils.FileExists(ns.teleportConfigFile) {
		dirsToLabel = append(dirsToLabel, filepath.Dir(ns.teleportConfigFile))
	} else {
		confFile, err := os.OpenFile(ns.teleportConfigFile, os.O_RDONLY|os.O_CREATE|os.O_EXCL, 0o640)
		if err != nil {
			ns.log.WarnContext(ctx, "Failed to create teleport.yaml.", errorKey, err, "path", ns.teleportConfigFile)
			ns.log.WarnContext(ctx, "You will likely need to create teleport.yaml and re-run 'sudo teleport-update enable' for Teleport SSH to work correctly with SELinux.")
		} else {
			confFile.Close()
			dirsToLabel = append(dirsToLabel, filepath.Dir(ns.teleportConfigFile))
			ns.log.WarnContext(ctx, "Created an empty teleport.yaml.", "path", ns.teleportConfigFile)
			ns.log.WarnContext(ctx, "If you move or copy another file onto this file you will likely need to re-run 'sudo teleport-update enable' for Teleport SSH to work correctly with SELinux.")
		}
	}

	labelArgs := append([]string{"-rv"}, dirsToLabel...)
	_, err := cmd.Run(ctx, "restorecon", labelArgs...)
	if err != nil {
		return trace.Wrap(err, "failed to restore file contexts")
	}

	return nil
}

func (ns *Namespace) removeSELinux(ctx context.Context) error {
	//nolint:staticcheck // SA4023. ModuleInstalled on non-Linux platforms always returns err.
	installed, err := selinux.ModuleInstalled()
	//nolint:staticcheck // SA4023. The check appears always-true because ModuleInstalled on non-Linux platforms always returns err.
	if err != nil {
		return trace.Wrap(err, "failed to check if SELinux module is installed")
	}
	if !installed {
		ns.log.DebugContext(ctx, "SELinux module is not installed.")
		return nil
	}

	ns.log.InfoContext(ctx, "Removing SELinux module.")

	if err := ns.checkSELinux(ctx, false); err != nil {
		return trace.Wrap(err)
	}

	cmd := localExec{
		Log:      ns.log,
		ErrLevel: slog.LevelDebug,
		OutLevel: slog.LevelDebug,
	}

	removeOutput, err := cmd.Output(ctx, "semodule", "-r", "teleport_ssh")
	if err != nil {
		// If the module is not installed, return without an error.
		if bytes.Contains(removeOutput, []byte("(No such file or directory).")) {
			return nil
		}
		return trace.Wrap(err, "failed to remove module")
	}

	_, err = cmd.Run(ctx, "restorecon", "-rv", filepath.Clean(ns.installDir), ns.dataDir, filepath.Dir(ns.teleportConfigFile))
	if err != nil {
		return trace.Wrap(err, "failed to restore file contexts")
	}

	return nil
}

func (ns *Namespace) checkSELinux(ctx context.Context, installing bool) error {
	// Ensure necessary files and binaries exist for building a module and
	// inform the user what packages need to be installed if they can't be
	// found.
	if installing {
		_, err := exec.LookPath("make")
		if err != nil {
			ns.log.ErrorContext(ctx, "Failed to find 'make', you may need to install the 'make' package.")
			return trace.Wrap(err)
		}
		if !libutils.FileExists(selinuxMakefile) {
			ns.log.ErrorContext(ctx, "Failed to find the SELinux Makefile, you may need to install the 'selinux-policy-devel' package.")
			return trace.NotFound("failed to find %s", selinuxMakefile)
		}
	}
	_, err := exec.LookPath("semodule")
	if err != nil {
		ns.log.ErrorContext(ctx, "Failed to find 'semodule', you may need to install the 'policycoreutils' package.")
		return trace.Wrap(err)
	}
	_, err = exec.LookPath("restorecon")
	if err != nil {
		ns.log.ErrorContext(ctx, "Failed to find 'restorecon', you may need to install the 'policycoreutils' package.")
		return trace.Wrap(err)
	}

	return nil
}

// Teardown removes all traces of the auto-updater, including its configuration.
// Teardown does not verify that the removed files were created by teleport-update.
func (ns *Namespace) Teardown(ctx context.Context) error {
	//nolint:staticcheck // SA4023. ModuleInstalled on non-Linux platforms always returns err.
	modInstalled, err := selinux.ModuleInstalled()
	//nolint:staticcheck // SA4023. The check appears always-true because ModuleInstalled on non-Linux platforms always returns err.
	if err != nil {
		return trace.Wrap(err, "failed to check if SELinux module is installed")
	}
	if modInstalled {
		if err := ns.removeSELinux(ctx); err != nil {
			ns.log.WarnContext(ctx, "Failed to remove SELinux module.", errorKey, err)
		}
	}

	if ok, err := hasServiceManager(); err == nil && !ok {
		ns.log.WarnContext(ctx, "Systemd is not running, skipping updater removal.")
		if err := os.RemoveAll(ns.Dir()); err != nil {
			return trace.Wrap(err, "failed to remove versions directory")
		}
		return nil
	}

	svc := &SystemdService{
		ServiceName: filepath.Base(ns.updaterTimerFile),
		Log:         ns.log,
	}
	if err := svc.Disable(ctx, true); err != nil {
		ns.log.WarnContext(ctx, "Unable to disable teleport-update systemd timer before removing.")
		ns.log.DebugContext(ctx, "Error disabling teleport-update systemd timer.", errorKey, err)
	}
	for _, p := range []string{
		ns.updaterServiceFile,
		ns.updaterTimerFile,
		ns.teleportDropInFile,
		ns.deprecatedDropInFile,
		ns.needrestartConfigFile,
	} {
		if p == "" {
			continue
		}
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return trace.Wrap(err, "failed to remove %s", filepath.Base(p))
		}
	}
	if err := svc.Sync(ctx); err != nil {
		return trace.Wrap(err, "failed to sync systemd config")
	}
	if err := os.RemoveAll(ns.Dir()); err != nil {
		return trace.Wrap(err, "failed to remove versions directory")
	}
	if ns.name == "" {
		oldTimer := &SystemdService{
			ServiceName: deprecatedTimerName,
			Log:         ns.log,
		}
		// If the old upgrader exists, attempt to re-enable it automatically
		present, err := oldTimer.IsPresent(ctx)
		if errors.Is(err, ErrNotAvailable) { // systemd too old
			if err := oldTimer.Enable(ctx, true); err != nil {
				ns.log.DebugContext(ctx, "The deprecated teleport-ent-updater package is either missing, or could not be enabled.", errorKey, err)
			}
			return nil
		}
		if err != nil {
			return trace.Wrap(err, "failed to determine if deprecated teleport-upgrade systemd timer is present")
		}
		if present {
			if err := oldTimer.Enable(ctx, true); err != nil {
				ns.log.ErrorContext(ctx, "The deprecated teleport-ent-updater package is installed on this server, and it cannot be re-enabled due to an error.", errorKey, err)
				ns.log.ErrorContext(ctx, "Please fix the systemd timer included in the teleport-ent-updater package if you intend to use the deprecated updater.")
			} else {
				ns.log.WarnContext(ctx, "The deprecated teleport-ent-updater package is installed on this server.")
				ns.log.WarnContext(ctx, "The systemd timer included in this package has been re-enabled to ensure continued updates.", "timer", deprecatedTimerName)
				ns.log.WarnContext(ctx, "To disable updates entirely, please remove the teleport-ent-updater package.")
			}
		}
	}
	return nil
}

var alphanum = regexp.MustCompile("^[a-zA-Z0-9-]*$")

// NewNamespace validates and returns a Namespace.
// Namespaces must be alphanumeric + `-`.
// defaultPathDir overrides the destination directory for namespace setup (i.e., /usr/local)
func NewNamespace(ctx context.Context, log *slog.Logger, name, installDir string) (ns *Namespace, err error) {
	defer func() { ns.overrideFromConfig(ctx) }()

	if name == defaultNamespace ||
		name == systemNamespace {
		return nil, trace.Errorf("namespace %s is reserved", name)
	}
	if !alphanum.MatchString(name) {
		return nil, trace.Errorf("invalid namespace name %s, must be alphanumeric", name)
	}
	if installDir == "" {
		installDir = defaultInstallDir
	}
	if name == "" {
		linkDir := defaultPathDir
		return &Namespace{
			log:                   log,
			name:                  name,
			installDir:            installDir,
			defaultPathDir:        linkDir,
			dataDir:               defaults.DataDir,
			teleportServiceFile:   filepath.Join("/", serviceDir, teleportServiceName), // /lib for backwards-compat
			teleportConfigFile:    defaults.ConfigFilePath,
			teleportPIDFile:       filepath.Join(systemdPIDDir, "teleport.pid"),
			needrestartConfigFile: filepath.Join(needrestartConfDir, BinaryName+".conf"),
			teleportDropInFile:    filepath.Join(systemdAdminDir, "teleport.service.d", BinaryName+".conf"),
			updaterIDFile:         filepath.Join(os.TempDir(), BinaryName+".id"),
			updaterServiceFile:    filepath.Join(systemdAdminDir, BinaryName+".service"),
			updaterTimerFile:      filepath.Join(systemdAdminDir, BinaryName+".timer"),
			deprecatedDropInFile:  filepath.Join(systemdAdminDir, deprecatedServiceName+".d", BinaryName+".conf"),
			tbotServiceFile:       filepath.Join(systemdAdminDir, "tbot.service"),
			tbotConfigFile:        filepath.Join("/etc", "tbot.yaml"),
			tbotPIDFile:           filepath.Join(systemdPIDDir, "tbot.pid"),
		}, nil
	}

	prefix := "teleport_" + name
	linkDir := filepath.Join(installDir, name, "bin")
	return &Namespace{
		log:                   log,
		name:                  name,
		installDir:            installDir,
		defaultPathDir:        linkDir,
		dataDir:               filepath.Join(filepath.Dir(defaults.DataDir), prefix),
		teleportServiceFile:   filepath.Join(systemdAdminDir, prefix+".service"),
		teleportConfigFile:    filepath.Join(filepath.Dir(defaults.ConfigFilePath), prefix+".yaml"),
		teleportPIDFile:       filepath.Join(systemdPIDDir, prefix+".pid"),
		needrestartConfigFile: filepath.Join(needrestartConfDir, BinaryName+"_"+name+".conf"),
		teleportDropInFile:    filepath.Join(systemdAdminDir, prefix+".service.d", BinaryName+"_"+name+".conf"),
		updaterIDFile:         filepath.Join(os.TempDir(), BinaryName+"_"+name+".id"),
		updaterServiceFile:    filepath.Join(systemdAdminDir, BinaryName+"_"+name+".service"),
		updaterTimerFile:      filepath.Join(systemdAdminDir, BinaryName+"_"+name+".timer"),
		tbotServiceFile:       filepath.Join(systemdAdminDir, "tbot_"+name+".service"),
		tbotConfigFile:        filepath.Join("/etc", "tbot_"+name+".yaml"),
		tbotPIDFile:           filepath.Join(systemdPIDDir, "tbot_"+name+".pid"),
		// no deprecatedDropInFile, as teleport-upgrade does not conflict with namespaced installs
	}, nil
}

// WriteTeleportService writes the Teleport systemd service for the version of Teleport
// that matches the version of Teleport compiled into the executing code.
func (ns *Namespace) WriteTeleportService(_ context.Context, pathDir string, rev Revision) error {
	if pathDir == "" {
		pathDir = ns.defaultPathDir
	}
	return trace.Wrap(writeAtomicWithinDir(ns.teleportServiceFile, configFileMode, func(w io.Writer) error {
		_, err := fmt.Fprint(w, genHeader(rev)+"\n")
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(systemd.WriteUnitFile(systemd.Flags{
			EnvironmentFile:          systemd.DefaultEnvironmentFile,
			PIDFile:                  ns.teleportPIDFile,
			FileDescriptorLimit:      systemd.DefaultFileDescriptorLimit,
			TeleportInstallationFile: filepath.Join(pathDir, "teleport"),
			TeleportConfigPath:       ns.teleportConfigFile,
			FIPS:                     rev.Flags&autoupdate.FlagFIPS != 0,
		}, w))
	}))
}

func (ns *Namespace) HasCustomTbot(ctx context.Context) (bool, error) {
	diskMarkerPrefix, err := readFileLimit(ns.tbotServiceFile, int64(len(markerPrefix)))
	if errors.Is(err, os.ErrNotExist) {
		ns.log.DebugContext(ctx, "Service not present.", "path", ns.tbotServiceFile)
		return false, nil
	}
	if err != nil {
		return false, trace.Wrap(err)
	}
	return string(diskMarkerPrefix) != markerPrefix, nil
}

// WriteTbotService writes the tbot systemd service for the version of tbot
// that matches the version of tbot compiled into the executing code.
func (ns *Namespace) WriteTbotService(ctx context.Context, pathDir string, rev Revision) error {
	if pathDir == "" {
		pathDir = ns.defaultPathDir
	}
	// This configuration preserves the original tbot systemd.tmpl params.
	// In the future, the template could be updated to accept PIDFile directly.
	unitName := "tbot"
	if ns.name != "" {
		unitName = "tbot_" + ns.name
	}
	if ns.tbotPIDFile != filepath.Join("/run", unitName+".pid") {
		return trace.Errorf("invalid PID file: %q", ns.tbotPIDFile)
	}
	diagDir := filepath.Join(ns.dataDir, "bot")

	if err := os.MkdirAll(diagDir, teleport.PrivateDirMode); err != nil {
		ns.log.WarnContext(ctx, "Failed to create tbot data directory.", errorKey, err)
	}

	return trace.Wrap(writeAtomicWithinDir(ns.tbotServiceFile, configFileMode, func(w io.Writer) error {
		_, err := fmt.Fprint(w, genHeader(rev)+"\n")
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(tbotsystemd.Template.Execute(w, tbotsystemd.TemplateParams{
			UnitName:             unitName,
			User:                 "root",
			Group:                "root",
			AnonymousTelemetry:   true,
			ConfigPath:           ns.tbotConfigFile,
			TBotPath:             filepath.Join(pathDir, "tbot"),
			DiagSocketForUpdater: filepath.Join(diagDir, teleport.DebugServiceSocketName),
		}))
	}))
}
