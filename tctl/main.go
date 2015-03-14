package main

import (
	"os"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/tctl/command" // TODO(klizhentas) fix the interface for logging
)

func main() {

	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

	cmd := command.NewCommand()
	err := cmd.Run(os.Args)
	if err != nil {
		log.Infof("error: %s\n", err)
	}
}
