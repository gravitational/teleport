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

// SPIFFESVIDCommand implements `tbot start spiffe-svid` and
// `tbot configure spiffe-svid`.
type SPIFFESVIDCommand struct {
	*sharedStartArgs
	*sharedDestinationArgs
	*genericMutatorHandler

	IncludeFederatedTrustBundles bool

	SVIDPath string
	SVIDHint string
	DNSSANs  []string
	IPSANs   []string
}

// NewSPIFFESVIDCommand initializes the command and flags for the
// `spiffe-svid` output and returns a struct that will contain the parse
// result.
func NewSPIFFESVIDCommand(parentCmd *kingpin.CmdClause, action MutatorAction, mode CommandMode) *SPIFFESVIDCommand {
	cmd := parentCmd.Command("spiffe-svid", fmt.Sprintf("%s tbot with a SPIFFE-compatible SVID output.", mode)).Hidden()

	c := &SPIFFESVIDCommand{}
	c.sharedStartArgs = newSharedStartArgs(cmd)
	c.sharedDestinationArgs = newSharedDestinationArgs(cmd)
	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	cmd.Flag("include-federated-trust-bundles", "If set, include federated trust bundles in the output").BoolVar(&c.IncludeFederatedTrustBundles)
	cmd.Flag("svid-path", "A SPIFFE ID to request, prefixed with '/'").Required().StringVar(&c.SVIDPath)
	cmd.Flag("svid-hint", "An optional hint for consumers of the SVID to aid in identification").StringVar(&c.SVIDHint)
	cmd.Flag("dns-san", "A DNS name that should be included in the SVID. Repeatable.").StringsVar(&c.DNSSANs)
	cmd.Flag("ip-san", "An IP address that should be included in the SVID. Repeatable.").StringsVar(&c.IPSANs)

	return c
}

func (c *SPIFFESVIDCommand) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	dest, err := c.BuildDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	cfg.Services = append(cfg.Services, &config.SPIFFESVIDOutput{
		Destination: dest,
		SVID: config.SVIDRequest{
			Path: c.SVIDPath,
			Hint: c.SVIDHint,
			SANS: config.SVIDRequestSANs{
				DNS: c.DNSSANs,
				IP:  c.IPSANs,
			},
		},
		IncludeFederatedTrustBundles: c.IncludeFederatedTrustBundles,
	})

	return nil
}
