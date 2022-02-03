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
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
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

	s.tlsca, s.tlscaCert, err = newSelfSignedCA(pemBytes)
	require.NoError(t, err)

	s.key, err = s.makeKey(s.username, []string{s.username}, 1*time.Minute)
	require.NoError(t, err)

	return s
}

// TestAddKey ensures correct adding of ssh keys. This test checks the following:
//   * When adding a key it's written to disk.
//   * When we add a key, it's added to both the teleport ssh agent as well
//     as the system ssh agent.
//   * When we add a key, both the certificate and private key are added into
//     the both the teleport ssh agent and the system ssh agent.
//   * When we add a key, it's tagged with a comment that indicates that it's
//     a teleport key with the teleport username.
func TestAddKey(t *testing.T) {
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

	// add the key to the local agent, this should write the key
	// to disk as well as load it in the agent
	_, err = lka.AddKey(s.key)
	require.NoError(t, err)

	// check that the key has been written to disk
	expectedFiles := []string{
		keypaths.UserKeyPath(s.keyDir, s.hostname, s.username),                    // private key
		keypaths.SSHCAsPath(s.keyDir, s.hostname, s.username),                     // public key
		keypaths.TLSCertPath(s.keyDir, s.hostname, s.username),                    // Teleport TLS certificate
		keypaths.SSHCertPath(s.keyDir, s.hostname, s.username, s.key.ClusterName), // SSH certificate
	}
	for _, file := range expectedFiles {
		require.FileExists(t, file)
	}

	// get all agent keys from teleport agent and system agent
	teleportAgentKeys, err := lka.Agent.List()
	require.NoError(t, err)
	systemAgentKeys, err := lka.sshAgent.List()
	require.NoError(t, err)

	// check that we've loaded a cert as well as a private key into the teleport agent
	// and it's for the user we expected to add a certificate for
	require.Len(t, teleportAgentKeys, 2)
	require.Equal(t, "ssh-rsa-cert-v01@openssh.com", teleportAgentKeys[0].Type())
	require.Equal(t, "teleport:"+s.username, teleportAgentKeys[0].Comment)
	require.Equal(t, "ssh-rsa", teleportAgentKeys[1].Type())
	require.Equal(t, "teleport:"+s.username, teleportAgentKeys[1].Comment)

	// check that we've loaded a cert as well as a private key into the system again
	found := false
	for _, sak := range systemAgentKeys {
		if sak.Comment == "teleport:"+s.username && sak.Type() == "ssh-rsa" {
			found = true
		}
	}
	require.True(t, found)
	found = false
	for _, sak := range systemAgentKeys {
		if sak.Comment == "teleport:"+s.username && sak.Type() == "ssh-rsa-cert-v01@openssh.com" {
			found = true
		}
	}
	require.True(t, found)

	// unload all keys for this user from the teleport agent and system agent
	err = lka.UnloadKey()
	require.NoError(t, err)
}

