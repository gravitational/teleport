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

package client

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type LocalKeyAgent struct {
	// implements ssh agent.Agent interface
	agent.Agent
	keyStore LocalKeyStore
}

// NewLocalAgent loads all the saved teleport certificates and
// creates ssh agent with them
func NewLocalAgent(keyDir, username string) (a *LocalKeyAgent, err error) {
	keystore, err := NewFSLocalKeyStore(keyDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	a = &LocalKeyAgent{
		Agent:    agent.NewKeyring(),
		keyStore: keystore,
	}
	// add all stored keys into the agent:
	keys, err := a.GetKeys(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for i := range keys {
		if err := a.Agent.Add(keys[i]); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return a, nil
}

// GetKeys return the list of keys for the given user
// from the local keystore (files in ~/.tsh)
func (a *LocalKeyAgent) GetKeys(username string) ([]agent.AddedKey, error) {
	keys, err := a.keyStore.GetKeys(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	retval := make([]agent.AddedKey, len(keys))
	for i, key := range keys {
		ak, err := key.AsAgentKey()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		retval[i] = *ak
	}
	return retval, nil
}

// AddHostSignersToCache takes a list of CAs whom we trust. This list is added to a database
// of "seen" CAs.
//
// Every time we connect to a new host, we'll request its certificaate to be signed by one
// of these trusted CAs.
//
// Why do we trust these CAs? Because we received them from a trusted Teleport Proxy.
// Why do we trust the proxy? Because we've connected to it via HTTPS + username + Password + HOTP.
func (a *LocalKeyAgent) AddHostSignersToCache(hostSigners []services.CertAuthority) error {
	for _, hostSigner := range hostSigners {
		publicKeys, err := hostSigner.Checkers()
		if err != nil {
			log.Error(err)
			return trace.Wrap(err)
		}
		a.keyStore.AddKnownHostKeys(hostSigner.DomainName, publicKeys)
	}
	return nil
}

// CheckHostSignature checks if the given host key was signed by one of the trusted
// certificaate authorities (CAs)
func (a *LocalKeyAgent) CheckHostSignature(hostId string, remote net.Addr, key ssh.PublicKey) error {
	log.Debugf("checking host key of %s\n", hostId)

	cert, ok := key.(*ssh.Certificate)
	if !ok {
		// not a signed cert? perhaps we're given a host public key (happens when the host is running
		// sshd instead of teleport daemon
		keys, _ := a.keyStore.GetKnownHostKeys(hostId)
		if len(keys) > 0 && sshutils.KeysEqual(key, keys[0]) {
			return nil
		}
		// ask the user if they want to trust this host
		fmt.Printf("The authenticity of host '%s' can't be established. "+
			"Its public key is:\n%s\nAre you sure you want to continue (yes/no)? ",
			hostId, ssh.MarshalAuthorizedKey(key))

		bytes := make([]byte, 12)
		os.Stdin.Read(bytes)
		if strings.TrimSpace(strings.ToLower(string(bytes)))[0] != 'y' {
			return trace.AccessDenied("Host key verification failed.")
		}
		// remember the host key (put it into 'known_hosts')
		if err := a.keyStore.AddKnownHostKeys(hostId, []ssh.PublicKey{key}); err != nil {
			log.Warnf("error saving the host key: %v", err)
		}
		// success
		return nil
	}
	// we are given a certificate. see if it was signed by any of the known_host keys:
	keys, err := a.keyStore.GetKnownHostKeys("")
	if err != nil {
		log.Error(err)
		return trace.Wrap(err)
	}
	for i := range keys {
		if sshutils.KeysEqual(cert.SignatureKey, keys[i]) {
			return nil
		}
	}
	err = trace.AccessDenied("could not find matching keys for %v at %v", hostId, remote)
	log.Error(err)
	return trace.Wrap(err)
}

func (a *LocalKeyAgent) AddKey(host string, username string, key *Key) error {
	err := a.keyStore.AddKey(host, username, key)
	if err != nil {
		return trace.Wrap(err)
	}
	agentKey, err := key.AsAgentKey()
	if err != nil {
		return trace.Wrap(err)
	}
	return a.Agent.Add(*agentKey)
}

func (a *LocalKeyAgent) DeleteKey(host string, username string) error {
	return trace.Wrap(a.keyStore.DeleteKey(host, username))
}
