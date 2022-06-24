/*
Copyright 2017-2022 Gravitational, Inc.
Package main contains the enterprise edition of tctl CLI tool.
*/

package main

import (
	"github.com/gravitational/teleport/e/tool/modules"
	"github.com/gravitational/teleport/e/tool/tctl/sso/configure"
	"github.com/gravitational/teleport/e/tool/tctl/sso/tester"
	"github.com/gravitational/teleport/tool/tctl/common"
)

func main() {
	modules.SetModules(nil)

	commands := []common.CLICommand{
		&common.UserCommand{},
		&common.NodeCommand{},
		&common.TokensCommand{},
		&common.AuthCommand{},
		&ResourceCommandE{},
		&common.StatusCommand{},
		&common.TopCommand{},
		&common.AccessRequestCommand{},
		&SAMLCommand{},
		&configure.SSOConfigureCommandE{},
		&tester.SSOTestCommandE{},
		&common.AppsCommand{},
		&common.DBCommand{},
		&common.KubeCommand{},
		&common.DesktopCommand{},
		&common.LockCommand{},
		&common.BotsCommand{},
		&common.InventoryCommand{},
	}
	common.Run(commands)
}
