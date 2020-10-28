package fuzz

import (
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"
)

func FuzzParseProxyJump(data []byte) int {
	_, err := utils.ParseProxyJump(string(data))
	if err != nil {
		return 0
	}
	return 1
}

func FuzzParseRoleVariable(data []byte) int {
	_, err := parse.RoleVariable(string(data))
	if err != nil {
		return 0
	}
	return 1
}
