/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/gcp"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// UserCommand implements `tctl users` set of commands
// It implements CLICommand interface
type UserCommand struct {
	config                    *servicecfg.Config
	login                     string
	allowedLogins             []string
	allowedWindowsLogins      []string
	allowedKubeUsers          []string
	allowedKubeGroups         []string
	allowedDatabaseUsers      []string
	allowedDatabaseNames      []string
	allowedDatabaseRoles      []string
	allowedAWSRoleARNs        []string
	allowedAzureIdentities    []string
	allowedGCPServiceAccounts []string
	allowedRoles              []string
	hostUserUID               string
	hostUserUIDProvided       bool
	hostUserGID               string
	hostUserGIDProvided       bool

	ttl time.Duration

	// format is the output format, e.g. text or json
	format string

	userAdd           *kingpin.CmdClause
	userUpdate        *kingpin.CmdClause
	userList          *kingpin.CmdClause
	userDelete        *kingpin.CmdClause
	userResetPassword *kingpin.CmdClause
}

// Initialize allows UserCommand to plug itself into the CLI parser
func (u *UserCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	const helpPrefix string = "[Teleport local users only]"

	u.config = config
	users := app.Command("users", "Manage user accounts.")

	u.userAdd = users.Command("add", "Generate a user invitation token "+helpPrefix+".")
	u.userAdd.Arg("account", "Teleport user account name").Required().StringVar(&u.login)

	u.userAdd.Flag("logins", "List of allowed SSH logins for the new user").StringsVar(&u.allowedLogins)
	u.userAdd.Flag("windows-logins", "List of allowed Windows logins for the new user").StringsVar(&u.allowedWindowsLogins)
	u.userAdd.Flag("kubernetes-users", "List of allowed Kubernetes users for the new user").StringsVar(&u.allowedKubeUsers)
	u.userAdd.Flag("kubernetes-groups", "List of allowed Kubernetes groups for the new user").StringsVar(&u.allowedKubeGroups)
	u.userAdd.Flag("db-users", "List of allowed database users for the new user").StringsVar(&u.allowedDatabaseUsers)
	u.userAdd.Flag("db-names", "List of allowed database names for the new user").StringsVar(&u.allowedDatabaseNames)
	u.userAdd.Flag("db-roles", "List of database roles for automatic database user provisioning").StringsVar(&u.allowedDatabaseRoles)
	u.userAdd.Flag("aws-role-arns", "List of allowed AWS role ARNs for the new user").StringsVar(&u.allowedAWSRoleARNs)
	u.userAdd.Flag("azure-identities", "List of allowed Azure identities for the new user").StringsVar(&u.allowedAzureIdentities)
	u.userAdd.Flag("gcp-service-accounts", "List of allowed GCP service accounts for the new user").StringsVar(&u.allowedGCPServiceAccounts)
	u.userAdd.Flag("host-user-uid", "UID for auto provisioned host users to use").IsSetByUser(&u.hostUserUIDProvided).StringVar(&u.hostUserUID)
	u.userAdd.Flag("host-user-gid", "GID for auto provisioned host users to use").IsSetByUser(&u.hostUserGIDProvided).StringVar(&u.hostUserGID)

	u.userAdd.Flag("roles", "List of roles for the new user to assume").Required().StringsVar(&u.allowedRoles)

	u.userAdd.Flag("ttl", fmt.Sprintf("Set expiration time for token, default is %v, maximum is %v",
		defaults.SignupTokenTTL, defaults.MaxSignupTokenTTL)).
		Default(fmt.Sprintf("%v", defaults.SignupTokenTTL)).DurationVar(&u.ttl)
	u.userAdd.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).StringVar(&u.format)
	u.userAdd.Alias(AddUserHelp)

	u.userUpdate = users.Command("update", "Update user account.")
	u.userUpdate.Arg("account", "Teleport user account name").Required().StringVar(&u.login)
	u.userUpdate.Flag("set-roles", "List of roles for the user to assume, replaces current roles").
		StringsVar(&u.allowedRoles)
	u.userUpdate.Flag("set-logins", "List of allowed SSH logins for the user, replaces current logins").
		StringsVar(&u.allowedLogins)
	u.userUpdate.Flag("set-windows-logins", "List of allowed Windows logins for the user, replaces current Windows logins").
		StringsVar(&u.allowedWindowsLogins)
	u.userUpdate.Flag("set-kubernetes-users", "List of allowed Kubernetes users for the user, replaces current Kubernetes users").
		StringsVar(&u.allowedKubeUsers)
	u.userUpdate.Flag("set-kubernetes-groups", "List of allowed Kubernetes groups for the user, replaces current Kubernetes groups").
		StringsVar(&u.allowedKubeGroups)
	u.userUpdate.Flag("set-db-users", "List of allowed database users for the user, replaces current database users").
		StringsVar(&u.allowedDatabaseUsers)
	u.userUpdate.Flag("set-db-names", "List of allowed database names for the user, replaces current database names").
		StringsVar(&u.allowedDatabaseNames)
	u.userUpdate.Flag("set-db-roles", "List of allowed database roles for automatic database user provisioning, replaces current database roles").
		StringsVar(&u.allowedDatabaseRoles)
	u.userUpdate.Flag("set-aws-role-arns", "List of allowed AWS role ARNs for the user, replaces current AWS role ARNs").
		StringsVar(&u.allowedAWSRoleARNs)
	u.userUpdate.Flag("set-azure-identities", "List of allowed Azure identities for the user, replaces current Azure identities").
		StringsVar(&u.allowedAzureIdentities)
	u.userUpdate.Flag("set-gcp-service-accounts", "List of allowed GCP service accounts for the user, replaces current service accounts").
		StringsVar(&u.allowedGCPServiceAccounts)
	u.userUpdate.Flag("set-host-user-uid", "UID for auto provisioned host users to use. Value can be reset by providing an empty string").IsSetByUser(&u.hostUserUIDProvided).StringVar(&u.hostUserUID)
	u.userUpdate.Flag("set-host-user-gid", "GID for auto provisioned host users to use. Value can be reset by providing an empty string").IsSetByUser(&u.hostUserGIDProvided).StringVar(&u.hostUserGID)

	u.userList = users.Command("ls", "Lists all user accounts.")
	u.userList.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).StringVar(&u.format)

	u.userDelete = users.Command("rm", "Deletes user accounts.").Alias("del")
	u.userDelete.Arg("logins", "Comma-separated list of user logins to delete").
		Required().StringVar(&u.login)

	u.userResetPassword = users.Command("reset", "Reset user password and generate a new token "+helpPrefix+".")
	u.userResetPassword.Arg("account", "Teleport user account name").Required().StringVar(&u.login)
	u.userResetPassword.Flag("ttl", fmt.Sprintf("Set expiration time for token, default is %v, maximum is %v",
		defaults.ChangePasswordTokenTTL, defaults.MaxChangePasswordTokenTTL)).
		Default(fmt.Sprintf("%v", defaults.ChangePasswordTokenTTL)).DurationVar(&u.ttl)
	u.userResetPassword.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).StringVar(&u.format)
}

