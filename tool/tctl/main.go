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
package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/buger/goterm"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

type CLIConfig struct {
	Debug bool
}

type UserCommand struct {
	login    string
	mappings string
	config   *service.Config
}

func main() {
	utils.InitLoggerCLI()
	app := utils.InitCLIParser("tctl", GlobalHelpString)

	// generate default tctl configuration:
	cfg, err := service.MakeDefaultConfig()
	if err != nil {
		utils.FatalError(err)
	}

	// define global flags:
	var ccf CLIConfig
	app.Flag("debug", "Enable verbose logging to stderr").
		Short('d').
		BoolVar(&ccf.Debug)

	// commands:
	ver := app.Command("version", "Print the version.")
	app.HelpFlag.Short('h')

	users := app.Command("users", "Manage users logins")
	cmdUsers := UserCommand{config: cfg}

	// user add command:
	userAdd := users.Command("add", "Creates a new user")
	userAdd.Arg("login", "Teleport user login").Required().StringVar(&cmdUsers.login)
	userAdd.Arg("local-logins", "Local UNIX users this account can log in as [login]").
		Default("").StringVar(&cmdUsers.mappings)
	userAdd.Alias(AddUserHelp)

	// list users command
	userList := users.Command("ls", "Lists all user logins")

	// Delete user command
	userDelete := users.Command("rm", "Delete a user(s)")
	userDelete.Arg("logins", "Comma-separated list of user logins to delete").
		Required().StringVar(&cmdUsers.login)

	// parse CLI commands+flags:
	command, err := app.Parse(os.Args[1:])
	if err != nil {
		utils.FatalError(err)
	}

	// --debug flag
	if ccf.Debug {
		utils.InitLoggerDebug()
	}

	// connect to the teleport auth service:
	client, err := connectToAuthService(cfg)
	if err != nil {
		utils.FatalError(err)
	}

	// execute the selected command:
	switch command {
	case ver.FullCommand():
		onVersion()
	case userAdd.FullCommand():
		cmdUsers.Add(client)
	case userList.FullCommand():
		cmdUsers.List(client)
	case userDelete.FullCommand():
		cmdUsers.Delete(client)
	}

	if err != nil {
		utils.Consolef(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func onVersion() {
	fmt.Println("TODO: Version command has not been implemented yet")
}

// Add() creates a new sign-up token and prints a token URL to stdout.
// A user is not created until he visits the sign-up URL and completes the process
func (this *UserCommand) Add(client *auth.TunClient) error {
	// if no local logis were specified, default to 'login'
	if this.mappings == "" {
		this.mappings = this.login
	}
	token, err := client.CreateSignupToken(this.login, strings.Split(this.mappings, ","))
	if err != nil {
		utils.FatalError(err)
	}

	hostname, _ := os.Hostname()
	url := web.CreateSignupLink(net.JoinHostPort(hostname, strconv.Itoa(defaults.HTTPListenPort)), token)
	fmt.Printf("Signup token has been created. Share this URL with the user:\n%v\n\nNOTE: make sure the hostname is accessible!\n", url)
	return nil
}

func (this *UserCommand) List(client *auth.TunClient) {
	users, err := client.GetUsers()
	if err != nil {
		utils.FatalError(err)
	}

	usersView := func(users []string) string {
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
	fmt.Printf(usersView(users))
}

// Delete() deletes the teleport user
func (this *UserCommand) Delete(client *auth.TunClient) {
	err := client.DeleteUser(this.login)
	if err != nil {
		utils.FatalError(err)
	}
	fmt.Printf("User '%v' has been deleted\n", this.login)
}

// connectToAuthService creates a valid client connection to the auth service
func connectToAuthService(cfg *service.Config) (client *auth.TunClient, err error) {
	// connect to the local auth server by default:
	cfg.Auth.Enabled = true
	cfg.AuthServers = []utils.NetAddr{
		*defaults.AuthConnectAddr(),
	}

	// login via keys:
	signer, err := auth.ReadKeys(cfg.Hostname, cfg.DataDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err = auth.NewTunClient(
		cfg.AuthServers[0],
		cfg.Hostname,
		[]ssh.AuthMethod{ssh.PublicKeys(signer)})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// check connectivity by calling something on a clinet:
	_, err = client.GetDialer()()
	if err != nil {
		utils.Consolef(os.Stderr,
			"Cannot connect to the auth server: %v.\nIs the auth server running on %v?", err, cfg.AuthServers[0].Addr)
		os.Exit(1)
	}
	return client, nil
}
