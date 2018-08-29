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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

// LocalKeyAgent holds Teleport certificates for a user connected to a cluster.
type LocalKeyAgent struct {
	// log holds the structured logger.
	log *logrus.Entry

	// Agent is the teleport agent
	agent.Agent

	// keyStore is the storage backend for certificates and keys
	keyStore LocalKeyStore

	// sshAgent is the system ssh agent
	sshAgent agent.Agent

	// map of "no hosts". these are hosts that user manually (via keyboard
	// input) refused connecting to.
	noHosts map[string]bool

	// function which asks a user to trust host/key combination (during host auth)
	hostPromptFunc func(host string, k ssh.PublicKey) error

	// username is the Teleport username for who the keys will be loaded in the
	// local agent.
	username string

	// proxyHost is the proxy for the cluster that his key agent holds keys for.
	proxyHost string
}

// NewLocalAgent reads all Teleport certificates from disk (using FSLocalKeyStore),
// creates a LocalKeyAgent, loads all certificates into it, and returns the agent.
func NewLocalAgent(keyDir string, proxyHost string, username string) (a *LocalKeyAgent, err error) {
	keystore, err := NewFSLocalKeyStore(keyDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a = &LocalKeyAgent{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentKeyAgent,
		}),
		Agent:     agent.NewKeyring(),
		keyStore:  keystore,
		sshAgent:  connectToSSHAgent(),
		noHosts:   make(map[string]bool),
		username:  username,
		proxyHost: proxyHost,
	}

	// unload all teleport keys from the agent first to ensure
	// we don't leave stale keys in the agent
	err = a.UnloadKeys()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// read in key for this user in proxy
	key, err := a.GetKey()
	if err != nil {
		if trace.IsNotFound(err) {
			return a, nil
		}
		return nil, trace.Wrap(err)
	}

	a.log.Infof("Loading key for %q", username)

	// load key into the agent
	_, err = a.LoadKey(*key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return a, nil
}

// UpdateProxyHost changes the proxy host that the local agent operates on.
func (a *LocalKeyAgent) UpdateProxyHost(proxyHost string) {
	a.proxyHost = proxyHost
}

// LoadKey adds a key into the Teleport ssh agent as well as the system ssh
// agent.
func (a *LocalKeyAgent) LoadKey(key Key) (*agent.AddedKey, error) {
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
	err = a.UnloadKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// iterate over all teleport and system agent and load key
	for i, _ := range agents {
		for _, agentKey := range agentKeys {
			err = agents[i].Add(*agentKey)
			if err != nil {
				a.log.Warnf("Unable to communicate with agent and add key: %v", err)
			}
		}
	}

	// return the first key because it has the embedded private key in it.
	// see docs for AsAgentKeys for more details.
	return agentKeys[0], nil
}

// UnloadKey will unload key for user from the teleport ssh agent as well as
// the system agent.
func (a *LocalKeyAgent) UnloadKey() error {
	agents := []agent.Agent{a.Agent}
	if a.sshAgent != nil {
		agents = append(agents, a.sshAgent)
	}

	// iterate over all agents we have and unload keys for this user
	for i, _ := range agents {
		// get a list of all keys in the agent
		keyList, err := agents[i].List()
		if err != nil {
			a.log.Warnf("Unable to communicate with agent and list keys: %v", err)
		}

		// remove any teleport keys we currently have loaded in the agent for this user
		for _, key := range keyList {
			if key.Comment == fmt.Sprintf("teleport:%v", a.username) {
				err = agents[i].Remove(key)
				if err != nil {
					a.log.Warnf("Unable to communicate with agent and remove key: %v", err)
				}
			}
		}
	}

	return nil
}

// UnloadKeys will unload all Teleport keys from the teleport agent as well as
// the system agent.
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
			a.log.Warnf("Unable to communicate with agent and list keys: %v", err)
		}

		// remove any teleport keys we currently have loaded in the agent
		for _, key := range keyList {
			if strings.HasPrefix(key.Comment, "teleport:") {
				err = agents[i].Remove(key)
				if err != nil {
					a.log.Warnf("Unable to communicate with agent and remove key: %v", err)
				}
			}
		}
	}

	return nil
}

// GetKey returns the key for this user in a proxy from the filesystem keystore
// at ~/.tsh.
func (a *LocalKeyAgent) GetKey() (*Key, error) {
	return a.keyStore.GetKey(a.proxyHost, a.username)
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
			a.log.Error(err)
			return trace.Wrap(err)
		}
		a.log.Debugf("Adding CA key for %s", ca.ClusterName)
		err = a.keyStore.AddKnownHostKeys(ca.ClusterName, publicKeys)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (a *LocalKeyAgent) SaveCerts(certAuthorities []auth.TrustedCerts) error {
	return a.keyStore.SaveCerts(a.proxyHost, certAuthorities)
}

func (a *LocalKeyAgent) GetCerts() (*x509.CertPool, error) {
	return a.keyStore.GetCerts(a.proxyHost)
}

func (a *LocalKeyAgent) GetCertsPEM() ([]byte, error) {
	return a.keyStore.GetCertsPEM(a.proxyHost)
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
			a.log.Debugf("Verified host %s", host)
			return nil
		}
		// ask user:
		if err := hostPromptFunc(host, key); err != nil {
			a.noHosts[host] = true
			return trace.Wrap(err)
		}
		// remember the host key (put it into 'known_hosts')
		if err := a.keyStore.AddKnownHostKeys(host, []ssh.PublicKey{key}); err != nil {
			a.log.Warnf("Error saving the host key: %v", err)
		}
		return nil
	}
	key = cert.SignatureKey
	// we are given a certificate. see if it was signed by any of the known_host keys:
	keys, err := a.keyStore.GetKnownHostKeys("")
	if err != nil {
		a.log.Error(err)
		return trace.Wrap(err)
	}
	a.log.Debugf("Got %d known hosts", len(keys))
	for i := range keys {
		if sshutils.KeysEqual(cert.SignatureKey, keys[i]) {
			a.log.Debugf("Verified host %s", host)
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
		a.log.Warn(err)
	}
	return err
}

// AddKey activates a new signed session key by adding it into the keystore and also
// by loading it into the SSH agent
func (a *LocalKeyAgent) AddKey(key *Key) (*agent.AddedKey, error) {
	// save it to disk (usually into ~/.tsh)
	err := a.keyStore.AddKey(a.proxyHost, a.username, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// load key into the teleport agent and system agent
	return a.LoadKey(*key)
}

// DeleteKey removes the key from the key store as well as unloading the key
// from the agent.
func (a *LocalKeyAgent) DeleteKey() error {
	// remove key from key store
	err := a.keyStore.DeleteKey(a.proxyHost, a.username)
	if err != nil {
		return trace.Wrap(err)
	}

	// remove any keys that are loaded for this user from the teleport and
	// system agents
	err = a.UnloadKey()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DeleteKeys removes all keys from the keystore as well as unloads keys
// from the agent.
func (a *LocalKeyAgent) DeleteKeys() error {
	// Remove keys from the filesystem.
	err := a.keyStore.DeleteKeys()
	if err != nil {
		return trace.Wrap(err)
	}

	// Remove all keys from the Teleport and system agents.
	err = a.UnloadKeys()
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
