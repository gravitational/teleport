/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

	"github.com/gravitational/teleport/lib/tbot/config"
)

// KubernetesCommand implements `tbot start kubernetes` and
// `tbot configure kubernetes`.
type KubernetesCommand struct {
	*sharedStartArgs
	*sharedDestinationArgs
	*genericMutatorHandler

	KubernetesCluster string
	DisableExecPlugin bool
}

// NewKubernetesCommand initializes the command and flags for kubernetes outputs
// and returns a struct to contain the parse result.
func NewKubernetesCommand(parentCmd *kingpin.CmdClause, action MutatorAction, mode CommandMode) *KubernetesCommand {
	cmd := parentCmd.Command("kubernetes", fmt.Sprintf("%s tbot with a Kubernetes output.", mode)).Alias("k8s")

	c := &KubernetesCommand{}
	c.sharedStartArgs = newSharedStartArgs(cmd)
	c.sharedDestinationArgs = newSharedDestinationArgs(cmd)
	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	cmd.Flag("kubernetes-cluster", "The name of the Kubernetes cluster in Teleport for which to fetch credentials").Required().StringVar(&c.KubernetesCluster)
	cmd.Flag("disable-exec-plugin", "If set, disables the exec plugin. This allows credentials to be used without the `tbot` binary.").BoolVar(&c.DisableExecPlugin)

	// Note: excluding roles; the bot will fetch all available in CLI mode.

	return c
}

func (c *KubernetesCommand) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	dest, err := c.BuildDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	cfg.Services = append(cfg.Services, &config.KubernetesOutput{
		Destination:       dest,
		KubernetesCluster: c.KubernetesCluster,
		DisableExecPlugin: c.DisableExecPlugin,
	})

	return nil
}
