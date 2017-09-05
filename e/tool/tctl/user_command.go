/*
Copyright 2017 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This file implements the enterprise version of `tctl users` subcommands

*/

package main

import (
	"fmt"
	"strings"

	"github.com/buger/goterm"
	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common"
	"github.com/gravitational/trace"
)

// implements common.CLICommand interface
type UserCommandE struct {
	// OSS implementation of 'tctl users'
	common.UserCommand

	// Enterprise versions of 'user' CLI subcommands
	userAdd  *kingpin.CmdClause
	userList *kingpin.CmdClause

	username string
	roles    []string
}

// Initialize allows a caller-defined command to plug itself into CLI
// argument parsing
func (cmd *UserCommandE) Initialize(app *kingpin.Application, cfg *service.Config) {
	cmd.UserCommand.Initialize(app, cfg)

	const helpPrefix string = "[Teleport DB users only]"

	// re-initialize 'users add' with our own implementation
	users := app.GetCommand("users")
	cmd.userAdd = users.Command("add", "Generate a user invitation token "+helpPrefix)
	cmd.userAdd.Arg("account", "Teleport user account name").Required().StringVar(&cmd.username)
	cmd.userAdd.Flag("roles", "List of roles for the new user to assume").Required().StringsVar(&cmd.roles)
	cmd.userAdd.Alias(AddUserHelp)

	cmd.userList = users.Command("ls", "List all user accounts "+helpPrefix)
}

// TryRun is executed after the CLI parsing is done. The command must
// determine if selectedCommand belongs to it and return match=true
func (cmd *UserCommandE) TryRun(selectedCommand string, c *auth.TunClient) (match bool, err error) {
	switch selectedCommand {
	// tctl users add? execute enterprise version of it:
	case cmd.userAdd.FullCommand():
		return true, trace.Wrap(cmd.Add(c))
	case cmd.userList.FullCommand():
		return true, trace.Wrap(cmd.List(c))
	}

	// OSS implementation of user subcommands commands:
	return cmd.UserCommand.TryRun(selectedCommand, c)
}

// List implements `tctl users ls` for the enterprise edition. Unlike the OSS
// version, this implementation prints user roles (instead of "allowed logins")
func (cmd *UserCommandE) List(client *auth.TunClient) error {
	users, err := client.GetUsers()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(users) == 0 {
		fmt.Println("No users found")
		return nil
	}
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintHeader(t, []string{"User", "Roles"})
	for _, u := range users {
		fmt.Fprintf(t, "%v\t%v\n", u.GetName(), strings.Join(u.GetRoles(), ","))
	}
	fmt.Println(t.String())
	return nil
}

// Add implements `tctl users add` for the enterprise edition. Unlike the OSS
// version, this one requires --roles flag to be set
func (cmd *UserCommandE) Add(client *auth.TunClient) error {
	cmd.roles = flattenRoles(cmd.roles)

	// validate roles (server does not do this yet)
	for _, roleName := range cmd.roles {
		_, err := client.GetRole(roleName)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	user := services.UserV1{
		Name:  cmd.username,
		Roles: cmd.roles,
	}
	token, err := client.CreateSignupToken(user)
	if err != nil {
		return trace.Wrap(err)
	}
	cmd.UserCommand.PrintSignupURL(client, token)
	fmt.Printf("When the user '%s' activates their account, they will be assigned roles %s\n",
		cmd.username, cmd.roles)
	return nil
}

// flattenRoles takes a slice of strings like ["one,two", "three"] and returns
// ["one", "two", "three"]
func flattenRoles(slice []string) (retval []string) {
	for i := range slice {
		for _, role := range strings.Split(slice[i], ",") {
			retval = append(retval, strings.TrimSpace(role))
		}
	}
	return retval
}

const (
	AddUserHelp = `Notes:

  1. tctl will generate a signup token and give you a URL to share with a user.
     A user will have to complete account creation by visiting the URL.

  2. A Teleport user account is not the same as a local UNIX users on SSH nodes.
     You must assign a list of allowed local users for every Teleport login.

Examples:

  > tctl users add --roles=admin joe 

  This creates a Teleport account 'joe' who will assume the role 'admin'
  To see 'admin' permissions, you can execute 'tctl get role/admin'
`
)
