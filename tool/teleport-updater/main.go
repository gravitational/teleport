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
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const appHelp = `Teleport Updater

The Teleport Updater updates the version a Teleport agent on a Linux server
that is being used as agent to provide connectivity to Teleport resources.

The Teleport Updater supports upgrade schedules and automated rollbacks. 

Find out more at https://goteleport.com/docs/updater`

const (
	templateEnvVar    = "TELEPORT_URL_TEMPLATE"
	proxyServerEnvVar = "TELEPORT_PROXY"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentTBot)

func main() {
	if err := Run(os.Args[1:], os.Stdout); err != nil {
		utils.FatalError(err)
	}
}

type cliConfig struct {
	Debug bool

	ProxyServer string
	Template    string

	// LogFormat controls the format of logging. Can be either `json` or `text`.
	// By default, this is `text`.
	LogFormat string
}

func Run(args []string, stdout io.Writer) error {
	var cf cliConfig
	ctx := context.Background()

	app := utils.InitCLIParser("teleport-updater", appHelp).Interspersed(false)
	app.Flag("debug", "Verbose logging to stdout.").Short('d').BoolVar(&cf.Debug)
	app.HelpFlag.Short('h')

	versionCmd := app.Command("version", "Print the version of your teleport-updater binary.")

	enableCmd := app.Command("enable", "Enable agent auto-updates and perform initial updates.")
	enableCmd.Flag("proxy", "Address of the Teleport Proxy.").Short('p').Envar(proxyServerEnvVar).StringVar(&cf.ProxyServer)
	enableCmd.Flag("template", "Go template to override Teleport tgz download URL.").Short('t').Envar(templateEnvVar).StringVar(&cf.Template)

	updateCmd := app.Command("update", "Update agent to the latest version, if a new version is available.")

	utils.UpdateAppUsageTemplate(app, args)
	command, err := app.Parse(args)
	if err != nil {
		app.Usage(args)
		return trace.Wrap(err)
	}
	// Logging must be configured as early as possible to ensure all log
	// message are formatted correctly.
	if err := setupLogger(cf.Debug, cf.LogFormat); err != nil {
		return trace.Wrap(err, "setting up logger")
	}

	err = validate(&cf)
	if err != nil {
		return trace.Wrap(err)
	}

	switch command {
	case enableCmd.FullCommand():
		err = doEnable(ctx, &cf)
	case updateCmd.FullCommand():
		err = doUpdate(ctx, &cf)
	case versionCmd.FullCommand():
		err = outputVersion(ctx)
	default:
		// This should only happen when there's a missing switch case above.
		err = trace.BadParameter("command %q not configured", command)
	}

	return err
}

func setupLogger(debug bool, format string) error {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	switch format {
	case utils.LogFormatJSON:
	case utils.LogFormatText, "":
	default:
		return trace.BadParameter("unsupported log format %q", format)
	}

	utils.InitLogger(utils.LoggingForDaemon, level, utils.WithLogFormat(format))
	return nil
}

var (
	versionsDir = filepath.Join(defaults.DataDir, "versions")
	updatesYAML = filepath.Join(versionsDir, "updates.yaml")
)

type UpdatesConfig struct {
	Version string `yaml:"version"`
	Kind    string `yaml:"kind"`
	Spec    struct {
		Proxy         string `yaml:"proxy"`
		Enabled       bool   `yaml:"enabled"`
		ActiveVersion string `yaml:"active_version"`
	} `yaml:"spec"`
}

func readUpdatesConfig(path string) (*UpdatesConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err, "failed to open updates.yaml")
	}
	defer f.Close()
	var cfg UpdatesConfig
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, trace.Wrap(err, "failed to parse updates.yaml")
	}
	return &cfg, nil
}

func doEnable(ctx context.Context, cf *cliConfig) error {
	addr, err := utils.ParseAddr(cf.ProxyServer)
	if err != nil {
		return trace.Wrap(err, "failed to parse proxy server address")
	}
	resp, err := webclient.Find(&webclient.Config{
		Context:   ctx,
		ProxyAddr: addr.Addr,
		Timeout:   30 * time.Second,
	})
	if err != nil {
		return trace.Wrap(err, "failed to request version from proxy")
	}
	cfg, err := readUpdatesConfig(updatesYAML)
	if err != nil {
		return trace.Wrap(err)
	}

}

func validate(cf *cliConfig) error {

}
