package common

import (
	"context"
	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/trace"
	"io"
	"os"
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

	auth := app.Command("proxy", "Operations with information for cluster proxies").Hidden()
	p.lsCmd = auth.Command("ls", "List connected auth servers")
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
		return false, nil
	}
	return true, trace.Wrap(err)
}
