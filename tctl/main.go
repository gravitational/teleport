package main

import (
	"os"

	"github.com/mailgun/log" // TODO(klizhentas) fix the interface for logging
	"github.com/gravitational/teleport/tctl/command"
)

func main() {

	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

	cmd := command.NewCommand()
	err := cmd.Run(os.Args)
	if err != nil {
		log.Infof("error: %s\n", err)
	}
}
