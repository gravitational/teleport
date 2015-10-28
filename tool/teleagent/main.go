package main

import (
	"os"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/alecthomas/kingpin.v2"

	"github.com/gravitational/teleport/tool/teleagent/teleagent"
)

func main() {
	app := kingpin.New("Agent", "SSH agent for Teleport")
	agentAddr := app.Flag("agent-addr", "Agent listening address").Default(teleagent.DefaultAgentAddress).String()
	apiAddr := app.Flag("api-addr", "Agent API listeng address").Default(teleagent.DefaultAgentAPIAddress).String()

	kingpin.MustParse(app.Parse(os.Args[1:]))

	log.Initialize("console", "INFO")

	agent := teleagent.TeleAgent{}
	apiServer := teleagent.NewAgentAPIServer(&agent)
	if err := agent.Start(*agentAddr); err != nil {
		log.Errorf(err.Error())
		os.Exit(-1)
	}

	if err := apiServer.Start(*apiAddr); err != nil {
		log.Errorf(err.Error())
		os.Exit(-1)
	}

}
