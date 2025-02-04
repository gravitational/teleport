/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package cli

import (
	"fmt"
	"log/slog"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tbot/config"
)

// KubernetesV2Command implements `tbot start kubernetes` and
// `tbot configure kubernetes`.
type KubernetesV2Command struct {
	*sharedStartArgs
	*sharedDestinationArgs
	*genericMutatorHandler

	DisableExecPlugin      bool
	KubernetesClusterNames []string

	// KubernetesClusterLabels contains a list of strings representing label
	// selectors. Each entry generates one selector, but may contain several
	// comma-separated strings to match multiple labels at once.
	KubernetesClusterLabels []string
}

// NewKubernetesCommand initializes the command and flags for kubernetes outputs
// and returns a struct to contain the parse result.
func NewKubernetesV2Command(parentCmd *kingpin.CmdClause, action MutatorAction, mode CommandMode) *KubernetesV2Command {
	cmd := parentCmd.Command("kubernetes/v2", fmt.Sprintf("%s tbot with a Kubernetes V2 output.", mode)).Alias("k8s/v2")

	c := &KubernetesV2Command{}
	c.sharedStartArgs = newSharedStartArgs(cmd)
	c.sharedDestinationArgs = newSharedDestinationArgs(cmd)
	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	cmd.Flag("disable-exec-plugin", "If set, disables the exec plugin. This allows credentials to be used without the `tbot` binary.").BoolVar(&c.DisableExecPlugin)
	cmd.Flag("name-selector", "An explicit Kubernetes cluster name to include. Repeatable.").StringsVar(&c.KubernetesClusterNames)
	cmd.Flag("label-selector", "A set of Kubernetes labels to match in k1=v1,k2=v2 form. Repeatable.").StringsVar(&c.KubernetesClusterLabels)

	return c
}

func (c *KubernetesV2Command) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	dest, err := c.BuildDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	selectors := []*config.KubernetesSelector{}
	for _, name := range c.KubernetesClusterNames {
		selectors = append(selectors, &config.KubernetesSelector{
			Name: name,
		})
	}

	for _, s := range c.KubernetesClusterLabels {
		labels, err := client.ParseLabelSpec(s)
		if err != nil {
			return trace.Wrap(err)
		}

		selectors = append(selectors, &config.KubernetesSelector{
			Labels: labels,
		})
	}

	if len(selectors) == 0 {
		return trace.BadParameter("at least one name-selector or label-selector must be provided")
	}

	cfg.Services = append(cfg.Services, &config.KubernetesV2Output{
		Destination:       dest,
		DisableExecPlugin: c.DisableExecPlugin,
		Selectors:         selectors,
	})

	return nil
}
