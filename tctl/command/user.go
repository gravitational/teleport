package command

import (
	"fmt"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/buger/goterm"
	"github.com/gravitational/teleport/services"
)

func (cmd *Command) setPass(user, pass string) {
	err := cmd.client.UpsertPassword(user, []byte(pass))
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("password has been set for user '%v'", user)
}

func (cmd *Command) upsertKey(user, keyID, key string, ttl time.Duration) {
	bytes, err := cmd.readInput(key)
	if err != nil {
		cmd.printError(err)
		return
	}
	signed, err := cmd.client.UpsertUserKey(
		user, services.AuthorizedKey{ID: keyID, Value: bytes}, ttl)
	if err != nil {
		cmd.printError(err)
		return
	}
	fmt.Fprintf(cmd.out, "%v", string(signed))
}

func (cmd *Command) deleteUser(user string) {
	if err := cmd.client.DeleteUser(user); err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("User %v deleted", user)
}

func (cmd *Command) getUsers() {
	users, err := cmd.client.GetUsers()
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("Users")
	fmt.Fprintf(cmd.out, usersView(users))
}

func (cmd *Command) getUserKeys(user string) {
	keys, err := cmd.client.GetUserKeys(user)
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

func keysView(keys []services.AuthorizedKey) string {
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
