/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/autoupdate/tools"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

const (
	searchHelp = `List of comma separated search keywords or phrases enclosed in quotations (e.g. --search=foo,bar,"some phrase")`
	queryHelp  = `Query by predicate language enclosed in single quotes. Supports ==, !=, &&, and || (e.g. --query='labels["key1"] == "value1" && labels["key2"] != "value2"')`
	labelHelp  = "List of comma separated labels to filter by labels (e.g. key1=value1,key2=value2)"
)

const (
	identityFileEnvVar = "TELEPORT_IDENTITY_FILE"
	authAddrEnvVar     = "TELEPORT_AUTH_SERVER"
)

// CLICommand interface must be implemented by every CLI command
//
// This allows OSS and Enterprise Teleport editions to plug their own
// implementations of different CLI commands into the common execution
// framework
type CLICommand interface {
	// Initialize allows a caller-defined command to plug itself into CLI
	// argument parsing
	Initialize(*kingpin.Application, *tctlcfg.GlobalCLIFlags, *servicecfg.Config)

	// TryRun is executed after the CLI parsing is done. The command must
	// determine if selectedCommand belongs to it and return match=true
	TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error)
}

// Run is the same as 'make'. It helps to share the code between different
// "distributions" like OSS or Enterprise
//
// distribution: name of the Teleport distribution
func Run(ctx context.Context, commands []CLICommand) {
	if err := tools.CheckAndUpdateLocal(ctx, os.Args[1:]); err != nil {
		utils.FatalError(err)
	}

	err := TryRun(commands, os.Args[1:])
	if err != nil {
		var exitError *common.ExitCodeError
		if errors.As(err, &exitError) {
			os.Exit(exitError.Code)
		}
		utils.FatalError(err)
	}
}

// TryRun is a helper function for Run to call - it runs a tctl command and returns an error.
// This is useful for testing tctl, because we can capture the returned error in tests.
func TryRun(commands []CLICommand, args []string) error {
	utils.InitLogger(utils.LoggingForCLI, slog.LevelWarn)

	// app is the command line parser
	app := utils.InitCLIParser("tctl", GlobalHelpString)

	// cfg (teleport auth server configuration) is going to be shared by all
	// commands
	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	var ccf tctlcfg.GlobalCLIFlags

	// Each command will add itself to the CLI parser.
	for i := range commands {
		commands[i].Initialize(app, &ccf, cfg)
	}

	// If the config file path is being overridden by environment variable, set that.
	// If not, check whether the default config file path exists and set that if so.
	// This preserves tctl's default behavior for backwards compatibility.
	if configFileEnv, ok := os.LookupEnv(defaults.ConfigFileEnvar); ok {
		ccf.ConfigFile = configFileEnv
	} else {
		if utils.FileExists(defaults.ConfigFilePath) {
			ccf.ConfigFile = defaults.ConfigFilePath
		}
	}

	// these global flags apply to all commands
	app.Flag("debug", "Enable verbose logging to stderr").
		Short('d').
		BoolVar(&ccf.Debug)
	app.Flag("config", fmt.Sprintf("Path to a configuration file [%v] for an Auth Service instance. Can also be set via the %v environment variable. Ignored if the auth_service is disabled.", defaults.ConfigFilePath, defaults.ConfigFileEnvar)).
		Short('c').
		ExistingFileVar(&ccf.ConfigFile)
	app.Flag("config-string",
		"Base64 encoded configuration string. Ignored if the config auth_service is disabled.").Hidden().Envar(defaults.ConfigEnvar).StringVar(&ccf.ConfigString)
	app.Flag("auth-server",
		fmt.Sprintf("Attempts to connect to specific auth/proxy address(es) instead of local auth [%v]", defaults.AuthConnectAddr().Addr)).
		Envar(authAddrEnvVar).
		StringsVar(&ccf.AuthServerAddr)
	app.Flag("identity",
		"Path to an identity file. Must be provided to make remote connections to auth. An identity file can be exported with 'tctl auth sign'").
		Short('i').
		Envar(identityFileEnvVar).
		StringVar(&ccf.IdentityFilePath)
	app.Flag("insecure", "When specifying a proxy address in --auth-server, do not verify its TLS certificate. Danger: any data you send can be intercepted or modified by an attacker.").
		BoolVar(&ccf.Insecure)
	app.HelpFlag.Short('h')

	// parse CLI commands+flags:
	utils.UpdateAppUsageTemplate(app, args)
	selectedCmd, err := app.Parse(args)
	if err != nil {
		app.Usage(args)
		return trace.Wrap(err)
	}

	// Identity files do not currently contain a proxy address. When loading an
	// Identity file, an auth server address must be passed on the command line
	// as well.
	if ccf.IdentityFilePath != "" && len(ccf.AuthServerAddr) == 0 {
		return trace.BadParameter("tctl --identity also requires --auth-server")
	}

	cfg.TeleportHome = os.Getenv(types.HomeEnvVar)
	if cfg.TeleportHome != "" {
		cfg.TeleportHome = filepath.Clean(cfg.TeleportHome)
	}

	cfg.Debug = ccf.Debug

	ctx := context.Background()
	clientFunc := commonclient.GetInitFunc(ccf, cfg)
	// Execute whatever is selected.
	for _, c := range commands {
		match, err := c.TryRun(ctx, selectedCmd, clientFunc)
		if err != nil {
			return trace.Wrap(err)
		}
		if match {
			break
		}
	}

	return nil
}
