/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package git

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/gitserver"
	"github.com/gravitational/teleport/api/constants"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cryptosuites"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestForwardServer(t *testing.T) {
	caSigner, err := apisshutils.MakeTestSSHCA()
	require.NoError(t, err)
	userCert := makeUserCert(t, caSigner)

	tests := []struct {
		name               string
		allowedGitHubOrg   string
		clientLogin        string
		verifyRemoteHost   ssh.HostKeyCallback
		wantNewClientError bool
		verifyWithClient   func(t *testing.T, ctx context.Context, client *tracessh.Client, m *mockGitHostingService)
		verifyEvent        func(t *testing.T, event apievents.AuditEvent)
	}{
		{
			name:               "success",
			allowedGitHubOrg:   "*",
			clientLogin:        "git",
			verifyRemoteHost:   ssh.InsecureIgnoreHostKey(),
			wantNewClientError: false,
			verifyWithClient: func(t *testing.T, ctx context.Context, client *tracessh.Client, m *mockGitHostingService) {
				session, err := client.NewSession(ctx)
				require.NoError(t, err)
				defer session.Close()

				gitCommand := "git-upload-pack 'org/my-repo.git'"
				session.Stderr = io.Discard
				session.Stdout = io.Discard
				err = session.Run(ctx, gitCommand)
				require.NoError(t, err)
				require.Equal(t, gitCommand, m.receivedExec.Command)
			},
			verifyEvent: func(t *testing.T, event apievents.AuditEvent) {
				gitEvent, ok := event.(*apievents.GitCommand)
				require.True(t, ok)
				assert.Equal(t, libevents.GitCommandEvent, gitEvent.Metadata.Type)
				assert.Equal(t, libevents.GitCommandCode, gitEvent.Metadata.Code)
				assert.Equal(t, "alice", gitEvent.User)
				assert.Equal(t, "0", gitEvent.CommandMetadata.ExitCode)
				assert.Equal(t, "git-upload-pack", gitEvent.Service)
				assert.Equal(t, "org/my-repo.git", gitEvent.Path)
			},
		},
		{
			name:               "command failure",
			allowedGitHubOrg:   "*",
			clientLogin:        "git",
			verifyRemoteHost:   ssh.InsecureIgnoreHostKey(),
			wantNewClientError: false,
			verifyWithClient: func(t *testing.T, ctx context.Context, client *tracessh.Client, m *mockGitHostingService) {
				m.exitCode = 1

				session, err := client.NewSession(ctx)
				require.NoError(t, err)
				defer session.Close()

				gitCommand := "git-receive-pack 'org/my-repo.git'"
				session.Stderr = io.Discard
				session.Stdout = io.Discard
				require.Error(t, session.Run(ctx, gitCommand))
			},
			verifyEvent: func(t *testing.T, event apievents.AuditEvent) {
				gitEvent, ok := event.(*apievents.GitCommand)
				require.True(t, ok)
				assert.Equal(t, libevents.GitCommandEvent, gitEvent.Metadata.Type)
				assert.Equal(t, libevents.GitCommandFailureCode, gitEvent.Metadata.Code)
				assert.Equal(t, "alice", gitEvent.User)
				assert.Equal(t, "1", gitEvent.CommandMetadata.ExitCode)
				assert.Equal(t, "git-receive-pack", gitEvent.Service)
				assert.Equal(t, "org/my-repo.git", gitEvent.Path)
			},
		},
		{
			name:               "failed RBAC",
			allowedGitHubOrg:   "no-org-allowed",
			clientLogin:        "git",
			verifyRemoteHost:   ssh.InsecureIgnoreHostKey(),
			wantNewClientError: true,
			verifyEvent: func(t *testing.T, event apievents.AuditEvent) {
				authFailureEvent, ok := event.(*apievents.AuthAttempt)
				require.True(t, ok)
				assert.Equal(t, libevents.AuthAttemptEvent, authFailureEvent.Metadata.Type)
				assert.Equal(t, libevents.AuthAttemptFailureCode, authFailureEvent.Metadata.Code)
				assert.Equal(t, "alice", authFailureEvent.User)
				assert.Contains(t, authFailureEvent.Error, "access denied")
			},
		},
		{
			name:               "failed client login check",
			allowedGitHubOrg:   "*",
			clientLogin:        "not-git",
			verifyRemoteHost:   ssh.InsecureIgnoreHostKey(),
			wantNewClientError: true,
		},
		{
			name:             "failed remote host check",
			allowedGitHubOrg: "*",
			clientLogin:      "git",
			verifyRemoteHost: func(string, net.Addr, ssh.PublicKey) error {
				return trace.AccessDenied("fake a remote host check error")
			},
			verifyWithClient: func(t *testing.T, ctx context.Context, client *tracessh.Client, m *mockGitHostingService) {
				// Connection is accepted but anything following fails.
				_, err := client.NewSession(ctx)
				require.Error(t, err)
			},
		},
		{
			name:             "invalid channel type",
			allowedGitHubOrg: "*",
			clientLogin:      "git",
			verifyRemoteHost: ssh.InsecureIgnoreHostKey(),
			verifyWithClient: func(t *testing.T, ctx context.Context, client *tracessh.Client, m *mockGitHostingService) {
				_, _, err := client.OpenChannel(ctx, "unknown", nil)
				require.Error(t, err)
				require.Contains(t, err.Error(), "unknown channel type")
			},
		},
		{
			name:               "org mismatch",
			allowedGitHubOrg:   "*",
			clientLogin:        "git",
			verifyRemoteHost:   ssh.InsecureIgnoreHostKey(),
			wantNewClientError: false,
			verifyWithClient: func(t *testing.T, ctx context.Context, client *tracessh.Client, m *mockGitHostingService) {
				session, err := client.NewSession(ctx)
				require.NoError(t, err)
				defer session.Close()

				gitCommand := "git-upload-pack 'some-other-org/my-repo.git'"
				session.Stderr = io.Discard
				session.Stdout = io.Discard
				require.Error(t, session.Run(ctx, gitCommand))
			},
			verifyEvent: func(t *testing.T, event apievents.AuditEvent) {
				gitEvent, ok := event.(*apievents.GitCommand)
				require.True(t, ok)
				assert.Equal(t, libevents.GitCommandEvent, gitEvent.Metadata.Type)
				assert.Equal(t, libevents.GitCommandFailureCode, gitEvent.Metadata.Code)
				assert.Equal(t, "alice", gitEvent.User)
				assert.Contains(t, gitEvent.Error, "expect organization")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()

			mockEmitter := &eventstest.MockRecorderEmitter{}
			mockGitService := newMockGitHostingService(t, caSigner)
			hostCert, err := apisshutils.MakeRealHostCert(caSigner)
			require.NoError(t, err)
			targetConn, err := net.Dial("tcp", mockGitService.Addr())
			require.NoError(t, err)

			s, err := NewForwardServer(&ForwardServerConfig{
				TargetServer: makeGitServer(t, "org"),
				TargetConn:   targetConn,
				AuthClient:   mockAuthClient{},
				AccessPoint: mockAccessPoint{
					ca:               caSigner,
					allowedGitHubOrg: test.allowedGitHubOrg,
				},
				Emitter:         mockEmitter,
				HostCertificate: hostCert,
				ParentContext:   ctx,
				LockWatcher:     makeLockWatcher(t),
				SrcAddr:         utils.MustParseAddr("127.0.0.1:12345"),
				DstAddr:         utils.MustParseAddr("127.0.0.1:2222"),
				// Not used in test, yet.
				KeyManager: new(KeyManager),
			})
			require.NoError(t, err)

			s.verifyRemoteHost = test.verifyRemoteHost
			s.makeRemoteSigner = func(context.Context, *ForwardServerConfig, srv.IdentityContext) (ssh.Signer, error) {
				// mock server does not validate this, just put whatever.
				return userCert, nil
			}
			go s.Serve()

			clientDialConn, err := s.Dial()
			require.NoError(t, err)

			conn, chCh, reqCh, err := ssh.NewClientConn(
				clientDialConn,
				"127.0.0.1:222",
				&ssh.ClientConfig{
					User: test.clientLogin,
					Auth: []ssh.AuthMethod{
						ssh.PublicKeys(userCert),
					},
					HostKeyCallback: ssh.InsecureIgnoreHostKey(),
					Timeout:         5 * time.Second,
				},
			)
			if test.wantNewClientError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			client := tracessh.NewClient(conn, chCh, reqCh)
			defer client.Close()

			test.verifyWithClient(t, ctx, client, mockGitService)
			if test.verifyEvent != nil {
				// Server emits exec event after sending result to client.
				require.EventuallyWithT(t, func(t *assert.CollectT) {
					assert.NotNil(t, mockEmitter.LastEvent())
				}, time.Second*2, time.Millisecond*100, "Timeout waiting for audit event.")
				test.verifyEvent(t, mockEmitter.LastEvent())
			}
		})
	}

}

