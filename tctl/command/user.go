package command

import (
	"fmt"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/buger/goterm"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/gravitational/teleport/backend"
)

func newUserCommand(c *Command) cli.Command {
	return cli.Command{
		Name:  "user",
		Usage: "Operations with registered users",
		Subcommands: []cli.Command{
			{
				Name:   "ls",
				Usage:  "List users registered in teleport",
				Action: c.getUsers,
			},
			{
				Name:   "delete",
				Usage:  "Delete user",
				Action: c.deleteUser,
				Flags: []cli.Flag{
					cli.StringFlag{Name: "user", Usage: "User to delete"},
				},
			},
			{
				Name:  "upsert_key",
				Usage: "Grant access to the user key, returns signed certificate",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "user", Usage: "User holding the key"},
					cli.StringFlag{Name: "keyid", Usage: "SSH key ID"},
					cli.StringFlag{Name: "key", Usage: "Path to public key"},
					cli.DurationFlag{Name: "ttl", Usage: "Access time to live, certificate and access entry will expire when set"},
				},
				Action: c.upsertKey,
			},
			{
				Name:   "ls_keys",
				Usage:  "List user's keys registered in teleport",
				Action: c.getUserKeys,
				Flags: []cli.Flag{
					cli.StringFlag{Name: "user", Usage: "User to list keys form"},
				},
			},
		},
	}
}

func (cmd *Command) upsertKey(c *cli.Context) {
	bytes, err := cmd.readInput(c.String("key"))
	if err != nil {
		cmd.printError(err)
		return
	}
	signed, err := cmd.client.UpsertUserKey(
		c.String("user"), backend.AuthorizedKey{ID: c.String("keyid"), Value: bytes}, c.Duration("ttl"))
	if err != nil {
		cmd.printError(err)
		return
	}
	fmt.Fprintf(cmd.out, "certificate:\n%v", string(signed))
}

func (cmd *Command) deleteUser(c *cli.Context) {
	if err := cmd.client.DeleteUser(c.String("user")); err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("User %v deleted", c.String("user"))
}

func (cmd *Command) getUsers(c *cli.Context) {
	users, err := cmd.client.GetUsers()
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("Users")
	fmt.Fprintf(cmd.out, usersView(users))
}

func (cmd *Command) getUserKeys(c *cli.Context) {
	keys, err := cmd.client.GetUserKeys(c.String("user"))
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("Users")
	fmt.Fprintf(cmd.out, keysView(keys))
}

func usersView(users []string) string {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	fmt.Fprint(t, "User\n")
	if len(users) == 0 {
		return t.String()
	}
	for _, u := range users {
		fmt.Fprintf(t, "%v\n", u)
	}
	return t.String()
}

func keysView(keys []backend.AuthorizedKey) string {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	fmt.Fprint(t, "KeyID\tKey\n")
	if len(keys) == 0 {
		return t.String()
	}
	for _, k := range keys {
		fmt.Fprintf(t, "%v\t%v\n", k.ID, string(k.Value))
	}
	return t.String()
}
