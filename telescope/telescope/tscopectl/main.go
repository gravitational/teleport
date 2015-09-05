package main

import (
	"os"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/tctl/command"
	ctl "github.com/gravitational/teleport/telescope/telescope/tscopectl/command"
)

func main() {

	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

	cmd := command.NewCommand()
	err := ctl.RunCmd(cmd, os.Args)
	if err != nil {
		log.Infof("error: %s\n", err)
	}
}
