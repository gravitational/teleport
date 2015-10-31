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
	"io/ioutil"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/buger/goterm"
	"github.com/gravitational/teleport/lib/services"
)

func (cmd *Command) SetPass(user, pass string) {
	hotpURL, hotpQR, err := cmd.client.UpsertPassword(user, []byte(pass))
	if err != nil {
		cmd.printError(err)
		return
	}
	qrPath := "QR.png"
	err = ioutil.WriteFile(qrPath, hotpQR, 0666)
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK(
		"password has been set for user '%v', token: %v, QR token: %v",
		user, hotpURL, qrPath)
}

func (cmd *Command) UpsertKey(user, keyID, key string, ttl time.Duration) {
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

func (cmd *Command) DeleteUser(user string) {
	if err := cmd.client.DeleteUser(user); err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("User %v deleted", user)
}

func (cmd *Command) GetUsers() {
	users, err := cmd.client.GetUsers()
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("Users")
	fmt.Fprintf(cmd.out, usersView(users))
}

func (cmd *Command) GetUserKeys(user string) {
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
