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
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// implements common.CLICommand interface
type SAMLCommand struct {
	config *servicecfg.Config

	exportCmd *kingpin.CmdClause

	// connectorName is passed as a CLI flag
	connectorName string
}

// Initialize allows a caller-defined command to plug itself into CLI
// argument parsing
func (cmd *SAMLCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, cfg *servicecfg.Config) {
	cmd.config = cfg

	saml := app.Command("saml", "Operations on SAML auth connectors.")
	cmd.exportCmd = saml.Command("export", "Export a SAML signing key in .crt format.")
	cmd.exportCmd.Arg("connector_name", "name of the SAML connector to export the key from").Required().StringVar(&cmd.connectorName)
}

// TryRun is executed after the CLI parsing is done. The command must
// determine if selectedCommand belongs to it and return match=true
func (cmd *SAMLCommand) TryRun(ctx context.Context, selectedCommand string, clientFunc commonclient.InitFunc) (match bool, err error) {
	if selectedCommand == cmd.exportCmd.FullCommand() {
		client, closeFn, err := clientFunc(ctx)
		if err != nil {
			return false, trace.Wrap(err)
		}
		defer closeFn(ctx)
		return true, trace.Wrap(cmd.export(ctx, client))
	}
	return false, nil
}

// export executes 'tctl saml export <connector_name>'
func (cmd *SAMLCommand) export(ctx context.Context, c *authclient.Client) error {
	sc, err := c.GetSAMLConnector(ctx, cmd.connectorName, false)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println(sc.GetSigningKeyPair().Cert)
	return nil
}
