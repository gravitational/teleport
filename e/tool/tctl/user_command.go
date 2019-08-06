/*
Copyright 2017-2019 Gravitational, Inc.
This file implements the enterprise version of `tctl users` subcommands

*/

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
)

// implements common.CLICommand interface
type UserCommandE struct {
	// OSS implementation of 'tctl users'
	common.UserCommand

	// Enterprise versions of 'user' CLI subcommands
	userAdd  *kingpin.CmdClause
	userList *kingpin.CmdClause

	format string

	username      string
	roles         []string
	allowedLogins []string
	ttl           time.Duration
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
	cmd.userAdd.Flag("logins", "List of allowed logins for the new user").StringsVar(&cmd.allowedLogins)
	cmd.userAdd.Flag("ttl", fmt.Sprintf("Set expiration time for token, default is %v hour, maximum is %v hours",
		int(defaults.SignupTokenTTL/time.Hour), int(defaults.MaxSignupTokenTTL/time.Hour))).
		Default(fmt.Sprintf("%v", defaults.SignupTokenTTL)).DurationVar(&cmd.ttl)
	cmd.userAdd.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).StringVar(&cmd.format)
	cmd.userAdd.Alias(AddUserHelp)

	cmd.userList = users.Command("ls", "List all user accounts "+helpPrefix)
	cmd.userList.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).StringVar(&cmd.format)
}

// TryRun is executed after the CLI parsing is done. The command must
// determine if selectedCommand belongs to it and return match=true
func (cmd *UserCommandE) TryRun(selectedCommand string, c auth.ClientI) (match bool, err error) {
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
func (cmd *UserCommandE) List(client auth.ClientI) error {
	users, err := client.GetUsers(false)
	if err != nil {
		return trace.Wrap(err)
	}
	if cmd.format == teleport.Text {
		if len(users) == 0 {
			fmt.Println("No users found")
			return nil
		}
		t := asciitable.MakeTable([]string{"User", "Roles"})
		for _, u := range users {
			t.AddRow([]string{
				u.GetName(), strings.Join(u.GetRoles(), ","),
			})
		}
		fmt.Println(t.AsBuffer().String())
	} else {
		out, err := json.MarshalIndent(users, "", "  ")
		if err != nil {
			return trace.Wrap(err, "failed to marshal users")
		}
		fmt.Printf(string(out))
	}
	return nil
}

// Add implements `tctl users add` for the enterprise edition. Unlike the OSS
// version, this one requires --roles flag to be set
func (cmd *UserCommandE) Add(client auth.ClientI) error {
	cmd.roles = flattenSlice(cmd.roles)
	cmd.allowedLogins = flattenSlice(cmd.allowedLogins)

	// validate roles (server does not do this yet)
	for _, roleName := range cmd.roles {
		_, err := client.GetRole(roleName)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	user := services.UserV1{
		Name:          cmd.username,
		Roles:         cmd.roles,
		AllowedLogins: cmd.allowedLogins,
	}
	token, err := client.CreateSignupToken(user, cmd.ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	cmd.UserCommand.PrintSignupURL(client, token, cmd.ttl, cmd.format)
	if cmd.format == teleport.Text {
		fmt.Printf("When the user '%s' activates their account, they will be assigned roles %s\n",
			cmd.username, cmd.roles)
	}
	return nil
}

// flattenSlice takes a slice of strings like ["one,two", "three"] and returns
// ["one", "two", "three"]
func flattenSlice(slice []string) (retval []string) {
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

  2. The allowed logins of the account only apply if a role uses them by including
     '{{ internal.logins }}' variable in a role definition.

Examples:

  > tctl users add --roles=admin,dba joe

  This creates a Teleport account 'joe' who will assume the roles 'admin' and 'dba'
  To see the permissions of 'admin' role, execute 'tctl get role/admin'
`
)
