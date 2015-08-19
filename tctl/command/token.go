package command

import (
	"fmt"
	"io/ioutil"
	"time"
)

/*func newTokenCommand(c *Command) cli.Command {
	return cli.Command{
		Name:  "token",
		Usage: "Generates provisioning tokens",
		Subcommands: []cli.Command{
			{
				Name:  "generate",
				Usage: "Generate provisioning token for server with fqdn",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "fqdn", Usage: "FQDN of the server"},
					cli.DurationFlag{Name: "ttl", Value: 120 * time.Second, Usage: "TTL"},
					cli.StringFlag{Name: "output", Usage: "Optional output file"},
				},
				Action: c.generateToken,
			},
		},
	}
}*/

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
