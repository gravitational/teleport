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
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

type KeyAgentTestSuite struct {
	keyDir      string
	key         *Key
	username    string
	hostname    string
	clusterName string
	tlsca       *tlsca.CertAuthority
	tlscaCert   auth.TrustedCerts
}

func makeSuite(t *testing.T) *KeyAgentTestSuite {
	t.Helper()

	err := startDebugAgent(t)
	require.NoError(t, err)

	s := &KeyAgentTestSuite{
		keyDir:      t.TempDir(),
		username:    "foo",
		hostname:    "bar",
		clusterName: "some-cluster",
	}

	pemBytes, ok := fixtures.PEMBytes["rsa"]
	require.True(t, ok)

	s.tlsca, s.tlscaCert, err = newSelfSignedCA(pemBytes, "localhost")
	require.NoError(t, err)

	keygen := testauthority.New()
	priv, err := keygen.GeneratePrivateKey()
	require.NoError(t, err)

	s.key = s.makeKey(t, s.username, s.hostname, priv)

	return s
}

// TestAddKey ensures correct adding of ssh keys. This test checks the following:
//   - When adding a key it's written to disk.
//   - When we add a key, it's added to both the teleport ssh agent as well
//     as the system ssh agent.
//   - When we add a key, both the certificate and private key are added into
//     the both the teleport ssh agent and the system ssh agent.
//   - When we add a key, it's tagged with a comment that indicates that it's
//     a teleport key with the teleport username.
func TestAddKey(t *testing.T) {
	s := makeSuite(t)
	lka := s.newKeyAgent(t)

	// add the key to the local agent, this should write the key
	// to disk as well as load it in the agent
	err := lka.AddKey(s.key)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = lka.UnloadKey(s.key.KeyIndex)
		require.NoError(t, err)
	})

	// check that the key has been written to disk
	expectedFiles := []string{
		keypaths.UserKeyPath(s.keyDir, s.hostname, s.username),                    // private key
		keypaths.TLSCertPath(s.keyDir, s.hostname, s.username),                    // Teleport TLS certificate
		keypaths.SSHCertPath(s.keyDir, s.hostname, s.username, s.key.ClusterName), // SSH certificate
	}
	for _, file := range expectedFiles {
		require.FileExists(t, file)
	}

	// get all agent keys from teleport agent and system agent
	teleportAgentKeys, err := lka.ExtendedAgent.List()
	require.NoError(t, err)
	systemAgentKeys, err := lka.sshAgent.List()
	require.NoError(t, err)

	// check that we've loaded a cert as well as a private key into the teleport agent
	// and it's for the user we expected to add a certificate for
	expectComment := teleportAgentKeyComment(s.key.KeyIndex)
	require.Len(t, teleportAgentKeys, 2)
	require.Equal(t, ssh.CertAlgoRSAv01, teleportAgentKeys[0].Type())
	require.Equal(t, expectComment, teleportAgentKeys[0].Comment)
	require.Equal(t, "ssh-rsa", teleportAgentKeys[1].Type())
	require.Equal(t, expectComment, teleportAgentKeys[1].Comment)

	// check that we've loaded a cert as well as a private key into the system again
	found := false
	for _, sak := range systemAgentKeys {
		if sak.Comment == expectComment && sak.Type() == "ssh-rsa" {
			found = true
		}
	}
	require.True(t, found)
	found = false
	for _, sak := range systemAgentKeys {
		if sak.Comment == expectComment && sak.Type() == ssh.CertAlgoRSAv01 {
			found = true
		}
	}
	require.True(t, found)

}

