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

package integration

import (
	"context"
	"os"
	"os/user"
	"path"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/regular"
	"github.com/gravitational/teleport/lib/srv/uacc"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
)

// teleportFakeUser is a user that doesn't exist, used for tests.
const teleportFakeUser = "teleport-fake"

// wildcardAllow is used in tests to allow access to all labels.
var wildcardAllow = types.Labels{
	types.Wildcard: []string{types.Wildcard},
}

type SrvCtx struct {
	srv        *regular.Server
	signer     ssh.Signer
	server     *auth.TestServer
	clock      clockwork.FakeClock
	nodeClient *authclient.Client
	nodeID     string
	utmpPath   string
	wtmpPath   string
	btmpPath   string
}

// TestRootUTMPEntryExists verifies that user accounting is done on supported systems.
func TestRootUTMPEntryExists(t *testing.T) {
	if !isRoot() {
		t.Skip("This test will be skipped because tests are not being run as root.")
	}

	user, err := user.Current()
	require.NoError(t, err)
	teleportTestUser := user.Name

	ctx := context.Background()
	s := newSrvCtx(ctx, t)
	up, err := newUpack(ctx, s, teleportTestUser, []string{teleportTestUser, teleportFakeUser}, wildcardAllow)
	require.NoError(t, err)

	t.Run("successful login is logged in utmp and wtmp", func(t *testing.T) {
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

		require.EventuallyWithTf(t, func(collect *assert.CollectT) {
			require.NoError(collect, uacc.UserWithPtyInDatabase(s.utmpPath, teleportTestUser))
			require.NoError(collect, uacc.UserWithPtyInDatabase(s.wtmpPath, teleportTestUser))
			// Ensure than an entry was not written to btmp.
			require.True(collect, trace.IsNotFound(uacc.UserWithPtyInDatabase(s.btmpPath, teleportTestUser)), "unexpected error: %v", err)
		}, 5*time.Minute, time.Second, "did not detect utmp entry within 5 minutes")
	})

	t.Run("unsuccessful login is logged in btmp", func(t *testing.T) {
		sshConfig := &ssh.ClientConfig{
			User:            teleportFakeUser,
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

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			require.NoError(collect, uacc.UserWithPtyInDatabase(s.btmpPath, teleportFakeUser))
			// Ensure that entries were not written to utmp and wtmp
			require.True(collect, trace.IsNotFound(uacc.UserWithPtyInDatabase(s.utmpPath, teleportFakeUser)), "unexpected error: %v", err)
			require.True(collect, trace.IsNotFound(uacc.UserWithPtyInDatabase(s.wtmpPath, teleportFakeUser)), "unexpected error: %v", err)
		}, 5*time.Minute, time.Second, "did not detect btmp entry within 5 minutes")
	})

}

// TestUsernameLimit tests that the maximum length of usernames is a hard error.
func TestRootUsernameLimit(t *testing.T) {
	if !isRoot() {
		t.Skip("This test will be skipped because tests are not being run as root.")
	}

	dir := t.TempDir()
	utmpPath := path.Join(dir, "utmp")
	wtmpPath := path.Join(dir, "wtmp")

	err := TouchFile(utmpPath)
	require.NoError(t, err)
	err = TouchFile(wtmpPath)
	require.NoError(t, err)

	// A 33 character long username.
	username := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	host := [4]int32{0, 0, 0, 0}
	tty := os.NewFile(uintptr(0), "/proc/self/fd/0")
	err = uacc.Open(utmpPath, wtmpPath, username, "localhost", host, tty)
	require.True(t, trace.IsBadParameter(err))

	// A 32 character long username.
	username = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	err = uacc.Open(utmpPath, wtmpPath, username, "localhost", host, tty)
	require.False(t, trace.IsBadParameter(err))
}

