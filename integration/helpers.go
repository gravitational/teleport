/*
Copyright 2018 Gravitational, Inc.

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

package integration

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	Loopback = "127.0.0.1"
	Host     = "localhost"
)

// commandOptions controls how the SSH command is built.
type commandOptions struct {
	forwardAgent bool
	forcePTY     bool
	controlPath  string
	socketPath   string
	proxyPort    string
	nodePort     string
	command      string
}

// externalSSHCommand runs an external SSH command (if an external ssh binary
// exists) with the passed in parameters.
func externalSSHCommand(o commandOptions) (*exec.Cmd, error) {
	var execArgs []string

	// Don't check the host certificate as part of the testing an external SSH
	// client, this is done elsewhere.
	execArgs = append(execArgs, "-oStrictHostKeyChecking=no")
	execArgs = append(execArgs, "-oUserKnownHostsFile=/dev/null")

	// ControlMaster is often used by applications like Ansible.
	if o.controlPath != "" {
		execArgs = append(execArgs, "-oControlMaster=auto")
		execArgs = append(execArgs, "-oControlPersist=1s")
		execArgs = append(execArgs, "-oConnectTimeout=2")
		execArgs = append(execArgs, fmt.Sprintf("-oControlPath=%v", o.controlPath))
	}

	// The -tt flag is used to force PTY allocation. It's often used by
	// applications like Ansible.
	if o.forcePTY {
		execArgs = append(execArgs, "-tt")
	}

	// Connect to node on the passed in port.
	execArgs = append(execArgs, fmt.Sprintf("-p %v", o.nodePort))

	// Build proxy command.
	proxyCommand := []string{"ssh"}
	proxyCommand = append(proxyCommand, "-oStrictHostKeyChecking=no")
	proxyCommand = append(proxyCommand, "-oUserKnownHostsFile=/dev/null")
	if o.forwardAgent {
		proxyCommand = append(proxyCommand, "-oForwardAgent=yes")
	}
	proxyCommand = append(proxyCommand, fmt.Sprintf("-p %v", o.proxyPort))
	proxyCommand = append(proxyCommand, `%r@localhost -s proxy:%h:%p`)

	// Add in ProxyCommand option, needed for all Teleport connections.
	execArgs = append(execArgs, fmt.Sprintf("-oProxyCommand=%v", strings.Join(proxyCommand, " ")))

	// Add in the host to connect to and the command to run when connected.
	execArgs = append(execArgs, Host)
	execArgs = append(execArgs, o.command)

	// Find the OpenSSH binary.
	sshpath, err := exec.LookPath("ssh")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create an exec.Command and tell it where to find the SSH agent.
	cmd, err := exec.Command(sshpath, execArgs...), nil
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cmd.Env = []string{fmt.Sprintf("SSH_AUTH_SOCK=%v", o.socketPath)}

	return cmd, nil
}

// createAgent creates a SSH agent with the passed in private key and
// certificate that can be used in tests. This is useful so tests don't
// clobber your system agent.
func createAgent(me *user.User, privateKeyByte []byte, certificateBytes []byte) (*teleagent.AgentServer, string, string, error) {
	// create a path to the unix socket
	sockDirName := "int-test"
	sockName := "agent.sock"

	// transform the key and certificate bytes into something the agent can understand
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(certificateBytes)
	if err != nil {
		return nil, "", "", trace.Wrap(err)
	}
	privateKey, err := ssh.ParseRawPrivateKey(privateKeyByte)
	if err != nil {
		return nil, "", "", trace.Wrap(err)
	}
	agentKey := agent.AddedKey{
		PrivateKey:       privateKey,
		Certificate:      publicKey.(*ssh.Certificate),
		Comment:          "",
		LifetimeSecs:     0,
		ConfirmBeforeUse: false,
	}

	// create a (unstarted) agent and add the key to it
	keyring := agent.NewKeyring()
	if err := keyring.Add(agentKey); err != nil {
		return nil, "", "", trace.Wrap(err)
	}
	teleAgent := teleagent.NewServer(func() (teleagent.Agent, error) {
		return teleagent.NopCloser(keyring), nil
	})

	// start the SSH agent
	err = teleAgent.ListenUnixSocket(sockDirName, sockName, me)
	if err != nil {
		return nil, "", "", trace.Wrap(err)
	}
	go teleAgent.Serve()

	return teleAgent, teleAgent.Dir, teleAgent.Path, nil
}

func closeAgent(teleAgent *teleagent.AgentServer, socketDirPath string) error {
	err := teleAgent.Close()
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.RemoveAll(socketDirPath)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// getLocalIP gets the non-loopback IP address of this host.
func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", trace.Wrap(err)
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		default:
			continue
		}
		if !ip.IsLoopback() && ip.IsPrivate() {
			return ip.String(), nil
		}
	}
	return "", trace.NotFound("No non-loopback local IP address found")
}

func MustCreateUserIdentityFile(t *testing.T, tc *helpers.TeleInstance, username string, ttl time.Duration) string {
	key, err := libclient.NewKey()
	require.NoError(t, err)
	key.ClusterName = tc.Secrets.SiteName

	sshCert, tlsCert, err := tc.Process.GetAuthServer().GenerateUserTestCerts(
		key.Pub, username, ttl,
		constants.CertificateFormatStandard,
		tc.Secrets.SiteName, "",
	)
	require.NoError(t, err)

	key.Cert = sshCert
	key.TLSCert = tlsCert

	hostCAs, err := tc.Process.GetAuthServer().GetCertAuthorities(context.Background(), types.HostCA, false)
	require.NoError(t, err)
	key.TrustedCA = auth.AuthoritiesToTrustedCerts(hostCAs)

	idPath := filepath.Join(t.TempDir(), "user_identity")
	_, err = identityfile.Write(identityfile.WriteConfig{
		OutputPath: idPath,
		Key:        key,
		Format:     identityfile.FormatFile,
	})
	require.NoError(t, err)
	return idPath
}
