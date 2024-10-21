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
	// versionsDirName specifies the name of the subdirectory inside of the Teleport data dir for storing Teleport versions.
	versionsDirName = "versions"
	// lockFileName specifies the name of the file inside versionsDirName containing the flock lock preventing concurrent updater execution.
	lockFileName = ".lock"
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
}

func (c *cliConfig) CheckAndSetDefaults() error {
	if c.DataDir == "" {
		c.DataDir = libdefaults.DataDir
	}
	if c.LogFormat == "" {
		c.LogFormat = libutils.LogFormatText
	}
	return nil
}

func Run(args []string) error {
	var ccfg cliConfig
	ctx := context.Background()
	ctx, _ = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)

	app := libutils.InitCLIParser("teleport-updater", appHelp).Interspersed(false)
	app.Flag("debug", "Verbose logging to stdout.").
		Short('d').BoolVar(&ccfg.Debug)
	app.Flag("data-dir", "Teleport data directory. Access to this directory should be limited.").
		Default(libdefaults.DataDir).StringVar(&ccfg.DataDir)
	app.Flag("log-format", "Controls the format of output logs. Can be `json` or `text`. Defaults to `text`.").
		Default(libutils.LogFormatText).EnumVar(&ccfg.LogFormat, libutils.LogFormatJSON, libutils.LogFormatText)

	app.HelpFlag.Short('h')

	versionCmd := app.Command("version", "Print the version of your teleport-updater binary.")

	enableCmd := app.Command("enable", "Enable agent auto-updates and perform initial update.")
	enableCmd.Flag("proxy", "Address of the Teleport Proxy.").
		Short('p').Envar(proxyServerEnvVar).StringVar(&ccfg.Proxy)
	enableCmd.Flag("group", "Update group for this agent installation.").
		Short('g').Envar(updateGroupEnvVar).StringVar(&ccfg.Group)
	enableCmd.Flag("template", "Go template used to override Teleport download URL.").
		Short('t').Envar(templateEnvVar).StringVar(&ccfg.URLTemplate)
	enableCmd.Flag("force-version", "Force the provided version instead of querying it from the Teleport cluster.").
		Short('f').Envar(updateVersionEnvVar).Hidden().StringVar(&ccfg.ForceVersion)

	disableCmd := app.Command("disable", "Disable agent auto-updates.")

	updateCmd := app.Command("update", "Update agent to the latest version, if a new version is available.")
	updateCmd.Flag("force-version", "Use the provided version instead of querying it from the Teleport cluster.").
		Short('f').Envar(updateVersionEnvVar).Hidden().StringVar(&ccfg.ForceVersion)

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

	if err := ccfg.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	switch command {
	case enableCmd.FullCommand():
		err = cmdEnable(ctx, &ccfg)
	case disableCmd.FullCommand():
		err = cmdDisable(ctx, &ccfg)
	case updateCmd.FullCommand():
		err = cmdUpdate(ctx, &ccfg)
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
	versionsDir := filepath.Join(ccfg.DataDir, versionsDirName)
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		return trace.Errorf("failed to create versions directory: %w", err)
	}

	unlock, err := libutils.FSWriteLock(filepath.Join(versionsDir, lockFileName))
	if err != nil {
		return trace.Errorf("failed to grab concurrent execution lock: %w", err)
	}
	defer func() {
		if err := unlock(); err != nil {
			plog.DebugContext(ctx, "Failed to close lock file", "error", err)
		}
	}()
	updater, err := autoupdate.NewLocalUpdater(autoupdate.LocalUpdaterConfig{
		VersionsDir: versionsDir,
		Log:         plog,
	})
	if err != nil {
		return trace.Errorf("failed to setup updater: %w", err)
	}
	if err := updater.Disable(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// cmdEnable enables updates and triggers an initial update.
func cmdEnable(ctx context.Context, ccfg *cliConfig) error {
	versionsDir := filepath.Join(ccfg.DataDir, versionsDirName)
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		return trace.Errorf("failed to create versions directory: %w", err)
	}

	// Ensure enable can't run concurrently.
	unlock, err := libutils.FSWriteLock(filepath.Join(versionsDir, lockFileName))
	if err != nil {
		return trace.Errorf("failed to grab concurrent execution lock: %w", err)
	}
	defer func() {
		if err := unlock(); err != nil {
			plog.DebugContext(ctx, "Failed to close lock file", "error", err)
		}
	}()

	updater, err := autoupdate.NewLocalUpdater(autoupdate.LocalUpdaterConfig{
		VersionsDir: versionsDir,
		Log:         plog,
	})
	if err != nil {
		return trace.Errorf("failed to setup updater: %w", err)
	}
	if err := updater.Enable(ctx, ccfg.OverrideConfig); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// cmdUpdate updates Teleport to the version specified by cluster reachable at the proxy address.
func cmdUpdate(ctx context.Context, ccfg *cliConfig) error {
	return trace.NotImplemented("TODO")
}
