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
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
)

type KeyAgentTestSuite struct {
	keyDir   string
	key      *Key
	username string
	hostname string
	tlsca    *tlsca.CertAuthority
}

var _ = check.Suite(&KeyAgentTestSuite{})
var _ = fmt.Printf

func (s *KeyAgentTestSuite) SetUpSuite(c *check.C) {
	var err error
	utils.InitLoggerForTests()

	// path to temporary  ~/.tsh directory to use during tests
	s.keyDir, err = ioutil.TempDir("", "keyagent-test-")
	c.Assert(err, check.IsNil)

	// temporary username and hostname to use during tests
	s.username = "foo"
	s.hostname = "bar"

	pemBytes, ok := fixtures.PEMBytes["rsa"]
	c.Assert(ok, check.Equals, true)

	s.tlsca, _, err = newSelfSignedCA(pemBytes)
	c.Assert(err, check.IsNil)

	// temporary key to use during tests
	s.key, err = s.makeKey(s.username, []string{s.username}, 1*time.Minute)
	c.Assert(err, check.IsNil)

	// start a debug agent that will be used in tests
	err = startDebugAgent()
	c.Assert(err, check.IsNil)
}

func (s *KeyAgentTestSuite) TearDownSuite(c *check.C) {
	err := os.RemoveAll(s.keyDir)
	c.Assert(err, check.IsNil)
}

func (s *KeyAgentTestSuite) SetUpTest(c *check.C) {
}

// TestAddKey ensures correct adding of ssh keys. This test checks the following:
//   * When adding a key it's written to disk.
//   * When we add a key, it's added to both the teleport ssh agent as well
//     as the system ssh agent.
//   * When we add a key, both the certificate and private key are added into
//     the both the teleport ssh agent and the system ssh agent.
//   * When we add a key, it's tagged with a comment that indicates that it's
//     a teleport key with the teleport username.
func (s *KeyAgentTestSuite) TestAddKey(c *check.C) {
	// make a new local agent
	lka, err := NewLocalAgent(s.keyDir, s.hostname, s.username, true)
	c.Assert(err, check.IsNil)

	// add the key to the local agent, this should write the key
	// to disk as well as load it in the agent
	_, err = lka.AddKey(s.key)
	c.Assert(err, check.IsNil)

	// check that the key has been written to disk
	for _, ext := range []string{fileExtCert, "", fileExtPub} {
		_, err := os.Stat(fmt.Sprintf("%v/keys/%v/%v%v", s.keyDir, s.hostname, s.username, ext))
		c.Assert(err, check.IsNil)
	}

	// get all agent keys from teleport agent and system agent
	teleportAgentKeys, err := lka.Agent.List()
	c.Assert(err, check.IsNil)
	systemAgentKeys, err := lka.sshAgent.List()
	c.Assert(err, check.IsNil)

	// check that we've loaded a cert as well as a private key into the teleport agent
	// and it's for the user we expected to add a certificate for
	c.Assert(teleportAgentKeys, check.HasLen, 2)
	c.Assert(teleportAgentKeys[0].Type(), check.Equals, "ssh-rsa-cert-v01@openssh.com")
	c.Assert(teleportAgentKeys[0].Comment, check.Equals, "teleport:"+s.username)
	c.Assert(teleportAgentKeys[1].Type(), check.Equals, "ssh-rsa")
	c.Assert(teleportAgentKeys[1].Comment, check.Equals, "teleport:"+s.username)

	// check that we've loaded a cert as well as a private key into the system again
	found := false
	for _, sak := range systemAgentKeys {
		if sak.Comment == "teleport:"+s.username && sak.Type() == "ssh-rsa" {
			found = true
		}
	}
	c.Assert(true, check.Equals, found)
	found = false
	for _, sak := range systemAgentKeys {
		if sak.Comment == "teleport:"+s.username && sak.Type() == "ssh-rsa-cert-v01@openssh.com" {
			found = true
		}
	}
	c.Assert(true, check.Equals, found)

	// unload all keys for this user from the teleport agent and system agent
	err = lka.UnloadKey()
	c.Assert(err, check.IsNil)
}