// TestLoadKey ensures correct loading of a key into an agent. This test
// checks the following:
//   - Loading a key multiple times overwrites the same key.
//   - Loading a key for separate teleport users/clusters does not overwrite existing keys.
//   - The key is correctly loaded into the agent. This is tested by having
//     the agent sign data that is then verified using the public key
//     directly.
func TestLoadKey(t *testing.T) {
	s := makeSuite(t)
	keyAgent := s.newKeyAgent(t)

	// get all the keys in the teleport and system agent
	teleportAgentKeys, err := keyAgent.ExtendedAgent.List()
	require.NoError(t, err)
	systemAgentKeys, err := keyAgent.sshAgent.List()
	require.NoError(t, err)

	// Create 3 separate keys, with overlapping user and cluster names
	keys := []*Key{
		s.key,
		s.genKey(t, s.key.Username, "other-proxy-host"),
		s.genKey(t, "other-user", s.key.ProxyHost),
	}

	// We should see two agent keys for each key added
	agentsPerKey := 2
	if runtime.GOOS == constants.WindowsOS {
		// or just 1 agent key for windows
		agentsPerKey = 1
	}

	for i, key := range keys {
		t.Cleanup(func() {
			err = keyAgent.UnloadKey(key.KeyIndex)
			require.NoError(t, err)
		})

		t.Run(fmt.Sprintf("key %v", i+1), func(t *testing.T) {
			// load each key to the agent twice, this should not
			// lead to duplicate keys in the agent.
			keyAgent.username = key.Username
			keyAgent.proxyHost = key.ProxyHost
			err = keyAgent.LoadKey(*key)
			require.NoError(t, err)
			err = keyAgent.LoadKey(*key)
			require.NoError(t, err)

			// get an updated list of all keys in the teleport and system agent,
			// and check that each list has grown by the expected amount.
			expectTeleportAgentKeyCount := len(teleportAgentKeys) + agentsPerKey
			expectSystemAgentKeyCount := len(systemAgentKeys) + agentsPerKey
			teleportAgentKeys, err = keyAgent.ExtendedAgent.List()
			require.NoError(t, err)
			require.Len(t, teleportAgentKeys, expectTeleportAgentKeyCount)
			systemAgentKeys, err = keyAgent.sshAgent.List()
			require.NoError(t, err)
			require.Len(t, systemAgentKeys, expectSystemAgentKeyCount)

			// gather all agent keys for the added key, making sure
			// we added the correct amount to each agent.
			keyAgentName := teleportAgentKeyComment(key.KeyIndex)
			var agentKeysForKey []*agent.Key
			for _, agentKey := range teleportAgentKeys {
				if agentKey.Comment == keyAgentName {
					agentKeysForKey = append(agentKeysForKey, agentKey)
				}
			}
			require.Len(t, agentKeysForKey, agentsPerKey)
			for _, agentKey := range systemAgentKeys {
				if agentKey.Comment == keyAgentName {
					agentKeysForKey = append(agentKeysForKey, agentKey)
				}
			}
			require.Len(t, agentKeysForKey, agentsPerKey*2)

			// verify that each new agent key can be used to sign
			for _, agentKey := range agentKeysForKey {
				// now sign data using the retrieved agent keys
				userdata := []byte("hello, world")
				teleportAgentSignature, err := keyAgent.ExtendedAgent.Sign(agentKey, userdata)
				require.NoError(t, err)
				systemAgentSignature, err := keyAgent.sshAgent.Sign(agentKey, userdata)
				require.NoError(t, err)

				// verify data signed by both the teleport agent and system agent was signed correctly
				err = key.PrivateKey.SSHPublicKey().Verify(userdata, teleportAgentSignature)
				require.NoError(t, err)
				err = key.PrivateKey.SSHPublicKey().Verify(userdata, systemAgentSignature)
				require.NoError(t, err)
			}
		})
	}
}

