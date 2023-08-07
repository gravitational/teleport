// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"context"
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service/servicecfg"
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
func (cmd *SAMLCommand) Initialize(app *kingpin.Application, cfg *servicecfg.Config) {
	cmd.config = cfg

	saml := app.Command("saml", "Operations on SAML auth connectors.")
	cmd.exportCmd = saml.Command("export", "Export a SAML signing key in .crt format.")
	cmd.exportCmd.Arg("connector_name", "name of the SAML connector to export the key from").Required().StringVar(&cmd.connectorName)
}

// TryRun is executed after the CLI parsing is done. The command must
// determine if selectedCommand belongs to it and return match=true
func (cmd *SAMLCommand) TryRun(ctx context.Context, selectedCommand string, c auth.ClientI) (match bool, err error) {
	if selectedCommand == cmd.exportCmd.FullCommand() {
		return true, trace.Wrap(cmd.export(ctx, c))
	}
	return false, nil
}

// export executes 'tctl saml export <connector_name>'
func (cmd *SAMLCommand) export(ctx context.Context, c auth.ClientI) error {
	sc, err := c.GetSAMLConnector(ctx, cmd.connectorName, false)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println(sc.GetSigningKeyPair().Cert)
	return nil
}
