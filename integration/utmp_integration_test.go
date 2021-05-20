/*
Copyright 2021 Gravitational, Inc.

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

package integration

import (
	"context"
	"os"
	"path"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/regular"
	"github.com/gravitational/teleport/lib/srv/uacc"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// teleportTestUser is additional user used for tests
const teleportTestUser = "teleport-test"

// wildcardAllow is used in tests to allow access to all labels.
var wildcardAllow = types.Labels{
	services.Wildcard: []string{services.Wildcard},
}

type SrvCtx struct {
	srv        *regular.Server
	signer     ssh.Signer
	server     *auth.TestServer
	clock      clockwork.FakeClock
	nodeClient *auth.Client
	nodeID     string
	utmpPath   string
}

// TestRootUTMPEntryExists verifies that user accounting is done on supported systems.
func TestRootUTMPEntryExists(t *testing.T) {
	if !isRoot() {
		t.Skip("This test will be skipped because tests are not being run as root.")
	}

	s := newSrvCtx(t)
	up, err := newUpack(s, teleportTestUser, []string{teleportTestUser}, wildcardAllow)
	require.NoError(t, err)

	sshConfig := &ssh.ClientConfig{
		User:            teleportTestUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}

	client, err := ssh.Dial("tcp", s.srv.Addr(), sshConfig)
	require.NoError(t, err)
	defer func() {
		err := client.Close()
		require.NoError(t, err)
	}()

	se, err := client.NewSession()
	require.NoError(t, err)
	defer se.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	require.NoError(t, se.RequestPty("xterm", 80, 80, modes), nil)
	err = se.Shell()
	require.NoError(t, err)

	start := time.Now()
	for time.Since(start) < 5*time.Minute {
		time.Sleep(time.Second)
		entryExists := uacc.UserWithPtyInDatabase(s.utmpPath, teleportTestUser)
		if entryExists == nil {
			return
		}
		if !trace.IsNotFound(entryExists) {
			require.NoError(t, err)
		}
	}

	t.Errorf("did not detect utmp entry within 5 minutes")
}

// upack holds all ssh signing artefacts needed for signing and checking user keys
type upack struct {
	// key is a raw private user key
	key []byte

	// pkey is parsed private SSH key
	pkey interface{}

	// pub is a public user key
	pub []byte

	// cert is a certificate signed by user CA
	cert []byte

	// pcert is a parsed SSH Certificae
	pcert *ssh.Certificate

	// signer is a signer that answers signing challenges using private key
	signer ssh.Signer

	// certSigner is a signer that answers signing challenges using private
	// key and a certificate issued by user certificate authority
	certSigner ssh.Signer
}

const hostID = "00000000-0000-0000-0000-000000000000"

func TouchFile(name string) error {
	file, err := os.OpenFile(name, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	return file.Close()
}

// This returns the utmp path.
func newSrvCtx(t *testing.T) *SrvCtx {
	s := &SrvCtx{}

	t.Cleanup(func() {
		if s.srv != nil {
			require.NoError(t, s.srv.Close())
		}
		if s.server != nil {
			require.NoError(t, s.server.Shutdown(context.Background()))
		}
	})

	s.clock = clockwork.NewFakeClock()
	tempdir := t.TempDir()

	var err error
	s.server, err = auth.NewTestServer(auth.TestServerConfig{
		Auth: auth.TestAuthServerConfig{
			ClusterName: "localhost",
			Dir:         tempdir,
			Clock:       s.clock,
		}})
	require.NoError(t, err)

	// set up host private key and certificate
	certs, err := s.server.Auth().GenerateServerKeys(auth.GenerateServerKeysRequest{
		HostID:   hostID,
		NodeName: s.server.ClusterName(),
		Roles:    teleport.Roles{teleport.RoleNode},
	})
	require.NoError(t, err)

	// set up user CA and set up a user that has access to the server
	s.signer, err = sshutils.NewSigner(certs.Key, certs.Cert)
	require.NoError(t, err)

	s.nodeID = uuid.New()
	s.nodeClient, err = s.server.NewClient(auth.TestIdentity{
		I: auth.BuiltinRole{
			Role:     teleport.RoleNode,
			Username: s.nodeID,
		},
	})
	require.NoError(t, err)

	uaccDir := t.TempDir()
	utmpPath := path.Join(uaccDir, "utmp")
	wtmpPath := path.Join(uaccDir, "wtmp")
	err = TouchFile(utmpPath)
	require.NoError(t, err)
	err = TouchFile(wtmpPath)
	require.NoError(t, err)
	s.utmpPath = utmpPath

	nodeDir := t.TempDir()
	srv, err := regular.New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		s.server.ClusterName(),
		[]ssh.Signer{s.signer},
		s.nodeClient,
		nodeDir,
		"",
		utils.NetAddr{},
		regular.SetUUID(s.nodeID),
		regular.SetNamespace(defaults.Namespace),
		regular.SetEmitter(s.nodeClient),
		regular.SetShell("/bin/sh"),
		regular.SetSessionServer(s.nodeClient),
		regular.SetPAMConfig(&pam.Config{Enabled: false}),
		regular.SetLabels(
			map[string]string{"foo": "bar"},
			services.CommandLabels{
				"baz": &types.CommandLabelV2{
					Period:  types.NewDuration(time.Millisecond),
					Command: []string{"expr", "1", "+", "3"}},
			},
		),
		regular.SetBPF(&bpf.NOP{}),
		regular.SetClock(s.clock),
		regular.SetUtmpPath(utmpPath, utmpPath),
	)
	require.NoError(t, err)
	s.srv = srv
	require.NoError(t, auth.CreateUploaderDir(nodeDir))
	require.NoError(t, s.srv.Start())
	return s
}

func newUpack(s *SrvCtx, username string, allowedLogins []string, allowedLabels types.Labels) (*upack, error) {
	ctx := context.Background()
	auth := s.server.Auth()
	upriv, upub, err := auth.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := types.NewUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	role := services.RoleForUser(user)
	rules := role.GetRules(services.Allow)
	rules = append(rules, types.NewRule(services.Wildcard, services.RW()))
	role.SetRules(services.Allow, rules)
	opts := role.GetOptions()
	opts.PermitX11Forwarding = types.NewBool(true)
	role.SetOptions(opts)
	role.SetLogins(services.Allow, allowedLogins)
	role.SetNodeLabels(services.Allow, allowedLabels)
	err = auth.UpsertRole(ctx, role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.AddRole(role.GetName())
	err = auth.UpsertUser(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ucert, err := s.server.AuthServer.GenerateUserCert(upub, user.GetName(), 5*time.Minute, teleport.CertificateFormatStandard)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	upkey, err := ssh.ParseRawPrivateKey(upriv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	usigner, err := ssh.NewSignerFromKey(upkey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pcert, _, _, _, err := ssh.ParseAuthorizedKey(ucert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ucertSigner, err := ssh.NewCertSigner(pcert.(*ssh.Certificate), usigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &upack{
		key:        upriv,
		pkey:       upkey,
		pub:        upub,
		cert:       ucert,
		pcert:      pcert.(*ssh.Certificate),
		signer:     usigner,
		certSigner: ucertSigner,
	}, nil
}