func TestHostCertVerification(t *testing.T) {
	s := makeSuite(t)

	// Make a new local agent.
	keystore, err := NewFSLocalKeyStore(s.keyDir)
	require.NoError(t, err)
	lka, err := NewLocalAgent(LocalAgentConfig{
		Keystore:   keystore,
		ProxyHost:  s.hostname,
		Username:   s.username,
		KeysOption: AddKeysToAgentAuto,
	})
	require.NoError(t, err)

	// By default user has not refused any hosts.
	require.False(t, lka.UserRefusedHosts())

	err = lka.AddKey(s.key)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = lka.UnloadKey(s.key.KeyIndex)
		require.NoError(t, err)
	})

	// Create a CA, generate a keypair for the CA, and add it to the known
	// hosts cache (done by "tsh login").
	keygen := testauthority.New()

	type ca struct {
		signer       ssh.Signer
		trustedCerts auth.TrustedCerts
	}
	generateCA := func(hostnames ...string) []ca {
		result := make([]ca, 0, len(hostnames))
		usedKeys := make(map[string]struct{})

		for _, hostname := range hostnames {
			var caPriv, caPub []byte
			var err error

			// retry until we get a unique keypair
			attempts := 20
			for i := 0; i < attempts; i++ {
				if i == attempts-1 {
					require.FailNowf(t, "could not find a unique keypair", "made %d attempts", i)
				}
				caPriv, caPub, err = keygen.GenerateKeyPair()
				require.NoError(t, err)

				// ensure we don't reuse the same keypair for different hosts
				if _, ok := usedKeys[string(caPriv)]; ok {
					continue
				}
				usedKeys[string(caPriv)] = struct{}{}
				break
			}

			caSigner, err := ssh.ParsePrivateKey(caPriv)
			require.NoError(t, err)
			caPublicKey, _, _, _, err := ssh.ParseAuthorizedKey(caPub)
			require.NoError(t, err)
			err = lka.keyStore.AddKnownHostKeys(hostname, s.hostname, []ssh.PublicKey{caPublicKey})
			require.NoError(t, err)

			_, trustedCerts, err := newSelfSignedCA(caPriv, hostname)
			require.NoError(t, err)
			trustedCerts.ClusterName = hostname
			result = append(result, ca{signer: caSigner, trustedCerts: trustedCerts})
		}
		require.Len(t, result, len(hostnames))
		return result
	}
	cas := generateCA("example.com", "leaf.example.com")
	root, leaf := cas[0], cas[1]

	// Call SaveTrustedCerts to create cas profile dir - this step is needed to support migration from profile combined
	// CA file certs.pem to per cluster CA files in cas profile directory.
	err = lka.keyStore.SaveTrustedCerts(s.hostname, []auth.TrustedCerts{root.trustedCerts, leaf.trustedCerts})
	require.NoError(t, err)

	// Generate a host certificate for node with role "node".
	_, rootHostPub, err := keygen.GenerateKeyPair()
	require.NoError(t, err)
	rootHostCertBytes, err := keygen.GenerateHostCert(services.HostCertParams{
		CASigner:      root.signer,
		PublicHostKey: rootHostPub,
		HostID:        "5ff40d80-9007-4f28-8f49-7d4fda2f574d",
		NodeName:      "server01",
		Principals: []string{
			"127.0.0.1",
		},
		ClusterName: "example.com",
		Role:        types.RoleNode,
		TTL:         1 * time.Hour,
	})
	require.NoError(t, err)
	rootHostPublicKey, _, _, _, err := ssh.ParseAuthorizedKey(rootHostCertBytes)
	require.NoError(t, err)

	_, leafHostPub, err := keygen.GenerateKeyPair()
	require.NoError(t, err)
	leafHostCertBytes, err := keygen.GenerateHostCert(services.HostCertParams{
		CASigner:      leaf.signer,
		PublicHostKey: leafHostPub,
		HostID:        "620bb71c-c9eb-4f6d-9823-f7d9125ebb1d",
		NodeName:      "server02",
		ClusterName:   "leaf.example.com",
		Role:          types.RoleNode,
		TTL:           1 * time.Hour,
	})
	require.NoError(t, err)
	leafHostPublicKey, _, _, _, err := ssh.ParseAuthorizedKey(leafHostCertBytes)
	require.NoError(t, err)

	tests := []struct {
		name          string
		inAddr        string
		hostPublicKey ssh.PublicKey
		loadAllCAs    bool
		assert        require.ErrorAssertionFunc
	}{
		{
			name:          "Correct DNS is valid",
			inAddr:        "server01.example.com:3022",
			hostPublicKey: rootHostPublicKey,
			assert:        require.NoError,
		},
		{
			name:          "Hostname only is valid",
			inAddr:        "server01:3022",
			hostPublicKey: rootHostPublicKey,
			assert:        require.NoError,
		},
		{
			name:          "IP is valid",
			inAddr:        "127.0.0.1:3022",
			hostPublicKey: rootHostPublicKey,
			assert:        require.NoError,
		},
		{
			name:          "UUID is valid",
			inAddr:        "5ff40d80-9007-4f28-8f49-7d4fda2f574d.example.com:3022",
			hostPublicKey: rootHostPublicKey,
			assert:        require.NoError,
		},
		{
			name:          "Wrong DNS name is invalid",
			inAddr:        "server02.example.com:3022",
			hostPublicKey: rootHostPublicKey,
			assert:        require.Error,
		},
		{
			name:          "Alt cluster rejected by default",
			inAddr:        "server02.leaf.example.com:3022",
			hostPublicKey: leafHostPublicKey,
			assert:        require.Error,
		},
		{
			name:          "Alt cluster accepted",
			inAddr:        "server02.leaf.example.com:3022",
			hostPublicKey: leafHostPublicKey,
			loadAllCAs:    true,
			assert:        require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.loadAllCAs {
				lka.siteName = ""
				lka.loadAllCAs = true
			} else {
				lka.siteName = "example.com"
				lka.loadAllCAs = false
			}
			err = lka.CheckHostSignature(tt.inAddr, nil, tt.hostPublicKey)
			tt.assert(t, err)
		})
	}
}