// upack holds all ssh signing artifacts needed for signing and checking user keys
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
func newSrvCtx(ctx context.Context, t *testing.T) *SrvCtx {
	s := &SrvCtx{}

	t.Cleanup(func() {
		if s.srv != nil {
			require.NoError(t, s.srv.Close())
		}
		if s.server != nil {
			require.NoError(t, s.server.Shutdown(ctx))
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
		},
	})
	require.NoError(t, err)

	// set up host private key and certificate
	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	tlsPub, err := auth.PrivateKeyToPublicKeyTLS(priv)
	require.NoError(t, err)

	certs, err := s.server.Auth().GenerateHostCerts(ctx,
		&proto.HostCertsRequest{
			HostID:       hostID,
			NodeName:     s.server.ClusterName(),
			Role:         types.RoleNode,
			PublicSSHKey: pub,
			PublicTLSKey: tlsPub,
		})
	require.NoError(t, err)

	// set up user CA and set up a user that has access to the server
	s.signer, err = sshutils.NewSigner(priv, certs.SSH)
	require.NoError(t, err)

	s.nodeID = uuid.New().String()
	s.nodeClient, err = s.server.NewClient(auth.TestIdentity{
		I: authz.BuiltinRole{
			Role:     types.RoleNode,
			Username: s.nodeID,
		},
	})
	require.NoError(t, err)

	uaccDir := t.TempDir()
	utmpPath := path.Join(uaccDir, "utmp")
	wtmpPath := path.Join(uaccDir, "wtmp")
	btmpPath := path.Join(uaccDir, "btmp")
	require.NoError(t, TouchFile(utmpPath))
	require.NoError(t, TouchFile(wtmpPath))
	require.NoError(t, TouchFile(btmpPath))
	s.utmpPath = utmpPath
	s.wtmpPath = wtmpPath
	s.btmpPath = btmpPath

	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentNode,
			Client:    s.nodeClient,
		},
	})
	require.NoError(t, err)
	t.Cleanup(lockWatcher.Close)

	nodeSessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
		Semaphores:   s.nodeClient,
		AccessPoint:  s.nodeClient,
		LockEnforcer: lockWatcher,
		Emitter:      s.nodeClient,
		Component:    teleport.ComponentNode,
		ServerID:     s.nodeID,
	})
	require.NoError(t, err)

	nodeDir := t.TempDir()
	srv, err := regular.New(
		ctx,
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		s.server.ClusterName(),
		sshutils.StaticHostSigners(s.signer),
		s.nodeClient,
		nodeDir,
		"",
		utils.NetAddr{},
		s.nodeClient,
		regular.SetUUID(s.nodeID),
		regular.SetNamespace(apidefaults.Namespace),
		regular.SetEmitter(s.nodeClient),
		regular.SetShell("/bin/sh"),
		regular.SetPAMConfig(&servicecfg.PAMConfig{Enabled: false}),
		regular.SetLabels(
			map[string]string{"foo": "bar"},
			services.CommandLabels{
				"baz": &types.CommandLabelV2{
					Period:  types.NewDuration(time.Millisecond),
					Command: []string{"expr", "1", "+", "3"},
				},
			}, nil,
		),
		regular.SetBPF(&bpf.NOP{}),
		regular.SetClock(s.clock),
		regular.SetUserAccountingPaths(utmpPath, wtmpPath, btmpPath),
		regular.SetLockWatcher(lockWatcher),
		regular.SetSessionController(nodeSessionController),
	)
	require.NoError(t, err)
	s.srv = srv
	require.NoError(t, s.srv.Start())
	return s
}

func newUpack(ctx context.Context, s *SrvCtx, username string, allowedLogins []string, allowedLabels types.Labels) (*upack, error) {
	auth := s.server.Auth()
	upriv, upub, err := testauthority.New().GenerateKeyPair()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := types.NewUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	role := services.RoleForUser(user)
	rules := role.GetRules(types.Allow)
	rules = append(rules, types.NewRule(types.Wildcard, services.RW()))
	role.SetRules(types.Allow, rules)
	opts := role.GetOptions()
	opts.PermitX11Forwarding = types.NewBool(true)
	role.SetOptions(opts)
	role.SetLogins(types.Allow, allowedLogins)
	role.SetNodeLabels(types.Allow, allowedLabels)
	role, err = auth.UpsertRole(ctx, role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.AddRole(role.GetName())
	user, err = auth.UpsertUser(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ucert, err := s.server.AuthServer.GenerateUserCert(upub, user.GetName(), 5*time.Minute, constants.CertificateFormatStandard)
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
