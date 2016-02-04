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
	"path/filepath"
	"strconv"
	"time"

	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/web"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/terminal"
)

func AuthMethodFromAgent(ag agent.Agent) ssh.AuthMethod {
	return ssh.PublicKeysCallback(ag.Signers)
}

// GenerateCertificateCallback returns ssh.AuthMethod as
// a callback function. When callback is called, it tries to generate
// teleport certificate using password and hotpToken, adds the
// certificate to the provided agent, saves the certificate to the
// local folder and returns the agent as authenticator.
func NewWebAuth(ag agent.Agent,
	user string,
	passwordCallback PasswordCallback,
	webProxyAddress string,
	certificateTTL time.Duration) ssh.AuthMethod {

	callbackFunc := func() (signers []ssh.Signer, err error) {
		err = Login(ag, webProxyAddress, user, certificateTTL, passwordCallback)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return ag.Signers()
	}

	return ssh.PublicKeysCallback(callbackFunc)
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

	cert, err := web.SSHAgentLogin(webProxyAddr, user, password, hotpToken,
		pub, ttl)
	if err != nil {
		return trace.Wrap(err)
	}

	pcert, _, _, _, err := ssh.ParseAuthorizedKey(cert)
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
		Cert:     cert,
		Deadline: time.Now().Add(ttl),
	}

	keyID := int64(time.Now().Sub(time.Time{}).Seconds())*100 + rand.Int63n(100)
	keyPath := filepath.Join(KeysDir,
		KeyFilePrefix+strconv.FormatInt(keyID, 16)+KeyFileSuffix)

	err = saveKey(key, keyPath)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("Logged in successfully")
	return nil
}

func GetPasswordFromConsole(user string) PasswordCallback {
	return func() (password, hotpToken string, e error) {
		fmt.Printf("Enter password for user %v:\n", user)
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

func readPassword() (string, error) {
	password, err := terminal.ReadPassword(0)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(password), nil
}