func TestHostKeyVerification(t *testing.T) {
	s := makeSuite(t)

	// make a new local agent
	keystore, err := NewFSLocalKeyStore(s.keyDir)
	require.NoError(t, err)
	lka, err := NewLocalAgent(LocalAgentConfig{
		Keystore:   keystore,
		ProxyHost:  s.hostname,
		Username:   s.username,
		KeysOption: AddKeysToAgentAuto,
		Insecure:   true,
	})
	require.NoError(t, err)

	err = lka.AddKey(s.key)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = lka.UnloadKey(s.key.KeyIndex)
		require.NoError(t, err)
	})

	// Call SaveTrustedCerts to create cas profile dir - this step is needed to support migration from profile combined
	// CA file certs.pem to per cluster CA files in cas profile directory.
	err = lka.keyStore.SaveTrustedCerts(s.hostname, nil)
	require.NoError(t, err)

	// by default user has not refused any hosts:
	require.False(t, lka.UserRefusedHosts())

	// make a fake host key:
	keygen := testauthority.New()
	_, pub, err := keygen.GenerateKeyPair()
	require.NoError(t, err)
	pk, _, _, _, err := ssh.ParseAuthorizedKey(pub)
	require.NoError(t, err)

	// test user refusing connection:
	fakeErr := fmt.Errorf("luna cannot be trusted")
	lka.hostPromptFunc = func(host string, k ssh.PublicKey) error {
		require.Equal(t, "luna", host)
		require.Equal(t, pk, k)
		return fakeErr
	}
	var a net.TCPAddr
	err = lka.CheckHostSignature("luna", &a, pk)
	require.Error(t, err)
	require.Equal(t, "luna cannot be trusted", err.Error())
	require.True(t, lka.UserRefusedHosts())

	// clean user answer:
	delete(lka.noHosts, "luna")
	require.False(t, lka.UserRefusedHosts())

	// now lets simulate user being asked:
	userWasAsked := false
	lka.hostPromptFunc = func(host string, k ssh.PublicKey) error {
		// user answered "yes"
		userWasAsked = true
		return nil
	}
	require.False(t, lka.UserRefusedHosts())
	err = lka.CheckHostSignature("luna", &a, pk)
	require.NoError(t, err)
	require.True(t, userWasAsked)

	// now lets simulate automatic host verification (no need to ask user, he
	// just said "yes")
	userWasAsked = false
	require.False(t, lka.UserRefusedHosts())
	err = lka.CheckHostSignature("luna", &a, pk)
	require.NoError(t, err)
	require.False(t, userWasAsked)
}

