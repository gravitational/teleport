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
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// ProxyCommand returns information about connected proxies
type ProxyCommand struct {
	config *servicecfg.Config
	lsCmd  *kingpin.CmdClause

	format string
}

// Initialize creates the proxy command and subcommands
func (p *ProxyCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	p.config = config

	proxyCommand := app.Command("proxy", "Operations with information for cluster proxies.").Hidden()
	p.lsCmd = proxyCommand.Command("ls", "Lists proxies connected to the cluster.")
	p.lsCmd.Flag("format", "Output format: 'yaml', 'json' or 'text'").Default(teleport.YAML).StringVar(&p.format)

}

// ListProxies prints currently connected proxies
func (p *ProxyCommand) ListProxies(ctx context.Context, clusterAPI *authclient.Client) error {
	proxies, err := clusterAPI.GetProxies()
	if err != nil {
		return trace.Wrap(err)
	}

	sc := &serverCollection{proxies}

	switch p.format {
	case teleport.Text:
		// proxies don't have labels.
		verbose := false
		return sc.writeText(os.Stdout, verbose)
	case teleport.YAML:
		return writeYAML(sc, os.Stdout)
	case teleport.JSON:
		return writeJSON(sc, os.Stdout)
	}

	return nil
}

// TryRun runs the proxy command
func (p *ProxyCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case p.lsCmd.FullCommand():
		commandFunc = p.ListProxies
	default:
		return false, nil
	}
	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	err = commandFunc(ctx, client)
	closeFn(ctx)
	return true, trace.Wrap(err)
}
