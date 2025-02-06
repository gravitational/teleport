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

package cli

import (
	"fmt"
	"log/slog"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tbot/config"
)

// WorkloadIdentityX509Command implements `tbot start workload-identity-x509` and
// `tbot configure spiffe-svid`.
type WorkloadIdentityX509Command struct {
	*sharedStartArgs
	*sharedDestinationArgs
	*genericMutatorHandler

	IncludeFederatedTrustBundles bool
	// NameSelector is the name of the workload identity to use.
	// --workload-identity-name foo
	NameSelector string
	// LabelSelector is the labels of the workload identity to use.
	// --workload-identity-labels x=y,z=a
	LabelSelector string
}

// NewWorkloadIdentityX509Command initializes the command and flags for the
// `workload-identity-x509` output and returns a struct that will contain the parse
// result.
func NewWorkloadIdentityX509Command(parentCmd *kingpin.CmdClause, action MutatorAction, mode CommandMode) *WorkloadIdentityX509Command {
	// TODO(noah): Unhide this command when feature flag removed
	cmd := parentCmd.Command("workload-identity-x509", fmt.Sprintf("%s tbot with a SPIFFE-compatible SVID output.", mode))

	c := &WorkloadIdentityX509Command{}
	c.sharedStartArgs = newSharedStartArgs(cmd)
	c.sharedDestinationArgs = newSharedDestinationArgs(cmd)
	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	cmd.Flag(
		"include-federated-trust-bundles",
		"If set, include federated trust bundles in the output",
	).BoolVar(&c.IncludeFederatedTrustBundles)
	cmd.Flag(
		"name-selector",
		"The name of the workload identity to issue",
	).StringVar(&c.NameSelector)
	cmd.Flag(
		"label-selector",
		"A label-based selector for which workload identities to issue. Multiple labels can be provided using ','.",
	).StringVar(&c.LabelSelector)

	return c
}

// ApplyConfig applies the parsed flags to the bot configuration.
func (c *WorkloadIdentityX509Command) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	dest, err := c.BuildDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	svc := &config.WorkloadIdentityX509Service{
		Destination:                  dest,
		IncludeFederatedTrustBundles: c.IncludeFederatedTrustBundles,
	}

	switch {
	case c.NameSelector != "" && c.LabelSelector != "":
		return trace.BadParameter("name-selector and label-selector flags are mutually exclusive")
	case c.NameSelector != "":
		svc.Selector.Name = c.NameSelector
	case c.LabelSelector != "":
		labels, err := client.ParseLabelSpec(c.LabelSelector)
		if err != nil {
			return trace.Wrap(err, "parsing --label-selector")
		}
		svc.Selector.Labels = map[string][]string{}
		for k, v := range labels {
			svc.Selector.Labels[k] = []string{v}
		}
	default:
		return trace.BadParameter("name-selector or label-selector must be specified")
	}

	cfg.Services = append(cfg.Services, svc)

	return nil
}