func TestDefaultHostPromptFunc(t *testing.T) {
	s := makeSuite(t)

	keygen := testauthority.New()

	keystore, err := NewFSLocalKeyStore(s.keyDir)
	require.NoError(t, err)
	a, err := NewLocalAgent(LocalAgentConfig{
		Keystore:   keystore,
		ProxyHost:  s.hostname,
		Username:   s.username,
		KeysOption: AddKeysToAgentAuto,
	})
	require.NoError(t, err)

	_, keyBytes, err := keygen.GenerateKeyPair()
	require.NoError(t, err)
	key, _, _, _, err := ssh.ParseAuthorizedKey(keyBytes)
	require.NoError(t, err)

	tests := []struct {
		inAnswer []byte
		assert   require.ErrorAssertionFunc
	}{
		{
			inAnswer: []byte("y\n"),
			assert:   require.NoError,
		},
		{
			inAnswer: []byte("n\n"),
			assert:   require.Error,
		},
		{
			inAnswer: []byte("foo\n"),
			assert:   require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(string(bytes.TrimSpace(tt.inAnswer)), func(t *testing.T) {
			// Write an answer to the "keyboard".
			var buf bytes.Buffer
			buf.Write(tt.inAnswer)

			err = a.defaultHostPromptFunc("example.com", key, io.Discard, &buf)
			tt.assert(t, err)
		})
	}
}

func TestLocalKeyAgent_AddDatabaseKey(t *testing.T) {
	s := makeSuite(t)

	// make a new local agent
	keystore, err := NewFSLocalKeyStore(s.keyDir)
	require.NoError(t, err)
	lka, err := NewLocalAgent(
		LocalAgentConfig{
			Keystore:   keystore,
			ProxyHost:  s.hostname,
			Username:   s.username,
			KeysOption: AddKeysToAgentAuto,
		})
	require.NoError(t, err)

	t.Run("no database cert", func(t *testing.T) {
		require.Error(t, lka.AddDatabaseKey(s.key))
	})

	t.Run("success", func(t *testing.T) {
		// modify key to have db cert
		addKey := *s.key
		addKey.DBTLSCerts = map[string][]byte{"some-db": addKey.TLSCert}
		require.NoError(t, lka.SaveTrustedCerts([]auth.TrustedCerts{s.tlscaCert}))
		require.NoError(t, lka.AddDatabaseKey(&addKey))

		getKey, err := lka.GetKey(addKey.ClusterName, WithDBCerts{})
		require.NoError(t, err)
		require.Contains(t, getKey.DBTLSCerts, "some-db")
	})
}

func (s *KeyAgentTestSuite) makeKey(t *testing.T, username, proxyHost string, priv *keys.PrivateKey) *Key {
	keygen := testauthority.New()
	ttl := time.Minute

	clock := clockwork.NewRealClock()
	identity := tlsca.Identity{
		Username: username,
	}

	subject, err := identity.Subject()
	require.NoError(t, err)
	tlsCert, err := s.tlsca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: priv.Public(),
		Subject:   subject,
		NotAfter:  clock.Now().UTC().Add(ttl),
	})
	require.NoError(t, err)

	pemBytes, ok := fixtures.PEMBytes["rsa"]
	require.True(t, ok, "RSA key not found in fixtures")

	caSigner, err := ssh.ParsePrivateKey(pemBytes)
	require.NoError(t, err)

	certificate, err := keygen.GenerateUserCert(services.UserCertParams{
		CertificateFormat:     constants.CertificateFormatStandard,
		CASigner:              caSigner,
		PublicUserKey:         ssh.MarshalAuthorizedKey(priv.SSHPublicKey()),
		Username:              username,
		AllowedLogins:         []string{username},
		TTL:                   ttl,
		PermitAgentForwarding: true,
		PermitPortForwarding:  true,
		RouteToCluster:        s.clusterName,
	})
	require.NoError(t, err)

	return &Key{
		PrivateKey: priv,
		Cert:       certificate,
		TLSCert:    tlsCert,
		KeyIndex: KeyIndex{
			ProxyHost:   proxyHost,
			Username:    username,
			ClusterName: s.clusterName,
		},
	}
}

func (s *KeyAgentTestSuite) genKey(t *testing.T, username, proxyHost string) *Key {
	priv, err := native.GeneratePrivateKey()
	require.NoError(t, err)
	return s.makeKey(t, username, proxyHost, priv)
}

func startDebugAgent(t *testing.T) error {
	// Create own tmp dir instead of using t.TmpDir
	// because net.Listen("unix", path) has dir path length limitation
	tempDir, err := os.MkdirTemp("", "teleport-test")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	socketpath := filepath.Join(tempDir, "agent.sock")
	listener, err := net.Listen("unix", socketpath)
	if err != nil {
		return trace.Wrap(err)
	}

	systemAgent := agent.NewKeyring()
	t.Setenv(teleport.SSHAuthSock, socketpath)

	startedC := make(chan struct{})
	doneC := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		// agent is listening and environment variable is set, unblock now
		close(startedC)
		for {
			conn, err := listener.Accept()
			if err != nil {
				if !utils.IsUseOfClosedNetworkError(err) {
					log.Warnf("Unexpected response from listener.Accept: %v", err)
				}
				return
			}
			wg.Add(2)
			go func() {
				agent.ServeAgent(systemAgent, conn)
				wg.Done()
			}()
			go func() {
				<-doneC
				conn.Close()
				wg.Done()
			}()
		}
	}()

	go func() {
		<-doneC
		listener.Close()
		wg.Done()
	}()

	t.Cleanup(func() {
		close(doneC)
		wg.Wait()
	})

	// block until agent is started
	<-startedC
	return nil
}

func (s *KeyAgentTestSuite) newKeyAgent(t *testing.T) *LocalKeyAgent {
	// make a new local agent
	keystore, err := NewFSLocalKeyStore(s.keyDir)
	require.NoError(t, err)
	keyAgent, err := NewLocalAgent(LocalAgentConfig{
		Keystore:   keystore,
		ProxyHost:  s.hostname,
		Site:       s.clusterName,
		Username:   s.username,
		KeysOption: AddKeysToAgentAuto,
	})
	require.NoError(t, err)
	return keyAgent
}
