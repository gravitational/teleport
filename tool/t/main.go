/*
Copyright 2016 Gravitational, Inc.

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
	"os"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

func main() {
	run(os.Args[1:], false)
}

// command line arguments and flags:
type CLIConf struct {
	// UserHost contains "[login]@hostname" argument to SSH command
	UserHost string
	// Commands to execute on a remote host
	RemoteCommand []string
	// Login is the Teleport user login
	Login string
	// Proxy keeps the hostname:port of the SSH proxy to use
	Proxy string
	// TTL defines how long a session must be active (in minutes)
	MinsToLive int32
	// SSH Port on a remote SSH host
	NodePort int16
	// Login on a remote SSH host
	NodeLogin string

	// IsUnderTest is set to true for unit testing
	IsUnderTest bool
}

// run executes TSH client. same as main() but easier to test
func run(args []string, underTest bool) {
	var (
		cf CLIConf
	)
	cf.IsUnderTest = underTest
	utils.InitLoggerCLI()

	// configure CLI argument parser:
	app := utils.InitCLIParser("t", "TSH: Teleport SSH client")
	app.Flag("user", fmt.Sprintf("SSH proxy user [%s]", client.Username())).StringVar(&cf.Login)
	app.Flag("proxy", "SSH proxy host or IP address").StringVar(&cf.Proxy)
	app.Flag("ttl", "Minutes to live for a SSH session").Int32Var(&cf.MinsToLive)
	debugMode := app.Flag("debug", "Verbose logging to stdout").Short('d').Bool()
	app.HelpFlag.Short('h')
	ver := app.Command("version", "Print the version")
	// ssh
	ssh := app.Command("ssh", "Run shell or execute a command on a remote SSH node")
	ssh.Arg("[user@]host", "Remote hostname and the login to use").Required().StringVar(&cf.UserHost)
	ssh.Arg("command", "Command to execute on a remote host").StringsVar(&cf.RemoteCommand)
	ssh.Flag("port", "SSH port on a remote host").Short('p').Int16Var(&cf.NodePort)
	ssh.Flag("login", "Remote host login").Short('l').StringVar(&cf.NodeLogin)
	// ls
	ls := app.Command("ls", "List remote SSH nodes")
	ls.Arg("labels", "List of labels to filter node list").Default("*").StringVar(&cf.UserHost)

	// parse CLI commands+flags:
	command, err := app.Parse(args)
	if err != nil {
		utils.FatalError(err)
	}

	// apply -d flag:
	if *debugMode {
		utils.InitLoggerDebug()
	}

	switch command {
	case ver.FullCommand():
		onVersion()
	case ssh.FullCommand():
		onSSH(&cf)
	case ls.FullCommand():
		onListNodes(&cf)
	}
}

// onListNodes executes 'tsh ls' command
func onListNodes(cf *CLIConf) {
	_, err := makeClient(cf)
	if err != nil {
		utils.FatalError(err)
	}
}

// onSSH executes 'tsh ssh' command
func onSSH(cf *CLIConf) {
	tc, err := makeClient(cf)
	if err != nil {
		utils.FatalError(err)
	}

	if err = tc.SSH(strings.Join(cf.RemoteCommand, " ")); err != nil {
		utils.FatalError(err)
	}
}

// makeClient takes the command-line configuration and constructs & returns
// a fully configured TeleportClient object
func makeClient(cf *CLIConf) (*client.TeleportClient, error) {
	// apply defults
	if cf.NodePort == 0 {
		cf.NodePort = defaults.SSHServerListenPort
	}
	if cf.MinsToLive == 0 {
		cf.MinsToLive = defaults.CertDurationHours * 60
	}
	// split login & host
	parts := strings.Split(cf.UserHost, "@")
	hostLogin := cf.Login
	if len(parts) > 1 {
		hostLogin = parts[0]
		cf.UserHost = parts[1]
	}

	// prep client config:
	c := &client.Config{
		Login:     cf.Login,
		ProxyHost: cf.Proxy,
		Host:      cf.UserHost,
		HostPort:  int(cf.NodePort),
		HostLogin: hostLogin,
		KeyTTL:    time.Minute * time.Duration(cf.MinsToLive),
	}
	return client.NewClient(c)
}

func onVersion() {
	fmt.Println("Version!")
}

func parseLabelSpec(spec string) (map[string]string, error) {
	return nil, nil
}
