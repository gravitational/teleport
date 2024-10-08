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
	"log/slog"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot/config"
)

// SPIFFEX509SVIDCommand implements `tbot start spiffe-x509-svid` and
// `tbot configure spiffe-x509-svid`.
type SPIFFEX509SVIDCommand struct {
	*sharedStartArgs
	*genericMutatorHandler

	Destination                  string
	IncludeFederatedTrustBundles bool

	SVIDPath string
	SVIDHint string
	DNSSANs  []string
	IPSANs   []string
}

func NewSPIFFEX509SVIDCommand(parentCmd *kingpin.CmdClause, action MutatorAction) *SPIFFEX509SVIDCommand {
	cmd := parentCmd.Command("spiffe-x509-svid", "Starts with a SPIFFE-compatible X509 SVID output")

	c := &SPIFFEX509SVIDCommand{}
	c.sharedStartArgs = newSharedStartArgs(cmd)
	c.genericMutatorHandler = newGenericMutatorHandler(cmd, c, action)

	cmd.Flag("destination", "A destination URI, such as file:///foo/bar").Required().StringVar(&c.Destination)
	cmd.Flag("include-federated-trust-bundles", "If set, include federated trust bundles in the output").BoolVar(&c.IncludeFederatedTrustBundles)
	cmd.Flag("svid-path", "A SPIFFE ID to request, prefixed with '/'").Required().StringVar(&c.SVIDPath)
	cmd.Flag("svid-hint", "An optional hint for consumers of the SVID to aid in identification").StringVar(&c.SVIDHint)
	cmd.Flag("request-dns-san", "A DNS name that should be included in the SVID. Repeatable.").StringsVar(&c.DNSSANs)
	cmd.Flag("request-ip-san", "An IP address that should be included in the SVID. Repeatable.").StringsVar(&c.IPSANs)

	return c
}

func (c *SPIFFEX509SVIDCommand) ApplyConfig(cfg *config.BotConfig, l *slog.Logger) error {
	if err := c.sharedStartArgs.ApplyConfig(cfg, l); err != nil {
		return trace.Wrap(err)
	}

	dest, err := config.DestinationFromURI(c.Destination)
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
