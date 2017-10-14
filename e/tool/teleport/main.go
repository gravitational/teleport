package main

import (
	"os"

	"github.com/gravitational/teleport/e/tool/plugins"

	"github.com/gravitational/teleport/tool/teleport/common"
)

func main() {
	plugins.SetPlugins()
	const testRun = false
	common.Run(os.Args[1:], testRun)
}
