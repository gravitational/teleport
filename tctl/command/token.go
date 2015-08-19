package command

import (
	"fmt"
	"io/ioutil"
	"time"
)

func (cmd *Command) generateToken(fqdn string, ttl time.Duration,
	output string) {

	token, err := cmd.client.GenerateToken(fqdn, ttl)
	if err != nil {
		cmd.printError(err)
		return
	}
	if output == "" {
		fmt.Fprintf(cmd.out, token)
		return
	}
	err = ioutil.WriteFile(output, []byte(token), 0644)
	if err != nil {
		cmd.printError(err)
	}
}