// TryRun takes the CLI command as an argument (like "users add") and executes it.
func (u *UserCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case u.userAdd.FullCommand():
		commandFunc = u.Add
	case u.userUpdate.FullCommand():
		commandFunc = u.Update
	case u.userList.FullCommand():
		commandFunc = u.List
	case u.userDelete.FullCommand():
		commandFunc = u.Delete
	case u.userResetPassword.FullCommand():
		commandFunc = u.ResetPassword
	default:
		return false, nil
	}
	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	err = commandFunc(ctx, client)
	closeFn(ctx)

	return true, trace.Wrap(err)
}

// ResetPassword resets user password and generates a token to setup new password
func (u *UserCommand) ResetPassword(ctx context.Context, client *authclient.Client) error {
	req := authclient.CreateUserTokenRequest{
		Name: u.login,
		TTL:  u.ttl,
		Type: authclient.UserTokenTypeResetPassword,
	}
	token, err := client.CreateResetPasswordToken(ctx, req)
	if err != nil {
		return err
	}

	err = u.PrintResetPasswordToken(token)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// PrintResetPasswordToken prints ResetPasswordToken
func (u *UserCommand) PrintResetPasswordToken(token types.UserToken) error {
	err := u.printResetPasswordToken(token,
		"User %q has been reset. Share this URL with the user to complete password reset, link is valid for %v:\n%v\n\n",
	)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// PrintResetPasswordTokenAsInvite prints ResetPasswordToken as Invite
func (u *UserCommand) PrintResetPasswordTokenAsInvite(token types.UserToken) error {
	err := u.printResetPasswordToken(token,
		"User %q has been created but requires a password. Share this URL with the user to complete user setup, link is valid for %v:\n%v\n\n")
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// PrintResetPasswordToken prints ResetPasswordToken
func (u *UserCommand) printResetPasswordToken(token types.UserToken, messageFormat string) (err error) {
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
func (u *UserCommand) Add(ctx context.Context, client *authclient.Client) error {
	u.allowedRoles = flattenSlice(u.allowedRoles)
	u.allowedLogins = flattenSlice(u.allowedLogins)
	u.allowedWindowsLogins = flattenSlice(u.allowedWindowsLogins)

	// Validate roles (server does not do this yet).
	for _, roleName := range u.allowedRoles {
		if _, err := client.GetRole(ctx, roleName); err != nil {
			return trace.Wrap(err)
		}
	}

	azureIdentities := flattenSlice(u.allowedAzureIdentities)
	for _, identity := range azureIdentities {
		if !services.MatchValidAzureIdentity(identity) {
			return trace.BadParameter("Azure identity %q has invalid format.", identity)
		}
		if identity == types.Wildcard {
			return trace.BadParameter("Azure identity cannot be a wildcard.")
		}
	}

	gcpServiceAccounts := flattenSlice(u.allowedGCPServiceAccounts)
	for _, account := range gcpServiceAccounts {
		if err := gcp.ValidateGCPServiceAccountName(account); err != nil {
			return trace.Wrap(err, "GCP service account %q is invalid", account)
		}
	}

	if u.hostUserUIDProvided && u.hostUserUID != "" {
		if _, err := strconv.Atoi(u.hostUserUID); err != nil {
			return trace.BadParameter("host user UID must be a numeric ID")
		}
	}
	if u.hostUserGIDProvided && u.hostUserGID != "" {
		if _, err := strconv.Atoi(u.hostUserGID); err != nil {
			return trace.BadParameter("host user GID must be a numeric ID")
		}
	}

	traits := map[string][]string{
		constants.TraitLogins:             u.allowedLogins,
		constants.TraitWindowsLogins:      u.allowedWindowsLogins,
		constants.TraitKubeUsers:          flattenSlice(u.allowedKubeUsers),
		constants.TraitKubeGroups:         flattenSlice(u.allowedKubeGroups),
		constants.TraitDBUsers:            flattenSlice(u.allowedDatabaseUsers),
		constants.TraitDBNames:            flattenSlice(u.allowedDatabaseNames),
		constants.TraitDBRoles:            flattenSlice(u.allowedDatabaseRoles),
		constants.TraitAWSRoleARNs:        flattenSlice(u.allowedAWSRoleARNs),
		constants.TraitAzureIdentities:    azureIdentities,
		constants.TraitGCPServiceAccounts: gcpServiceAccounts,
		constants.TraitHostUserUID:        {u.hostUserUID},
		constants.TraitHostUserGID:        {u.hostUserGID},
	}

	user, err := types.NewUser(u.login)
	if err != nil {
		return trace.Wrap(err)
	}

	user.SetTraits(traits)
	user.SetRoles(u.allowedRoles)

	// Prompt for admin action MFA if required, allowing reuse for CreateResetPasswordToken.
	mfaResponse, err := mfa.PerformAdminActionMFACeremony(ctx, client.PerformMFACeremony, true /*allowReuse*/)
	if err == nil {
		ctx = mfa.ContextWithMFAResponse(ctx, mfaResponse)
	} else if !errors.Is(err, &mfa.ErrMFANotRequired) && !errors.Is(err, &mfa.ErrMFANotSupported) {
		return trace.Wrap(err)
	}

	if _, err := client.CreateUser(ctx, user); err != nil {
		if trace.IsAlreadyExists(err) {
			fmt.Printf(`NOTE: To update an existing local user:
> tctl users update %v --set-roles %v # replace roles

`, u.login, strings.Join(u.allowedRoles, ","))
		}
		return trace.Wrap(err)
	}

	token, err := client.CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
		Name: u.login,
		TTL:  u.ttl,
		Type: authclient.UserTokenTypeResetPasswordInvite,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := u.PrintResetPasswordTokenAsInvite(token); err != nil {
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
func (u *UserCommand) Update(ctx context.Context, client *authclient.Client) error {
	user, err := client.GetUser(ctx, u.login, false)
	if err != nil {
		return trace.Wrap(err)
	}

	updateMessages := make(map[string][]string)
	if len(u.allowedRoles) > 0 {
		roles := flattenSlice(u.allowedRoles)
		for _, role := range roles {
			if _, err := client.GetRole(ctx, role); err != nil {
				return trace.Wrap(err)
			}
		}
		user.SetRoles(roles)
		updateMessages["roles"] = roles
	}
	if len(u.allowedLogins) > 0 {
		logins := flattenSlice(u.allowedLogins)
		user.SetLogins(logins)
		updateMessages["logins"] = logins
	}
	if len(u.allowedWindowsLogins) > 0 {
		windowsLogins := flattenSlice(u.allowedWindowsLogins)
		user.SetWindowsLogins(windowsLogins)
		updateMessages["Windows logins"] = windowsLogins
	}
	if len(u.allowedKubeUsers) > 0 {
		kubeUsers := flattenSlice(u.allowedKubeUsers)
		user.SetKubeUsers(kubeUsers)
		updateMessages["Kubernetes users"] = kubeUsers
	}
	if len(u.allowedKubeGroups) > 0 {
		kubeGroups := flattenSlice(u.allowedKubeGroups)
		user.SetKubeGroups(kubeGroups)
		updateMessages["Kubernetes groups"] = kubeGroups
	}
	if len(u.allowedDatabaseUsers) > 0 {
		dbUsers := flattenSlice(u.allowedDatabaseUsers)
		user.SetDatabaseUsers(dbUsers)
		updateMessages["database users"] = dbUsers
	}
	if len(u.allowedDatabaseNames) > 0 {
		dbNames := flattenSlice(u.allowedDatabaseNames)
		user.SetDatabaseNames(dbNames)
		updateMessages["database names"] = dbNames
	}
	if len(u.allowedDatabaseRoles) > 0 {
		dbRoles := flattenSlice(u.allowedDatabaseRoles)
		for _, role := range dbRoles {
			if role == types.Wildcard {
				return trace.BadParameter("database role can't be a wildcard")
			}
		}
		user.SetDatabaseRoles(dbRoles)
		updateMessages["database roles"] = dbRoles
	}
	if len(u.allowedAWSRoleARNs) > 0 {
		awsRoleARNs := flattenSlice(u.allowedAWSRoleARNs)
		user.SetAWSRoleARNs(awsRoleARNs)
		updateMessages["AWS role ARNs"] = awsRoleARNs
	}
	if len(u.allowedAzureIdentities) > 0 {
		azureIdentities := flattenSlice(u.allowedAzureIdentities)
		for _, identity := range azureIdentities {
			if !services.MatchValidAzureIdentity(identity) {
				return trace.BadParameter("Azure identity %q has invalid format.", identity)
			}
			if identity == types.Wildcard {
				return trace.BadParameter("Azure identity cannot be a wildcard.")
			}
		}
		user.SetAzureIdentities(azureIdentities)
		updateMessages["Azure identities"] = azureIdentities
	}
	if len(u.allowedGCPServiceAccounts) > 0 {
		accounts := flattenSlice(u.allowedGCPServiceAccounts)
		for _, account := range accounts {
			if err := gcp.ValidateGCPServiceAccountName(account); err != nil {
				return trace.Wrap(err, "GCP service account %q is invalid", account)
			}
		}
		user.SetGCPServiceAccounts(accounts)
		updateMessages["GCP service accounts"] = accounts
	}

	if u.hostUserUIDProvided && u.hostUserUID != "" {
		if _, err := strconv.Atoi(u.hostUserUID); err != nil {
			return trace.BadParameter("host user UID must be a numeric ID")
		}

		user.SetHostUserUID(u.hostUserUID)
		updateMessages["Host user UID"] = []string{u.hostUserUID}
	}
	if u.hostUserGIDProvided && u.hostUserGID != "" {
		if _, err := strconv.Atoi(u.hostUserGID); err != nil {
			return trace.BadParameter("host user GID must be a numeric ID")
		}
		user.SetHostUserGID(u.hostUserGID)
		updateMessages["Host user GID"] = []string{u.hostUserGID}
	}

	if len(updateMessages) == 0 {
		return trace.BadParameter("Nothing to update. Please provide at least one --set flag.")
	}

	for _, roleName := range user.GetRoles() {
		if _, err := client.GetRole(ctx, roleName); err != nil {
			log.Warnf("Error checking role %q when upserting user %q: %v", roleName, user.GetName(), err)
		}
	}
	if _, err := client.UpsertUser(ctx, user); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("User %v has been updated:\n", user.GetName())
	for field, values := range updateMessages {
		fmt.Printf("\tNew %v: %v\n", field, strings.Join(values, ","))
	}
	return nil
}

// List prints all existing user accounts
func (u *UserCommand) List(ctx context.Context, client *authclient.Client) error {
	users, err := client.GetUsers(ctx, false)
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
		err := utils.WriteJSONArray(os.Stdout, users)
		if err != nil {
			return trace.Wrap(err, "failed to marshal users")
		}
	}
	return nil
}

// Delete deletes teleport user(s). User IDs are passed as a comma-separated
// list in UserCommand.login
func (u *UserCommand) Delete(ctx context.Context, client *authclient.Client) error {
	for _, l := range strings.Split(u.login, ",") {
		if err := client.DeleteUser(ctx, l); err != nil {
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
