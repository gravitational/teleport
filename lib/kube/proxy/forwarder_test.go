// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/julienschmidt/httprouter"
	"k8s.io/client-go/transport"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gopkg.in/check.v1"
)

type ForwarderSuite struct{}

var _ = check.Suite(ForwarderSuite{})

func Test(t *testing.T) {
	check.TestingT(t)
}

var (
	identity = auth.WrapIdentity(tlsca.Identity{
		Username:         "remote-bob",
		Groups:           []string{"remote group a", "remote group b"},
		Usage:            []string{"usage a", "usage b"},
		Principals:       []string{"principal a", "principal b"},
		KubernetesGroups: []string{"remote k8s group a", "remote k8s group b"},
		Traits:           map[string][]string{"trait a": []string{"b", "c"}},
	})
	unmappedIdentity = auth.WrapIdentity(tlsca.Identity{
		Username:         "bob",
		Groups:           []string{"group a", "group b"},
		Usage:            []string{"usage a", "usage b"},
		Principals:       []string{"principal a", "principal b"},
		KubernetesGroups: []string{"k8s group a", "k8s group b"},
		Traits:           map[string][]string{"trait a": []string{"b", "c"}},
	})
)

func (s ForwarderSuite) TestRequestCertificate(c *check.C) {
	cl, err := newMockCSRClient()
	c.Assert(err, check.IsNil)
	f := &Forwarder{
		cfg: ForwarderConfig{
			Keygen:     testauthority.New(),
			AuthClient: cl,
		},
		log: logrus.New(),
	}
	user, err := types.NewUser("bob")
	c.Assert(err, check.IsNil)
	ctx := authContext{
		teleportCluster: teleportClusterClient{
			name: "site a",
		},
		Context: auth.Context{
			User:             user,
			Identity:         identity,
			UnmappedIdentity: unmappedIdentity,
		},
	}

	b, err := f.requestCertificate(ctx)
	c.Assert(err, check.IsNil)
	// All fields except b.key are predictable.
	c.Assert(b.Certificates[0].Certificate[0], check.DeepEquals, cl.lastCert.Raw)
	c.Assert(len(b.RootCAs.Subjects()), check.Equals, 1)

	// Check the KubeCSR fields.
	c.Assert(cl.gotCSR.Username, check.DeepEquals, ctx.User.GetName())
	c.Assert(cl.gotCSR.ClusterName, check.DeepEquals, ctx.teleportCluster.name)

	// Parse x509 CSR and check the subject.
	csrBlock, _ := pem.Decode(cl.gotCSR.CSR)
	c.Assert(csrBlock, check.NotNil)
	csr, err := x509.ParseCertificateRequest(csrBlock.Bytes)
	c.Assert(err, check.IsNil)
	idFromCSR, err := tlsca.FromSubject(csr.Subject, time.Time{})
	c.Assert(err, check.IsNil)
	c.Assert(*idFromCSR, check.DeepEquals, ctx.UnmappedIdentity.GetIdentity())
}

