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

	log "github.com/Sirupsen/logrus"

	"github.com/gravitational/teleport/lib/utils"
)

func main() {
	log.Info("starting %v", os.Args)
	run(os.Args[1:], false)
}

// command line arguments and flags:
type CLIConf struct {
	// UserHost contains "[login]@hostname" argument to SSH command
	UserHost string
	// Commands to execute on a remote host
	RemoteCommand string
	// Login is the Teleport user login
	Login string
	// Proxy keeps the hostname:port of the SSH proxy to use
	Proxy string
	// TTL defines how long a session must be active (in minutes)
	TTL int32
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
	app.Flag("user", fmt.Sprintf("SSH proxy user [%s]", Username())).StringVar(&cf.Login)
	app.Flag("proxy", "SSH proxy host or IP address").StringVar(&cf.Proxy)
	app.Flag("ttl", "Minutes to live for a SSH session").Int32Var(&cf.TTL)
	app.HelpFlag.Short('h')
	ver := app.Command("version", "Print the version")
	ssh := app.Command("ssh", "SSH into a remote machine")
	ssh.Arg("[user@]host", "Remote hostname and the machine login [$USER]").Required().StringVar(&cf.UserHost)
	ssh.Arg("command", "Command to execute on a remote host").StringVar(&cf.RemoteCommand)
	ssh.Flag("port", "SSH port on a remote host").Short('p').Int16Var(&cf.NodePort)
	ssh.Flag("login", "Remote host login").Short('l').StringVar(&cf.NodeLogin)

	// parse CLI commands+flags:
	command, err := app.Parse(args)
	if err != nil {
		utils.FatalError(err)
	}

	switch command {
	case ver.FullCommand():
		onVersion()
	case ssh.FullCommand():
		onSSH(&cf)
	}
}

// onSSH executes 'tsh ssh' command
func onSSH(cf *CLIConf) {
}

func onVersion() {
	fmt.Println("Version!")
}
