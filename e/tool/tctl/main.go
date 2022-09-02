/*
Copyright 2017-2022 Gravitational, Inc.
Package main contains the enterprise edition of tctl CLI tool.
*/

package main

import (
	"github.com/gravitational/teleport/e/tool/modules"
	"github.com/gravitational/teleport/tool/tctl/common"
)

func main() {
	modules.SetModules(nil)

	// aggregate common and ent-specific commands
	commands := common.Commands()
	commands = append(commands, ENTCommands()...)

	common.Run(commands)
}