// TestLoadKey ensures correct loading of a key into an agent. This test
// checks the following:
//   * Loading a key multiple times overwrites the same key.
//   * The key is correctly loaded into the agent. This is tested by having
//     the agent sign data that is then verified using the public key
//     directly.
func TestLoadKey(t *testing.T) {
	s := makeSuite(t)

	userdata := []byte("hello, world")

	// make a new local agent
	keystore, err := NewFSLocalKeyStore(s.keyDir)
	require.NoError(t, err)
	lka, err := NewLocalAgent(LocalAgentConfig{
		Keystore:   keystore,
		ProxyHost:  s.hostname,
		Username:   s.username,
		KeysOption: AddKeysToAgentAuto,
	})
	require.NoError(t, err)

	// unload any keys that might be in the agent for this user
	err = lka.UnloadKey()
	require.NoError(t, err)

	// get all the keys in the teleport and system agent
	teleportAgentKeys, err := lka.Agent.List()
	require.NoError(t, err)
	teleportAgentInitialKeyCount := len(teleportAgentKeys)
	systemAgentKeys, err := lka.sshAgent.List()
	require.NoError(t, err)
	systemAgentInitialKeyCount := len(systemAgentKeys)

	// load the key to the twice, this should only
	// result in one key for this user in the agent
	_, err = lka.LoadKey(*s.key)
	require.NoError(t, err)
	_, err = lka.LoadKey(*s.key)
	require.NoError(t, err)

	// get all the keys in the teleport and system agent
	teleportAgentKeys, err = lka.Agent.List()
	require.NoError(t, err)
	systemAgentKeys, err = lka.sshAgent.List()
	require.NoError(t, err)

	// check if we have the correct counts
	require.Len(t, teleportAgentKeys, teleportAgentInitialKeyCount+2)
	require.Len(t, systemAgentKeys, systemAgentInitialKeyCount+2)

	// now sign data using the teleport agent and system agent
	teleportAgentSignature, err := lka.Agent.Sign(teleportAgentKeys[0], userdata)
	require.NoError(t, err)
	systemAgentSignature, err := lka.sshAgent.Sign(systemAgentKeys[0], userdata)
	require.NoError(t, err)

	// parse the pem bytes for the private key, create a signer, and extract the public key
	sshPrivateKey, err := ssh.ParseRawPrivateKey(s.key.Priv)
	require.NoError(t, err)
	sshSigner, err := ssh.NewSignerFromKey(sshPrivateKey)
	require.NoError(t, err)
	sshPublicKey := sshSigner.PublicKey()

	// verify data signed by both the teleport agent and system agent was signed correctly
	err = sshPublicKey.Verify(userdata, teleportAgentSignature)
	require.NoError(t, err)
	err = sshPublicKey.Verify(userdata, systemAgentSignature)
	require.NoError(t, err)

	// unload all keys from the teleport agent and system agent
	err = lka.UnloadKey()
	require.NoError(t, err)
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

	lka.AddKey(s.key)
	// Create a CA, generate a keypair for the CA, and add it to the known
	// hosts cache (done by "tsh login").
	keygen := testauthority.New()
	caPriv, caPub, err := keygen.GenerateKeyPair("")
	require.NoError(t, err)
	caSigner, err := ssh.ParsePrivateKey(caPriv)
	require.NoError(t, err)
	caPublicKey, _, _, _, err := ssh.ParseAuthorizedKey(caPub)
	require.NoError(t, err)
	err = lka.keyStore.AddKnownHostKeys("example.com", s.hostname, []ssh.PublicKey{caPublicKey})
	require.NoError(t, err)

	// Call SaveTrustedCerts to create cas profile dir - this step is needed to support migration from profile combined
	// CA file certs.pem to per cluster CA files in cas profile directory.
	err = lka.keyStore.SaveTrustedCerts(s.hostname, nil)
	require.NoError(t, err)

	// Generate a host certificate for node with role "node".
	_, hostPub, err := keygen.GenerateKeyPair("")
	require.NoError(t, err)
	hostCertBytes, err := keygen.GenerateHostCert(services.HostCertParams{
		CASigner:      caSigner,
		CASigningAlg:  defaults.CASignatureAlgorithm,
		PublicHostKey: hostPub,
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
	hostPublicKey, _, _, _, err := ssh.ParseAuthorizedKey(hostCertBytes)
	require.NoError(t, err)

	tests := []struct {
		inAddr string
		assert require.ErrorAssertionFunc
	}{
		// Correct DNS is valid.
		{
			inAddr: "server01.example.com:3022",
			assert: require.NoError,
		},
		// Hostname only is valid.
		{
			inAddr: "server01:3022",
			assert: require.NoError,
		},
		// IP is valid.
		{
			inAddr: "127.0.0.1:3022",
			assert: require.NoError,
		},
		// UUID is valid.
		{
			inAddr: "5ff40d80-9007-4f28-8f49-7d4fda2f574d.example.com:3022",
			assert: require.NoError,
		},
		// Wrong DNS name is invalid.
		{
			inAddr: "server02.example.com:3022",
			assert: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.inAddr, func(t *testing.T) {
			err = lka.CheckHostSignature(tt.inAddr, nil, hostPublicKey)
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

	lka.AddKey(s.key)
	// Call SaveTrustedCerts to create cas profile dir - this step is needed to support migration from profile combined
	// CA file certs.pem to per cluster CA files in cas profile directory.
	err = lka.keyStore.SaveTrustedCerts(s.hostname, nil)
	require.NoError(t, err)

	// by default user has not refused any hosts:
	require.False(t, lka.UserRefusedHosts())

	// make a fake host key:
	keygen := testauthority.New()
	_, pub, err := keygen.GenerateKeyPair("")
	require.NoError(t, err)
	pk, _, _, _, err := ssh.ParseAuthorizedKey(pub)
	require.NoError(t, err)

	// test user refusing connection:
	fakeErr := trace.Errorf("luna cannot be trusted!")
	lka.hostPromptFunc = func(host string, k ssh.PublicKey) error {
		require.Equal(t, "luna", host)
		require.Equal(t, pk, k)
		return fakeErr
	}
	var a net.TCPAddr
	err = lka.CheckHostSignature("luna", &a, pk)
	require.Error(t, err)
	require.Equal(t, "luna cannot be trusted!", err.Error())
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

	_, keyBytes, err := keygen.GenerateKeyPair("")
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

func (s *KeyAgentTestSuite) makeKey(username string, allowedLogins []string, ttl time.Duration) (*Key, error) {
	keygen := testauthority.New()

	privateKey, publicKey, err := keygen.GenerateKeyPair("")
	if err != nil {
		return nil, err
	}

	// reuse the same RSA keys for SSH and TLS keys
	cryptoPubKey, err := sshutils.CryptoPublicKey(publicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clock := clockwork.NewRealClock()
	identity := tlsca.Identity{
		Username: username,
	}

	subject, err := identity.Subject()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCert, err := s.tlsca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: cryptoPubKey,
		Subject:   subject,
		NotAfter:  clock.Now().UTC().Add(ttl),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pemBytes, ok := fixtures.PEMBytes["rsa"]
	if !ok {
		return nil, trace.BadParameter("RSA key not found in fixtures")
	}
	caSigner, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certificate, err := keygen.GenerateUserCert(services.UserCertParams{
		CASigner:              caSigner,
		CASigningAlg:          defaults.CASignatureAlgorithm,
		PublicUserKey:         publicKey,
		Username:              username,
		AllowedLogins:         allowedLogins,
		TTL:                   ttl,
		PermitAgentForwarding: true,
		PermitPortForwarding:  true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Key{
		Priv:    privateKey,
		Pub:     publicKey,
		Cert:    certificate,
		TLSCert: tlsCert,
		KeyIndex: KeyIndex{
			ProxyHost:   s.hostname,
			Username:    username,
			ClusterName: s.clusterName,
		},
	}, nil
}

func startDebugAgent(t *testing.T) error {
	// Create own tmp dir instead of using t.TmpDir
	// because net.Listen("unix", path) has dir path length limitation
	tempDir, err := ioutil.TempDir("", "teleport-test")
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