func makeUserCert(t *testing.T, caSigner ssh.Signer) ssh.Signer {
	t.Helper()
	keygen := testauthority.New()
	clientPrivateKey, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	clientCertBytes, err := keygen.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:          caSigner,
		PublicUserKey:     clientPrivateKey.MarshalSSHPublicKey(),
		CertificateFormat: constants.CertificateFormatStandard,
		Identity: sshca.Identity{
			Username:     "alice",
			Principals:   []string{"does-not-matter"},
			GitHubUserID: "1234567",
			Traits:       wrappers.Traits{},
			Roles:        []string{"editor"},
		},
	})
	require.NoError(t, err)
	clientAuthorizedCert, _, _, _, err := ssh.ParseAuthorizedKey(clientCertBytes)
	require.NoError(t, err)
	clientSigner, err := apisshutils.SSHSigner(clientAuthorizedCert.(*ssh.Certificate), clientPrivateKey)
	require.NoError(t, err)
	return clientSigner
}

func makeLockWatcher(t *testing.T) *services.LockWatcher {
	t.Helper()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	lockWatcher, err := services.NewLockWatcher(context.Background(), services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "git.test",
			Client:    local.NewEventsService(backend),
		},
		LockGetter: local.NewAccessService(backend),
	})
	require.NoError(t, err)
	return lockWatcher
}

