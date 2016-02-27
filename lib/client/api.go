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
package client

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/trace"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/terminal"
)

type Config struct {
	// teleport user login
	Login string

	// hostname of the proxy
	ProxyHost string

	// port (https) of the proxy
	ProxyPort int

	// TTL for the temporary SSH keypair to remain valid:
	KeyTTL time.Duration
}

type TeleportClient struct {
	Config
	localAgent  agent.Agent
	authMethods []ssh.AuthMethod
}

func NewClient(c *Config) (tc *TeleportClient, err error) {
	tc = &TeleportClient{
		Config:      *c,
		authMethods: make([]ssh.AuthMethod, 2),
	}

	// first, see if we can authenticate with credentials stored in
	// a local SSH agent:
	if sshAgent := connectToSSHAgent(); sshAgent != nil {
		tc.authMethods = append(tc.authMethods, AuthMethodFromAgent(sshAgent))
	}

	// then, we can authenticate via a locally stored cert previously
	// signed by the CA:
	localAgent, err := GetLocalAgent()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if localAgent != nil {
		tc.localAgent = localAgent
		tc.authMethods = append(tc.authMethods, AuthMethodFromAgent(localAgent))
	} else {
		log.Errorf("unable to obtain locally stored credentials")
	}

	// finally, interactive auth (via password + HTOP 2nd factor):
	tc.authMethods = append(tc.authMethods, tc.makeInteractiveAuthMethod())
	return tc, nil
}

// makeInteractiveAuthMethod creates an 'ssh.AuthMethod' which authenticates
// interactively by asknig a Teleport password + HTOP token
func (tc *TeleportClient) makeInteractiveAuthMethod() ssh.AuthMethod {
	callbackFunc := func() (signers []ssh.Signer, err error) {
		if err = tc.login(); err != nil {
			log.Error(err)
			return nil, trace.Wrap(err)
		}
		return tc.localAgent.Signers()
	}
	return ssh.PublicKeysCallback(callbackFunc)
}

// login
func (tc *TeleportClient) login() error {
	password, hotpToken, err := tc.AskPasswordAndHOTP()
	if err != nil {
		return trace.Wrap(err)
	}

	// generate a new keypair. the public key will be signed via proxy if our password+HOTP
	// are legit
	priv, pub, err := native.New().GenerateKeyPair("")
	if err != nil {
		return trace.Wrap(err)
	}

	// ask the CA (via proxy) to sign our public key:
	proxyHostPort := net.JoinHostPort(tc.ProxyHost, string(tc.ProxyPort))
	response, err := web.SSHAgentLogin(proxyHostPort, tc.Login, password, hotpToken, pub, tc.KeyTTL)
	if err != nil {
		return trace.Wrap(err)
	}

	// parse the returned&signed key:
	pcert, _, _, _, err := ssh.ParseAuthorizedKey(response.Cert)
	if err != nil {
		return trace.Wrap(err)
	}
	pk, err := ssh.ParseRawPrivateKey(priv)
	if err != nil {
		return trace.Wrap(err)
	}

	// store the newly generated key in the local key store
	addedKey := agent.AddedKey{
		PrivateKey:       pk,
		Certificate:      pcert.(*ssh.Certificate),
		Comment:          "",
		LifetimeSecs:     0,
		ConfirmBeforeUse: false,
	}
	if err := tc.localAgent.Add(addedKey); err != nil {
		return trace.Wrap(err)
	}

	key := Key{
		Priv:     priv,
		Cert:     response.Cert,
		Deadline: time.Now().Add(tc.KeyTTL),
	}

	keyID := int64(time.Now().Sub(time.Time{}).Seconds())*100 + rand.Int63n(100)
	keyPath := filepath.Join(KeysDir,
		KeyFilePrefix+strconv.FormatInt(keyID, 16)+KeyFileSuffix)

	err = saveKey(key, keyPath)
	if err != nil {
		return trace.Wrap(err)
	}

	err = AddHostSignersToCache(response.HostSigners)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("Logged in successfully")
	return nil
}

/*
func (this *TeleportClient) Connect() {
	var node *client.NodeClient
	if len(proxyAddress) > 0 {
		proxyClient, err := client.ConnectToProxy(proxyAddress, authMethods, hostKeyCallback, user)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()
		node, err = proxyClient.ConnectToNode(address, authMethods, hostKeyCallback, user)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		var err error
		node, err = client.ConnectToNode(nil, address, authMethods, hostKeyCallback, user)
		if err != nil {
			return trace.Wrap(err)
		}
	}
}
*/

// connects to a local SSH agent of exits, printing an error message
func connectToSSHAgent() agent.Agent {
	socketPath := os.Getenv("SSH_AUTH_SOCK")
	if socketPath == "" {
		log.Infof("SSH_AUTH_SOCK is not set. Is local SSH agent running?")
		return nil
	}
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Errorf("Failed connecting to local SSH agent via %s", socketPath)
		return nil
	}
	return agent.NewClient(conn)
}

// username returns the current user's username
func Username() string {
	u, err := user.Current()
	if err != nil {
		utils.FatalError(err)
	}
	return u.Username
}

// AskPasswordAndHOTP prompts the user to enter the password + HTOP 2nd factor
func (tc *TeleportClient) AskPasswordAndHOTP() (pwd string, token string, err error) {
	fmt.Printf("Enter password for %v:\n", tc.Login)
	pwd, err = readPassword()
	if err != nil {
		fmt.Println(err)
		return "", "", trace.Wrap(err)
	}

	fmt.Printf("Enter your HOTP token:\n")
	token, err = readLine()
	if err != nil {
		fmt.Println(err)
		return "", "", trace.Wrap(err)
	}
	return pwd, token, nil
}

func readPassword() (string, error) {
	bytes, err := terminal.ReadPassword(syscall.Stdin)
	return string(bytes), err
}

func readLine() (string, error) {
	bytes, _, err := bufio.NewReader(os.Stdin).ReadLine()
	return string(bytes), err
}
