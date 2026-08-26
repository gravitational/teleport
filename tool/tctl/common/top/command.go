// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package top

import (
	"context"
	"time"

	"github.com/alecthomas/kingpin/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitational/trace"
	dto "github.com/prometheus/client_model/go"

	"github.com/gravitational/teleport/lib/client/debug"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
	"github.com/gravitational/teleport/tool/tctl/common/top/client/diag"
)

// Command is a debug command that consumes the
// Teleport /metrics endpoint and displays diagnostic
// information an easy to consume way.
type Command struct {
	global        *tctlcfg.GlobalCLIFlags
	config        *servicecfg.Config
	top           *kingpin.CmdClause
	diagURL       string
	refreshPeriod time.Duration
}

const defaultDiagAddr = "http://127.0.0.1:3000"

// Initialize sets up the "tctl top" command.
func (c *Command) Initialize(app *kingpin.Application, global *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config
	c.global = global
	c.top = app.Command("top", "Report diagnostic information.")
	c.top.Arg("diag-addr", "Diagnostic HTTP URL").StringVar(&c.diagURL)
	c.top.Arg("refresh", "Refresh period").Default("5s").DurationVar(&c.refreshPeriod)
}

type MetricsClient interface {
	GetMetrics(ctx context.Context) (map[string]*dto.MetricFamily, error)
}

func (c *Command) newMetricsClient(ctx context.Context) (string, MetricsClient, error) {
	if c.diagURL != "" {
		clt, err := diag.NewClient(c.diagURL)
		return c.diagURL, clt, trace.Wrap(err)
	}

	// Try the local UNIX debug service client first.
	debugClient := debug.NewClient(c.config.DataDir)
	_, debugErr := debugClient.GetMetrics(ctx)
	if debugErr == nil {
		return debugClient.SocketPath(), debugClient, nil
	}
	debugErr = trace.Wrap(debugErr, "retrieving metrics from debug service")

	// Try default diagnostic address
	diagClient, defErr := diag.NewClient(defaultDiagAddr)
	if defErr != nil {
		return "", nil, trace.Wrap(
			trace.NewAggregate(
				trace.Wrap(defErr, "creating diagnostics client for default address %q", defaultDiagAddr),
				debugErr),
			"unable to connect to Teleport metrics server")
	}

	_, defErr = diagClient.GetMetrics(ctx)
	if defErr == nil {
		return defaultDiagAddr, diagClient, nil
	}

	return "", nil, trace.Wrap(
		trace.NewAggregate(
			trace.Wrap(defErr, "getting metrics from diagnostics client at default address %q", defaultDiagAddr),
			debugErr,
		),
		"connecting to Teleport metrics server")
}

// initConfig populates the [*servicecfg.Config]
//
// This is required by `tctl top` as it does not make use of the [commonclient.InitFunc]
// which lazy loads the configuration during client creation, meaning the required configuration
// fields are not initilized as this command uses a standalone metrics client instead on a local connection.
//
// TODO(okraport): remove this workaround once the caller can safely parse the teleport config file without common client.
func (c *Command) initConfig() error {
	var fileConf *config.FileConfig
	var err error
	if c.global.ConfigFile != "" {
		fileConf, err = config.ReadConfigFile(c.global.ConfigFile)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if c.global.ConfigString != "" {
		fileConf, err = config.ReadFromString(c.global.ConfigString)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if fileConf != nil {
		if err = config.ApplyFileConfig(fileConf, c.config); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// TryRun attempts to run subcommands.
func (c *Command) TryRun(ctx context.Context, cmd string, _ commonclient.InitFunc) (match bool, err error) {
	if cmd != c.top.FullCommand() {
		return false, nil
	}

	if err := c.initConfig(); err != nil {
		return true, trace.Wrap(err)
	}

	addr, metricsClient, err := c.newMetricsClient(ctx)
	if err != nil {
		return true, trace.Wrap(err)
	}

	p := tea.NewProgram(
		newTopModel(c.refreshPeriod, metricsClient, addr),
		tea.WithAltScreen(),
		tea.WithContext(ctx),
	)

	_, err = p.Run()
	return true, trace.Wrap(err)
}
