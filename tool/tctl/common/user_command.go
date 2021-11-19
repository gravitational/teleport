/*
Copyright 2015-2021 Gravitational, Inc.

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

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/trace"
)

// UserCommand implements `tctl users` set of commands
// It implements CLICommand interface
type UserCommand struct {
	config               *service.Config
	login                string
	allowedLogins        []string
	allowedWindowsLogins []string
	createRoles          []string
	kubeUsers            string
	kubeGroups           string

	ttl time.Duration

	// updateRoles is used for update users command
	updateRoles string

	// format is the output format, e.g. text or json
	format string

	userAdd           *kingpin.CmdClause
	userUpdate        *kingpin.CmdClause
	userList          *kingpin.CmdClause
	userDelete        *kingpin.CmdClause
	userResetPassword *kingpin.CmdClause
}

// Initialize allows UserCommand to plug itself into the CLI parser
func (u *UserCommand) Initialize(app *kingpin.Application, config *service.Config) {
	const helpPrefix string = "[Teleport DB users only]"

	u.config = config
	users := app.Command("users", "Manage user accounts")

	u.userAdd = users.Command("add", "Generate a user invitation token "+helpPrefix)
	u.userAdd.Arg("account", "Teleport user account name").Required().StringVar(&u.login)

	u.userAdd.Flag("logins", "List of allowed SSH logins for the new user").StringsVar(&u.allowedLogins)
	u.userAdd.Flag("windows-logins", "List of allowed Windows logins for the new user").StringsVar(&u.allowedWindowsLogins)
	u.userAdd.Flag("roles", "List of roles for the new user to assume").Required().StringsVar(&u.createRoles)

	u.userAdd.Flag("ttl", fmt.Sprintf("Set expiration time for token, default is %v, maximum is %v",
		defaults.SignupTokenTTL, defaults.MaxSignupTokenTTL)).
		Default(fmt.Sprintf("%v", defaults.SignupTokenTTL)).DurationVar(&u.ttl)
	u.userAdd.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).StringVar(&u.format)
	u.userAdd.Alias(AddUserHelp)

	u.userUpdate = users.Command("update", "Update user account")
	u.userUpdate.Arg("account", "Teleport user account name").Required().StringVar(&u.login)
	u.userUpdate.Flag("set-roles", "List of roles for the user to assume").
		Default("").StringVar(&u.updateRoles)

	u.userList = users.Command("ls", "List all user accounts "+helpPrefix)
	u.userList.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).StringVar(&u.format)

	u.userDelete = users.Command("rm", "Deletes user accounts").Alias("del")
	u.userDelete.Arg("logins", "Comma-separated list of user logins to delete").
		Required().StringVar(&u.login)

	u.userResetPassword = users.Command("reset", "Reset user password and generate a new token "+helpPrefix)
	u.userResetPassword.Arg("account", "Teleport user account name").Required().StringVar(&u.login)
	u.userResetPassword.Flag("ttl", fmt.Sprintf("Set expiration time for token, default is %v, maximum is %v",
		defaults.ChangePasswordTokenTTL, defaults.MaxChangePasswordTokenTTL)).
		Default(fmt.Sprintf("%v", defaults.ChangePasswordTokenTTL)).DurationVar(&u.ttl)
	u.userResetPassword.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).StringVar(&u.format)
}

// TryRun takes the CLI command as an argument (like "users add") and executes it.
func (u *UserCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case u.userAdd.FullCommand():
		err = u.Add(client)
	case u.userUpdate.FullCommand():
		err = u.Update(client)
	case u.userList.FullCommand():
		err = u.List(client)
	case u.userDelete.FullCommand():
		err = u.Delete(client)
	case u.userResetPassword.FullCommand():
		err = u.ResetPassword(client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

// ResetPassword resets user password and generates a token to setup new password
func (u *UserCommand) ResetPassword(client auth.ClientI) error {
	req := auth.CreateUserTokenRequest{
		Name: u.login,
		TTL:  u.ttl,
		Type: auth.UserTokenTypeResetPassword,
	}
	token, err := client.CreateResetPasswordToken(context.TODO(), req)
	if err != nil {
		return err
	}

	err = u.PrintResetPasswordToken(token, u.format)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// PrintResetPasswordToken prints ResetPasswordToken
func (u *UserCommand) PrintResetPasswordToken(token types.UserToken, format string) error {
	err := u.printResetPasswordToken(token,
		format,
		"User %q has been reset. Share this URL with the user to complete password reset, link is valid for %v:\n%v\n\n",
	)

	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// PrintResetPasswordTokenAsInvite prints ResetPasswordToken as Invite
func (u *UserCommand) PrintResetPasswordTokenAsInvite(token types.UserToken, format string) error {
	err := u.printResetPasswordToken(token,
		format,
		"User %q has been created but requires a password. Share this URL with the user to complete user setup, link is valid for %v:\n%v\n\n")
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// PrintResetPasswordToken prints ResetPasswordToken
func (u *UserCommand) printResetPasswordToken(token types.UserToken, format string, messageFormat string) (err error) {
	switch strings.ToLower(u.format) {
	case teleport.JSON:
		err = printTokenAsJSON(token)
	case teleport.Text:
		err = printTokenAsText(token, messageFormat)
	default:
		err = printTokenAsText(token, messageFormat)
	}

	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Add implements `tctl users add` for the enterprise edition. Unlike the OSS
// version, this one requires --roles flag to be set
func (u *UserCommand) Add(client auth.ClientI) error {
	u.createRoles = flattenSlice(u.createRoles)
	u.allowedLogins = flattenSlice(u.allowedLogins)
	u.allowedWindowsLogins = flattenSlice(u.allowedWindowsLogins)

	// Validate roles (server does not do this yet).
	for _, roleName := range u.createRoles {
		if _, err := client.GetRole(context.TODO(), roleName); err != nil {
			return trace.Wrap(err)
		}
	}

	traits := map[string][]string{
		teleport.TraitLogins:        u.allowedLogins,
		teleport.TraitWindowsLogins: u.allowedWindowsLogins,
		teleport.TraitKubeUsers:     flattenSlice([]string{u.kubeUsers}),
		teleport.TraitKubeGroups:    flattenSlice([]string{u.kubeGroups}),
	}

	user, err := types.NewUser(u.login)
	if err != nil {
		return trace.Wrap(err)
	}

	user.SetTraits(traits)
	user.SetRoles(u.createRoles)

	if err := client.CreateUser(context.TODO(), user); err != nil {
		return trace.Wrap(err)
	}

	token, err := client.CreateResetPasswordToken(context.TODO(), auth.CreateUserTokenRequest{
		Name: u.login,
		TTL:  u.ttl,
		Type: auth.UserTokenTypeResetPasswordInvite,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := u.PrintResetPasswordTokenAsInvite(token, u.format); err != nil {
		return trace.Wrap(err)
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

func printTokenAsJSON(token types.UserToken) error {
	out, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return trace.Wrap(err, "failed to marshal reset password token")
	}
	fmt.Print(string(out))
	return nil
}

func printTokenAsText(token types.UserToken, messageFormat string) error {
	url, err := url.Parse(token.GetURL())
	if err != nil {
		return trace.Wrap(err, "failed to parse reset password token url")
	}

	ttl := trimDurationZeroSuffix(token.Expiry().Sub(time.Now().UTC()))
	fmt.Printf(messageFormat, token.GetUser(), ttl, url)
	fmt.Printf("NOTE: Make sure %v points at a Teleport proxy which users can access.\n", url.Host)
	return nil
}

// Update updates existing user
func (u *UserCommand) Update(client auth.ClientI) error {
	user, err := client.GetUser(u.login, false)
	if err != nil {
		return trace.Wrap(err)
	}
	roles := flattenSlice([]string{u.updateRoles})
	for _, role := range roles {
		if _, err := client.GetRole(context.TODO(), role); err != nil {
			return trace.Wrap(err)
		}
	}
	user.SetRoles(roles)
	if err := client.UpsertUser(user); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("%v has been updated with roles %v\n", user.GetName(), strings.Join(user.GetRoles(), ","))
	return nil
}

// List prints all existing user accounts
func (u *UserCommand) List(client auth.ClientI) error {
	users, err := client.GetUsers(false)
	if err != nil {
		return trace.Wrap(err)
	}
	if u.format == teleport.Text {
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
		fmt.Print(string(out))
	}
	return nil
}

// Delete deletes teleport user(s). User IDs are passed as a comma-separated
// list in UserCommand.login
func (u *UserCommand) Delete(client auth.ClientI) error {
	for _, l := range strings.Split(u.login, ",") {
		if err := client.DeleteUser(context.TODO(), l); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("User %q has been deleted\n", l)
	}
	return nil
}

func trimDurationZeroSuffix(d time.Duration) string {
	s := d.Round(time.Second).String()
	switch {
	case strings.HasSuffix(s, "h0m0s"):
		return strings.TrimSuffix(s, "0m0s")
	case strings.HasSuffix(s, "m0s"):
		return strings.TrimSuffix(s, "0s")
	default:
		return s
	}
}
