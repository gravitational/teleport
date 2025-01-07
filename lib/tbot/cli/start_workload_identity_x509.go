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

	"github.com/gravitational/teleport/lib/tbot/config"
)

// WorkloadIdentityX509Command implements `tbot start workload-identity-x509` and
// `tbot configure spiffe-svid`.
type WorkloadIdentityX509Command struct {
	*sharedStartArgs
	*sharedDestinationArgs
	*genericMutatorHandler

	IncludeFederatedTrustBundles bool
}

// NewWorkloadIdentityX509Command initializes the command and flags for the
// `workload-identity-x509` output and returns a struct that will contain the parse
// result.
func NewWorkloadIdentityX509Command(parentCmd *kingpin.CmdClause, action MutatorAction, mode CommandMode) *WorkloadIdentityX509Command {
	cmd := parentCmd.Command("workload-identity-x509", fmt.Sprintf("%s tbot with a SPIFFE-compatible SVID output.", mode)).Hidden()

	c := &WorkloadIdentityX509Command{}
	c.sharedStartArgs = newSharedStartArgs(cmd)
	c.sharedDestinationArgs = newSharedDestinationArgs(cmd)
	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	cmd.Flag("include-federated-trust-bundles", "If set, include federated trust bundles in the output").BoolVar(&c.IncludeFederatedTrustBundles)

	return c
}

func (c *WorkloadIdentityX509Command) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	dest, err := c.BuildDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	cfg.Services = append(cfg.Services, &config.WorkloadIdentityX509Service{
		Destination: dest,

		IncludeFederatedTrustBundles: c.IncludeFederatedTrustBundles,
	})

	return nil
}