func makeGitServer(t *testing.T, org string) types.Server {
	t.Helper()
	server, err := types.NewGitHubServer(types.GitHubServerMetadata{
		Integration:  org,
		Organization: org,
	})
	require.NoError(t, err)
	return server
}

type mockGitHostingService struct {
	*sshutils.Server
	*sshutils.Reply
	receivedExec sshutils.ExecReq
	exitCode     int
	hostKey      ssh.PublicKey
}

func newMockGitHostingService(t *testing.T, caSigner ssh.Signer) *mockGitHostingService {
	t.Helper()
	hostCert, err := apisshutils.MakeRealHostCert(caSigner)
	require.NoError(t, err)
	m := &mockGitHostingService{
		Reply:   &sshutils.Reply{},
		hostKey: hostCert.PublicKey(),
	}
	server, err := sshutils.NewServer(
		"git.test",
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		m,
		sshutils.StaticHostSigners(hostCert),
		sshutils.AuthMethods{NoClient: true},
		sshutils.SetNewConnHandler(m),
	)
	require.NoError(t, err)
	require.NoError(t, server.Start())
	t.Cleanup(func() {
		server.Close()
	})
	m.Server = server
	return m
}
func (m *mockGitHostingService) HandleNewConn(ctx context.Context, ccx *sshutils.ConnectionContext) (context.Context, error) {
	slog.DebugContext(ctx, "mock git service receives new connection")
	return ctx, nil
}
func (m *mockGitHostingService) HandleNewChan(ctx context.Context, ccx *sshutils.ConnectionContext, nch ssh.NewChannel) {
	slog.DebugContext(ctx, "mock git service receives new chan")
	ch, in, err := nch.Accept()
	if err != nil {
		m.RejectWithAcceptError(ctx, nch, err)
		return
	}
	defer ch.Close()
	for {
		select {
		case req := <-in:
			if req == nil {
				return
			}

			if err := ssh.Unmarshal(req.Payload, &m.receivedExec); err != nil {
				m.ReplyError(ctx, req, err)
				return
			}
			if req.WantReply {
				m.ReplyRequest(ctx, req, true, nil)
			}
			slog.DebugContext(ctx, "mock git service receives new exec request", "req", m.receivedExec)
			m.SendExitStatus(ctx, ch, m.exitCode)
			return

		case <-ctx.Done():
			return
		}
	}
}

type mockAuthClient struct {
	authclient.ClientI
	events types.Events
}

func (m mockAuthClient) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	if m.events == nil {
		return nil, trace.AccessDenied("unauthorized")
	}
	return m.events.NewWatcher(ctx, watch)
}

type mockAccessPoint struct {
	srv.AccessPoint
	ca               ssh.Signer
	allowedGitHubOrg string
	services.GitServers
}

func (m mockAccessPoint) GetClusterName(ctx context.Context) (types.ClusterName, error) {
	return types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: "git.test",
		ClusterID:   "git.test",
	})
}
func (m mockAccessPoint) GetClusterNetworkingConfig(context.Context) (types.ClusterNetworkingConfig, error) {
	return types.DefaultClusterNetworkingConfig(), nil
}
func (m mockAccessPoint) GetSessionRecordingConfig(context.Context) (types.SessionRecordingConfig, error) {
	return types.DefaultSessionRecordingConfig(), nil
}
func (m mockAccessPoint) GetAuthPreference(context.Context) (types.AuthPreference, error) {
	return types.DefaultAuthPreference(), nil
}
func (m mockAccessPoint) GetRole(_ context.Context, name string) (types.Role, error) {
	return types.NewRole(name, types.RoleSpecV6{
		Allow: types.RoleConditions{
			GitHubPermissions: []types.GitHubPermission{{
				Organizations: []string{m.allowedGitHubOrg},
			}},
		},
	})
}
func (m mockAccessPoint) GetCertAuthorities(_ context.Context, caType types.CertAuthType, _ bool) ([]types.CertAuthority, error) {
	if m.ca == nil {
		return nil, trace.NotFound("no certificate authority found")
	}
	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        caType,
		ClusterName: "git.test",
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{{
				PublicKey: ssh.MarshalAuthorizedKey(m.ca.PublicKey()),
			}},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []types.CertAuthority{ca}, nil
}
func (m mockAccessPoint) GitServerReadOnlyClient() gitserver.ReadOnlyClient {
	return m.GitServers
}
