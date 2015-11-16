/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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

	fmt.Fprintf(cmd.out, "Logging in...\n")

	err = teleagent.Login(agentAddr, proxyAddr, string(user), password,
		hotpToken, ttl)
	if err != nil {
		cmd.printError(err)
		return
	}

	cmd.printOK("Logged in successfully")
}

func (cmd *Command) AgentStart(agentAddr string, apiAddr string) {
	agent := teleagent.NewTeleAgent()
	apiServer := teleagent.NewAgentAPIServer(agent)
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
