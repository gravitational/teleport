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

	"github.com/gravitational/kingpin"
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

	kube := app.Command("kube", "Operate on registered kubernetes clusters.")
	c.kubeList = kube.Command("ls", "List all kubernetes clusters registered with the cluster.")
	c.kubeList.Flag("format", "Output format, 'text', or 'yaml'").Default("text").StringVar(&c.format)
	c.kubeList.Flag("verbose", "Verbose table output, shows full label output").Short('v').BoolVar(&c.verbose)
}

// TryRun attempts to run subcommands like "kube ls".
func (c *KubeCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.kubeList.FullCommand():
		err = c.ListKube(client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

// ListKube prints the list of kube clusters that have recently sent heartbeats
// to the cluster.
func (c *KubeCommand) ListKube(client auth.ClientI) error {
	kubes, err := client.GetKubeServices(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}
	coll := &kubeServerCollection{servers: kubes}
	switch c.format {
	case teleport.Text:
		err = coll.writeText(c.verbose, os.Stdout)
	case teleport.YAML:
		err = coll.writeYAML(os.Stdout)
	default:
		return trace.BadParameter("unknown format %q", c.format)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
