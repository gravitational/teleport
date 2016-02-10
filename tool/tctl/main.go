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

func main() {
	utils.InitLoggerCLI()
	app := utils.InitCLIParser("tctl", GlobalHelpString)

	// define global flags:
	var ccf CLIConfig
	app.Flag("debug", "Enable verbose logging to stderr").
		Short('d').
		BoolVar(&ccf.Debug)

	var (
		login string
	)

	// commands:
	ver := app.Command("version", "Print the version.")
	app.HelpFlag.Short('h')

	users := app.Command("users", "Manage users logins")
	userAdd := users.Command("add", "Creates a new user")
	userAdd.Alias("Using user add!")
	userAdd.Arg("login", "user login").Required().StringVar(&login)
	userList := users.Command("ls", "Lists all user logins")
	userDelete := users.Command("del", "Delete user login")
	userDelete.Arg("login", "user login to delete").Required().StringVar(&login)

	// parse CLI commands+flags:
	command, err := app.Parse(os.Args[1:])
	if err != nil {
		utils.FatalError(err)
	}

	// --debug flag
	if ccf.Debug {
		utils.InitLoggerDebug()
	}

	// apply configuration:
	cfg, err := service.MakeDefaultConfig()
	if err != nil {
		utils.FatalError(err)
	}

	// execute the selected command:
	switch command {
	case ver.FullCommand():
		onVersion()
	case userAdd.FullCommand():
		onUserAdd(login, cfg)
	case userList.FullCommand():
		onUserList()
	case userDelete.FullCommand():
		onUserDelete(login)
	}

	if err != nil {
		utils.Consolef(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func onVersion() {
	fmt.Println("TODO: Version command has not been implemented yet")
}

func onUserAdd(login string, cfg *service.Config) error {
	client, err := connectToAuthService(cfg)
	if err != nil {
		utils.FatalError(err)
	}
	fmt.Println("TODO: Adding user: ", login, client)
	token, err := client.CreateSignupToken(login, []string{login, "root", "centos"})
	if err != nil {
		utils.FatalError(err)
	}

	hostname, _ := os.Hostname()
	url := web.CreateSignupLink(net.JoinHostPort(hostname, strconv.Itoa(defaults.HTTPListenPort)), token)
	fmt.Println("Got token: ", url)
	return nil
}

func onUserList() {
	fmt.Println("TODO: User list is not implemented")
}

func onUserDelete(login string) {
	fmt.Println("TODO: Deleting user: ", login)
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
