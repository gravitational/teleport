package main

import (
	"os"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/tool/tctl/command" // TODO(klizhentas) fix the interface for logging
)

func main() {

	log.Initialize("console", "INFO")

	cmd := command.NewCommand()
	err := cmd.Run(os.Args)
	if err != nil {
		log.Infof("error: %s\n", err)
	}
}
