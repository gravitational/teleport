/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	common "github.com/gravitational/teleport/lib/autoupdate"
	autoupdate "github.com/gravitational/teleport/lib/autoupdate/agent"
	"github.com/gravitational/teleport/lib/modules"
	libutils "github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const appHelp = `Teleport Updater

The Teleport Updater applies Managed Updates to a Teleport agent installation.

The Teleport Updater supports update scheduling and automated rollbacks.

Find out more at https://goteleport.com/docs/upgrading/agent-managed-updates`

const (
	// proxyServerEnvVar allows the proxy server address to be specified via env var.
	proxyServerEnvVar = "TELEPORT_PROXY"
	// updateGroupEnvVar allows the update group to be specified via env var.
	updateGroupEnvVar = "TELEPORT_UPDATE_GROUP"
	// updateVersionEnvVar forces the version to specified value.
	updateVersionEnvVar = "TELEPORT_UPDATE_VERSION"
	// updateLockTimeout is the duration commands will wait for update to complete before failing.
	updateLockTimeout = 10 * time.Minute
)

var plog = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentUpdater)

func main() {
	// Set the umask to match the systemd umask that the teleport-update service will execute with.
	// This ensures consistent file permissions.
	// NOTE: This must be run before any goroutines that create files are started.
	syscall.Umask(0o022)
	if code := Run(os.Args[1:]); code != 0 {
		os.Exit(code)
	}
}

type cliConfig struct {
	autoupdate.OverrideConfig
	// Debug logs enabled
	Debug bool
	// LogFormat controls the format of logging. Can be either `json` or `text`.
	// By default, this is `text`.
	LogFormat string
	// InstallDir for Teleport (usually /opt/teleport)
	InstallDir string
	// InstallSuffix is the isolated suffix for the installation.
	InstallSuffix string
	// SelfSetup mode for using the current version of the teleport-update to setup the update service.
	SelfSetup bool
	// UpdateNow forces an immediate update.
	UpdateNow bool
	// Reload reloads Teleport.
	Reload bool
	// ForceUninstall allows Teleport to be completely removed.
	ForceUninstall bool
	// Insecure skips TLS certificate verification.
	Insecure bool
}

