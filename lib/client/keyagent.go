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
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/prompt"
)

// LocalKeyAgent holds Teleport certificates for a user connected to a cluster.
type LocalKeyAgent struct {
	// log holds the structured logger.
	log *logrus.Entry

	// ExtendedAgent is the teleport agent
	agent.ExtendedAgent

	// keyStore is the storage backend for certificates and keys
	keyStore LocalKeyStore

	// sshAgent is the system ssh agent
	sshAgent agent.ExtendedAgent

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

	// insecure allows to accept public host keys.
	insecure bool

	// siteName specifies the site to execute operation.
	siteName string

	// loadAllCAs allows the agent to load all host CAs when checking a host
	// signature.
	loadAllCAs bool
}

// sshKnowHostGetter allows to fetch key for particular host - trusted cluster.
type sshKnowHostGetter interface {
	// GetKnownHostKeys returns all public keys for a hostname.
	GetKnownHostKeys(hostname string) ([]ssh.PublicKey, error)
}

// NewKeyStoreCertChecker returns a new certificate checker
// using trusted certs from key store
func NewKeyStoreCertChecker(keyStore sshKnowHostGetter, host string) ssh.HostKeyCallback {
	// CheckHostSignature checks if the given host key was signed by a Teleport
	// certificate authority (CA) or a host certificate the user has seen before.
	return func(addr string, remote net.Addr, key ssh.PublicKey) error {
		certChecker := sshutils.CertChecker{
			CertChecker: ssh.CertChecker{
				IsHostAuthority: func(key ssh.PublicKey, addr string) bool {
					keys, err := keyStore.GetKnownHostKeys(host)
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

// LocalAgentConfig contains parameters for creating the local keys agent.
type LocalAgentConfig struct {
	Keystore   LocalKeyStore
	Agent      agent.ExtendedAgent
	ProxyHost  string
	Username   string
	KeysOption string
	Insecure   bool
	Site       string
	LoadAllCAs bool
}

// NewLocalAgent reads all available credentials from the provided LocalKeyStore
// and loads them into the local and system agent
func NewLocalAgent(conf LocalAgentConfig) (a *LocalKeyAgent, err error) {
	if conf.Agent == nil {
		keyring, ok := agent.NewKeyring().(agent.ExtendedAgent)
		if !ok {
			return nil, trace.Errorf("unexpected keyring type: %T, expected agent.ExtendedKeyring", keyring)
		}
		conf.Agent = keyring
	}
	a = &LocalKeyAgent{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentKeyAgent,
		}),
		ExtendedAgent: conf.Agent,
		keyStore:      conf.Keystore,
		noHosts:       make(map[string]bool),
		username:      conf.Username,
		proxyHost:     conf.ProxyHost,
		insecure:      conf.Insecure,
		siteName:      conf.Site,
		loadAllCAs:    conf.LoadAllCAs,
	}

	if shouldAddKeysToAgent(conf.KeysOption) {
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

// UpdateUsername changes username that the local agent operates on.
func (a *LocalKeyAgent) UpdateUsername(username string) {
	a.username = username
}

// UpdateCluster changes the cluster that the local agent operates on.
func (a *LocalKeyAgent) UpdateCluster(cluster string) {
	a.siteName = cluster
}

// UpdateLoadAllCAs changes whether or not the local agent should load all
// host CAs.
func (a *LocalKeyAgent) UpdateLoadAllCAs(loadAllCAs bool) {
	a.loadAllCAs = loadAllCAs
}

// LoadKeyForCluster fetches a cluster-specific SSH key and loads it into the
// SSH agent.
func (a *LocalKeyAgent) LoadKeyForCluster(clusterName string) error {
	key, err := a.GetKey(clusterName, WithSSHCerts{})
	if err != nil {
		return trace.Wrap(err)
	}

	return a.LoadKey(*key)
}

// LoadKey adds a key into the Teleport ssh agent as well as the system ssh
// agent.
func (a *LocalKeyAgent) LoadKey(key Key) error {
	// convert keys into a format understood by the ssh agent
	agentKey, err := key.AsAgentKey()
	if err != nil {
		return trace.Wrap(err)
	}

	// On all OS'es, load the certificate with the private key embedded.
	agentKeys := []agent.AddedKey{agentKey}
	if runtime.GOOS != constants.WindowsOS {
		// On Unix, also load a lone private key.
		//
		// (2016-08-01) have a bug in how they use certificates that have been lo
		// This is done because OpenSSH clients older than OpenSSH 7.3/7.3p1aded
		// in an agent. Specifically when you add a certificate to an agent, you can't
		// just embed the private key within the certificate, you have to add the
		// certificate and private key to the agent separately. Teleport works around
		// this behavior to ensure OpenSSH interoperability.
		//
		// For more details see the following: https://bugzilla.mindrot.org/show_bug.cgi?id=2550
		// WARNING: callers expect the returned slice to be __exactly as it is__
		agentKey.Certificate = nil
		agentKeys = append(agentKeys, agentKey)
	}

	a.log.Infof("Loading SSH key for user %q and cluster %q.", a.username, key.ClusterName)
	agents := []agent.ExtendedAgent{a.ExtendedAgent}
	if a.sshAgent != nil {
		agents = append(agents, a.sshAgent)
	}

	// remove any keys that the user may already have loaded
	err = a.UnloadKey()
	if err != nil {
		return trace.Wrap(err)
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
	return nil
}

// UnloadKey will unload key for user from the teleport ssh agent as well as
// the system agent.
func (a *LocalKeyAgent) UnloadKey() error {
	agents := []agent.ExtendedAgent{a.ExtendedAgent}
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
	agents := []agent.ExtendedAgent{a.ExtendedAgent}
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
// i.e. including only the private key and the Teleport TLS certificate.
func (a *LocalKeyAgent) GetCoreKey() (*Key, error) {
	return a.GetKey("")
}

// AddHostSignersToCache takes a list of CAs whom we trust. This list is added to a database
// of "seen" CAs.
//
// Every time we connect to a new host, we'll request its certificate to be signed by one
// of these trusted CAs.
//
// Why do we trust these CAs? Because we received them from a trusted Teleport Proxy.
// Why do we trust the proxy? Because we've connected to it via HTTPS + username + Password + OTP.
func (a *LocalKeyAgent) AddHostSignersToCache(certAuthorities []auth.TrustedCerts) error {
	for _, ca := range certAuthorities {
		publicKeys, err := ca.SSHCertPublicKeys()
		if err != nil {
			a.log.Error(err)
			return trace.Wrap(err)
		}
		a.log.Debugf("Adding CA key for %s", ca.ClusterName)
		err = a.keyStore.AddKnownHostKeys(ca.ClusterName, a.proxyHost, publicKeys)
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
func (a *LocalKeyAgent) CheckHostSignature(addr string, remote net.Addr, hostKey ssh.PublicKey) error {
	key, err := a.GetCoreKey()
	if err != nil {
		return trace.Wrap(err)
	}
	rootCluster, err := key.RootClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	clusters := []string{rootCluster}
	if rootCluster != a.siteName {
		// In case of establishing connection to leaf cluster the client validate ssh cert against root
		// cluster proxy cert and leaf cluster cert.
		clusters = append(clusters, a.siteName)
	} else if a.loadAllCAs {
		clusters, err = a.GetClusterNames()
		if err != nil {
			return trace.Wrap(err)
		}
	}

	certChecker := sshutils.CertChecker{
		CertChecker: ssh.CertChecker{
			IsHostAuthority: a.checkHostCertificateForClusters(clusters...),
			HostKeyFallback: a.checkHostKey,
		},
		FIPS: isFIPS(),
	}
	a.log.Debugf("Checking key: %s.", ssh.MarshalAuthorizedKey(hostKey))
	err = certChecker.CheckHostKey(addr, remote, hostKey)
	if err != nil {
		a.log.Debugf("Host validation failed: %v.", err)
		return trace.Wrap(err)
	}
	a.log.Debugf("Validated host %v.", addr)
	return nil
}

// checkHostCertificateForClusters validates a host certificate and check if remote key matches the know
// trusted cluster key based on  ~/.tsh/known_hosts. If server key is not known, the users is prompted to accept or
// reject the server key.
func (a *LocalKeyAgent) checkHostCertificateForClusters(clusters ...string) func(key ssh.PublicKey, addr string) bool {
	return func(key ssh.PublicKey, addr string) bool {
		// Check the local cache (where all Teleport CAs are placed upon login) to
		// see if any of them match.

		var keys []ssh.PublicKey
		for _, cluster := range clusters {
			key, err := a.keyStore.GetKnownHostKeys(cluster)
			if err != nil {
				a.log.Errorf("Unable to fetch certificate authorities: %v.", err)
				return false
			}
			keys = append(keys, key...)

		}
		for i := range keys {
			if sshutils.KeysEqual(key, keys[i]) {
				return true
			}
		}

		// If this certificate was not seen before, prompt the user essentially
		// treating it like a key.
		err := a.checkHostKey(addr, nil, key)
		return err == nil
	}
}

// checkHostKey validates a host key. First checks the
// ~/.tsh/known_hosts cache and if not found, prompts the user to accept
// or reject.
func (a *LocalKeyAgent) checkHostKey(addr string, remote net.Addr, key ssh.PublicKey) error {
	var err error

	// Unless --insecure flag was given, prohibit public keys or host certs
	// not signed by Teleport.
	if !a.insecure {
		a.log.Debugf("Host %s presented a public key not signed by Teleport. Rejecting due to insecure mode being OFF.", addr)
		return trace.BadParameter("host %s presented a public key not signed by Teleport", addr)
	}

	a.log.Warnf("Host %s presented a public key not signed by Teleport. Proceeding due to insecure mode being ON.", addr)

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
	err = a.keyStore.AddKnownHostKeys(addr, a.proxyHost, []ssh.PublicKey{key})
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
		cr := prompt.NewContextReader(reader)
		defer cr.Close()
		ok, err = prompt.Confirmation(context.Background(), writer, cr,
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
func (a *LocalKeyAgent) AddKey(key *Key) error {
	if err := a.addKey(key); err != nil {
		return trace.Wrap(err)
	}

	// Load key into the teleport agent and system agent.
	err := a.LoadKey(*key)
	if trace.IsNotImplemented(err) {
		// If the key is not supported as an agent key, then log the error and
		// continue without the agent key. This will only affect Agent forwarding,
		// so we log this as INFO and continue with a non-agent login session.
		a.log.WithError(err).Warn("Failed to add key to agent. Some integrations related to agent forwarding will not work with this key agent.")
		return nil
	}
	return trace.Wrap(err)
}

// AddDatabaseKey activates a new signed database key by adding it into the keystore.
// key must contain at least one db cert. ssh cert is not required.
func (a *LocalKeyAgent) AddDatabaseKey(key *Key) error {
	if len(key.DBTLSCerts) == 0 {
		return trace.BadParameter("key must contains at least one database access certificate")
	}
	return a.addKey(key)
}

// AddKubeKey activates a new signed Kubernetes key by adding it into the keystore.
// key must contain at least one Kubernetes cert. ssh cert is not required.
func (a *LocalKeyAgent) AddKubeKey(key *Key) error {
	if len(key.KubeTLSCerts) == 0 {
		return trace.BadParameter("key must contains at least one Kubernetes access certificate")
	}
	return a.addKey(key)
}

// addKey activates a new signed session key by adding it into the keystore.
func (a *LocalKeyAgent) addKey(key *Key) error {
	if key == nil {
		return trace.BadParameter("key is nil")
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
			return trace.Wrap(err)
		}
	} else {
		if !key.EqualPrivateKey(storedKey) {
			a.log.Debugf("Deleting obsolete stored key with index %+v.", storedKey.KeyIndex)
			if err := a.keyStore.DeleteKey(storedKey.KeyIndex); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	// Save the new key to the keystore (usually into ~/.tsh).
	if err := a.keyStore.AddKey(key); err != nil {
		return trace.Wrap(err)
	}
	return nil
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

// Signers returns a set of ssh.Signers using all certificates
// for the current proxy and user.
func (a *LocalKeyAgent) Signers() ([]ssh.Signer, error) {
	var signers []ssh.Signer

	// If we find a valid key store, load all valid ssh certificates as signers.
	if k, err := a.GetCoreKey(); err == nil {
		certs, err := a.keyStore.GetSSHCertificates(a.proxyHost, a.username)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, cert := range certs {
			if err := k.checkCert(cert); err != nil {
				return nil, trace.Wrap(err)
			}
			signer, err := sshutils.SSHSigner(cert, k)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			signers = append(signers, signer)
		}
	}

	// Load all agent certs, including the ones from a local SSH agent.
	agentSigners, err := a.ExtendedAgent.Signers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if a.sshAgent != nil {
		sshAgentSigners, err := a.sshAgent.Signers()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		agentSigners = append(signers, sshAgentSigners...)
	}

	// Filter out non-certificates (like regular public SSH keys stored in the SSH agent).
	for _, s := range agentSigners {
		if _, ok := s.PublicKey().(*ssh.Certificate); ok {
			signers = append(signers, s)
		} else if k, ok := s.PublicKey().(*agent.Key); ok && sshutils.IsSSHCertType(k.Type()) {
			signers = append(signers, s)
		}
	}

	return signers, nil
}

// signersForCluster returns a set of ssh.Signers using certificates for a specific cluster.
func (a *LocalKeyAgent) signersForCluster(clusterName string) ([]ssh.Signer, error) {
	k, err := a.GetKey(clusterName, WithSSHCerts{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer, err := k.SSHSigner()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []ssh.Signer{signer}, nil
}

// ClientCertPool returns x509.CertPool containing trusted CA.
func (a *LocalKeyAgent) ClientCertPool(cluster string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	key, err := a.GetKey(cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, caPEM := range key.TLSCAs() {
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, trace.BadParameter("failed to parse TLS CA certificate")
		}
	}
	return pool, nil
}

// GetClusterNames gets the names of the Teleport clusters this
// key agent knows about.
func (a *LocalKeyAgent) GetClusterNames() ([]string, error) {
	certs, err := a.GetTrustedCertsPEM()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var clusters []string
	for _, cert := range certs {
		cert, err := tlsca.ParseCertificatePEM(cert)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		clusters = append(clusters, cert.Subject.CommonName)
	}
	return clusters, nil
}
