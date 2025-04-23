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

// WorkloadIdentityJWTCommand implements `tbot start workload-identity-jwt` and
// `tbot configure workload-identity-jwt`.
type WorkloadIdentityJWTCommand struct {
	*sharedStartArgs
	*sharedDestinationArgs
	*genericMutatorHandler

	// NameSelector is the name of the workload identity to use.
	// --workload-identity-name foo
	NameSelector string
	// LabelSelector is the labels of the workload identity to use.
	// --workload-identity-labels x=y,z=a
	LabelSelector string

	// Audiences is the list of audiences to include in the JWT.
	Audiences []string
}

// NewWorkloadIdentityJWTCommand initializes the command and flags for the
// `workload-identity-jwt` output and returns a struct that will contain the parse
// result.
func NewWorkloadIdentityJWTCommand(parentCmd *kingpin.CmdClause, action MutatorAction, mode CommandMode) *WorkloadIdentityJWTCommand {
	cmd := parentCmd.Command("workload-identity-jwt", fmt.Sprintf("%s tbot with a SPIFFE-compatible JWT SVID output.", mode))

	c := &WorkloadIdentityJWTCommand{}
	c.sharedStartArgs = newSharedStartArgs(cmd)
	c.sharedDestinationArgs = newSharedDestinationArgs(cmd)
	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	cmd.Flag(
		"name-selector",
		"The name of the workload identity to issue",
	).StringVar(&c.NameSelector)
	cmd.Flag(
		"label-selector",
		"A label-based selector for which workload identities to issue. Multiple labels can be provided using ','.",
	).StringVar(&c.LabelSelector)
	cmd.Flag(
		"audience",
		"Specify the audiences to include in the JWT. At least one audience must be specified.",
	).Required().StringsVar(&c.Audiences)

	return c
}

// ApplyConfig applies the parsed flags to the bot configuration.
func (c *WorkloadIdentityJWTCommand) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	dest, err := c.BuildDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	svc := &config.WorkloadIdentityJWTService{
		Destination: dest,
		Audiences:   c.Audiences,
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
