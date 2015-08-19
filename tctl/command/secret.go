package command

import (
	"fmt"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
)

/*func newSecretCommand(c *Command) cli.Command {
	return cli.Command{
		Name:  "secret",
		Usage: "Operations with secret tokens",
		Subcommands: []cli.Command{
			{
				Name:   "new",
				Usage:  "Generate new secret key",
				Action: c.newKey,
			},
		},
	}
}*/

func (cmd *Command) newKey() {
	key, err := secret.NewKey()
	if err != nil {
		cmd.printError(fmt.Errorf("failed to generate key: %v", err))
		return
	}
	fmt.Fprintf(cmd.out, "%v\n", secret.KeyToEncodedString(key))
}