func TestAuthenticate(t *testing.T) {
	t.Parallel()

	nc, err := types.NewClusterNetworkingConfigFromConfigFile(types.ClusterNetworkingConfigSpecV2{
		ClientIdleTimeout: types.NewDuration(time.Hour),
	})
	require.NoError(t, err)
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		DisconnectExpiredCert: types.NewBoolOption(true),
	})
	require.NoError(t, err)
	ap := &mockAccessPoint{
		netConfig:       nc,
		recordingConfig: types.DefaultSessionRecordingConfig(),
		authPref:        authPref,
	}

	user, err := types.NewUser("user-a")
	require.NoError(t, err)

	tun := mockRevTunnel{
		sites: map[string]reversetunnel.RemoteSite{
			"remote": mockRemoteSite{name: "remote"},
			"local":  mockRemoteSite{name: "local"},
		},
	}

	f := &Forwarder{
		log: logrus.New(),
		cfg: ForwarderConfig{
			ClusterName:       "local",
			CachingAuthClient: ap,
		},
	}

	const remoteAddr = "user.example.com"

	tests := []struct {
		desc              string
		user              auth.IdentityGetter
		authzErr          bool
		roleKubeUsers     []string
		roleKubeGroups    []string
		routeToCluster    string
		kubernetesCluster string
		haveKubeCreds     bool
		tunnel            reversetunnel.Server
		kubeServices      []types.Server

		wantCtx     *authContext
		wantErr     bool
		wantAuthErr bool
	}{
		{
			desc:           "local user and cluster",
			user:           auth.LocalUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "local",
			haveKubeCreds:  true,
			tunnel:         tun,

			wantCtx: &authContext{
				kubeUsers:   utils.StringsSet([]string{"user-a"}),
				kubeGroups:  utils.StringsSet([]string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated}),
				kubeCluster: "local",
				teleportCluster: teleportClusterClient{
					name:       "local",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
				},
			},
		},
		{
			desc:           "local user and cluster, no kubeconfig",
			user:           auth.LocalUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "local",
			haveKubeCreds:  false,
			tunnel:         tun,

			wantCtx: &authContext{
				kubeUsers:   utils.StringsSet([]string{"user-a"}),
				kubeGroups:  utils.StringsSet([]string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated}),
				kubeCluster: "local",
				teleportCluster: teleportClusterClient{
					name:       "local",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
				},
			},
		},
		{
			desc:           "remote user and local cluster",
			user:           auth.RemoteUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "local",
			haveKubeCreds:  true,
			tunnel:         tun,

			wantCtx: &authContext{
				kubeUsers:   utils.StringsSet([]string{"user-a"}),
				kubeGroups:  utils.StringsSet([]string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated}),
				kubeCluster: "local",
				teleportCluster: teleportClusterClient{
					name:       "local",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
				},
			},
		},
		{
			desc:           "local user and remote cluster",
			user:           auth.LocalUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "remote",
			haveKubeCreds:  true,
			tunnel:         tun,

			wantCtx: &authContext{
				kubeUsers:  utils.StringsSet([]string{"user-a"}),
				kubeGroups: utils.StringsSet([]string{teleport.KubeSystemAuthenticated}),
				teleportCluster: teleportClusterClient{
					name:       "remote",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
					isRemote:   true,
				},
			},
		},
		{
			desc:           "local user and remote cluster, no kubeconfig",
			user:           auth.LocalUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "remote",
			haveKubeCreds:  false,
			tunnel:         tun,

			wantCtx: &authContext{
				kubeUsers:  utils.StringsSet([]string{"user-a"}),
				kubeGroups: utils.StringsSet([]string{teleport.KubeSystemAuthenticated}),
				teleportCluster: teleportClusterClient{
					name:       "remote",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
					isRemote:   true,
				},
			},
		},
		{
			desc:           "local user and remote cluster, no local kube users or groups",
			user:           auth.LocalUser{},
			roleKubeGroups: nil,
			routeToCluster: "remote",
			haveKubeCreds:  true,
			tunnel:         tun,

			wantCtx: &authContext{
				kubeUsers:  utils.StringsSet([]string{"user-a"}),
				kubeGroups: utils.StringsSet([]string{teleport.KubeSystemAuthenticated}),
				teleportCluster: teleportClusterClient{
					name:       "remote",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
					isRemote:   true,
				},
			},
		},
		{
			desc:           "remote user and remote cluster",
			user:           auth.RemoteUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "remote",
			haveKubeCreds:  true,
			tunnel:         tun,

			wantErr:     true,
			wantAuthErr: true,
		},
		{
			desc:           "kube users passed in request",
			user:           auth.LocalUser{},
			roleKubeUsers:  []string{"kube-user-a", "kube-user-b"},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "local",
			haveKubeCreds:  true,
			tunnel:         tun,

			wantCtx: &authContext{
				kubeUsers:   utils.StringsSet([]string{"kube-user-a", "kube-user-b"}),
				kubeGroups:  utils.StringsSet([]string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated}),
				kubeCluster: "local",
				teleportCluster: teleportClusterClient{
					name:       "local",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
				},
			},
		},
		{
			desc:     "authorization failure",
			user:     auth.LocalUser{},
			authzErr: true,
			tunnel:   tun,

			wantErr:     true,
			wantAuthErr: true,
		},
		{
			desc:   "unsupported user type",
			user:   auth.BuiltinRole{},
			tunnel: tun,

			wantErr:     true,
			wantAuthErr: true,
		},
		{
			desc:           "local user and cluster, no tunnel",
			user:           auth.LocalUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "local",
			haveKubeCreds:  true,

			wantCtx: &authContext{
				kubeUsers:   utils.StringsSet([]string{"user-a"}),
				kubeGroups:  utils.StringsSet([]string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated}),
				kubeCluster: "local",
				teleportCluster: teleportClusterClient{
					name:       "local",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
				},
			},
		},
		{
			desc:           "local user and remote cluster, no tunnel",
			user:           auth.LocalUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "remote",
			haveKubeCreds:  true,

			wantErr: true,
		},
		{
			desc:              "unknown kubernetes cluster in local cluster",
			user:              auth.LocalUser{},
			roleKubeGroups:    []string{"kube-group-a", "kube-group-b"},
			routeToCluster:    "local",
			kubernetesCluster: "foo",
			haveKubeCreds:     true,
			tunnel:            tun,

			wantErr: true,
		},
		{
			desc:              "custom kubernetes cluster in local cluster",
			user:              auth.LocalUser{},
			roleKubeGroups:    []string{"kube-group-a", "kube-group-b"},
			routeToCluster:    "local",
			kubernetesCluster: "foo",
			haveKubeCreds:     true,
			tunnel:            tun,
			kubeServices: []types.Server{&types.ServerV2{
				Spec: types.ServerSpecV2{
					KubernetesClusters: []*types.KubernetesCluster{{
						Name: "foo",
					}},
				},
			}},

			wantCtx: &authContext{
				kubeUsers:   utils.StringsSet([]string{"user-a"}),
				kubeGroups:  utils.StringsSet([]string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated}),
				kubeCluster: "foo",
				teleportCluster: teleportClusterClient{
					name:       "local",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
				},
			},
		},
		{
			desc:              "custom kubernetes cluster in remote cluster",
			user:              auth.LocalUser{},
			roleKubeGroups:    []string{"kube-group-a", "kube-group-b"},
			routeToCluster:    "remote",
			kubernetesCluster: "foo",
			haveKubeCreds:     true,
			tunnel:            tun,

			wantCtx: &authContext{
				kubeUsers:   utils.StringsSet([]string{"user-a"}),
				kubeGroups:  utils.StringsSet([]string{teleport.KubeSystemAuthenticated}),
				kubeCluster: "foo",
				teleportCluster: teleportClusterClient{
					name:       "remote",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
					isRemote:   true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			f.cfg.ReverseTunnelSrv = tt.tunnel
			ap.kubeServices = tt.kubeServices
			roles, err := services.FromSpec("ops", types.RoleSpecV5{
				Allow: types.RoleConditions{
					KubeUsers:  tt.roleKubeUsers,
					KubeGroups: tt.roleKubeGroups,
				},
			})
			require.NoError(t, err)
			authCtx := auth.Context{
				User:    user,
				Checker: roles,
				Identity: auth.WrapIdentity(tlsca.Identity{
					RouteToCluster:    tt.routeToCluster,
					KubernetesCluster: tt.kubernetesCluster,
				}),
			}
			authz := mockAuthorizer{ctx: &authCtx}
			if tt.authzErr {
				authz.err = trace.AccessDenied("denied!")
			}
			f.cfg.Authz = authz

			req := &http.Request{
				Host:       "example.com",
				RemoteAddr: remoteAddr,
				TLS: &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{
						{NotAfter: time.Now().Add(time.Hour)},
					},
				},
			}
			ctx := context.WithValue(context.Background(), auth.ContextUser, tt.user)
			req = req.WithContext(ctx)

			if tt.haveKubeCreds {
				f.creds = map[string]*kubeCreds{tt.routeToCluster: {targetAddr: "k8s.example.com"}}
			} else {
				f.creds = nil
			}

			gotCtx, err := f.authenticate(req)
			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, trace.IsAccessDenied(err), tt.wantAuthErr)
				return
			}
			require.NoError(t, err)

			require.Empty(t, cmp.Diff(gotCtx, tt.wantCtx,
				cmp.AllowUnexported(authContext{}, teleportClusterClient{}),
				cmpopts.IgnoreFields(authContext{}, "clientIdleTimeout", "sessionTTL", "Context", "recordingConfig", "disconnectExpiredCert"),
				cmpopts.IgnoreFields(teleportClusterClient{}, "dial", "isRemoteClosed"),
			))
		})
	}
}

