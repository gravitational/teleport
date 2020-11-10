// +build gofuzz

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

func FuzzNewExpression(data []byte) int {
	_, err := parse.NewExpression(string(data))
	if err != nil {
		return 0
	}
	return 1
}
