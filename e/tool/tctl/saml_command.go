package main

import (
	"context"
	"fmt"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
)

// implements common.CLICommand interface
type SAMLCommand struct {
	config *service.Config

	exportCmd *kingpin.CmdClause

	// connectorName is passed as a CLI flag
	connectorName string
}

// Initialize allows a caller-defined command to plug itself into CLI
// argument parsing
func (cmd *SAMLCommand) Initialize(app *kingpin.Application, cfg *service.Config) {
	cmd.config = cfg

	saml := app.Command("saml", "Operations on SAML auth connectors")
	cmd.exportCmd = saml.Command("export", "Export a SAML signing key in .crt format")
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

// export executes 'tctl saml export <connector_name'
func (cmd *SAMLCommand) export(ctx context.Context, c auth.ClientI) error {
	sc, err := c.GetSAMLConnector(ctx, cmd.connectorName, false)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println(sc.GetSigningKeyPair().Cert)
	return nil
}