// TestLoadKey ensures correct loading of a key into an agent. This test
// checks the following:
//   * Loading a key multiple times overwrites the same key.
//   * The key is correctly loaded into the agent. This is tested by having
//     the agent sign data that is then verified using the public key
//     directly.
func (s *KeyAgentTestSuite) TestLoadKey(c *check.C) {
	userdata := []byte("hello, world")

	// make a new local agent
	lka, err := NewLocalAgent(s.keyDir, s.hostname, s.username, true)
	c.Assert(err, check.IsNil)

	// unload any keys that might be in the agent for this user
	err = lka.UnloadKey()
	c.Assert(err, check.IsNil)

	// get all the keys in the teleport and system agent
	teleportAgentKeys, err := lka.Agent.List()
	c.Assert(err, check.IsNil)
	teleportAgentInitialKeyCount := len(teleportAgentKeys)
	systemAgentKeys, err := lka.sshAgent.List()
	c.Assert(err, check.IsNil)
	systemAgentInitialKeyCount := len(systemAgentKeys)

	// load the key to the twice, this should only
	// result in one key for this user in the agent
	_, err = lka.LoadKey(*s.key)
	c.Assert(err, check.IsNil)
	_, err = lka.LoadKey(*s.key)
	c.Assert(err, check.IsNil)

	// get all the keys in the teleport and system agent
	teleportAgentKeys, err = lka.Agent.List()
	c.Assert(err, check.IsNil)
	systemAgentKeys, err = lka.sshAgent.List()
	c.Assert(err, check.IsNil)

	// check if we have the correct counts
	c.Assert(teleportAgentKeys, check.HasLen, teleportAgentInitialKeyCount+2)
	c.Assert(systemAgentKeys, check.HasLen, systemAgentInitialKeyCount+2)

	// now sign data using the teleport agent and system agent
	teleportAgentSignature, err := lka.Agent.Sign(teleportAgentKeys[0], userdata)
	c.Assert(err, check.IsNil)
	systemAgentSignature, err := lka.sshAgent.Sign(systemAgentKeys[0], userdata)
	c.Assert(err, check.IsNil)

	// parse the pem bytes for the private key, create a signer, and extract the public key
	sshPrivateKey, err := ssh.ParseRawPrivateKey(s.key.Priv)
	c.Assert(err, check.IsNil)
	sshSigner, err := ssh.NewSignerFromKey(sshPrivateKey)
	c.Assert(err, check.IsNil)
	sshPublicKey := sshSigner.PublicKey()

	// verify data signed by both the teleport agent and system agent was signed correctly
	err = sshPublicKey.Verify(userdata, teleportAgentSignature)
	c.Assert(err, check.IsNil)
	err = sshPublicKey.Verify(userdata, systemAgentSignature)
	c.Assert(err, check.IsNil)

	// unload all keys from the teleport agent and system agent
	err = lka.UnloadKey()
	c.Assert(err, check.IsNil)
}

func (s *KeyAgentTestSuite) TestHostCertVerification(c *check.C) {
	// Make a new local agent.
	lka, err := NewLocalAgent(s.keyDir, s.hostname, s.username, true)
	c.Assert(err, check.IsNil)

	// By default user has not refused any hosts.
	c.Assert(lka.UserRefusedHosts(), check.Equals, false)

	// Create a CA, generate a keypair for the CA, and add it to the known
	// hosts cache (done by "tsh login").
	keygen := testauthority.New()
	caPriv, caPub, err := keygen.GenerateKeyPair("")
	c.Assert(err, check.IsNil)
	caPublicKey, _, _, _, err := ssh.ParseAuthorizedKey(caPub)
	c.Assert(err, check.IsNil)
	err = lka.keyStore.AddKnownHostKeys("example.com", []ssh.PublicKey{caPublicKey})
	c.Assert(err, check.IsNil)

	// Generate a host certificate for node with role "node".
	_, hostPub, err := keygen.GenerateKeyPair("")
	c.Assert(err, check.IsNil)
	roles, err := teleport.ParseRoles("node")
	c.Assert(err, check.IsNil)
	hostCertBytes, err := keygen.GenerateHostCert(services.HostCertParams{
		PrivateCASigningKey: caPriv,
		CASigningAlg:        defaults.CASignatureAlgorithm,
		PublicHostKey:       hostPub,
		HostID:              "5ff40d80-9007-4f28-8f49-7d4fda2f574d",
		NodeName:            "server01",
		Principals: []string{
			"127.0.0.1",
		},
		ClusterName: "example.com",
		Roles:       roles,
		TTL:         1 * time.Hour,
	})
	c.Assert(err, check.IsNil)
	hostPublicKey, _, _, _, err := ssh.ParseAuthorizedKey(hostCertBytes)
	c.Assert(err, check.IsNil)

	tests := []struct {
		inAddr   string
		outError bool
	}{
		// Correct DNS is valid.
		{
			inAddr:   "server01.example.com:3022",
			outError: false,
		},
		// Hostname only is valid.
		{
			inAddr:   "server01:3022",
			outError: false,
		},
		// IP is valid.
		{
			inAddr:   "127.0.0.1:3022",
			outError: false,
		},
		// UUID is valid.
		{
			inAddr:   "5ff40d80-9007-4f28-8f49-7d4fda2f574d.example.com:3022",
			outError: false,
		},
		// Wrong DNS name is invalid.
		{
			inAddr:   "server02.example.com:3022",
			outError: true,
		},
	}

	for _, tt := range tests {
		err = lka.CheckHostSignature(tt.inAddr, nil, hostPublicKey)
		if tt.outError {
			c.Assert(err, check.NotNil)
		} else {
			c.Assert(err, check.IsNil)
		}
	}
}

