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
	"crypto/subtle"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/prompt"
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

	// noHosts is a in-memory map used in tests to track which hosts a user has
	// manually (via keyboard input) refused connecting to.
	noHosts map[string]bool

	// function which asks a user to trust host/key combination (during host auth)
	hostPromptFunc func(host string, k ssh.PublicKey) error

	// username is the Teleport username for who the keys will be loaded in the
	// local agent.
	username string

	// proxyHost is the proxy for the cluster that his key agent holds keys for.
	proxyHost string
}

// NewKeyStoreCertChecker returns a new certificate checker
// using trusted certs from key store
func NewKeyStoreCertChecker(keyStore LocalKeyStore) ssh.HostKeyCallback {
	// CheckHostSignature checks if the given host key was signed by a Teleport
	// certificate authority (CA) or a host certificate the user has seen before.
	return func(addr string, remote net.Addr, key ssh.PublicKey) error {
		certChecker := utils.CertChecker{
			CertChecker: ssh.CertChecker{
				IsHostAuthority: func(key ssh.PublicKey, addr string) bool {
					keys, err := keyStore.GetKnownHostKeys("")
					if err != nil {
						log.Errorf("Unable to fetch certificate authorities: %v.", err)
						return false
					}
					for i := range keys {
						if sshutils.KeysEqual(key, keys[i]) {
							return true
						}
					}
					return false
				},
			},
			FIPS: isFIPS(),
		}
		err := certChecker.CheckHostKey(addr, remote, key)
		if err != nil {
			log.Debugf("Host validation failed: %v.", err)
			return trace.Wrap(err)
		}
		log.Debugf("Validated host %v.", addr)
		return nil
	}
}

func agentIsPresent() bool {
	return os.Getenv(teleport.SSHAuthSock) != ""
}

// agentSupportsSSHCertificates checks if the running agent supports SSH certificates.
// This detection implementation is as described in RFD 18 and works by simply checking for
// presence of gpg-agent which is a common agent known to not support SSH certificates.
func agentSupportsSSHCertificates() bool {
	agent := os.Getenv(teleport.SSHAuthSock)
	return !strings.Contains(agent, "gpg-agent")
}

func shouldAddKeysToAgent(addKeysToAgent string) bool {
	return (addKeysToAgent == AddKeysToAgentAuto && agentSupportsSSHCertificates()) || addKeysToAgent == AddKeysToAgentOnly || addKeysToAgent == AddKeysToAgentYes
}

// NewLocalAgent reads all available credentials from the provided LocalKeyStore
// and loads them into the local and system agent
func NewLocalAgent(keystore LocalKeyStore, proxyHost, username string, keysOption string) (a *LocalKeyAgent, err error) {
	a = &LocalKeyAgent{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentKeyAgent,
		}),
		Agent:     agent.NewKeyring(),
		keyStore:  keystore,
		noHosts:   make(map[string]bool),
		username:  username,
		proxyHost: proxyHost,
	}

	if shouldAddKeysToAgent(keysOption) {
		a.sshAgent = connectToSSHAgent()
	} else {
		log.Debug("Skipping connection to the local ssh-agent.")

		if !agentSupportsSSHCertificates() && agentIsPresent() {
			log.Warn(`Certificate was not loaded into agent because the agent at SSH_AUTH_SOCK does not appear
to support SSH certificates. To force load the certificate into the running agent, use
the --add-keys-to-agent=yes flag.`)
		}
	}
	return a, nil
}

// UpdateProxyHost changes the proxy host that the local agent operates on.
func (a *LocalKeyAgent) UpdateProxyHost(proxyHost string) {
	a.proxyHost = proxyHost
}

