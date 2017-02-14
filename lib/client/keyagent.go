/*
Copyright 2017 Gravitational, Inc.

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

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/trace"

	log "github.com/Sirupsen/logrus"
)

type LocalKeyAgent struct {
	agent.Agent               // Agent is the teleport agent
	keyStore    LocalKeyStore // keyStore is the storage backend for certificates and keys
	sshAgent    agent.Agent   // sshAgent is the system ssh agent
}

// NewLocalAgent reads all Teleport certificates from disk (using FSLocalKeyStore),
// creates a LocalKeyAgent, loads all certificates into it, and returns the agent.
func NewLocalAgent(keyDir, username string) (a *LocalKeyAgent, err error) {
	keystore, err := NewFSLocalKeyStore(keyDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a = &LocalKeyAgent{
		Agent:    agent.NewKeyring(),
		keyStore: keystore,
		sshAgent: connectToSSHAgent(),
	}

	// read all keys from disk (~/.tsh usually)
	keys, err := a.GetKeys(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// load all keys into the agent
	for _, key := range keys {
		_, err = a.LoadKey(username, key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return a, nil
}

// LoadKey adds a key into the teleport ssh agent as well as the system ssh agent.
func (a *LocalKeyAgent) LoadKey(username string, key Key) (*agent.AddedKey, error) {
	agents := []agent.Agent{a.Agent}
	if a.sshAgent != nil {
		agents = append(agents, a.sshAgent)
	}

	// convert keys into a format understood by the ssh agent
	agentKeys, err := key.AsAgentKeys()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// remove any keys that the user may already have loaded
	err = a.UnloadKey(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// iterate over all teleport and system agent and load key
	for i, _ := range agents {
		for _, agentKey := range agentKeys {
			err = agents[i].Add(*agentKey)
			if err != nil {
				log.Warnf("Unable to communicate with agent and add key: %v", err)
			}
		}
	}

	// return the first key because it has the embedded private key in it.
	// see docs for AsAgentKeys for more details.
	return agentKeys[0], nil
}

// UnloadKey will unload a key from the teleport ssh agent as well as the system agent.
func (a *LocalKeyAgent) UnloadKey(username string) error {
	agents := []agent.Agent{a.Agent}
	if a.sshAgent != nil {
		agents = append(agents, a.sshAgent)
	}

	// iterate over all agents we have and unload keys for this user
	for i, _ := range agents {
		// get a list of all keys in the agent
		keyList, err := agents[i].List()
		if err != nil {
			log.Warnf("Unable to communicate with agent and list keys: %v", err)
		}

		// remove any teleport keys we currently have loaded in the agent for this user
		for _, key := range keyList {
			if key.Comment == fmt.Sprintf("teleport:%v", username) {
				err = agents[i].Remove(key)
				if err != nil {
					log.Warnf("Unable to communicate with agent and remove key: %v", err)
				}
			}
		}
	}

	return nil
}

// GetKeys returns a slice of keys that it has read in from the local keystore (~/.tsh)
func (a *LocalKeyAgent) GetKeys(username string) ([]Key, error) {
	return a.keyStore.GetKeys(username)
}

// AddHostSignersToCache takes a list of CAs whom we trust. This list is added to a database
// of "seen" CAs.
//
// Every time we connect to a new host, we'll request its certificaate to be signed by one
// of these trusted CAs.
//
// Why do we trust these CAs? Because we received them from a trusted Teleport Proxy.
// Why do we trust the proxy? Because we've connected to it via HTTPS + username + Password + HOTP.
func (a *LocalKeyAgent) AddHostSignersToCache(hostSigners []services.CertAuthorityV1) error {
	for _, hostSigner := range hostSigners {
		publicKeys, err := hostSigner.V2().Checkers()
		if err != nil {
			log.Error(err)
			return trace.Wrap(err)
		}
		log.Debugf("[KEY AGENT] adding CA key for %s", hostSigner.DomainName)
		err = a.keyStore.AddKnownHostKeys(hostSigner.DomainName, publicKeys)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// CheckHostSignature checks if the given host key was signed by one of the trusted
// certificaate authorities (CAs)
func (a *LocalKeyAgent) CheckHostSignature(hostId string, remote net.Addr, key ssh.PublicKey) error {
	cert, ok := key.(*ssh.Certificate)
	if !ok {
		// not a signed cert? perhaps we're given a host public key (happens when the host is running
		// sshd instead of teleport daemon
		keys, _ := a.keyStore.GetKnownHostKeys(hostId)
		if len(keys) > 0 && sshutils.KeysEqual(key, keys[0]) {
			log.Debugf("[KEY AGENT] verified host %s", hostId)
			return nil
		}
		// ask the user if they want to trust this host
		fmt.Printf("The authenticity of host '%s' can't be established. "+
			"Its public key is:\n%s\nAre you sure you want to continue (yes/no)? ",
			hostId, ssh.MarshalAuthorizedKey(key))

		bytes := make([]byte, 12)
		os.Stdin.Read(bytes)
		if strings.TrimSpace(strings.ToLower(string(bytes)))[0] != 'y' {
			err := trace.AccessDenied("untrusted host %v", hostId)
			log.Error(err)
			return err
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
	log.Debugf("[KEY AGENT] got %d known hosts", len(keys))
	for i := range keys {
		if sshutils.KeysEqual(cert.SignatureKey, keys[i]) {
			log.Debugf("[KEY AGENT] verified host %s", hostId)
			return nil
		}
	}
	err = trace.AccessDenied("untrusted host %v", hostId)
	log.Error(err)
	return err
}

// AddKey stores a new signed session key for future use.
//
// It returns an implementation of ssh.Authmethod which can be passed to ssh.Config
// to make new SSH connections authenticated by this key.
//
func (a *LocalKeyAgent) AddKey(host string, username string, key *Key) (*CertAuthMethod, error) {
	// save it to disk (usually into ~/.tsh)
	err := a.keyStore.AddKey(host, username, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// load key into the teleport agent and system agent
	agentKey, err := a.LoadKey(username, *key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// generate SSH auth method based on the given signed key and return
	// it to the caller:
	signer, err := ssh.NewSignerFromKey(agentKey.PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if signer, err = ssh.NewCertSigner(agentKey.Certificate, signer); err != nil {
		return nil, trace.Wrap(err)
	}

	return methodForCert(signer), nil
}

// DeleteKey removes the key from the key store as well as unloading the key from the agent.
func (a *LocalKeyAgent) DeleteKey(proxyHost string, username string) error {
	// remove key from key store
	err := a.keyStore.DeleteKey(proxyHost, username)
	if err != nil {
		return trace.Wrap(err)
	}

	// remove any keys that are loaded for this user from the teleport and system agents
	err = a.UnloadKey(username)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// AuthMethods returns the list of differnt authentication methods this agent supports
// It returns two:
//	  1. First to try is the external SSH agent
//    2. Itself (disk-based local agent)
func (a *LocalKeyAgent) AuthMethods() (m []ssh.AuthMethod) {
	// combine our certificates with external SSH agent's:
	var certs []ssh.Signer
	if ourCerts, _ := a.Signers(); ourCerts != nil {
		certs = append(certs, ourCerts...)
	}
	if a.sshAgent != nil {
		if sshAgentCerts, _ := a.sshAgent.Signers(); sshAgentCerts != nil {
			certs = append(certs, sshAgentCerts...)
		}
	}
	// for every certificate create a new "auth method" and return them
	m = make([]ssh.AuthMethod, len(certs))
	for i := range certs {
		m[i] = methodForCert(certs[i])
	}
	return m
}

// CertAuthMethod is a wrapper around ssh.Signer (certificate signer) object.
// CertAuthMethod then implements ssh.Authmethod interface around this one certificate signer.
//
// We need this wrapper because Golang's SSH library's unfortunate API design. It uses
// callbacks with 'authMethod' interfaces and without this wrapper it is impossible to
// tell which certificate an 'authMethod' passed via a callback had succeeded authenticating with.
type CertAuthMethod struct {
	ssh.AuthMethod
	Cert ssh.Signer
}

func methodForCert(cert ssh.Signer) *CertAuthMethod {
	return &CertAuthMethod{
		Cert: cert,
		AuthMethod: ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
			return []ssh.Signer{cert}, nil
		}),
	}
}