func (s *KeyAgentTestSuite) TestHostKeyVerification(c *check.C) {
	// make a new local agent
	lka, err := NewLocalAgent(s.keyDir, s.hostname, s.username, true)
	c.Assert(err, check.IsNil)

	// by default user has not refused any hosts:
	c.Assert(lka.UserRefusedHosts(), check.Equals, false)

	// make a fake host key:
	keygen := testauthority.New()
	_, pub, err := keygen.GenerateKeyPair("")
	c.Assert(err, check.IsNil)
	pk, _, _, _, err := ssh.ParseAuthorizedKey(pub)
	c.Assert(err, check.IsNil)

	// test user refusing connection:
	fakeErr := trace.Errorf("luna cannot be trusted!")
	lka.hostPromptFunc = func(host string, k ssh.PublicKey) error {
		c.Assert(host, check.Equals, "luna")
		c.Assert(k, check.Equals, pk)
		return fakeErr
	}
	var a net.TCPAddr
	err = lka.CheckHostSignature("luna", &a, pk)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "luna cannot be trusted!")
	c.Assert(lka.UserRefusedHosts(), check.Equals, true)

	// clean user answer:
	delete(lka.noHosts, "luna")
	c.Assert(lka.UserRefusedHosts(), check.Equals, false)

	// now lets simulate user being asked:
	userWasAsked := false
	lka.hostPromptFunc = func(host string, k ssh.PublicKey) error {
		// user answered "yes"
		userWasAsked = true
		return nil
	}
	c.Assert(lka.UserRefusedHosts(), check.Equals, false)
	err = lka.CheckHostSignature("luna", &a, pk)
	c.Assert(err, check.IsNil)
	c.Assert(userWasAsked, check.Equals, true)

	// now lets simulate automatic host verification (no need to ask user, he
	// just said "yes")
	userWasAsked = false
	c.Assert(lka.UserRefusedHosts(), check.Equals, false)
	err = lka.CheckHostSignature("luna", &a, pk)
	c.Assert(err, check.IsNil)
	c.Assert(userWasAsked, check.Equals, false)
}

func (s *KeyAgentTestSuite) TestDefaultHostPromptFunc(c *check.C) {
	keygen := testauthority.New()

	a, err := NewLocalAgent(s.keyDir, s.hostname, s.username, true)
	c.Assert(err, check.IsNil)

	_, keyBytes, err := keygen.GenerateKeyPair("")
	c.Assert(err, check.IsNil)
	key, _, _, _, err := ssh.ParseAuthorizedKey(keyBytes)
	c.Assert(err, check.IsNil)

	tests := []struct {
		inAnswer []byte
		outError bool
	}{
		{
			inAnswer: []byte("y\n"),
			outError: false,
		},
		{
			inAnswer: []byte("n\n"),
			outError: true,
		},
		{
			inAnswer: []byte("foo\n"),
			outError: true,
		},
	}

	for _, tt := range tests {
		// Write an answer to the "keyboard".
		var buf bytes.Buffer
		buf.Write(tt.inAnswer)

		err = a.defaultHostPromptFunc("example.com", key, ioutil.Discard, &buf)
		if tt.outError {
			c.Assert(err, check.NotNil)
		} else {
			c.Assert(err, check.IsNil)
		}
	}
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

	certificate, err := keygen.GenerateUserCert(services.UserCertParams{
		PrivateCASigningKey:   pemBytes,
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
	}, nil
}

func startDebugAgent() error {
	errorC := make(chan error)
	rand.Seed(time.Now().Unix())

	go func() {
		socketpath := filepath.Join(os.TempDir(),
			fmt.Sprintf("teleport-%d-%d.socket", os.Getpid(), rand.Uint32()))

		systemAgent := agent.NewKeyring()

		listener, err := net.Listen("unix", socketpath)
		if err != nil {
			errorC <- trace.Wrap(err)
			return
		}
		defer listener.Close()

		os.Setenv(teleport.SSHAuthSock, socketpath)

		// agent is listeninging and environment variable is set unblock now
		close(errorC)

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Warnf("Unexpected response from listener.Accept: %v", err)
				continue
			}

			go agent.ServeAgent(systemAgent, conn)
		}
	}()

	// block until agent is started
	return <-errorC
}
