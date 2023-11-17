package main

import (
	"github.com/gravitational/teleport/e/tool/tctl/accessmonitoring"
	"github.com/gravitational/teleport/e/tool/tctl/sso/configure"
	"github.com/gravitational/teleport/e/tool/tctl/sso/tester"
	"github.com/gravitational/teleport/tool/tctl/common"
)

// ENTCommands returns the ent variants of commands that use different variants
// for oss and ent, and commands that are unique to ent.
func ENTCommands() []common.CLICommand {
	return []common.CLICommand{
		&configure.SSOConfigureCommandE{},
		&tester.SSOTestCommandE{},
		&accessmonitoring.Command{},
	}
}
