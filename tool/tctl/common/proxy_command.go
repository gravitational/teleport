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
	"io"
	"os"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/trace"
)

// ProxyCommand returns information about connected proxies
type ProxyCommand struct {
	config *service.Config
	lsCmd  *kingpin.CmdClause

	format string

	stdout io.Writer
}

// Initialize creates the proxy command and subcommands
func (p *ProxyCommand) Initialize(app *kingpin.Application, config *service.Config) {
	p.config = config

	authCommand := app.Command("proxy", "Operations with information for cluster proxies").Hidden()
	p.lsCmd = authCommand.Command("ls", "List connected auth servers")
	p.lsCmd.Flag("format", "Output format: 'yaml', 'json' or 'text'").Default(teleport.YAML).StringVar(&p.format)

	if p.stdout == nil {
		p.stdout = os.Stdout
	}
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
		return sc.writeText(p.stdout)
	case teleport.YAML:
		return writeYAML(sc, p.stdout)
	case teleport.JSON:
		return writeJSON(sc, p.stdout)
	}

	return nil
}

// TryRun runs the proxy command
func (p *ProxyCommand) TryRun(ctx context.Context, cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case p.lsCmd.FullCommand():
		err = p.ListProxies(ctx, client)
		if err != nil {
			return false, err
		}
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}
