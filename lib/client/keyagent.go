/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/tlsca"
)

// LocalKeyAgent holds Teleport certificates for a user connected to a cluster.
type LocalKeyAgent struct {
	// log holds the structured logger.
	log *logrus.Entry

	// ExtendedAgent is the teleport agent
	agent.ExtendedAgent

	// clientStore is the local storage backend for the client.
	clientStore *Store

	// systemAgent is the system ssh agent
	systemAgent agent.ExtendedAgent

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
	ClientStore *Store
	Agent       agent.ExtendedAgent
	ProxyHost   string
	Username    string
	KeysOption  string
	Insecure    bool
	Site        string
	LoadAllCAs  bool
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
			teleport.ComponentKey: teleport.ComponentKeyAgent,
		}),
		ExtendedAgent: conf.Agent,
		clientStore:   conf.ClientStore,
		noHosts:       make(map[string]bool),
		username:      conf.Username,
		proxyHost:     conf.ProxyHost,
		insecure:      conf.Insecure,
		siteName:      conf.Site,
		loadAllCAs:    conf.LoadAllCAs,
	}

	if shouldAddKeysToAgent(conf.KeysOption) {
		a.systemAgent = connectToSSHAgent()
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

// LoadKey adds a key into the local agent as well as the system agent.
// Some agent keys are only supported by the local agent, such as those
// for a YubiKeyPrivateKey. Any failures to add the key will be aggregated
// into the returned error to be handled by the caller if necessary.
func (a *LocalKeyAgent) LoadKey(key Key) error {
	// convert key into a format understood by x/crypto/ssh/agent
	agentKey, err := key.AsAgentKey()
	if err != nil {
		return trace.Wrap(err)
	}

	// remove any keys that the user may already have loaded
	if err = a.UnloadKey(key.KeyIndex); err != nil {
		return trace.Wrap(err)
	}

	a.log.Infof("Loading SSH key for user %q and cluster %q.", a.username, key.ClusterName)
	agents := []agent.ExtendedAgent{a.ExtendedAgent}
	if a.systemAgent != nil {
		if canAddToSystemAgent(agentKey) {
			agents = append(agents, a.systemAgent)
		} else {
			a.log.Infof("Skipping adding key to SSH system agent for non-standard key type %T", agentKey.PrivateKey)
		}
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

	// iterate over all teleport and system agent and load key
	var errs []error
	for _, agent := range agents {
		for _, agentKey := range agentKeys {
			if err = agent.Add(agentKey); err != nil {
				errs = append(errs, trace.Wrap(err))
			}
		}
	}

	return trace.Wrap(trace.NewAggregate(errs...), "failed to add one or more keys to the agent.")
}

// UnloadKey will unload keys matching the given KeyIndex from
// the teleport ssh agent and the system agent.
func (a *LocalKeyAgent) UnloadKey(key KeyIndex) error {
	agents := []agent.Agent{a.ExtendedAgent}
	if a.systemAgent != nil {
		agents = append(agents, a.systemAgent)
	}

	// iterate over all agents we have and unload keys matching the given key
	for _, agent := range agents {
		// get a list of all keys in the agent
		keyList, err := agent.List()
		if err != nil {
			a.log.Warnf("Unable to communicate with agent and list keys: %v", err)
		}

		// remove any teleport keys we currently have loaded in the agent for this user and proxy
		for _, agentKey := range keyList {
			if agentKeyIdx, ok := parseTeleportAgentKeyComment(agentKey.Comment); ok && agentKeyIdx.Match(key) {
				if err = agent.Remove(agentKey); err != nil {
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
	if a.systemAgent != nil {
		agents = append(agents, a.systemAgent)
	}

	// iterate over all agents we have and unload keys
	for _, agent := range agents {
		// get a list of all keys in the agent
		keyList, err := agent.List()
		if err != nil {
			a.log.Warnf("Unable to communicate with agent and list keys: %v", err)
		}

		// remove any teleport keys we currently have loaded in the agent
		for _, key := range keyList {
			if isTeleportAgentKey(key) {
				if err = agent.Remove(key); err != nil {
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
	key, err := a.clientStore.GetKey(idx, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustedCerts, err := a.clientStore.GetTrustedCerts(idx.ProxyHost)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key.TrustedCerts = trustedCerts
	return key, nil
}

// GetCoreKey returns the key without any cluster-dependent certificates,
// i.e. including only the private key and the Teleport TLS certificate.
func (a *LocalKeyAgent) GetCoreKey() (*Key, error) {
	return a.GetKey("")
}

// SaveTrustedCerts saves trusted TLS certificates and host keys of certificate authorities.
// SaveTrustedCerts adds the given trusted CA TLS certificates and SSH host keys to the store.
// Existing TLS certificates for the given trusted certs will be overwritten, while host keys
// will be appended to existing entries.
func (a *LocalKeyAgent) SaveTrustedCerts(certAuthorities []auth.TrustedCerts) error {
	return a.clientStore.SaveTrustedCerts(a.proxyHost, certAuthorities)
}

// GetTrustedCertsPEM returns trusted TLS certificates of certificate authorities PEM
// blocks.
func (a *LocalKeyAgent) GetTrustedCertsPEM() ([][]byte, error) {
	return a.clientStore.GetTrustedCertsPEM(a.proxyHost)
}

// UserRefusedHosts returns 'true' if a user refuses connecting to remote hosts
// when prompted during host authorization
func (a *LocalKeyAgent) UserRefusedHosts() bool {
	return len(a.noHosts) > 0
}

// HostKeyCallback checks if the given host key was signed by a Teleport
// certificate authority (CA) or a host certificate the user has seen before.
func (a *LocalKeyAgent) HostKeyCallback(addr string, remote net.Addr, hostKey ssh.PublicKey) error {
	key, err := a.GetCoreKey()
	if err != nil {
		return trace.Wrap(err)
	}
	rootCluster, err := key.RootClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	clusters := []string{rootCluster}
	if a.loadAllCAs {
		clusters, err = a.GetClusterNames()
		if err != nil {
			return trace.Wrap(err)
		}
	} else if rootCluster != a.siteName {
		// In case of establishing connection to leaf cluster the client validate ssh cert against root
		// cluster proxy cert and leaf cluster cert.
		clusters = append(clusters, a.siteName)
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
		trustedCerts, err := a.clientStore.GetTrustedCerts(a.proxyHost)
		if err != nil {
			a.log.Errorf("Failed to get trusted certs: %v.", err)
			return false
		}

		// In case of a know host entry added by insecure flow (checkHostKey function)
		// the parsed know_hosts entry (trustedCerts.ClusterName) contains node address.
		clusters = append(clusters, addr)
		for _, cert := range trustedCerts {
			if !a.loadAllCAs && !slices.Contains(clusters, cert.ClusterName) {
				continue
			}
			key, err := sshutils.ParseAuthorizedKeys(cert.AuthorizedKeys)
			if err != nil {
				a.log.Errorf("Failed to parse authorized keys: %v.", err)
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
		err = a.checkHostKey(addr, nil, key)
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
	keys, err := a.clientStore.GetTrustedHostKeys(addr)
	if err != nil {
		a.log.WithError(err).Debugf("Failed to retrieve client's trusted host keys.")
	} else {
		for _, trustedHostKey := range keys {
			if sshutils.KeysEqual(key, trustedHostKey) {
				a.log.Debugf("Verified host %s.", addr)
				return nil
			}
		}
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

	// If the user trusts the key, store the key in the client trusted certs store.
	err = a.clientStore.AddTrustedHostKeys(a.proxyHost, addr, key)
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
	if err := a.LoadKey(*key); err != nil {
		return trace.Wrap(err)
	}
	return nil
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

// AddAppKey activates a new signed app key by adding it into the keystore.
// key must contain at least one app cert. ssh cert is not required.
func (a *LocalKeyAgent) AddAppKey(key *Key) error {
	if len(key.AppTLSCerts) == 0 {
		return trace.BadParameter("key must contains at least one App access certificate")
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
	storedKey, err := a.clientStore.GetKey(key.KeyIndex)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	} else {
		if !key.EqualPrivateKey(storedKey) {
			a.log.Debugf("Deleting obsolete stored key with index %+v.", storedKey.KeyIndex)
			if err := a.clientStore.DeleteKey(storedKey.KeyIndex); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	// Save the new key to the keystore (usually into ~/.tsh).
	if err := a.clientStore.AddKey(key); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DeleteKey removes the key with all its certs from the key store
// and unloads the key from the agent.
func (a *LocalKeyAgent) DeleteKey() error {
	// remove key from key store
	err := a.clientStore.DeleteKey(KeyIndex{ProxyHost: a.proxyHost, Username: a.username})
	if err != nil {
		return trace.Wrap(err)
	}

	// remove any keys that are loaded for this user from the teleport and
	// system agents
	err = a.UnloadKey(KeyIndex{ProxyHost: a.proxyHost, Username: a.username})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DeleteUserCerts deletes only the specified certs of the user's key,
// keeping the private key intact.
func (a *LocalKeyAgent) DeleteUserCerts(clusterName string, opts ...CertOption) error {
	err := a.clientStore.DeleteUserCerts(KeyIndex{a.proxyHost, a.username, clusterName}, opts...)
	return trace.Wrap(err)
}

// DeleteKeys removes all keys from the keystore as well as unloads keys
// from the agent.
func (a *LocalKeyAgent) DeleteKeys() error {
	// Remove keys from the filesystem.
	err := a.clientStore.DeleteKeys()
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
		certs, err := a.clientStore.GetSSHCertificates(a.proxyHost, a.username)
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
	if a.systemAgent != nil {
		sshAgentSigners, err := a.systemAgent.Signers()
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
