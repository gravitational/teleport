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
	"path/filepath"
	"syscall"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	autoupdate "github.com/gravitational/teleport/lib/autoupdate/agent"
	libdefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	libutils "github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const appHelp = `Teleport Updater

The Teleport Updater updates the version a Teleport agent on a Linux server
that is being used as agent to provide connectivity to Teleport resources.

The Teleport Updater supports upgrade schedules and automated rollbacks. 

Find out more at https://goteleport.com/docs/updater`

const (
	// templateEnvVar allows the template for the Teleport tgz to be specified via env var.
	templateEnvVar = "TELEPORT_URL_TEMPLATE"
	// proxyServerEnvVar allows the proxy server address to be specified via env var.
	proxyServerEnvVar = "TELEPORT_PROXY"
	// updateGroupEnvVar allows the update group to be specified via env var.
	updateGroupEnvVar = "TELEPORT_UPDATE_GROUP"
	// updateVersionEnvVar forces the version to specified value.
	updateVersionEnvVar = "TELEPORT_UPDATE_VERSION"
)

const (
	// lockFileName specifies the name of the file containing the flock lock preventing concurrent updater execution.
	lockFileName = ".update-lock"
)

var plog = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentUpdater)

func main() {
	if err := Run(os.Args[1:]); err != nil {
		libutils.FatalError(err)
	}
}

type cliConfig struct {
	autoupdate.OverrideConfig

	// Debug logs enabled
	Debug bool
	// LogFormat controls the format of logging. Can be either `json` or `text`.
	// By default, this is `text`.
	LogFormat string
	// DataDir for Teleport (usually /var/lib/teleport)
	DataDir string
	// LinkDir for linking binaries and systemd services
	LinkDir string
	// SelfSetup mode for using the current version of the teleport-update to setup the update service.
	SelfSetup bool
}

func Run(args []string) error {
	var ccfg cliConfig
	ctx := context.Background()
	ctx, _ = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)

	app := libutils.InitCLIParser(autoupdate.BinaryName, appHelp).Interspersed(false)
	app.Flag("debug", "Verbose logging to stdout.").
		Short('d').BoolVar(&ccfg.Debug)
	app.Flag("data-dir", "Teleport data directory. Access to this directory should be limited.").
		Default(libdefaults.DataDir).StringVar(&ccfg.DataDir)
	app.Flag("log-format", "Controls the format of output logs. Can be `json` or `text`. Defaults to `text`.").
		Default(libutils.LogFormatText).EnumVar(&ccfg.LogFormat, libutils.LogFormatJSON, libutils.LogFormatText)
	app.Flag("link-dir", "Directory to link the active Teleport installation into.").
		Default(autoupdate.DefaultLinkDir).Hidden().StringVar(&ccfg.LinkDir)

	app.HelpFlag.Short('h')

	versionCmd := app.Command("version", fmt.Sprintf("Print the version of your %s binary.", autoupdate.BinaryName))

	enableCmd := app.Command("enable", "Enable agent auto-updates and perform initial update.")
	enableCmd.Flag("proxy", "Address of the Teleport Proxy.").
		Short('p').Envar(proxyServerEnvVar).StringVar(&ccfg.Proxy)
	enableCmd.Flag("group", "Update group for this agent installation.").
		Short('g').Envar(updateGroupEnvVar).StringVar(&ccfg.Group)
	enableCmd.Flag("template", "Go template used to override Teleport download URL.").
		Short('t').Envar(templateEnvVar).StringVar(&ccfg.URLTemplate)
	enableCmd.Flag("force-version", "Force the provided version instead of querying it from the Teleport cluster.").
		Short('f').Envar(updateVersionEnvVar).Hidden().StringVar(&ccfg.ForceVersion)
	enableCmd.Flag("self-setup", "Use the current teleport-update binary to create systemd service config for auto-updates.").
		Short('s').Hidden().BoolVar(&ccfg.SelfSetup)
	// TODO(sclevine): add force-fips and force-enterprise as hidden flags

	disableCmd := app.Command("disable", "Disable agent auto-updates.")

	updateCmd := app.Command("update", "Update agent to the latest version, if a new version is available.")
	updateCmd.Flag("self-setup", "Use the current teleport-update binary to create systemd service config for auto-updates.").
		Short('s').Hidden().BoolVar(&ccfg.SelfSetup)

	linkCmd := app.Command("link-package", "Link the system installation of Teleport from the Teleport package, if auto-updates is disabled.")
	unlinkCmd := app.Command("unlink-package", "Unlink the system installation of Teleport from the Teleport package.")

	setupCmd := app.Command("setup", "Write configuration files that run the update subcommand on a timer.").
		Hidden()

	libutils.UpdateAppUsageTemplate(app, args)
	command, err := app.Parse(args)
	if err != nil {
		app.Usage(args)
		return trace.Wrap(err)
	}
	// Logging must be configured as early as possible to ensure all log
	// message are formatted correctly.
	if err := setupLogger(ccfg.Debug, ccfg.LogFormat); err != nil {
		return trace.Errorf("failed to set up logger")
	}

	switch command {
	case enableCmd.FullCommand():
		err = cmdEnable(ctx, &ccfg)
	case disableCmd.FullCommand():
		err = cmdDisable(ctx, &ccfg)
	case updateCmd.FullCommand():
		err = cmdUpdate(ctx, &ccfg)
	case linkCmd.FullCommand():
		err = cmdLink(ctx, &ccfg)
	case unlinkCmd.FullCommand():
		err = cmdUnlink(ctx, &ccfg)
	case setupCmd.FullCommand():
		err = cmdSetup(ctx, &ccfg)
	case versionCmd.FullCommand():
		modules.GetModules().PrintVersion()
	default:
		// This should only happen when there's a missing switch case above.
		err = trace.Errorf("command %q not configured", command)
	}

	return err
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

