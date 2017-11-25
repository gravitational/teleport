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
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

type LocalKeyAgent struct {
	agent.Agent               // Agent is the teleport agent
	keyStore    LocalKeyStore // keyStore is the storage backend for certificates and keys
	sshAgent    agent.Agent   // sshAgent is the system ssh agent

	// map of "no hosts". these are hosts that user manually (via keyboard
	// input) refused connecting to.
	noHosts map[string]bool

	// function which asks a user to trust host/key combination (during host auth)
	hostPromptFunc func(host string, k ssh.PublicKey) error
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
		noHosts:  make(map[string]bool),
	}

	// unload all teleport keys from the agent first to ensure
	// we don't leave stale keys in the agent
	err = a.UnloadKeys()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// read all keys from disk (~/.tsh usually)
	keys, err := a.GetKeys(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Infof("[KEY AGENT] Loading %v keys for %q", len(keys), username)

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
				log.Warnf("[KEY AGENT] Unable to communicate with agent and add key: %v", err)
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

// UnloadKeys will unload all Teleport keys from the teleport agent as well as the system agent.
func (a *LocalKeyAgent) UnloadKeys() error {
	agents := []agent.Agent{a.Agent}
	if a.sshAgent != nil {
		agents = append(agents, a.sshAgent)
	}

	// iterate over all agents we have
	for i, _ := range agents {
		// get a list of all keys in the agent
		keyList, err := agents[i].List()
		if err != nil {
			log.Warnf("Unable to communicate with agent and list keys: %v", err)
		}

		// remove any teleport keys we currently have loaded in the agent
		for _, key := range keyList {
			if strings.HasPrefix(key.Comment, "teleport:") {
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
func (a *LocalKeyAgent) AddHostSignersToCache(certAuthorities []auth.TrustedCerts) error {
	for _, ca := range certAuthorities {
		publicKeys, err := ca.SSHCertPublicKeys()
		if err != nil {
			log.Error(err)
			return trace.Wrap(err)
		}
		log.Debugf("[KEY AGENT] adding CA key for %s", ca.ClusterName)
		err = a.keyStore.AddKnownHostKeys(ca.ClusterName, publicKeys)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (a *LocalKeyAgent) SaveCerts(proxy string, certAuthorities []auth.TrustedCerts) error {
	return a.keyStore.SaveCerts(proxy, certAuthorities)
}

func (a *LocalKeyAgent) GetCerts(proxy string) (*x509.CertPool, error) {
	return a.keyStore.GetCerts(proxy)
}

// UserRefusedHosts returns 'true' if a user refuses connecting to remote hosts
// when prompted during host authorization
func (a *LocalKeyAgent) UserRefusedHosts() bool {
	return len(a.noHosts) > 0
}

// CheckHostSignature checks if the given host key was signed by one of the trusted
// certificaate authorities (CAs)
func (a *LocalKeyAgent) CheckHostSignature(host string, remote net.Addr, key ssh.PublicKey) error {
	hostPromptFunc := func(host string, key ssh.PublicKey) error {
		userAnswer := "no"
		if !a.noHosts[host] {
			fmt.Printf("The authenticity of host '%s' can't be established. "+
				"Its public key is:\n%s\nAre you sure you want to continue (yes/no)? ",
				host, ssh.MarshalAuthorizedKey(key))

			bytes := make([]byte, 12)
			os.Stdin.Read(bytes)
			userAnswer = strings.TrimSpace(strings.ToLower(string(bytes)))
		}
		if !strings.HasPrefix(userAnswer, "y") {
			return trace.AccessDenied("untrusted host %v", host)
		}
		// success
		return nil
	}
	// overwritten host prompt func? (probably for tests)
	if a.hostPromptFunc != nil {
		hostPromptFunc = a.hostPromptFunc
	}
	cert, ok := key.(*ssh.Certificate)
	if !ok {
		// not a signed cert? perhaps we're given a host public key (happens when the host is running
		// sshd instead of teleport daemon
		keys, _ := a.keyStore.GetKnownHostKeys(host)
		if len(keys) > 0 && sshutils.KeysEqual(key, keys[0]) {
			log.Debugf("[KEY AGENT] verified host %s", host)
			return nil
		}
		// ask user:
		if err := hostPromptFunc(host, key); err != nil {
			a.noHosts[host] = true
			return trace.Wrap(err)
		}
		// remember the host key (put it into 'known_hosts')
		if err := a.keyStore.AddKnownHostKeys(host, []ssh.PublicKey{key}); err != nil {
			log.Warnf("error saving the host key: %v", err)
		}
		return nil
	}
	key = cert.SignatureKey
	// we are given a certificate. see if it was signed by any of the known_host keys:
	keys, err := a.keyStore.GetKnownHostKeys("")
	if err != nil {
		log.Error(err)
		return trace.Wrap(err)
	}
	log.Debugf("[KEY AGENT] got %d known hosts", len(keys))
	for i := range keys {
		if sshutils.KeysEqual(cert.SignatureKey, keys[i]) {
			log.Debugf("[KEY AGENT] verified host %s", host)
			return nil
		}
	}
	// final step: if we have not seen the host key/cert before, lets ask the user if
	// he trusts it, and add to the known_hosts if he says "yes"
	if err = hostPromptFunc(host, key); err != nil {
		// he said "no"
		a.noHosts[host] = true
		return trace.Wrap(err)
	}
	// user said "yes"
	err = a.keyStore.AddKnownHostKeys(host, []ssh.PublicKey{key})
	if err != nil {
		log.Warn(err)
	}
	return err
}

// AddKey activates a new signed session key by adding it into the keystore and also
// by loading it into the SSH agent
func (a *LocalKeyAgent) AddKey(host string, username string, key *Key) (*agent.AddedKey, error) {
	// save it to disk (usually into ~/.tsh)
	err := a.keyStore.AddKey(host, username, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// load key into the teleport agent and system agent
	return a.LoadKey(username, *key)
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

// AuthMethods returns the list of different authentication methods this agent supports
// It returns two:
//	  1. First to try is the external SSH agent
//    2. Itself (disk-based local agent)
func (a *LocalKeyAgent) AuthMethods() (m []ssh.AuthMethod) {
	// combine our certificates with external SSH agent's:
	var signers []ssh.Signer
	if ourCerts, _ := a.Signers(); ourCerts != nil {
		signers = append(signers, ourCerts...)
	}
	if a.sshAgent != nil {
		if sshAgentCerts, _ := a.sshAgent.Signers(); sshAgentCerts != nil {
			signers = append(signers, sshAgentCerts...)
		}
	}
	// for every certificate create a new "auth method" and return them
	m = make([]ssh.AuthMethod, 0)
	for i := range signers {
		// filter out non-certificates (like regular public SSH keys stored in the SSH agent):
		_, ok := signers[i].PublicKey().(*ssh.Certificate)
		if ok {
			m = append(m, NewAuthMethodForCert(signers[i]))
		}
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

func NewAuthMethodForCert(cert ssh.Signer) *CertAuthMethod {
	return &CertAuthMethod{
		Cert: cert,
		AuthMethod: ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
			return []ssh.Signer{cert}, nil
		}),
	}
}
