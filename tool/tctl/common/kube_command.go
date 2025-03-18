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
	"text/template"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// KubeCommand implements "tctl kube" group of commands.
type KubeCommand struct {
	config *servicecfg.Config

	// format is the output format (text or yaml)
	format string

	searchKeywords string
	predicateExpr  string
	labels         string

	// verbose sets whether full table output should be shown for labels
	verbose bool

	// kubeList implements the "tctl kube ls" subcommand.
	kubeList *kingpin.CmdClause
}

// Initialize allows KubeCommand to plug itself into the CLI parser
func (c *KubeCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config

	kube := app.Command("kube", "Operate on registered Kubernetes clusters.")
	c.kubeList = kube.Command("ls", "List all Kubernetes clusters registered with the cluster.")
	c.kubeList.Arg("labels", labelHelp).StringVar(&c.labels)
	c.kubeList.Flag("format", "Output format, 'text', 'json', or 'yaml'").Default(teleport.Text).StringVar(&c.format)
	c.kubeList.Flag("verbose", "Verbose table output, shows full label output").Short('v').BoolVar(&c.verbose)
	c.kubeList.Flag("search", searchHelp).StringVar(&c.searchKeywords)
	c.kubeList.Flag("query", queryHelp).StringVar(&c.predicateExpr)
}

// TryRun attempts to run subcommands like "kube ls".
func (c *KubeCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.kubeList.FullCommand():
		commandFunc = c.ListKube
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

// ListKube prints the list of kube clusters that have recently sent heartbeats
// to the cluster.
func (c *KubeCommand) ListKube(ctx context.Context, clt *authclient.Client) error {
	labels, err := libclient.ParseLabelSpec(c.labels)
	if err != nil {
		return trace.Wrap(err)
	}

	kubes, err := client.GetAllResources[types.KubeServer](ctx, clt, &proto.ListResourcesRequest{
		ResourceType:        types.KindKubeServer,
		Labels:              labels,
		PredicateExpression: c.predicateExpr,
		SearchKeywords:      libclient.ParseSearchKeywords(c.searchKeywords, ','),
	})
	if err != nil {
		if utils.IsPredicateError(err) {
			return trace.Wrap(utils.PredicateError{Err: err})
		}
		return trace.Wrap(err)
	}

	coll := &kubeServerCollection{servers: kubes}
	switch c.format {
	case teleport.Text:
		return trace.Wrap(coll.writeText(os.Stdout, c.verbose))
	case teleport.JSON:
		return trace.Wrap(coll.writeJSON(os.Stdout))
	case teleport.YAML:
		return trace.Wrap(coll.writeYAML(os.Stdout))
	default:
		return trace.BadParameter("unknown format %q", c.format)
	}
}

var kubeMessageTemplate = template.Must(template.New("kube").Parse(`The invite token: {{.token}}
This token will expire in {{.minutes}} minutes.

To use with Helm installation follow these steps:

# Retrieve the Teleport helm charts
helm repo add teleport https://charts.releases.teleport.dev
# Refresh the helm charts
helm repo update

> helm install teleport-agent teleport/teleport-kube-agent \
  --set kubeClusterName=cluster ` + "`" + `# Change kubeClusterName variable to your preferred name.` + "`" + ` \
  --set roles="{{.set_roles}}" \
  --set proxyAddr={{.auth_server}} \
  --set authToken={{.token}} \
  --create-namespace \
  --namespace=teleport-agent \
  --version={{.version}}

Please note:

  - This invitation token will expire in {{.minutes}} minutes.
  - {{.auth_server}} must be reachable from Kubernetes cluster.
  - The token is usable in a standalone Linux server with kubernetes_service.
  - See https://goteleport.com/docs/kubernetes-access/ for detailed installation information.

`))
