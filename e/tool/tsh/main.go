package main

import (
	"github.com/gravitational/teleport/tool/tsh/common"
)

import (
	"os"
)

func main() {
	common.Run(os.Args[1:], false)
}