func Run(args []string) int {
	var ccfg cliConfig

	ctx := context.Background()
	ctx, _ = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)

	app := libutils.InitCLIParser(autoupdate.BinaryName, appHelp).Interspersed(false)
	app.Flag("debug", "Verbose logging to stdout.").
		Short('d').BoolVar(&ccfg.Debug)
	app.Flag("log-format", "Controls the format of output logs. Can be `json` or `text`. Defaults to `text`.").
		Default(libutils.LogFormatText).EnumVar(&ccfg.LogFormat, libutils.LogFormatJSON, libutils.LogFormatText)
	app.Flag("install-suffix", "Suffix for creating an agent installation outside of the default $PATH. Note: this changes the default data directory.").
		Short('i').StringVar(&ccfg.InstallSuffix)
	app.Flag("install-dir", "Directory containing Teleport installations.").
		Hidden().StringVar(&ccfg.InstallDir)
	app.Flag("insecure", "Insecure mode disables certificate verification. Do not use in production.").
		BoolVar(&ccfg.Insecure)

	app.HelpFlag.Short('h')

	versionCmd := app.Command("version", fmt.Sprintf("Print the version of your %s binary.", autoupdate.BinaryName))

	enableCmd := app.Command("enable", "Enable agent managed updates and perform initial installation or update. This creates a systemd timer that periodically runs the update subcommand.")
	enableCmd.Flag("proxy", "Address of the Teleport Proxy.").
		Short('p').Envar(proxyServerEnvVar).StringVar(&ccfg.Proxy)
	enableCmd.Flag("group", "Update group for this agent installation.").
		Short('g').Envar(updateGroupEnvVar).StringVar(&ccfg.Group)
	enableCmd.Flag("base-url", "Base URL used to override the Teleport download URL.").
		Short('b').Envar(common.BaseURLEnvVar).StringVar(&ccfg.BaseURL)
	enableCmd.Flag("overwrite", "Allow existing installed Teleport binaries to be overwritten.").
		Short('o').BoolVar(&ccfg.AllowOverwrite)
	enableCmd.Flag("force-version", "Force the provided version instead of using the version provided by the Teleport cluster.").
		Hidden().Short('f').Envar(updateVersionEnvVar).StringVar(&ccfg.ForceVersion)
	enableCmd.Flag("self-setup", "Use the current teleport-update binary to create systemd service config for managed updates.").
		Hidden().BoolVar(&ccfg.SelfSetup)
	enableCmd.Flag("path", "Directory to link the active Teleport installation's binaries into.").
		Hidden().StringVar(&ccfg.Path)
	// TODO(sclevine): add force-fips and force-enterprise as hidden flags

	disableCmd := app.Command("disable", "Disable agent managed updates. Does not affect the active installation of Teleport.")

	pinCmd := app.Command("pin", "Install Teleport and lock the updater to the installed version.")
	pinCmd.Flag("proxy", "Address of the Teleport Proxy.").
		Short('p').Envar(proxyServerEnvVar).StringVar(&ccfg.Proxy)
	pinCmd.Flag("group", "Update group for this agent installation.").
		Short('g').Envar(updateGroupEnvVar).StringVar(&ccfg.Group)
	pinCmd.Flag("base-url", "Base URL used to override the Teleport download URL.").
		Short('b').Envar(common.BaseURLEnvVar).StringVar(&ccfg.BaseURL)
	pinCmd.Flag("overwrite", "Allow existing installed Teleport binaries to be overwritten.").
		Short('o').BoolVar(&ccfg.AllowOverwrite)
	pinCmd.Flag("force-version", "Force the provided version instead of using the version provided by the Teleport cluster.").
		Short('f').Envar(updateVersionEnvVar).StringVar(&ccfg.ForceVersion)
	pinCmd.Flag("self-setup", "Use the current teleport-update binary to create systemd service config for managed updates.").
		Hidden().BoolVar(&ccfg.SelfSetup)
	pinCmd.Flag("path", "Directory to link the active Teleport installation's binaries into.").
		Hidden().StringVar(&ccfg.Path)

	unpinCmd := app.Command("unpin", "Unpin the current version, allowing it to be updated.")

	updateCmd := app.Command("update", "Update the agent to the latest version, if a new version is available.")
	updateCmd.Flag("now", "Force immediate update even if update window is not active.").
		Short('n').BoolVar(&ccfg.UpdateNow)
	updateCmd.Flag("self-setup", "Use the current teleport-update binary to create systemd service config for managed updates and verify the Teleport installation.").
		Hidden().BoolVar(&ccfg.SelfSetup)

	linkCmd := app.Command("link-package", "Link the system installation of Teleport from the Teleport package, if managed updates is disabled.")
	unlinkCmd := app.Command("unlink-package", "Unlink the system installation of Teleport from the Teleport package.")

	setupCmd := app.Command("setup", "Write configuration files that run the update subcommand on a timer and verify the Teleport installation.").
		Hidden()
	setupCmd.Flag("reload", "Reload the Teleport agent. If not set, Teleport is not reloaded or restarted.").
		BoolVar(&ccfg.Reload)
	setupCmd.Flag("path", "Directory that the active Teleport installation's binaries are linked into.").
		Required().StringVar(&ccfg.Path)

	statusCmd := app.Command("status", "Show Teleport agent auto-update status.")

	uninstallCmd := app.Command("uninstall", "Uninstall the updater-managed installation of Teleport. If the Teleport package is installed, it is restored as the primary installation.")
	uninstallCmd.Flag("force", "Force complete uninstallation of Teleport, even if there is no packaged version of Teleport to revert to.").
		Short('f').BoolVar(&ccfg.ForceUninstall)

	libutils.UpdateAppUsageTemplate(app, args)
	command, err := app.Parse(args)
	if err != nil {
		app.Usage(args)
		libutils.FatalError(err)
	}

	// Logging must be configured as early as possible to ensure all log
	// message are formatted correctly.
	if err := setupLogger(ccfg.Debug, ccfg.LogFormat); err != nil {
		plog.ErrorContext(ctx, "Failed to set up logger.", "error", err)
		return 1
	}

	switch command {
	case enableCmd.FullCommand():
		ccfg.Enabled = true
		err = cmdInstall(ctx, &ccfg)
	case pinCmd.FullCommand():
		ccfg.Pinned = true
		err = cmdInstall(ctx, &ccfg)
	case disableCmd.FullCommand():
		err = cmdDisable(ctx, &ccfg)
	case unpinCmd.FullCommand():
		err = cmdUnpin(ctx, &ccfg)
	case updateCmd.FullCommand():
		err = cmdUpdate(ctx, &ccfg)
	case linkCmd.FullCommand():
		err = cmdLinkPackage(ctx, &ccfg)
	case unlinkCmd.FullCommand():
		err = cmdUnlinkPackage(ctx, &ccfg)
	case setupCmd.FullCommand():
		err = cmdSetup(ctx, &ccfg)
	case statusCmd.FullCommand():
		err = cmdStatus(ctx, &ccfg)
	case uninstallCmd.FullCommand():
		err = cmdUninstall(ctx, &ccfg)
	case versionCmd.FullCommand():
		modules.GetModules().PrintVersion()
	default:
		// This should only happen when there's a missing switch case above.
		err = trace.Errorf("command %s not configured", command)
	}
	if err != nil {
		plog.ErrorContext(ctx, "Command failed.", "error", err)
		return 1
	}
	return 0
}