// LoadKeyForCluster fetches a cluster-specific SSH key and loads it into the
// SSH agent.
func (a *LocalKeyAgent) LoadKeyForCluster(clusterName string) (*agent.AddedKey, error) {
	key, err := a.GetKey(clusterName, WithSSHCerts{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return a.LoadKey(*key)
}

// LoadKey adds a key into the Teleport ssh agent as well as the system ssh
// agent.
func (a *LocalKeyAgent) LoadKey(key Key) (*agent.AddedKey, error) {
	a.log.Infof("Loading SSH key for user %q and cluster %q.", a.username, key.ClusterName)

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
	for _, agent := range agents {
		for _, agentKey := range agentKeys {
			err = agent.Add(agentKey)
			if err != nil {
				a.log.Warnf("Unable to communicate with agent and add key: %v", err)
			}
		}
	}

	// return the first key because it has the embedded private key in it.
	// see docs for AsAgentKeys for more details.
	return &agentKeys[0], nil
}

// UnloadKey will unload key for user from the teleport ssh agent as well as
// the system agent.
func (a *LocalKeyAgent) UnloadKey() error {
	agents := []agent.Agent{a.Agent}
	if a.sshAgent != nil {
		agents = append(agents, a.sshAgent)
	}

	// iterate over all agents we have and unload keys for this user
	for _, agent := range agents {
		// get a list of all keys in the agent
		keyList, err := agent.List()
		if err != nil {
			a.log.Warnf("Unable to communicate with agent and list keys: %v", err)
		}

		// remove any teleport keys we currently have loaded in the agent for this user
		for _, key := range keyList {
			if key.Comment == fmt.Sprintf("teleport:%v", a.username) {
				err = agent.Remove(key)
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
	for _, agent := range agents {
		// get a list of all keys in the agent
		keyList, err := agent.List()
		if err != nil {
			a.log.Warnf("Unable to communicate with agent and list keys: %v", err)
		}

		// remove any teleport keys we currently have loaded in the agent
		for _, key := range keyList {
			if strings.HasPrefix(key.Comment, "teleport:") {
				err = agent.Remove(key)
				if err != nil {
					a.log.Warnf("Unable to communicate with agent and remove key: %v", err)
				}
			}
		}
	}

	return nil
}

// GetKey returns the key for the given cluster of the proxy from
// the backing keystore.
func (a *LocalKeyAgent) GetKey(clusterName string, opts ...CertOption) (*Key, error) {
	idx := KeyIndex{a.proxyHost, a.username, clusterName}
	return a.keyStore.GetKey(idx, opts...)
}

// GetCoreKey returns the key without any cluster-dependent certificates,
// i.e. including only the RSA keypair and the Teleport TLS certificate.
func (a *LocalKeyAgent) GetCoreKey() (*Key, error) {
	return a.GetKey("")
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

// SaveTrustedCerts saves trusted TLS certificates of certificate authorities.
func (a *LocalKeyAgent) SaveTrustedCerts(certAuthorities []auth.TrustedCerts) error {
	return a.keyStore.SaveTrustedCerts(a.proxyHost, certAuthorities)
}

// GetTrustedCertsPEM returns trusted TLS certificates of certificate authorities PEM
// blocks.
func (a *LocalKeyAgent) GetTrustedCertsPEM() ([][]byte, error) {
	return a.keyStore.GetTrustedCertsPEM(a.proxyHost)
}

// UserRefusedHosts returns 'true' if a user refuses connecting to remote hosts
// when prompted during host authorization
func (a *LocalKeyAgent) UserRefusedHosts() bool {
	return len(a.noHosts) > 0
}

// CheckHostSignature checks if the given host key was signed by a Teleport
// certificate authority (CA) or a host certificate the user has seen before.
func (a *LocalKeyAgent) CheckHostSignature(addr string, remote net.Addr, key ssh.PublicKey) error {
	certChecker := utils.CertChecker{
		CertChecker: ssh.CertChecker{
			IsHostAuthority: a.checkHostCertificate,
			HostKeyFallback: a.checkHostKey,
		},
		FIPS: isFIPS(),
	}
	err := certChecker.CheckHostKey(addr, remote, key)
	if err != nil {
		a.log.Debugf("Host validation failed: %v.", err)
		return trace.Wrap(err)
	}
	a.log.Debugf("Validated host %v.", addr)
	return nil
}

// checkHostCertificate validates a host certificate. First checks the
// ~/.tsh/known_hosts cache and if not found, prompts the user to accept
// or reject.
func (a *LocalKeyAgent) checkHostCertificate(key ssh.PublicKey, addr string) bool {
	// Check the local cache (where all Teleport CAs are placed upon login) to
	// see if any of them match.
	keys, err := a.keyStore.GetKnownHostKeys("")
	if err != nil {
		a.log.Errorf("Unable to fetch certificate authorities: %v.", err)
		return false
	}
	for i := range keys {
		if sshutils.KeysEqual(key, keys[i]) {
			return true
		}
	}

	// If this certificate was not seen before, prompt the user essentially
	// treating it like a key.
	err = a.checkHostKey(addr, nil, key)
	return err == nil
}

// checkHostKey validates a host key. First checks the
// ~/.tsh/known_hosts cache and if not found, prompts the user to accept
// or reject.
func (a *LocalKeyAgent) checkHostKey(addr string, remote net.Addr, key ssh.PublicKey) error {
	var err error

	// Check if this exact host is in the local cache.
	keys, _ := a.keyStore.GetKnownHostKeys(addr)
	if len(keys) > 0 && sshutils.KeysEqual(key, keys[0]) {
		a.log.Debugf("Verified host %s.", addr)
		return nil
	}

	// If this key was not seen before, prompt the user with a fingerprint.
	if a.hostPromptFunc != nil {
		err = a.hostPromptFunc(addr, key)
	} else {
		err = a.defaultHostPromptFunc(addr, key, os.Stdout, os.Stdin)
	}
	if err != nil {
		a.noHosts[addr] = true
		return trace.Wrap(err)
	}

	// If the user trusts the key, store the key in the local known hosts
	// cache ~/.tsh/known_hosts.
	err = a.keyStore.AddKnownHostKeys(addr, []ssh.PublicKey{key})
	if err != nil {
		a.log.Warnf("Failed to save the host key: %v.", err)
		return trace.Wrap(err)
	}
	return nil
}

// defaultHostPromptFunc is the default host key/certificates prompt.
func (a *LocalKeyAgent) defaultHostPromptFunc(host string, key ssh.PublicKey, writer io.Writer, reader io.Reader) error {
	var err error
	ok := false
	if !a.noHosts[host] {
		ok, err = prompt.Confirmation(writer, reader,
			fmt.Sprintf("The authenticity of host '%s' can't be established. Its public key is:\n%s\nAre you sure you want to continue?",
				host,
				ssh.MarshalAuthorizedKey(key),
			),
		)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if !ok {
		return trace.BadParameter("not trusted")
	}
	return nil
}

// AddKey activates a new signed session key by adding it into the keystore and also
// by loading it into the SSH agent.
func (a *LocalKeyAgent) AddKey(key *Key) (*agent.AddedKey, error) {
	if key == nil {
		return nil, trace.BadParameter("key is nil")
	}
	if key.ProxyHost == "" {
		key.ProxyHost = a.proxyHost
	}
	if key.Username == "" {
		key.Username = a.username
	}

	// In order to prevent unrelated key data to be left over after the new
	// key is added, delete any already stored key with the same index if their
	// RSA private keys do not match.
	storedKey, err := a.keyStore.GetKey(key.KeyIndex)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		if subtle.ConstantTimeCompare(storedKey.Priv, key.Priv) == 0 {
			a.log.Debugf("Deleting obsolete stored key with index %+v.", storedKey.KeyIndex)
			if err := a.keyStore.DeleteKey(storedKey.KeyIndex); err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}

	// Save the new key to the keystore (usually into ~/.tsh).
	if err := a.keyStore.AddKey(key); err != nil {
		return nil, trace.Wrap(err)
	}
	// Load key into the teleport agent and system agent.
	return a.LoadKey(*key)
}

// DeleteKey removes the key with all its certs from the key store
// and unloads the key from the agent.
func (a *LocalKeyAgent) DeleteKey() error {
	// remove key from key store
	err := a.keyStore.DeleteKey(KeyIndex{ProxyHost: a.proxyHost, Username: a.username})
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

// DeleteUserCerts deletes only the specified certs of the user's key,
// keeping the private key intact.
func (a *LocalKeyAgent) DeleteUserCerts(clusterName string, opts ...CertOption) error {
	err := a.keyStore.DeleteUserCerts(KeyIndex{a.proxyHost, a.username, clusterName}, opts...)
	return trace.Wrap(err)
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

// AuthMethods returns the list of different authentication methods this agent supports:
//	  1. First to try is the external SSH agent
//    2. Itself (disk-based local agent)
// It returns an error in case there is no auth method method available.
func (a *LocalKeyAgent) AuthMethods() ([]ssh.AuthMethod, error) {
	// combine our certificates with external SSH agent's:
	var signers []ssh.Signer
	if a.sshAgent != nil {
		if sshAgentCerts, _ := a.sshAgent.Signers(); sshAgentCerts != nil {
			signers = append(signers, sshAgentCerts...)
		}
	}
	if ourCerts, _ := a.Signers(); ourCerts != nil {
		signers = append(signers, ourCerts...)
	}
	// for every certificate create a new "auth method" and return them
	m := []ssh.AuthMethod{}
	for i := range signers {
		// filter out non-certificates (like regular public SSH keys stored in the SSH agent):
		_, ok := signers[i].PublicKey().(*ssh.Certificate)
		if ok {
			m = append(m, sshutils.NewAuthMethodForCert(signers[i]))
		}
	}
	if len(m) == 0 {
		return nil, trace.BadParameter("no auth method available")
	}
	return m, nil
}
