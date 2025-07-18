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
	"errors"
	"net/url"
	"path/filepath"
	"time"

	"github.com/alecthomas/kingpin/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
	"github.com/gravitational/teleport/tool/tctl/common/top/client"
)

// Command is a debug command that consumes the
// Teleport /metrics endpoint and displays diagnostic
// information an easy to consume way.
type Command struct {
	config        *servicecfg.Config
	top           *kingpin.CmdClause
	diagURL       string
	refreshPeriod time.Duration
}

const fallbackHTTPAddr = "http://127.0.0.1:3000"

// Initialize sets up the "tctl top" command.
func (c *Command) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config
	c.top = app.Command("top", "Report diagnostic information.")
	c.top.Arg("diag-addr", "Diagnostic HTTP URL").StringVar(&c.diagURL)
	c.top.Arg("refresh", "Refresh period").Default("5s").DurationVar(&c.refreshPeriod)
}

func (c *Command) newDiagClient(ctx context.Context) (string, client.MetricCient, error) {
	if c.diagURL != "" {
		clt, err := client.NewMetricCient(c.diagURL)
		return c.diagURL, clt, err
	}

	sockURL := url.URL{
		Scheme: "unix",
		Path:   filepath.Join(c.config.DataDir, "debug.sock"),
	}

	var errs []error

	sockClient, err := client.NewMetricCient(sockURL.String())
	if err != nil {
		errs = append(errs, trace.Wrap(err, "failed instanciate local debug client"))
	}
	if _, err = sockClient.GetMetrics(ctx); err != nil {
		errs = append(errs, trace.Wrap(err, "failed connect to local debug service, is Teleport running?"))
	} else {
		return sockURL.String(), sockClient, nil
	}

	fallbackClient, err := client.NewMetricCient(fallbackHTTPAddr)
	if err != nil {
		errs = append(errs, trace.Wrap(err, "failed instanciate http metric client"))
	}

	if _, err = fallbackClient.GetMetrics(ctx); err != nil {
		errs = append(errs, trace.Wrap(err, "failed connect to http metric service, restart Teleport with --diag-addr"))
	} else {
		return fallbackHTTPAddr, fallbackClient, nil
	}

	return "", nil, errors.Join(errs...)
}

// TryRun attempts to run subcommands.
func (c *Command) TryRun(ctx context.Context, cmd string, _ commonclient.InitFunc) (match bool, err error) {
	if cmd != c.top.FullCommand() {
		return false, nil
	}

	addr, diagClient, err := c.newDiagClient(ctx)
	if err != nil {
		return true, trace.Wrap(err)
	}

	p := tea.NewProgram(
		newTopModel(c.refreshPeriod, diagClient, addr),
		tea.WithAltScreen(),
		tea.WithContext(ctx),
	)

	_, err = p.Run()
	return true, trace.Wrap(err)
}
