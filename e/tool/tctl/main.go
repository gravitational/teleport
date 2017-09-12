/*
Copyright 2017 Gravitational, Inc.
Package main contains the enterprise edition of tctl CLI tool.
*/

package main

import (
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/tool/tctl/common"
)

func main() {
	commands := []common.CLICommand{
		&UserCommandE{},
		&common.NodeCommand{},
		&common.TokenCommand{},
		&common.AuthCommand{},
		&ResourceCommandE{},
		&SAMLCommand{},
	}
	common.Run(teleport.DistroTypeEnterprise, commands)
}
