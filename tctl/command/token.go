package command

import (
	"fmt"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codegangsta/cli"
)

func newTokenCommand(c *Command) cli.Command {
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
				},
				Action: c.generateToken,
			},
		},
	}
}

func (cmd *Command) generateToken(c *cli.Context) {
	token, err := cmd.client.GenerateToken(
		c.String("fqdn"), c.Duration("ttl"))
	if err != nil {
		cmd.printError(err)
		return
	}
	fmt.Fprintf(cmd.out, token)
}
