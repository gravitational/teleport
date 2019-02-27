/*
Copyright 2017-2019 Gravitational, Inc.
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
		&UserCommandE{},
		&common.NodeCommand{},
		&common.TokenCommand{},
		&common.AuthCommand{},
		&common.StatusCommand{},
		&common.TopCommand{},
		&ResourceCommandE{},
		&SAMLCommand{},
	}
	common.Run(commands)
}
