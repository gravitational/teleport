/*
Copyright 2017-2021 Gravitational, Inc.
Package main contains the enterprise edition of tctl CLI tool.
*/

package main

import (
	"github.com/gravitational/teleport/e/tool/modules"

	"github.com/gravitational/teleport/tool/tctl/common"
)

func main() {
	modules.SetModules(nil)
	commands := []common.CLICommand{
		&common.UserCommand{},
		&common.NodeCommand{},
		&common.TokenCommand{},
		&common.AuthCommand{},
		&common.StatusCommand{},
		&common.TopCommand{},
		&common.AccessRequestCommand{},
		&ResourceCommandE{},
		&SAMLCommand{},
		&common.AppsCommand{},
		&common.DBCommand{},
		&common.LockCommand{},
		&common.AccessCommand{},
	}
	common.Run(commands)
}
