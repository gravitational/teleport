package configure

import (
	"context"

	"github.com/gravitational/kingpin"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/tool/tctl/sso/configure"
)

// SSOConfigureCommandE implements common.CLICommand interface.
// The enterprise version adds support for SAML and OIDC.
type SSOConfigureCommandE struct {
	base configure.SSOConfigureCommand
}

// Initialize allows a caller-defined command to plug itself into CLI
// argument parsing
func (cmd *SSOConfigureCommandE) Initialize(app *kingpin.Application, cfg *service.Config) {
	cmd.base.Initialize(app, cfg)
	cmd.base.AuthCommands = append(cmd.base.AuthCommands, addSAMLCommand(&cmd.base))
	cmd.base.AuthCommands = append(cmd.base.AuthCommands, addOIDCCommand(&cmd.base))
}

// TryRun is executed after the CLI parsing is done. The command must
// determine if selectedCommand belongs to it and return match=true
func (cmd *SSOConfigureCommandE) TryRun(ctx context.Context, selectedCommand string, clt auth.ClientI) (match bool, err error) {
	return cmd.base.TryRun(ctx, selectedCommand, clt)
}
