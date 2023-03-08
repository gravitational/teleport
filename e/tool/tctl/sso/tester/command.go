package tester

import (
	"context"

	"github.com/gravitational/kingpin"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/tool/tctl/sso/tester"
)

// SSOTestCommandE implements common.CLICommand interface
type SSOTestCommandE struct {
	base tester.SSOTestCommand
}

// Initialize allows a caller-defined command to plug itself into CLI
// argument parsing
func (cmd *SSOTestCommandE) Initialize(app *kingpin.Application, cfg *servicecfg.Config) {
	cmd.base.Initialize(app, cfg)

	cmd.base.Handlers[types.KindSAMLConnector] = handleSAMLConnector
	cmd.base.Handlers[types.KindOIDCConnector] = handleOIDCConnector

	cmd.base.GetDiagInfoFields[types.KindSAMLConnector] = getInfoFieldsSAML
	cmd.base.GetDiagInfoFields[types.KindOIDCConnector] = getInfoFieldsOIDC
}

// TryRun is executed after the CLI parsing is done. The command must
// determine if selectedCommand belongs to it and return match=true
func (cmd *SSOTestCommandE) TryRun(ctx context.Context, selectedCommand string, c auth.ClientI) (match bool, err error) {
	return cmd.base.TryRun(ctx, selectedCommand, c)
}