// cmdDisable disables updates.
func cmdDisable(ctx context.Context, ccfg *cliConfig) error {
	updater, err := autoupdate.NewLocalUpdater(autoupdate.LocalUpdaterConfig{
		DataDir:   ccfg.DataDir,
		LinkDir:   ccfg.LinkDir,
		SystemDir: autoupdate.DefaultSystemDir,
		SelfSetup: ccfg.SelfSetup,
		Log:       plog,
	})
	if err != nil {
		return trace.Errorf("failed to setup updater: %w", err)
	}
	unlock, err := libutils.FSWriteLock(filepath.Join(ccfg.DataDir, lockFileName))
	if err != nil {
		return trace.Errorf("failed to grab concurrent execution lock: %w", err)
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

// cmdEnable enables updates and triggers an initial update.
func cmdEnable(ctx context.Context, ccfg *cliConfig) error {
	updater, err := autoupdate.NewLocalUpdater(autoupdate.LocalUpdaterConfig{
		DataDir:   ccfg.DataDir,
		LinkDir:   ccfg.LinkDir,
		SystemDir: autoupdate.DefaultSystemDir,
		SelfSetup: ccfg.SelfSetup,
		Log:       plog,
	})
	if err != nil {
		return trace.Errorf("failed to setup updater: %w", err)
	}

	// Ensure enable can't run concurrently.
	unlock, err := libutils.FSWriteLock(filepath.Join(ccfg.DataDir, lockFileName))
	if err != nil {
		return trace.Errorf("failed to grab concurrent execution lock: %w", err)
	}
	defer func() {
		if err := unlock(); err != nil {
			plog.DebugContext(ctx, "Failed to close lock file", "error", err)
		}
	}()
	if err := updater.Enable(ctx, ccfg.OverrideConfig); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// cmdUpdate updates Teleport to the version specified by cluster reachable at the proxy address.
func cmdUpdate(ctx context.Context, ccfg *cliConfig) error {
	updater, err := autoupdate.NewLocalUpdater(autoupdate.LocalUpdaterConfig{
		DataDir:   ccfg.DataDir,
		LinkDir:   ccfg.LinkDir,
		SystemDir: autoupdate.DefaultSystemDir,
		SelfSetup: ccfg.SelfSetup,
		Log:       plog,
	})
	if err != nil {
		return trace.Errorf("failed to setup updater: %w", err)
	}
	// Ensure update can't run concurrently.
	unlock, err := libutils.FSWriteLock(filepath.Join(ccfg.DataDir, lockFileName))
	if err != nil {
		return trace.Errorf("failed to grab concurrent execution lock: %w", err)
	}
	defer func() {
		if err := unlock(); err != nil {
			plog.DebugContext(ctx, "Failed to close lock file", "error", err)
		}
	}()

	if err := updater.Update(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// cmdLink creates system package links if no version is linked and auto-updates is disabled.
func cmdLink(ctx context.Context, ccfg *cliConfig) error {
	updater, err := autoupdate.NewLocalUpdater(autoupdate.LocalUpdaterConfig{
		DataDir:   ccfg.DataDir,
		LinkDir:   ccfg.LinkDir,
		SystemDir: autoupdate.DefaultSystemDir,
		SelfSetup: ccfg.SelfSetup,
		Log:       plog,
	})
	if err != nil {
		return trace.Errorf("failed to setup updater: %w", err)
	}

	// Skip operation and warn if the updater is currently running.
	unlock, err := libutils.FSTryReadLock(filepath.Join(ccfg.DataDir, lockFileName))
	if errors.Is(err, libutils.ErrUnsuccessfulLockTry) {
		plog.WarnContext(ctx, "Updater is currently running. Skipping package linking.")
		return nil
	}
	if err != nil {
		return trace.Errorf("failed to grab concurrent execution lock: %w", err)
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

// cmdUnlink remove system package links.
func cmdUnlink(ctx context.Context, ccfg *cliConfig) error {
	updater, err := autoupdate.NewLocalUpdater(autoupdate.LocalUpdaterConfig{
		DataDir:   ccfg.DataDir,
		LinkDir:   ccfg.LinkDir,
		SystemDir: autoupdate.DefaultSystemDir,
		SelfSetup: ccfg.SelfSetup,
		Log:       plog,
	})
	if err != nil {
		return trace.Errorf("failed to setup updater: %w", err)
	}

	// Error if the updater is running. We could remove its links by accident.
	unlock, err := libutils.FSTryWriteLock(filepath.Join(ccfg.DataDir, lockFileName))
	if errors.Is(err, libutils.ErrUnsuccessfulLockTry) {
		return trace.Errorf("updater is currently running")
	}
	if err != nil {
		return trace.Errorf("failed to grab concurrent execution lock: %w", err)
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
	err := autoupdate.Setup(ctx, plog, ccfg.LinkDir, ccfg.DataDir)
	if errors.Is(err, autoupdate.ErrNotSupported) {
		plog.WarnContext(ctx, "Not enabling systemd service because systemd is not running.")
		os.Exit(autoupdate.CodeNotSupported)
	}
	if err != nil {
		return trace.Errorf("failed to setup teleport-update service: %w", err)
	}
	return nil
}
