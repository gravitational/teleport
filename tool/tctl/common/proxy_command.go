// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"context"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
)

// ProxyCommand returns information about connected proxies
type ProxyCommand struct {
	config *service.Config
	lsCmd  *kingpin.CmdClause

	format string
}

// Initialize creates the proxy command and subcommands
func (p *ProxyCommand) Initialize(app *kingpin.Application, config *service.Config) {
	p.config = config

	proxyCommand := app.Command("proxy", "Operations with information for cluster proxies.").Hidden()
	p.lsCmd = proxyCommand.Command("ls", "Lists proxies connected to the cluster.")
	p.lsCmd.Flag("format", "Output format: 'yaml', 'json' or 'text'").Default(teleport.YAML).StringVar(&p.format)

}

// ListProxies prints currently connected proxies
func (p *ProxyCommand) ListProxies(ctx context.Context, clusterAPI auth.ClientI) error {
	proxies, err := clusterAPI.GetProxies()
	if err != nil {
		return trace.Wrap(err)
	}

	sc := &serverCollection{proxies, false}

	switch p.format {
	case teleport.Text:
		return sc.writeText(os.Stdout)
	case teleport.YAML:
		return writeYAML(sc, os.Stdout)
	case teleport.JSON:
		return writeJSON(sc, os.Stdout)
	}

	return nil
}

// TryRun runs the proxy command
func (p *ProxyCommand) TryRun(ctx context.Context, cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case p.lsCmd.FullCommand():
		err = p.ListProxies(ctx, client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}