func (s ForwarderSuite) TestSetupImpersonationHeaders(c *check.C) {
	tests := []struct {
		desc          string
		kubeUsers     []string
		kubeGroups    []string
		remoteCluster bool
		inHeaders     http.Header
		wantHeaders   http.Header
		wantErr       bool
	}{
		{
			desc:       "no existing impersonation headers",
			kubeUsers:  []string{"kube-user-a"},
			kubeGroups: []string{"kube-group-a", "kube-group-b"},
			inHeaders: http.Header{
				"Host": []string{"example.com"},
			},
			wantHeaders: http.Header{
				"Host":                 []string{"example.com"},
				ImpersonateUserHeader:  []string{"kube-user-a"},
				ImpersonateGroupHeader: []string{"kube-group-a", "kube-group-b"},
			},
		},
		{
			desc:       "no existing impersonation headers, no default kube users",
			kubeGroups: []string{"kube-group-a", "kube-group-b"},
			inHeaders:  http.Header{},
			wantErr:    true,
		},
		{
			desc:       "no existing impersonation headers, multiple default kube users",
			kubeUsers:  []string{"kube-user-a", "kube-user-b"},
			kubeGroups: []string{"kube-group-a", "kube-group-b"},
			inHeaders:  http.Header{},
			wantErr:    true,
		},
		{
			desc:          "no existing impersonation headers, remote cluster",
			kubeUsers:     []string{"kube-user-a"},
			kubeGroups:    []string{"kube-group-a", "kube-group-b"},
			remoteCluster: true,
			inHeaders:     http.Header{},
			wantHeaders:   http.Header{},
		},
		{
			desc:       "existing user and group headers",
			kubeUsers:  []string{"kube-user-a"},
			kubeGroups: []string{"kube-group-a", "kube-group-b"},
			inHeaders: http.Header{
				ImpersonateUserHeader:  []string{"kube-user-a"},
				ImpersonateGroupHeader: []string{"kube-group-b"},
			},
			wantHeaders: http.Header{
				ImpersonateUserHeader:  []string{"kube-user-a"},
				ImpersonateGroupHeader: []string{"kube-group-b"},
			},
		},
		{
			desc:       "existing user headers not allowed",
			kubeUsers:  []string{"kube-user-a"},
			kubeGroups: []string{"kube-group-a", "kube-group-b"},
			inHeaders: http.Header{
				ImpersonateUserHeader:  []string{"kube-user-other"},
				ImpersonateGroupHeader: []string{"kube-group-b"},
			},
			wantErr: true,
		},
		{
			desc:       "existing group headers not allowed",
			kubeUsers:  []string{"kube-user-a"},
			kubeGroups: []string{"kube-group-a", "kube-group-b"},
			inHeaders: http.Header{
				ImpersonateGroupHeader: []string{"kube-group-other"},
			},
			wantErr: true,
		},
		{
			desc:       "multiple existing user headers",
			kubeUsers:  []string{"kube-user-a", "kube-user-b"},
			kubeGroups: []string{"kube-group-a", "kube-group-b"},
			inHeaders: http.Header{
				ImpersonateUserHeader: []string{"kube-user-a", "kube-user-b"},
			},
			wantErr: true,
		},
		{
			desc:       "unrecognized impersonation header",
			kubeUsers:  []string{"kube-user-a", "kube-user-b"},
			kubeGroups: []string{"kube-group-a", "kube-group-b"},
			inHeaders: http.Header{
				"Impersonate-ev": []string{"evil-ev"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		c.Log(tt.desc)

		err := setupImpersonationHeaders(
			logrus.NewEntry(logrus.New()),
			authContext{
				kubeUsers:       utils.StringsSet(tt.kubeUsers),
				kubeGroups:      utils.StringsSet(tt.kubeGroups),
				teleportCluster: teleportClusterClient{isRemote: tt.remoteCluster},
			},
			tt.inHeaders,
		)
		c.Log("got error:", err)
		c.Assert(err != nil, check.Equals, tt.wantErr)
		if err == nil {
			// Sort header values to get predictable ordering.
			for _, vals := range tt.inHeaders {
				sort.Strings(vals)
			}
			c.Assert(tt.inHeaders, check.DeepEquals, tt.wantHeaders)
		}
	}
}

func mockAuthCtx(ctx context.Context, t *testing.T, kubeCluster string, isRemote bool) authContext {
	t.Helper()
	user, err := types.NewUser("bob")
	require.NoError(t, err)

	return authContext{
		Context: auth.Context{
			User:             user,
			Identity:         identity,
			UnmappedIdentity: unmappedIdentity,
		},
		teleportCluster: teleportClusterClient{
			name:     "kube-cluster",
			isRemote: isRemote,
		},
		kubeCluster: "kube-cluster",
		sessionTTL:  time.Minute,
	}
}

func TestNewClusterSessionLocal(t *testing.T) {
	ctx := context.Background()
	f := newMockForwader(ctx, t)
	authCtx := mockAuthCtx(ctx, t, "kube-cluster", false)

	// Set creds for kube cluster local
	f.creds = map[string]*kubeCreds{
		"local": {
			targetAddr: "k8s.example.com:443",
			tlsConfig: &tls.Config{
				Certificates: []tls.Certificate{
					{
						Certificate: [][]byte{[]byte("cert")},
					},
				},
			},
			transportConfig: &transport.Config{},
		},
	}

	// Fail when kubeCluster is not specified
	authCtx.kubeCluster = ""
	_, err := f.newClusterSession(authCtx)
	require.Error(t, err)
	require.Equal(t, trace.IsNotFound(err), true)
	require.Empty(t, 0, f.clientCredentials.Len())

	// Fail when creds aren't available
	authCtx.kubeCluster = "other"
	_, err = f.newClusterSession(authCtx)
	require.Error(t, err)
	require.Equal(t, trace.IsNotFound(err), true)
	require.Empty(t, 0, f.clientCredentials.Len())

	// Succeed when creds are available
	authCtx.kubeCluster = "local"
	sess, err := f.newClusterSession(authCtx)
	require.NoError(t, err)
	require.NotNil(t, sess.forwarder)
	require.Equal(t, []kubeClusterEndpoint{{addr: f.creds["local"].targetAddr}}, sess.kubeClusterEndpoints)

	// Make sure newClusterSession used provided creds
	// instead of requesting a Teleport client cert.
	require.Equal(t, f.creds["local"].tlsConfig, sess.tlsConfig)
	require.Nil(t, f.cfg.AuthClient.(*mockCSRClient).lastCert)
	require.Empty(t, 0, f.clientCredentials.Len())
}

func TestNewClusterSessionRemote(t *testing.T) {
	ctx := context.Background()
	f := newMockForwader(ctx, t)
	authCtx := mockAuthCtx(ctx, t, "kube-cluster", true)

	// Succeed on remote cluster session
	sess, err := f.newClusterSession(authCtx)
	require.NoError(t, err)
	require.NotNil(t, sess.forwarder)
	require.Equal(t, []kubeClusterEndpoint{{addr: reversetunnel.LocalKubernetes}}, sess.kubeClusterEndpoints)

	// Make sure newClusterSession obtained a new client cert instead of using f.creds.
	require.Equal(t, f.cfg.AuthClient.(*mockCSRClient).lastCert.Raw, sess.tlsConfig.Certificates[0].Certificate[0])
	require.Equal(t, [][]byte{f.cfg.AuthClient.(*mockCSRClient).ca.Cert.RawSubject}, sess.tlsConfig.RootCAs.Subjects())
	require.Equal(t, 1, f.clientCredentials.Len())
}

func TestNewClusterSessionDirect(t *testing.T) {
	ctx := context.Background()
	f := newMockForwader(ctx, t)
	authCtx := mockAuthCtx(ctx, t, "kube-cluster", false)

	// helper function to create kube services
	newKubeService := func(name, addr, kubeCluster string) (types.Server, kubeClusterEndpoint) {
		kubeService, err := types.NewServer(name, types.KindKubeService,
			types.ServerSpecV2{
				Addr: addr,
				KubernetesClusters: []*types.KubernetesCluster{{
					Name: kubeCluster,
				}},
			},
		)
		require.NoError(t, err)
		kubeServiceEndpoint := kubeClusterEndpoint{
			addr:     addr,
			serverID: fmt.Sprintf("%s.%s", name, authCtx.teleportCluster.name),
		}
		return kubeService, kubeServiceEndpoint
	}

	// no kube services for kube cluster
	otherKubeService, _ := newKubeService("other", "other.example.com", "other-kube-cluster")
	f.cfg.CachingAuthClient = mockAccessPoint{
		kubeServices: []types.Server{otherKubeService, otherKubeService, otherKubeService},
	}
	_, err := f.newClusterSession(authCtx)
	require.Error(t, err)

	// multiple kube services for kube cluster
	publicKubeService, publicEndpoint := newKubeService("public", "k8s.example.com", "kube-cluster")
	tunnelKubeService, tunnelEndpoint := newKubeService("tunnel", reversetunnel.LocalKubernetes, "kube-cluster")
	f.cfg.CachingAuthClient = mockAccessPoint{
		kubeServices: []types.Server{publicKubeService, otherKubeService, tunnelKubeService, otherKubeService},
	}
	sess, err := f.newClusterSession(authCtx)
	require.NoError(t, err)
	require.NotNil(t, sess.forwarder)
	require.Equal(t, []kubeClusterEndpoint{publicEndpoint, tunnelEndpoint}, sess.kubeClusterEndpoints)

	// Make sure newClusterSession obtained a new client cert instead of using f.creds.
	require.Equal(t, f.cfg.AuthClient.(*mockCSRClient).lastCert.Raw, sess.tlsConfig.Certificates[0].Certificate[0])
	require.Equal(t, [][]byte{f.cfg.AuthClient.(*mockCSRClient).ca.Cert.RawSubject}, sess.tlsConfig.RootCAs.Subjects())
	require.Equal(t, 1, f.clientCredentials.Len())
}

func TestClusterSessionDial(t *testing.T) {
	ctx := context.Background()
	sess := &clusterSession{
		authContext: authContext{
			teleportCluster: teleportClusterClient{
				dial: func(_ context.Context, _, addr, _ string) (net.Conn, error) {
					if addr == "" {
						return nil, trace.BadParameter("no addr")
					}
					return &net.TCPConn{}, nil
				},
			},
		},
	}

	// fail with no endpoints
	_, err := sess.dial(ctx, "")
	require.True(t, trace.IsBadParameter(err))

	// succeed with one endpoint
	sess.kubeClusterEndpoints = []kubeClusterEndpoint{{
		addr:     "addr1",
		serverID: "server1",
	}}
	_, err = sess.dial(ctx, "")
	require.NoError(t, err)
	require.Equal(t, sess.kubeAddress, "addr1")

	// fail if no endpoints are reachable
	sess.kubeClusterEndpoints = make([]kubeClusterEndpoint, 10)
	_, err = sess.dial(ctx, "")
	require.Error(t, err)

	// succeed if at least one endpoint is reachable
	sess.kubeClusterEndpoints[5] = kubeClusterEndpoint{addr: "addr1"}
	_, err = sess.dial(ctx, "")
	require.NoError(t, err)
	require.Equal(t, "addr1", sess.kubeAddress)
}

func newMockForwader(ctx context.Context, t *testing.T) *Forwarder {
	clientCreds, err := ttlmap.New(defaults.ClientCacheSize)
	require.NoError(t, err)

	csrClient, err := newMockCSRClient()
	require.NoError(t, err)

	return &Forwarder{
		log:    logrus.New(),
		router: *httprouter.New(),
		cfg: ForwarderConfig{
			Keygen:            testauthority.New(),
			AuthClient:        csrClient,
			CachingAuthClient: mockAccessPoint{},
			Clock:             clockwork.NewFakeClock(),
			Context:           ctx,
		},
		clientCredentials: clientCreds,
		activeRequests:    make(map[string]context.Context),
		ctx:               ctx,
	}
}

// mockCSRClient to intercept ProcessKubeCSR requests, record them and return a
// stub response.
type mockCSRClient struct {
	auth.ClientI

	ca       *tlsca.CertAuthority
	gotCSR   auth.KubeCSR
	lastCert *x509.Certificate
}

func newMockCSRClient() (*mockCSRClient, error) {
	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	if err != nil {
		return nil, err
	}
	return &mockCSRClient{ca: ca}, nil
}

func (c *mockCSRClient) ProcessKubeCSR(csr auth.KubeCSR) (*auth.KubeCSRResponse, error) {
	c.gotCSR = csr

	x509CSR, err := tlsca.ParseCertificateRequestPEM(csr.CSR)
	if err != nil {
		return nil, err
	}
	caCSR := tlsca.CertificateRequest{
		Clock:     clockwork.NewFakeClock(),
		PublicKey: x509CSR.PublicKey.(crypto.PublicKey),
		Subject:   x509CSR.Subject,
		NotAfter:  time.Now().Add(time.Minute),
		DNSNames:  x509CSR.DNSNames,
	}
	cert, err := c.ca.GenerateCertificate(caCSR)
	if err != nil {
		return nil, err
	}
	c.lastCert, err = tlsca.ParseCertificatePEM(cert)
	if err != nil {
		return nil, err
	}
	return &auth.KubeCSRResponse{
		Cert:            cert,
		CertAuthorities: [][]byte{[]byte(fixtures.TLSCACertPEM)},
		TargetAddr:      "mock addr",
	}, nil
}

// mockRemoteSite is a reversetunnel.RemoteSite implementation with hardcoded
// name, because there's no easy way to construct a real
// reversetunnel.RemoteSite.
type mockRemoteSite struct {
	reversetunnel.RemoteSite
	name string
}

func (s mockRemoteSite) GetName() string { return s.name }

type mockAccessPoint struct {
	auth.KubernetesAccessPoint

	netConfig       types.ClusterNetworkingConfig
	recordingConfig types.SessionRecordingConfig
	authPref        types.AuthPreference
	kubeServices    []types.Server
	cas             map[string]types.CertAuthority
}

func (ap mockAccessPoint) GetClusterNetworkingConfig(context.Context, ...services.MarshalOption) (types.ClusterNetworkingConfig, error) {
	return ap.netConfig, nil
}

func (ap mockAccessPoint) GetSessionRecordingConfig(context.Context, ...services.MarshalOption) (types.SessionRecordingConfig, error) {
	return ap.recordingConfig, nil
}

func (ap mockAccessPoint) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	return ap.authPref, nil
}

func (ap mockAccessPoint) GetKubeServices(ctx context.Context) ([]types.Server, error) {
	return ap.kubeServices, nil
}

func (ap mockAccessPoint) GetCertAuthorities(caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error) {
	var cas []types.CertAuthority
	for _, ca := range ap.cas {
		cas = append(cas, ca)
	}
	return cas, nil
}

func (ap mockAccessPoint) GetCertAuthority(id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error) {
	return ap.cas[id.DomainName], nil
}

type mockRevTunnel struct {
	reversetunnel.Server

	sites map[string]reversetunnel.RemoteSite
}

func (t mockRevTunnel) GetSite(name string) (reversetunnel.RemoteSite, error) {
	s, ok := t.sites[name]
	if !ok {
		return nil, trace.NotFound("remote site %q not found", name)
	}
	return s, nil
}

func (t mockRevTunnel) GetSites() ([]reversetunnel.RemoteSite, error) {
	var sites []reversetunnel.RemoteSite
	for _, s := range t.sites {
		sites = append(sites, s)
	}
	return sites, nil
}

type mockAuthorizer struct {
	ctx *auth.Context
	err error
}

func (a mockAuthorizer) Authorize(context.Context) (*auth.Context, error) {
	return a.ctx, a.err
}
