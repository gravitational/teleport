/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"
	"os"
	"text/template"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
)

// KubeCommand implements "tctl kube" group of commands.
type KubeCommand struct {
	config *service.Config

	// format is the output format (text or yaml)
	format string

	// verbose sets whether full table output should be shown for labels
	verbose bool

	// kubeList implements the "tctl kube ls" subcommand.
	kubeList *kingpin.CmdClause
}

// Initialize allows KubeCommand to plug itself into the CLI parser
func (c *KubeCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config

	kube := app.Command("kube", "Operate on registered Kubernetes clusters.")
	c.kubeList = kube.Command("ls", "List all Kubernetes clusters registered with the cluster.")
	c.kubeList.Flag("format", "Output format, 'text', 'json', or 'yaml'").Default(teleport.Text).StringVar(&c.format)
	c.kubeList.Flag("verbose", "Verbose table output, shows full label output").Short('v').BoolVar(&c.verbose)
}

// TryRun attempts to run subcommands like "kube ls".
func (c *KubeCommand) TryRun(ctx context.Context, cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.kubeList.FullCommand():
		err = c.ListKube(ctx, client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

// ListKube prints the list of kube clusters that have recently sent heartbeats
// to the cluster.
func (c *KubeCommand) ListKube(ctx context.Context, client auth.ClientI) error {

	kubes, err := client.GetKubernetesServers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	coll := &kubeServerCollection{servers: kubes, verbose: c.verbose}
	switch c.format {
	case teleport.Text:
		return trace.Wrap(coll.writeText(os.Stdout))
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
  --set proxyAddr={{.auth_server}} \
  --set authToken={{.token}} \
  --create-namespace \
  --namespace=teleport-agent
        
Please note:
        
  - This invitation token will expire in {{.minutes}} minutes.
  - {{.auth_server}} must be reachable from Kubernetes cluster.
  - The token is usable in a standalone Linux server with kubernetes_service.
  - See https://goteleport.com/docs/kubernetes-access/ for detailed installation information.

`))
