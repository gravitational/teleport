package command

import (
	"fmt"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/lib/teleagent"
)

func (cmd *Command) AgentLogin(agentAddr string, proxyAddr string, ttl time.Duration) {

	fmt.Fprintf(cmd.out, "Enter your user name:\n")
	user, err := cmd.readInput("")
	if err != nil {
		cmd.printError(err)
		return
	}

	user = user[:len(user)-1]

	fmt.Fprintf(cmd.out, "Enter your password:\n")
	password, err := cmd.readPassword()
	if err != nil {
		cmd.printError(err)
		return
	}

	fmt.Fprintf(cmd.out, "Enter your HOTP token:\n")
	hotpToken, err := cmd.readPassword()
	if err != nil {
		cmd.printError(err)
		return
	}

	fmt.Println(string(user), password, hotpToken)

	err = teleagent.Login(agentAddr, proxyAddr, string(user), password,
		hotpToken, ttl)
	if err != nil {
		cmd.printError(err)
		return
	}
}

func (cmd *Command) AgentStart(agentAddr string, apiAddr string) {
	agent := teleagent.TeleAgent{}
	apiServer := teleagent.NewAgentAPIServer(&agent)
	if err := agent.Start(agentAddr); err != nil {
		cmd.printError(trace.Wrap(err))
		return
	}

	fmt.Fprintf(cmd.out, "Agent started")

	if err := apiServer.Start(apiAddr); err != nil {
		cmd.printError(trace.Wrap(err))
		return
	}
}
