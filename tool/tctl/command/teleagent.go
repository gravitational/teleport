package command

import (
	"fmt"
	"time"

	"github.com/gravitational/teleport/tool/teleagent/teleagent"
)

func (cmd *Command) TeleagentLogin(agentAddr string, proxyAddr string, ttl time.Duration) {

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

	cmd.printOK("Logged in successfully")
}
