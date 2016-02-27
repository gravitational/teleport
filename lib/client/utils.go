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
// package auth implements certificate signing authority and access control server
// Authority server is composed of several parts:
//
// * Authority server itself that implements signing and acl logic
// * HTTP server wrapper for authority server
// * HTTP client wrapper
//

package client

import (
	"fmt"
	"math/rand"
	"net"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func AuthMethodFromAgent(ag agent.Agent) ssh.AuthMethod {
	return ssh.PublicKeysCallback(ag.Signers)
}

// NewWebAuth returns ssh.AuthMethod as a callback function and SSH HostKeyCallback.
// When any of them is called it tries to generate certificates using password and
// hotpToken and adds the login certificate to the provided agent, saves the
// certificates to the local folder and returns the agent as authenticator.
func NewWebAuth(ag agent.Agent,
	user string,
	passwordCallback PasswordCallback,
	webProxyAddress string,
	certificateTTL time.Duration) (authMethod ssh.AuthMethod, hostKeyCallback utils.HostKeyCallback) {

	callbackFunc := func() (signers []ssh.Signer, err error) {
		err = Login(ag, webProxyAddress, user, certificateTTL, passwordCallback)
		if err != nil {
			fmt.Printf("Can't login to the server: %v\n", err)
			return nil, trace.Wrap(err)
		}

		return ag.Signers()
	}

	hostKeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := CheckHostSignerFromCache(hostname, remote, key)
		if err != nil {
			err = Login(ag, webProxyAddress, user, certificateTTL, passwordCallback)
			if err != nil {
				fmt.Printf("Can't login to %v\n", err)
				return trace.Wrap(err)
			}
			return CheckHostSignerFromCache(hostname, remote, key)
		}
		return nil
	}

	return ssh.PublicKeysCallback(callbackFunc), hostKeyCallback
}

type PasswordCallback func() (password, hotpToken string, e error)

// Login tries to generate
// teleport certificate using password and hotpToken, adds the
// certificate to the provided agent and saves the certificate to the
// local folder.
func Login(ag agent.Agent, webProxyAddr string, user string,
	ttl time.Duration, passwordCallback PasswordCallback) error {

	password, hotpToken, err := passwordCallback()
	if err != nil {
		fmt.Println(err)
		return trace.Wrap(err)
	}

	fmt.Printf("Logging in...\n")

	priv, pub, err := native.New().GenerateKeyPair("")
	if err != nil {
		return trace.Wrap(err)
	}

	login, err := web.SSHAgentLogin(webProxyAddr, user, password, hotpToken,
		pub, ttl)
	if err != nil {
		return trace.Wrap(err)
	}

	pcert, _, _, _, err := ssh.ParseAuthorizedKey(login.Cert)
	if err != nil {
		return trace.Wrap(err)
	}

	pk, err := ssh.ParseRawPrivateKey(priv)
	if err != nil {
		return trace.Wrap(err)
	}
	addedKey := agent.AddedKey{
		PrivateKey:       pk,
		Certificate:      pcert.(*ssh.Certificate),
		Comment:          "",
		LifetimeSecs:     0,
		ConfirmBeforeUse: false,
	}
	if err := ag.Add(addedKey); err != nil {
		return trace.Wrap(err)
	}

	key := Key{
		Priv:     priv,
		Cert:     login.Cert,
		Deadline: time.Now().Add(ttl),
	}

	keyID := int64(time.Now().Sub(time.Time{}).Seconds())*100 + rand.Int63n(100)
	keyPath := filepath.Join(KeysDir,
		KeyFilePrefix+strconv.FormatInt(keyID, 16)+KeyFileSuffix)

	err = saveKey(key, keyPath)
	if err != nil {
		return trace.Wrap(err)
	}

	err = AddHostSignersToCache(login.HostSigners)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("Logged in successfully")
	return nil
}

// GetPasswordFromConsole returns a function which requests password+HOTP token
// from the console
func GetPasswordFromConsole(user string) PasswordCallback {
	return func() (password, hotpToken string, e error) {
		fmt.Printf("Enter password for %v:\n", user)
		password, err := readPassword()
		if err != nil {
			fmt.Println(err)
			return "", "", trace.Wrap(err)
		}

		fmt.Printf("Enter your HOTP token:\n")
		hotpToken, err = readPassword()
		if err != nil {
			fmt.Println(err)
			return "", "", trace.Wrap(err)
		}

		return password, hotpToken, nil
	}
}
