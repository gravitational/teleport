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
package tsh

import (
	"fmt"
	"net"
	"os"

	"github.com/gravitational/teleport/lib/client"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func RunTSH(app *kingpin.Application) error {
	sshAgentAddress := app.Flag("ssh-agent", "SSH agent address").OverrideDefaultFromEnvar("SSH_AUTH_SOCK").String()
	sshAgentNetwork := app.Flag("ssh-agent-network", "SSH agent address network type('tcp','unix' etc.)").Default("unix").String()
	webProxyAddress := app.Flag("web-proxy", "Web proxy address(used for login)").String()
	loginTTL := app.Flag("login-ttl", "Temporary ssh certificate will work for that time").Default("10h").Duration()
	proxy := app.Flag("proxy", "Teleport proxy address").String()
	proxyUser := app.Flag("proxy-user", "Teleport authentication username").Required().String()

	connect := app.Command("ssh", "Connects to remote server and runs shell or provided command")
	connectAddress := connect.Arg("target", "Target server address. You can provide several servers using label searching target _label:value").Required().String()
	connectCommand := connect.Arg("command", "Run provided command instead of shell").String()
	connectPort := connect.Flag("port", "Remote server port").Short('p').String()
	connectSessionID := connect.Flag("session", "Session ID. You can connect to one shared shell from different clients using one session ID").String()

	getServers := app.Command("get-servers", "Returns list of servers")
	getServersLabelName := getServers.Flag("label", "Label name").String()
	getServersLabelValue := getServers.Flag("value", "Label value regexp").String()

	scp := app.Command("scp", "Copy file or files to the remote ssh server of from it")
	scpSource := scp.Arg("source", "source file or dir").Required().String()
	scpDest := scp.Arg("destination", "destination file or dir").Required().String()
	scpIsDir := scp.Flag("recursively", "Source path is a directory").Short('r').Bool()
	scpPort := scp.Flag("port", "Remote server port").Short('P').String()

	share := app.Command("share", "Creates new hangout")
	shareReverseProxy := share.Flag("rproxy", "Remote reverse proxy address").Required().String()
	shareNodeAddress := share.Flag("node-addr", "SSH server listening address").Default(DefaultNodeListeningAddress).String()
	shareAuthAddress := share.Flag("auth-addr", "auth server listening address").Default(DefaultAuthListeningAddress).String()
	shareReadOnly := share.Flag("readonly", "Remote users can't write to the shell").Bool()

	join := app.Command("join", "Join a remote hangout")
	joinURL := join.Arg("url", "The url from the hangout owner").Required().String()

	selectedCommand := kingpin.MustParse(app.Parse(os.Args[1:]))

	if (selectedCommand == getServers.FullCommand()) && (len(*proxy) == 0) {
		return fmt.Errorf("Error: please provide user name")
	}

	standartSSHAgent, err := connectToSSHAgent(*sshAgentNetwork, *sshAgentAddress)
	if err != nil {
		return trace.Wrap(err)
	}
	teleportFileSSHAgent, err := client.GetLocalAgent()
	if err != nil {
		return trace.Wrap(err)
	}
	passwordCallback := client.GetPasswordFromConsole(*proxyUser)

	webAuth, hostKeyCallback := client.NewWebAuth(
		teleportFileSSHAgent,
		*proxyUser,
		passwordCallback,
		*webProxyAddress,
		*loginTTL,
	)

	authMethods := []ssh.AuthMethod{
		client.AuthMethodFromAgent(standartSSHAgent),
		client.AuthMethodFromAgent(teleportFileSSHAgent),
		webAuth,
	}

	err = trace.Errorf("No command")

	switch selectedCommand {
	case connect.FullCommand():
		err = SSH(*connectAddress, *proxy, *connectCommand,
			*connectPort, *connectSessionID, authMethods, hostKeyCallback)
	case getServers.FullCommand():
		err = GetServers(*proxy, *getServersLabelName,
			*getServersLabelValue, authMethods, hostKeyCallback)
	case scp.FullCommand():
		err = SCP(*proxy, *scpSource, *scpDest, *scpIsDir, *scpPort,
			authMethods, hostKeyCallback)
	case share.FullCommand():
		err = Share(*proxy, *shareReverseProxy, *shareNodeAddress, *shareAuthAddress, *shareReadOnly, authMethods, hostKeyCallback)
	case join.FullCommand():
		err = Join(*joinURL, authMethods, hostKeyCallback)
	}

	return err
}

func connectToSSHAgent(network, address string) (agent.Agent, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return agent.NewClient(conn), nil

}