func setupLogger(debug bool, format string) error {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	switch format {
	case libutils.LogFormatJSON:
	case libutils.LogFormatText, "":
	default:
		return trace.Errorf("unsupported log format %q", format)
	}

	libutils.InitLogger(libutils.LoggingForDaemon, level, libutils.WithLogFormat(format))
	return nil
}

func initConfig(ctx context.Context, ccfg *cliConfig) (updater *autoupdate.Updater, lockFile string, err error) {
	ns, err := autoupdate.NewNamespace(ctx, plog, ccfg.InstallSuffix, ccfg.InstallDir)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	lockFile, err = ns.Init()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	updater, err = autoupdate.NewLocalUpdater(autoupdate.LocalUpdaterConfig{
		SelfSetup:          ccfg.SelfSetup,
		Log:                plog,
		LogFormat:          ccfg.LogFormat,
		Debug:              ccfg.Debug,
		InsecureSkipVerify: ccfg.Insecure,
	}, ns)
	return updater, lockFile, trace.Wrap(err)
}

func statusConfig(ctx context.Context, ccfg *cliConfig) (*autoupdate.Updater, error) {
	ns, err := autoupdate.NewNamespace(ctx, plog, ccfg.InstallSuffix, ccfg.InstallDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updater, err := autoupdate.NewLocalUpdater(autoupdate.LocalUpdaterConfig{
		SelfSetup:          ccfg.SelfSetup,
		Log:                plog,
		LogFormat:          ccfg.LogFormat,
		Debug:              ccfg.Debug,
		InsecureSkipVerify: ccfg.Insecure,
	}, ns)
	return updater, trace.Wrap(err)
}

// cmdDisable disables updates.
func cmdDisable(ctx context.Context, ccfg *cliConfig) error {
	updater, lockFile, err := initConfig(ctx, ccfg)
	if err != nil {
		return trace.Wrap(err, "failed to initialize updater")
	}
	unlock, err := libutils.FSTryWriteLockTimeout(ctx, lockFile, updateLockTimeout)
	if err != nil {
		return trace.Wrap(err, "failed to grab concurrent execution lock %s", lockFile)
	}
	defer func() {
		if err := unlock(); err != nil {
			plog.DebugContext(ctx, "Failed to close lock file", "error", err)
		}
	}()
	if err := updater.Disable(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// cmdUnpin unpins the current version.
func cmdUnpin(ctx context.Context, ccfg *cliConfig) error {
	updater, lockFile, err := initConfig(ctx, ccfg)
	if err != nil {
		return trace.Wrap(err, "failed to setup updater")
	}
	unlock, err := libutils.FSTryWriteLockTimeout(ctx, lockFile, updateLockTimeout)
	if err != nil {
		return trace.Wrap(err, "failed to grab concurrent execution lock %n", lockFile)
	}
	defer func() {
		if err := unlock(); err != nil {
			plog.DebugContext(ctx, "Failed to close lock file", "error", err)
		}
	}()
	if err := updater.Unpin(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// cmdInstall installs Teleport and sets configuration.
func cmdInstall(ctx context.Context, ccfg *cliConfig) error {
	updater, lockFile, err := initConfig(ctx, ccfg)
	if err != nil {
		return trace.Wrap(err, "failed to initialize updater")
	}

	// Ensure enable can't run concurrently.
	unlock, err := libutils.FSTryWriteLockTimeout(ctx, lockFile, updateLockTimeout)
	if err != nil {
		return trace.Wrap(err, "failed to grab concurrent execution lock %s", lockFile)
	}
	defer func() {
		if err := unlock(); err != nil {
			plog.DebugContext(ctx, "Failed to close lock file", "error", err)
		}
	}()
	if err := updater.Install(ctx, ccfg.OverrideConfig); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// cmdUpdate updates Teleport to the version specified by cluster reachable at the proxy address.
func cmdUpdate(ctx context.Context, ccfg *cliConfig) error {
	updater, lockFile, err := initConfig(ctx, ccfg)
	if err != nil {
		return trace.Wrap(err, "failed to initialize updater")
	}
	// Ensure update can't run concurrently.
	var unlock func() error
	if ccfg.UpdateNow {
		unlock, err = libutils.FSTryWriteLockTimeout(ctx, lockFile, updateLockTimeout)
	} else {
		unlock, err = libutils.FSWriteLock(lockFile)
	}
	if err != nil {
		return trace.Wrap(err, "failed to grab concurrent execution lock %s", lockFile)
	}
	defer func() {
		if err := unlock(); err != nil {
			plog.DebugContext(ctx, "Failed to close lock file", "error", err)
		}
	}()

	if err := updater.Update(ctx, ccfg.UpdateNow); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// cmdLinkPackage creates system package links if no version is linked and managed updates is disabled.
func cmdLinkPackage(ctx context.Context, ccfg *cliConfig) error {
	updater, lockFile, err := initConfig(ctx, ccfg)
	if err != nil {
		return trace.Wrap(err, "failed to initialize updater")
	}

	// Skip operation and warn if the updater is currently running.
	unlock, err := libutils.FSTryReadLock(lockFile)
	if errors.Is(err, libutils.ErrUnsuccessfulLockTry) {
		plog.WarnContext(ctx, "Updater is currently running. Skipping package linking.")
		return nil
	}
	if err != nil {
		return trace.Wrap(err, "failed to grab concurrent execution lock %q", lockFile)
	}
	defer func() {
		if err := unlock(); err != nil {
			plog.DebugContext(ctx, "Failed to close lock file", "error", err)
		}
	}()

	if err := updater.LinkPackage(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// cmdUnlinkPackage remove system package links.
func cmdUnlinkPackage(ctx context.Context, ccfg *cliConfig) error {
	updater, lockFile, err := initConfig(ctx, ccfg)
	if err != nil {
		return trace.Wrap(err, "failed to setup updater")
	}

	// Error if the updater is running. We could remove its links by accident.
	unlock, err := libutils.FSTryWriteLock(lockFile)
	if errors.Is(err, libutils.ErrUnsuccessfulLockTry) {
		plog.WarnContext(ctx, "Updater is currently running. Skipping package unlinking.")
		return nil
	}
	if err != nil {
		return trace.Wrap(err, "failed to grab concurrent execution lock %q", lockFile)
	}
	defer func() {
		if err := unlock(); err != nil {
			plog.DebugContext(ctx, "Failed to close lock file", "error", err)
		}
	}()

	if err := updater.UnlinkPackage(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// cmdSetup writes configuration files that are needed to run teleport-update update.
func cmdSetup(ctx context.Context, ccfg *cliConfig) error {
	ns, err := autoupdate.NewNamespace(ctx, plog, ccfg.InstallSuffix, ccfg.InstallDir)
	if err != nil {
		return trace.Wrap(err)
	}
	updater, err := autoupdate.NewLocalUpdater(autoupdate.LocalUpdaterConfig{
		SelfSetup:          ccfg.SelfSetup,
		Log:                plog,
		LogFormat:          ccfg.LogFormat,
		Debug:              ccfg.Debug,
		InsecureSkipVerify: ccfg.Insecure,
	}, ns)
	if err != nil {
		return trace.Wrap(err)
	}
	err = updater.Setup(ctx, ccfg.Path, ccfg.Reload)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// cmdStatus displays auto-update status.
func cmdStatus(ctx context.Context, ccfg *cliConfig) error {
	updater, err := statusConfig(ctx, ccfg)
	if err != nil {
		return trace.Wrap(err, "failed to initialize updater")
	}
	status, err := updater.Status(ctx)
	if err != nil {
		return trace.Wrap(err, "failed to get status")
	}
	enc := yaml.NewEncoder(os.Stdout)
	return trace.Wrap(enc.Encode(status))
}

// cmdUninstall removes the updater-managed install of Teleport and gracefully reverts back to the Teleport package.
func cmdUninstall(ctx context.Context, ccfg *cliConfig) error {
	updater, lockFile, err := initConfig(ctx, ccfg)
	if err != nil {
		return trace.Wrap(err, "failed to initialize updater")
	}
	// Ensure update can't run concurrently.
	unlock, err := libutils.FSTryWriteLockTimeout(ctx, lockFile, updateLockTimeout)
	if err != nil {
		return trace.Wrap(err, "failed to grab concurrent execution lock %s", lockFile)
	}
	defer func() {
		if err := unlock(); err != nil {
			plog.DebugContext(ctx, "Failed to close lock file", "error", err)
		}
	}()

	if err := updater.Remove(ctx, ccfg.ForceUninstall); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
