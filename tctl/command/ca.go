package command

import (
	"fmt"

	"github.com/codegangsta/cli"
)

func newHostCACommand(c *Command) cli.Command {
	return cli.Command{
		Name:  "hostca",
		Usage: "Operations with host certificate authority",
		Subcommands: []cli.Command{
			{
				Name:  "reset",
				Usage: "Reset host certificate authority keys",
				Flags: []cli.Flag{
					cli.BoolFlag{Name: "confirm", Usage: "Automatically apply the operation without confirmation"},
				},
				Action: c.resetHostCA,
			},
			{
				Name:   "pubkey",
				Usage:  "print host certificate authority public key",
				Action: c.getHostCAPub,
			},
		},
	}
}

func newUserCACommand(c *Command) cli.Command {
	return cli.Command{
		Name:  "userca",
		Usage: "Operations with user certificate authority",
		Subcommands: []cli.Command{
			{
				Name:  "reset",
				Usage: "Reset user certificate authority keys",
				Flags: []cli.Flag{
					cli.BoolFlag{Name: "confirm", Usage: "Automatically apply the operation without confirmation"},
				},
				Action: c.resetUserCA,
			},
			{
				Name:   "pubkey",
				Usage:  "print user certificate authority public key",
				Action: c.getUserCAPub,
			},
		},
	}
}

func (cmd *Command) resetHostCA(c *cli.Context) {
	if !c.Bool("confirm") && !cmd.confirm("Reseting private and public keys for Host CA. This will invalidate all signed host certs. Continue?") {
		cmd.printError(fmt.Errorf("aborted by user"))
		return
	}
	if err := cmd.client.ResetHostCA(); err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("CA keys have been regenerated")
}

func (cmd *Command) getHostCAPub(c *cli.Context) {
	key, err := cmd.client.GetHostCAPub()
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("Host CA Key")
	fmt.Fprintf(cmd.out, string(key))
}

func (cmd *Command) resetUserCA(c *cli.Context) {
	if !c.Bool("confirm") && !cmd.confirm("Reseting private and public keys for User CA. This will invalidate all signed user certs. Continue?") {
		cmd.printError(fmt.Errorf("aborted by user"))
		return
	}
	if err := cmd.client.ResetUserCA(); err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("CA keys have been regenerated")
}

func (cmd *Command) getUserCAPub(c *cli.Context) {
	key, err := cmd.client.GetUserCAPub()
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("User CA Key")
	fmt.Fprintf(cmd.out, string(key))
}
