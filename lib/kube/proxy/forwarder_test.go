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

package proxy

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/transport"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

var (
	identity = authz.WrapIdentity(tlsca.Identity{
		Username:         "remote-bob",
		Groups:           []string{"remote group a", "remote group b"},
		Usage:            []string{"usage a", "usage b"},
		Principals:       []string{"principal a", "principal b"},
		KubernetesGroups: []string{"remote k8s group a", "remote k8s group b"},
		Traits:           map[string][]string{"trait a": {"b", "c"}},
	})
	unmappedIdentity = authz.WrapIdentity(tlsca.Identity{
		Username:         "bob",
		Groups:           []string{"group a", "group b"},
		Usage:            []string{"usage a", "usage b"},
		Principals:       []string{"principal a", "principal b"},
		KubernetesGroups: []string{"k8s group a", "k8s group b"},
		Traits:           map[string][]string{"trait a": {"b", "c"}},
	})
)

func TestRequestCertificate(t *testing.T) {
	cl, err := newMockCSRClient()
	require.NoError(t, err)
	f := &Forwarder{
		cfg: ForwarderConfig{
			Keygen:     testauthority.New(),
			AuthClient: cl,
		},
		log: logrus.NewEntry(logrus.New()),
	}
	user, err := types.NewUser("bob")
	require.NoError(t, err)
	ctx := authContext{
		teleportCluster: teleportClusterClient{
			name: "site a",
		},
		Context: authz.Context{
			User:             user,
			Identity:         identity,
			UnmappedIdentity: unmappedIdentity,
		},
	}

	b, err := f.requestCertificate(ctx)
	require.NoError(t, err)
	// All fields except b.key are predictable.
	require.Empty(t, cmp.Diff(b.Certificates[0].Certificate[0], cl.lastCert.Raw))

	// Check the KubeCSR fields.
	require.Empty(t, cmp.Diff(cl.gotCSR.Username, ctx.User.GetName()))
	require.Empty(t, cmp.Diff(cl.gotCSR.ClusterName, ctx.teleportCluster.name))

	// Parse x509 CSR and check the subject.
	csrBlock, _ := pem.Decode(cl.gotCSR.CSR)
	require.NotNil(t, csrBlock)
	csr, err := x509.ParseCertificateRequest(csrBlock.Bytes)
	require.NoError(t, err)
	idFromCSR, err := tlsca.FromSubject(csr.Subject, time.Time{})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(*idFromCSR, ctx.UnmappedIdentity.GetIdentity()))
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

	const (
		username = "user-a"
	)
	certExpiration := time.Now().Add(time.Hour)

	user, err := types.NewUser(username)
	require.NoError(t, err)

	tun := mockRevTunnel{
		sites: map[string]reversetunnel.RemoteSite{
			"remote": mockRemoteSite{name: "remote"},
			"local":  mockRemoteSite{name: "local"},
		},
	}
	f := &Forwarder{
		log: logrus.NewEntry(logrus.New()),
		cfg: ForwarderConfig{
			ClusterName:       "local",
			CachingAuthClient: ap,
		},
		getKubernetesServersForKubeCluster: func(ctx context.Context, name string) ([]types.KubeServer, error) {
			servers, err := ap.GetKubernetesServers(ctx)
			if err != nil {
				return nil, err
			}
			var filtered []types.KubeServer
			for _, server := range servers {
				if server.GetCluster().GetName() == name {
					filtered = append(filtered, server)
				}
			}
			return filtered, nil
		},
	}

	const remoteAddr = "user.example.com"
	activeAccessRequests := []string{uuid.NewString(), uuid.NewString()}
	tests := []struct {
		desc              string
		user              authz.IdentityGetter
		authzErr          bool
		roleKubeUsers     []string
		roleKubeGroups    []string
		routeToCluster    string
		kubernetesCluster string
		haveKubeCreds     bool
		tunnel            reversetunnel.Server
		kubeServers       []types.KubeServer
		activeRequests    []string

		wantCtx     *authContext
		wantErr     bool
		wantAuthErr bool
	}{
		{
			desc:           "local user and cluster with active access request",
			user:           authz.LocalUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "local",
			haveKubeCreds:  true,
			tunnel:         tun,
			kubeServers: newKubeServersFromKubeClusters(
				t,
				&types.KubernetesClusterV3{
					Metadata: types.Metadata{
						Name: "local",
						Labels: map[string]string{
							"static_label1": "static_value1",
							"static_label2": "static_value2",
						},
					},
					Spec: types.KubernetesClusterSpecV3{
						DynamicLabels: map[string]types.CommandLabelV2{},
					},
				},
			),
			activeRequests: activeAccessRequests,
			wantCtx: &authContext{
				kubeUsers:       utils.StringsSet([]string{"user-a"}),
				kubeGroups:      utils.StringsSet([]string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated}),
				kubeClusterName: "local",
				kubeClusterLabels: map[string]string{
					"static_label1": "static_value1",
					"static_label2": "static_value2",
				},
				certExpires: certExpiration,
				teleportCluster: teleportClusterClient{
					name:       "local",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
				},
				kubeServers: newKubeServersFromKubeClusters(
					t,
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name: "local",
							Labels: map[string]string{
								"static_label1": "static_value1",
								"static_label2": "static_value2",
							},
						},
						Spec: types.KubernetesClusterSpecV3{
							DynamicLabels: map[string]types.CommandLabelV2{},
						},
					},
				),
			},
		},
		{
			desc:           "local user and cluster",
			user:           authz.LocalUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "local",
			haveKubeCreds:  true,
			tunnel:         tun,
			kubeServers: newKubeServersFromKubeClusters(
				t,
				&types.KubernetesClusterV3{
					Metadata: types.Metadata{
						Name: "local",
						Labels: map[string]string{
							"static_label1": "static_value1",
							"static_label2": "static_value2",
						},
					},
					Spec: types.KubernetesClusterSpecV3{
						DynamicLabels: map[string]types.CommandLabelV2{},
					},
				},
				&types.KubernetesClusterV3{
					Metadata: types.Metadata{
						Name: "foo",
						Labels: map[string]string{
							"static_label1": "static_value1",
							"static_label2": "static_value2",
						},
					},
					Spec: types.KubernetesClusterSpecV3{
						DynamicLabels: map[string]types.CommandLabelV2{},
					},
				},
				&types.KubernetesClusterV3{
					Metadata: types.Metadata{
						Name: "bar",
						Labels: map[string]string{
							"static_label1": "static_value1",
							"static_label2": "static_value2",
						},
					},
					Spec: types.KubernetesClusterSpecV3{
						DynamicLabels: map[string]types.CommandLabelV2{},
					},
				},
			),
			wantCtx: &authContext{
				kubeUsers:       utils.StringsSet([]string{"user-a"}),
				kubeGroups:      utils.StringsSet([]string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated}),
				kubeClusterName: "local",
				kubeClusterLabels: map[string]string{
					"static_label1": "static_value1",
					"static_label2": "static_value2",
				},
				certExpires: certExpiration,
				teleportCluster: teleportClusterClient{
					name:       "local",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
				},
				kubeServers: newKubeServersFromKubeClusters(
					t,
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name: "local",
							Labels: map[string]string{
								"static_label1": "static_value1",
								"static_label2": "static_value2",
							},
						},
						Spec: types.KubernetesClusterSpecV3{
							DynamicLabels: map[string]types.CommandLabelV2{},
						},
					},
				),
			},
		},
		{
			desc:           "local user and cluster, no kubeconfig",
			user:           authz.LocalUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "local",
			haveKubeCreds:  false,
			tunnel:         tun,
			kubeServers: newKubeServersFromKubeClusters(
				t,
				&types.KubernetesClusterV3{
					Metadata: types.Metadata{
						Name:   "local",
						Labels: map[string]string{},
					},
					Spec: types.KubernetesClusterSpecV3{
						DynamicLabels: map[string]types.CommandLabelV2{},
					},
				},
			),

			wantCtx: &authContext{
				kubeUsers:         utils.StringsSet([]string{"user-a"}),
				kubeGroups:        utils.StringsSet([]string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated}),
				kubeClusterName:   "local",
				kubeClusterLabels: make(map[string]string),
				certExpires:       certExpiration,
				teleportCluster: teleportClusterClient{
					name:       "local",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
				},
				kubeServers: newKubeServersFromKubeClusters(
					t,
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name:   "local",
							Labels: map[string]string{},
						},
						Spec: types.KubernetesClusterSpecV3{
							DynamicLabels: map[string]types.CommandLabelV2{},
						},
					},
				),
			},
		},
		{
			desc:           "remote user and local cluster",
			user:           authz.RemoteUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "local",
			haveKubeCreds:  true,
			tunnel:         tun,
			kubeServers: newKubeServersFromKubeClusters(
				t,
				&types.KubernetesClusterV3{
					Metadata: types.Metadata{
						Name:   "local",
						Labels: map[string]string{},
					},
					Spec: types.KubernetesClusterSpecV3{
						DynamicLabels: map[string]types.CommandLabelV2{},
					},
				},
			),
			wantCtx: &authContext{
				kubeUsers:         utils.StringsSet([]string{"user-a"}),
				kubeGroups:        utils.StringsSet([]string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated}),
				kubeClusterName:   "local",
				certExpires:       certExpiration,
				kubeClusterLabels: make(map[string]string),
				teleportCluster: teleportClusterClient{
					name:       "local",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
				},

				kubeServers: newKubeServersFromKubeClusters(
					t,
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name:   "local",
							Labels: map[string]string{},
						},
						Spec: types.KubernetesClusterSpecV3{
							DynamicLabels: map[string]types.CommandLabelV2{},
						},
					},
				),
			},
		},
		{
			desc:           "remote user and local cluster with active request id",
			user:           authz.RemoteUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "local",
			haveKubeCreds:  true,
			tunnel:         tun,
			activeRequests: activeAccessRequests,
			kubeServers: newKubeServersFromKubeClusters(
				t,
				&types.KubernetesClusterV3{
					Metadata: types.Metadata{
						Name:   "local",
						Labels: map[string]string{},
					},
					Spec: types.KubernetesClusterSpecV3{
						DynamicLabels: map[string]types.CommandLabelV2{},
					},
				},
			),
			wantCtx: &authContext{
				kubeUsers:         utils.StringsSet([]string{"user-a"}),
				kubeGroups:        utils.StringsSet([]string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated}),
				kubeClusterName:   "local",
				certExpires:       certExpiration,
				kubeClusterLabels: make(map[string]string),
				teleportCluster: teleportClusterClient{
					name:       "local",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
				},
				kubeServers: newKubeServersFromKubeClusters(
					t,
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name:   "local",
							Labels: map[string]string{},
						},
						Spec: types.KubernetesClusterSpecV3{
							DynamicLabels: map[string]types.CommandLabelV2{},
						},
					},
				),
			},
		},
		{
			desc:           "local user and remote cluster",
			user:           authz.LocalUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "remote",
			haveKubeCreds:  true,
			tunnel:         tun,

			wantCtx: &authContext{
				certExpires: certExpiration,
				teleportCluster: teleportClusterClient{
					name:       "remote",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
					isRemote:   true,
				},
			},
		},
		{
			desc:           "local user and remote cluster, no kubeconfig",
			user:           authz.LocalUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "remote",
			haveKubeCreds:  false,
			tunnel:         tun,

			wantCtx: &authContext{
				certExpires: certExpiration,
				teleportCluster: teleportClusterClient{
					name:       "remote",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
					isRemote:   true,
				},
			},
		},
		{
			desc:           "local user and remote cluster, no local kube users or groups",
			user:           authz.LocalUser{},
			roleKubeGroups: nil,
			routeToCluster: "remote",
			haveKubeCreds:  true,
			tunnel:         tun,

			wantCtx: &authContext{
				certExpires: certExpiration,
				teleportCluster: teleportClusterClient{
					name:       "remote",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
					isRemote:   true,
				},
			},
		},
		{
			desc:           "remote user and remote cluster",
			user:           authz.RemoteUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "remote",
			haveKubeCreds:  true,
			tunnel:         tun,

			wantErr:     true,
			wantAuthErr: true,
		},
		{
			desc:           "kube users passed in request",
			user:           authz.LocalUser{},
			roleKubeUsers:  []string{"kube-user-a", "kube-user-b"},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "local",
			haveKubeCreds:  true,
			tunnel:         tun,
			kubeServers: newKubeServersFromKubeClusters(
				t,
				&types.KubernetesClusterV3{
					Metadata: types.Metadata{
						Name:   "local",
						Labels: map[string]string{},
					},
					Spec: types.KubernetesClusterSpecV3{
						DynamicLabels: map[string]types.CommandLabelV2{},
					},
				},
			),

			wantCtx: &authContext{
				kubeUsers:         utils.StringsSet([]string{"kube-user-a", "kube-user-b"}),
				kubeGroups:        utils.StringsSet([]string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated}),
				kubeClusterName:   "local",
				kubeClusterLabels: make(map[string]string),
				certExpires:       certExpiration,
				teleportCluster: teleportClusterClient{
					name:       "local",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
				},
				kubeServers: newKubeServersFromKubeClusters(
					t,
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name:   "local",
							Labels: map[string]string{},
						},
						Spec: types.KubernetesClusterSpecV3{
							DynamicLabels: map[string]types.CommandLabelV2{},
						},
					},
				),
			},
		},
		{
			desc:     "authorization failure",
			user:     authz.LocalUser{},
			authzErr: true,
			tunnel:   tun,

			wantErr:     true,
			wantAuthErr: true,
		},
		{
			desc:   "unsupported user type",
			user:   authz.BuiltinRole{},
			tunnel: tun,

			wantErr:     true,
			wantAuthErr: true,
		},
		{
			desc:           "local user and cluster, no tunnel",
			user:           authz.LocalUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "local",
			haveKubeCreds:  true,
			kubeServers: newKubeServersFromKubeClusters(
				t,
				&types.KubernetesClusterV3{
					Metadata: types.Metadata{
						Name:   "local",
						Labels: map[string]string{},
					},
					Spec: types.KubernetesClusterSpecV3{
						DynamicLabels: map[string]types.CommandLabelV2{},
					},
				},
			),

			wantCtx: &authContext{
				kubeUsers:         utils.StringsSet([]string{"user-a"}),
				kubeGroups:        utils.StringsSet([]string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated}),
				kubeClusterName:   "local",
				kubeClusterLabels: make(map[string]string),
				certExpires:       certExpiration,
				teleportCluster: teleportClusterClient{
					name:       "local",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
				},
				kubeServers: newKubeServersFromKubeClusters(
					t,
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name:   "local",
							Labels: map[string]string{},
						},
						Spec: types.KubernetesClusterSpecV3{
							DynamicLabels: map[string]types.CommandLabelV2{},
						},
					},
				),
			},
		},
		{
			desc:           "local user and remote cluster, no tunnel",
			user:           authz.LocalUser{},
			roleKubeGroups: []string{"kube-group-a", "kube-group-b"},
			routeToCluster: "remote",
			haveKubeCreds:  true,

			wantErr: true,
		},
		{
			desc:              "unknown kubernetes cluster in local cluster",
			user:              authz.LocalUser{},
			roleKubeGroups:    []string{"kube-group-a", "kube-group-b"},
			routeToCluster:    "local",
			kubernetesCluster: "foo",
			haveKubeCreds:     true,
			tunnel:            tun,

			wantErr: true,
		},
		{
			desc:              "custom kubernetes cluster in local cluster",
			user:              authz.LocalUser{},
			roleKubeGroups:    []string{"kube-group-a", "kube-group-b"},
			routeToCluster:    "local",
			kubernetesCluster: "foo",
			haveKubeCreds:     true,
			tunnel:            tun,
			kubeServers: newKubeServersFromKubeClusters(
				t,
				&types.KubernetesClusterV3{
					Metadata: types.Metadata{
						Name: "foo",
						Labels: map[string]string{
							"static_label1": "static_value1",
							"static_label2": "static_value2",
						},
					},
					Spec: types.KubernetesClusterSpecV3{
						DynamicLabels: map[string]types.CommandLabelV2{},
					},
				},
			),
			wantCtx: &authContext{
				kubeUsers:       utils.StringsSet([]string{"user-a"}),
				kubeGroups:      utils.StringsSet([]string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated}),
				kubeClusterName: "foo",
				certExpires:     certExpiration,
				kubeClusterLabels: map[string]string{
					"static_label1": "static_value1",
					"static_label2": "static_value2",
				},
				teleportCluster: teleportClusterClient{
					name:       "local",
					remoteAddr: *utils.MustParseAddr(remoteAddr),
				},
				kubeServers: newKubeServersFromKubeClusters(
					t,
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name: "foo",
							Labels: map[string]string{
								"static_label1": "static_value1",
								"static_label2": "static_value2",
							},
						},
						Spec: types.KubernetesClusterSpecV3{
							DynamicLabels: map[string]types.CommandLabelV2{},
						},
					},
				),
			},
		},
		{
			desc:              "custom kubernetes cluster in remote cluster",
			user:              authz.LocalUser{},
			roleKubeGroups:    []string{"kube-group-a", "kube-group-b"},
			routeToCluster:    "remote",
			kubernetesCluster: "foo",
			haveKubeCreds:     true,
			tunnel:            tun,

			wantCtx: &authContext{
				kubeClusterName: "foo",
				certExpires:     certExpiration,
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
			ap.kubeServers = tt.kubeServers
			roles, err := services.RoleSetFromSpec("ops", types.RoleSpecV6{
				Allow: types.RoleConditions{
					KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					KubeUsers:        tt.roleKubeUsers,
					KubeGroups:       tt.roleKubeGroups,
				},
			})
			require.NoError(t, err)
			authCtx := authz.Context{
				User: user,
				Checker: services.NewAccessCheckerWithRoleSet(&services.AccessInfo{
					Roles: roles.RoleNames(),
				}, "local", roles),
				Identity: authz.WrapIdentity(tlsca.Identity{
					RouteToCluster:    tt.routeToCluster,
					KubernetesCluster: tt.kubernetesCluster,
					ActiveRequests:    tt.activeRequests,
				}),
			}
			authorizer := mockAuthorizer{ctx: &authCtx}
			if tt.authzErr {
				authorizer.err = trace.AccessDenied("denied!")
			}
			f.cfg.Authz = authorizer

			req := &http.Request{
				Host:       "example.com",
				RemoteAddr: remoteAddr,
				TLS: &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{
						{
							Subject: pkix.Name{
								CommonName:   username,
								Organization: []string{"example"},
							},
							NotAfter: certExpiration,
						},
					},
				},
			}
			ctx := authz.ContextWithUser(context.Background(), tt.user)
			req = req.WithContext(ctx)

			if tt.haveKubeCreds {
				f.clusterDetails = map[string]*kubeDetails{tt.routeToCluster: {kubeCreds: &staticKubeCreds{targetAddr: "k8s.example.com"}}}
			} else {
				f.clusterDetails = nil
			}

			gotCtx, err := f.authenticate(req)
			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, trace.IsAccessDenied(err), tt.wantAuthErr)
				return
			}
			err = f.authorize(context.Background(), gotCtx)
			require.NoError(t, err)

			require.Empty(t, cmp.Diff(gotCtx, tt.wantCtx,
				cmp.AllowUnexported(authContext{}, teleportClusterClient{}),
				cmpopts.IgnoreFields(authContext{}, "clientIdleTimeout", "sessionTTL", "Context", "recordingConfig", "disconnectExpiredCert", "kubeCluster"),
				cmpopts.IgnoreFields(teleportClusterClient{}, "dial", "isRemoteClosed"),
			))

			// validate authCtx.key() to make sure it includes certExpires timestamp.
			// this is important to make sure user's credentials are correctly cached
			// and once user's re-login, Teleport won't reuse their previous cache entry.
			ctxKey := fmt.Sprintf("%v:%v:%v:%v:%v:%v:%v",
				tt.wantCtx.teleportCluster.name,
				username,
				tt.wantCtx.kubeUsers,
				tt.wantCtx.kubeGroups,
				tt.wantCtx.kubeClusterName,
				certExpiration.Unix(),
				tt.activeRequests,
			)
			require.Equal(t, ctxKey, gotCtx.key())
		})
	}
}

func TestSetupImpersonationHeaders(t *testing.T) {
	tests := []struct {
		desc          string
		kubeUsers     []string
		kubeGroups    []string
		remoteCluster bool
		inHeaders     http.Header
		wantHeaders   http.Header
		errAssertion  require.ErrorAssertionFunc
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
			errAssertion: require.NoError,
		},
		{
			desc:         "no existing impersonation headers, no default kube users",
			kubeGroups:   []string{"kube-group-a", "kube-group-b"},
			inHeaders:    http.Header{},
			errAssertion: require.Error,
		},
		{
			desc:         "no existing impersonation headers, multiple default kube users",
			kubeUsers:    []string{"kube-user-a", "kube-user-b"},
			kubeGroups:   []string{"kube-group-a", "kube-group-b"},
			inHeaders:    http.Header{},
			errAssertion: require.Error,
		},
		{
			desc:          "no existing impersonation headers, remote cluster",
			kubeUsers:     []string{"kube-user-a"},
			kubeGroups:    []string{"kube-group-a", "kube-group-b"},
			remoteCluster: true,
			inHeaders:     http.Header{},
			wantHeaders:   http.Header{},
			errAssertion:  require.NoError,
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
			errAssertion: require.NoError,
		},
		{
			desc:       "existing user headers not allowed",
			kubeUsers:  []string{"kube-user-a"},
			kubeGroups: []string{"kube-group-a", "kube-group-b"},
			inHeaders: http.Header{
				ImpersonateUserHeader:  []string{"kube-user-other"},
				ImpersonateGroupHeader: []string{"kube-group-b"},
			},
			errAssertion: require.Error,
		},
		{
			desc:       "existing group headers not allowed",
			kubeUsers:  []string{"kube-user-a"},
			kubeGroups: []string{"kube-group-a", "kube-group-b"},
			inHeaders: http.Header{
				ImpersonateGroupHeader: []string{"kube-group-other"},
			},
			errAssertion: require.Error,
		},
		{
			desc:       "multiple existing user headers",
			kubeUsers:  []string{"kube-user-a", "kube-user-b"},
			kubeGroups: []string{"kube-group-a", "kube-group-b"},
			inHeaders: http.Header{
				ImpersonateUserHeader: []string{"kube-user-a", "kube-user-b"},
			},
			errAssertion: require.Error,
		},
		{
			desc:       "unrecognized impersonation header",
			kubeUsers:  []string{"kube-user-a", "kube-user-b"},
			kubeGroups: []string{"kube-group-a", "kube-group-b"},
			inHeaders: http.Header{
				"Impersonate-ev": []string{"evil-ev"},
			},
			errAssertion: require.Error,
		},
		{
			desc:       "empty impersonated user header ignored",
			kubeUsers:  []string{"kube-user-a"},
			kubeGroups: []string{"kube-group-a", "kube-group-b"},
			inHeaders: http.Header{
				"Host":                 []string{"example.com"},
				ImpersonateUserHeader:  []string{""},
				ImpersonateGroupHeader: []string{"kube-group-b"},
			},
			wantHeaders: http.Header{
				"Host":                 []string{"example.com"},
				ImpersonateUserHeader:  []string{"kube-user-a"},
				ImpersonateGroupHeader: []string{"kube-group-b"},
			},
			errAssertion: require.NoError,
		},
		{
			desc:       "empty impersonated group header ignored",
			kubeUsers:  []string{"kube-user-a"},
			kubeGroups: []string{},
			inHeaders: http.Header{
				"Host": []string{"example.com"},
			},
			wantHeaders: http.Header{
				"Host":                []string{"example.com"},
				ImpersonateUserHeader: []string{"kube-user-a"},
			},
			errAssertion: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Log(tt.desc)

		err := setupImpersonationHeaders(
			logrus.NewEntry(logrus.New()),
			authContext{
				kubeUsers:       utils.StringsSet(tt.kubeUsers),
				kubeGroups:      utils.StringsSet(tt.kubeGroups),
				teleportCluster: teleportClusterClient{isRemote: tt.remoteCluster},
			},
			tt.inHeaders,
		)
		t.Log("got error:", err)
		tt.errAssertion(t, err)

		if err == nil {
			// Sort header values to get predictable ordering.
			for _, vals := range tt.inHeaders {
				sort.Strings(vals)
			}
			require.Empty(t, cmp.Diff(tt.inHeaders, tt.wantHeaders))
		}
	}
}

func mockAuthCtx(ctx context.Context, t *testing.T, kubeCluster string, isRemote bool) authContext {
	t.Helper()
	user, err := types.NewUser("bob")
	require.NoError(t, err)

	return authContext{
		Context: authz.Context{
			User:             user,
			Identity:         identity,
			UnmappedIdentity: unmappedIdentity,
		},
		teleportCluster: teleportClusterClient{
			name:     "kube-cluster",
			isRemote: isRemote,
		},
		kubeClusterName: "kube-cluster",
		sessionTTL:      time.Minute,
	}
}

func TestNewClusterSessionLocal(t *testing.T) {
	ctx := context.Background()
	f := newMockForwader(ctx, t)
	authCtx := mockAuthCtx(ctx, t, "kube-cluster", false)

	// Set creds for kube cluster local
	f.clusterDetails = map[string]*kubeDetails{
		"local": {
			kubeCreds: &staticKubeCreds{
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
		},
	}

	// Fail when kubeCluster is not specified
	authCtx.kubeClusterName = ""
	_, err := f.newClusterSession(authCtx)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))
	require.Empty(t, 0, f.clientCredentials.Len())

	// Fail when creds aren't available
	authCtx.kubeClusterName = "other"
	_, err = f.newClusterSession(authCtx)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))
	require.Empty(t, 0, f.clientCredentials.Len())

	// Succeed when creds are available
	authCtx.kubeClusterName = "local"
	sess, err := f.newClusterSession(authCtx)
	require.NoError(t, err)
	require.Equal(t, []kubeClusterEndpoint{{addr: f.clusterDetails["local"].getTargetAddr()}}, sess.kubeClusterEndpoints)

	// Make sure newClusterSession used provided creds
	// instead of requesting a Teleport client cert.
	// this validates the equality of the referenced values
	require.Equal(t, f.clusterDetails["local"].getTLSConfig(), sess.tlsConfig)

	// Make sure that sess.tlsConfig was cloned from the value we store in f.creds["local"].tlsConfig.
	// This is important because each connection must refer to a different memory address so it can be manipulated to enable/disable http2
	require.NotSame(t, f.clusterDetails["local"].getTLSConfig(), sess.tlsConfig)

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
	require.Equal(t, []kubeClusterEndpoint{{addr: reversetunnel.LocalKubernetes}}, sess.kubeClusterEndpoints)

	// Make sure newClusterSession obtained a new client cert instead of using f.creds.
	require.Equal(t, f.cfg.AuthClient.(*mockCSRClient).lastCert.Raw, sess.tlsConfig.Certificates[0].Certificate[0])
	// Make sure that sess.tlsConfig was cloned from the value we store in the cache.
	// This is important because each connection must refer to a different memory address so it can be manipulated to enable/disable http2
	// getClientCreds returns the cached version of the client tlsConfig.
	require.NotNil(t, f.getClientCreds(authCtx))
	require.NotSame(t, f.getClientCreds(authCtx), sess.tlsConfig)
	//nolint:staticcheck // SA1019 there's no non-deprecated public API for testing the contents of the RootCAs pool
	require.Equal(t, [][]byte{f.cfg.AuthClient.(*mockCSRClient).ca.Cert.RawSubject}, sess.tlsConfig.RootCAs.Subjects())
	require.Equal(t, 1, f.clientCredentials.Len())
}

func TestNewClusterSessionDirect(t *testing.T) {
	ctx := context.Background()
	f := newMockForwader(ctx, t)
	authCtx := mockAuthCtx(ctx, t, "kube-cluster", false)

	// helper function to create kube services
	newKubeServer := func(name, addr, kubeCluster string) (types.KubeServer, kubeClusterEndpoint) {
		cluster, err := types.NewKubernetesClusterV3(types.Metadata{
			Name: kubeCluster,
		},
			types.KubernetesClusterSpecV3{})
		require.NoError(t, err)
		kubeService, err := types.NewKubernetesServerV3(types.Metadata{
			Name: name,
		}, types.KubernetesServerSpecV3{
			Hostname: addr,
			HostID:   name,
			Cluster:  cluster,
		})
		require.NoError(t, err)
		kubeServiceEndpoint := kubeClusterEndpoint{
			addr:     addr,
			serverID: fmt.Sprintf("%s.%s", name, authCtx.teleportCluster.name),
		}
		return kubeService, kubeServiceEndpoint
	}

	// no kube services for kube cluster
	otherKubeService, _ := newKubeServer("other", "other.example.com", "other-kube-cluster")
	f.cfg.CachingAuthClient = mockAccessPoint{
		kubeServers: []types.KubeServer{otherKubeService, otherKubeService, otherKubeService},
	}
	_, err := f.newClusterSession(authCtx)
	require.Error(t, err)

	// multiple kube services for kube cluster
	publicKubeService, publicEndpoint := newKubeServer("public", "k8s.example.com", "kube-cluster")
	tunnelKubeService, tunnelEndpoint := newKubeServer("tunnel", reversetunnel.LocalKubernetes, "kube-cluster")

	f.cfg.CachingAuthClient = mockAccessPoint{
		kubeServers: []types.KubeServer{publicKubeService, otherKubeService, tunnelKubeService, otherKubeService},
	}

	authCtx.kubeServers, err = f.cfg.CachingAuthClient.GetKubernetesServers(context.Background())
	require.NoError(t, err)
	sess, err := f.newClusterSession(authCtx)
	require.NoError(t, err)
	require.Equal(t, []kubeClusterEndpoint{publicEndpoint, tunnelEndpoint}, sess.kubeClusterEndpoints)

	// Make sure newClusterSession obtained a new client cert instead of using f.creds.
	require.Equal(t, f.cfg.AuthClient.(*mockCSRClient).lastCert.Raw, sess.tlsConfig.Certificates[0].Certificate[0])
	//nolint:staticcheck // SA1019 there's no non-deprecated public API for testing the contents of the RootCAs pool
	require.Equal(t, [][]byte{f.cfg.AuthClient.(*mockCSRClient).ca.Cert.RawSubject}, sess.tlsConfig.RootCAs.Subjects())
	// Make sure that sess.tlsConfig was cloned from the value we store in the cache.
	// This is important because each connection must refer to a different memory address so it can be manipulated to enable/disable http2
	require.NotNil(t, f.getClientCreds(authCtx))
	require.NotSame(t, f.getClientCreds(authCtx), sess.tlsConfig)
	require.Equal(t, 1, f.clientCredentials.Len())
}

func TestClusterSessionDial(t *testing.T) {
	ctx := context.Background()
	sess := &clusterSession{
		authContext: authContext{
			teleportCluster: teleportClusterClient{
				dial: func(_ context.Context, _ string, endpoint kubeClusterEndpoint) (net.Conn, error) {
					if endpoint.addr == "" {
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

// TestKubeFwdHTTPProxyEnv ensures that kube forwarder doesn't respect HTTPS_PROXY env
// and Kubernetes API is called directly.
func TestKubeFwdHTTPProxyEnv(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	f := newMockForwader(ctx, t)
	authCtx := mockAuthCtx(ctx, t, "kube-cluster", false)

	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentNode,
			Client:    &mockEventClient{},
		},
	})
	require.NoError(t, err)
	t.Cleanup(lockWatcher.Close)
	f.cfg.LockWatcher = lockWatcher

	var kubeAPICallCount uint32
	mockKubeAPI := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddUint32(&kubeAPICallCount, 1)
	}))

	t.Cleanup(mockKubeAPI.Close)

	authCtx.teleportCluster.dial = func(ctx context.Context, network string, endpoint kubeClusterEndpoint) (net.Conn, error) {
		return new(net.Dialer).DialContext(ctx, mockKubeAPI.Listener.Addr().Network(), mockKubeAPI.Listener.Addr().String())
	}

	checkTransportProxy := func(rt http.RoundTripper) http.RoundTripper {
		tr, ok := rt.(*http.Transport)
		require.True(t, ok)
		require.Nil(t, tr.Proxy, "kube forwarder should not take into account HTTPS_PROXY env")
		return rt
	}

	f.clusterDetails = map[string]*kubeDetails{
		"local": {
			kubeCreds: &staticKubeCreds{
				targetAddr: mockKubeAPI.URL,
				tlsConfig:  mockKubeAPI.TLS,
				transportConfig: &transport.Config{
					WrapTransport: checkTransportProxy,
				},
			},
		},
	}

	authCtx.kubeClusterName = "local"
	sess, err := f.newClusterSession(authCtx)
	require.NoError(t, err)
	t.Cleanup(sess.close)
	require.Equal(t, []kubeClusterEndpoint{{addr: f.clusterDetails["local"].getTargetAddr()}}, sess.kubeClusterEndpoints)

	sess.tlsConfig.InsecureSkipVerify = true

	t.Setenv("HTTP_PROXY", "example.com:9999")
	t.Setenv("HTTPS_PROXY", "example.com:9999")

	// Set upgradeToHTTP2 to trigger h2 transport upgrade logic.
	sess.upgradeToHTTP2 = true
	fwd, err := f.makeSessionForwarder(sess)
	require.NoError(t, err)

	// Create KubeProxy that uses fwd and forward incoming request to kubernetes API.
	kubeProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL, err = url.Parse(mockKubeAPI.URL)
		require.NoError(t, err)
		fwd.ServeHTTP(w, r)
	}))
	t.Cleanup(kubeProxy.Close)

	req, err := http.NewRequest("GET", kubeProxy.URL, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, uint32(1), atomic.LoadUint32(&kubeAPICallCount))
	require.NoError(t, resp.Body.Close())
}

func newMockForwader(ctx context.Context, t *testing.T) *Forwarder {
	clientCreds, err := ttlmap.New(defaults.ClientCacheSize)
	require.NoError(t, err)

	csrClient, err := newMockCSRClient()
	require.NoError(t, err)

	return &Forwarder{
		log:    logrus.NewEntry(logrus.New()),
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
	kubeServers     []types.KubeServer
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

func (ap mockAccessPoint) GetKubernetesServers(ctx context.Context) ([]types.KubeServer, error) {
	return ap.kubeServers, nil
}

func (ap mockAccessPoint) GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error) {
	var cas []types.CertAuthority
	for _, ca := range ap.cas {
		cas = append(cas, ca)
	}
	return cas, nil
}

func (ap mockAccessPoint) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error) {
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
	ctx *authz.Context
	err error
}

func (a mockAuthorizer) Authorize(context.Context) (*authz.Context, error) {
	return a.ctx, a.err
}

type mockEventClient struct {
	services.Presence
	types.Events
	services.LockGetter
}

func (c *mockEventClient) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	return &mockWatcher{
		ctx:    ctx,
		eventC: make(chan types.Event),
	}, nil
}

type mockWatcher struct {
	ctx context.Context
	types.Watcher
	eventC chan types.Event
}

func (m *mockWatcher) Close() error {
	return nil
}

func (m *mockWatcher) Error() error {
	return nil
}

func (m *mockWatcher) Events() <-chan types.Event {
	return m.eventC
}

func (m *mockWatcher) Done() <-chan struct{} {
	return m.ctx.Done()
}

func newTestForwarder(ctx context.Context, cfg ForwarderConfig) *Forwarder {
	return &Forwarder{
		log:            logrus.NewEntry(logrus.New()),
		router:         *httprouter.New(),
		cfg:            cfg,
		activeRequests: make(map[string]context.Context),
		ctx:            ctx,
	}
}

type mockSemaphoreClient struct {
	auth.ClientI
	sem   types.Semaphores
	roles map[string]types.Role
}

func (m *mockSemaphoreClient) AcquireSemaphore(ctx context.Context, params types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	return m.sem.AcquireSemaphore(ctx, params)
}

func (m *mockSemaphoreClient) CancelSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	return m.sem.CancelSemaphoreLease(ctx, lease)
}

func (m *mockSemaphoreClient) GetRole(ctx context.Context, name string) (types.Role, error) {
	role, ok := m.roles[name]
	if !ok {
		return nil, trace.NotFound("role %q not found", name)
	}

	return role, nil
}

func TestKubernetesConnectionLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type testCase struct {
		name        string
		connections int
		role        types.Role
		assert      require.ErrorAssertionFunc
	}

	unlimitedRole, err := types.NewRole("unlimited", types.RoleSpecV6{})
	require.NoError(t, err)

	limitedRole, err := types.NewRole("unlimited", types.RoleSpecV6{
		Options: types.RoleOptions{
			MaxKubernetesConnections: 5,
		},
	})
	require.NoError(t, err)

	testCases := []testCase{
		{
			name:        "unlimited",
			connections: 7,
			role:        unlimitedRole,
			assert:      require.NoError,
		},
		{
			name:        "limited-success",
			connections: 5,
			role:        limitedRole,
			assert:      require.NoError,
		},
		{
			name:        "limited-fail",
			connections: 6,
			role:        limitedRole,
			assert:      require.Error,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			user, err := types.NewUser("bob")
			require.NoError(t, err)
			user.SetRoles([]string{testCase.role.GetName()})

			backend, err := memory.New(memory.Config{})
			require.NoError(t, err)

			sem := local.NewPresenceService(backend)
			client := &mockSemaphoreClient{
				sem:   sem,
				roles: map[string]types.Role{testCase.role.GetName(): testCase.role},
			}

			forwarder := newTestForwarder(ctx, ForwarderConfig{
				AuthClient:        client,
				CachingAuthClient: client,
			})

			identity := &authContext{
				Context: authz.Context{
					User: user,
					Identity: authz.WrapIdentity(tlsca.Identity{
						Username: user.GetName(),
						Groups:   []string{testCase.role.GetName()},
					}),
				},
			}

			for i := 0; i < testCase.connections; i++ {
				err = forwarder.acquireConnectionLockWithIdentity(ctx, identity)
				if i == testCase.connections-1 {
					testCase.assert(t, err)
				}
			}
		})
	}
}

func newKubeServersFromKubeClusters(t *testing.T, kubeClusters ...*types.KubernetesClusterV3) []types.KubeServer {
	var kubeServers []types.KubeServer
	for _, kubeCluster := range kubeClusters {
		kubeServer, err := types.NewKubernetesServerV3FromCluster(kubeCluster, "", kubeCluster.Metadata.Name)
		require.NoError(t, err)
		kubeServers = append(kubeServers, kubeServer)
	}
	return kubeServers
}

func TestForwarder_clientCreds_cache(t *testing.T) {
	now := time.Now()

	cache, err := ttlmap.New(10)
	require.NoError(t, err)

	cl, err := newMockCSRClient()
	require.NoError(t, err)

	f := &Forwarder{
		cfg: ForwarderConfig{
			Keygen:     testauthority.New(),
			AuthClient: cl,
			Clock:      clockwork.NewFakeClockAt(time.Now().Add(-2 * time.Minute)),
		},
		clientCredentials: cache,
		log:               logrus.New(),
	}

	type args struct {
		ctx authContext
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "store first certificate",
			args: args{
				ctx: authContext{
					kubeUsers:       utils.StringsSet([]string{"user-a"}),
					kubeGroups:      utils.StringsSet([]string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated}),
					kubeClusterName: "local",
					Context: authz.Context{
						User: &types.UserV2{
							Metadata: types.Metadata{
								Name: "user-a",
							},
						},
						Identity:         identity,
						UnmappedIdentity: unmappedIdentity,
					},
					kubeClusterLabels: make(map[string]string),
					certExpires:       now.Add(1 * time.Hour),
					teleportCluster: teleportClusterClient{
						name: "local",
					},
					sessionTTL: 1 * time.Hour,
				},
			},
		},
		{
			name: "store certificate with different certExpires value",
			args: args{
				ctx: authContext{
					kubeUsers:       utils.StringsSet([]string{"user-a"}),
					kubeGroups:      utils.StringsSet([]string{"kube-group-a", "kube-group-b", teleport.KubeSystemAuthenticated}),
					kubeClusterName: "local",
					Context: authz.Context{
						User: &types.UserV2{
							Metadata: types.Metadata{
								Name: "user-a",
							},
						},
						Identity:         identity,
						UnmappedIdentity: unmappedIdentity,
					},
					kubeClusterLabels: make(map[string]string),
					certExpires:       now.Add(2 * time.Hour),
					teleportCluster: teleportClusterClient{
						name: "local",
					},
					sessionTTL: 1 * time.Hour,
				},
			},
		},
		{
			name: "store certificate with different kube groups",
			args: args{
				ctx: authContext{
					kubeUsers:       utils.StringsSet([]string{"user-a"}),
					kubeGroups:      utils.StringsSet([]string{"kube-group-b", teleport.KubeSystemAuthenticated}),
					kubeClusterName: "local",
					Context: authz.Context{
						User: &types.UserV2{
							Metadata: types.Metadata{
								Name: "user-a",
							},
						},
						Identity:         identity,
						UnmappedIdentity: unmappedIdentity,
					},
					kubeClusterLabels: make(map[string]string),
					certExpires:       now.Add(1 * time.Hour),
					teleportCluster: teleportClusterClient{
						name: "local",
					},
					sessionTTL: 1 * time.Hour,
				},
			},
		},
		{
			name: "store certificate with different kube user",
			args: args{
				ctx: authContext{
					kubeUsers:       utils.StringsSet([]string{"user-test"}),
					kubeGroups:      utils.StringsSet([]string{"kube-group-b", teleport.KubeSystemAuthenticated}),
					kubeClusterName: "local",
					Context: authz.Context{
						User: &types.UserV2{
							Metadata: types.Metadata{
								Name: "user-a",
							},
						},
						Identity:         identity,
						UnmappedIdentity: unmappedIdentity,
					},
					kubeClusterLabels: make(map[string]string),
					certExpires:       now.Add(1 * time.Hour),
					teleportCluster: teleportClusterClient{
						name: "local",
					},
					sessionTTL: 1 * time.Hour,
				},
			},
		},
		{
			name: "store certificate for different user",
			args: args{
				ctx: authContext{
					kubeUsers:       utils.StringsSet([]string{"user-test"}),
					kubeGroups:      utils.StringsSet([]string{"kube-group-b", teleport.KubeSystemAuthenticated}),
					kubeClusterName: "local",
					Context: authz.Context{
						User: &types.UserV2{
							Metadata: types.Metadata{
								Name: "user-b",
							},
						},
						Identity:         identity,
						UnmappedIdentity: unmappedIdentity,
					},
					kubeClusterLabels: make(map[string]string),
					certExpires:       now.Add(1 * time.Hour),
					teleportCluster: teleportClusterClient{
						name: "local",
					},
					sessionTTL: 1 * time.Hour,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// make sure cert is not cached
			cachedTLSCfg := f.getClientCreds(tt.args.ctx)
			require.Nil(t, cachedTLSCfg)

			// request a new cert
			tlsCfg, err := f.requestCertificate(tt.args.ctx)
			require.NoError(t, err)

			// store the certificate in cache
			err = f.saveClientCreds(tt.args.ctx, tlsCfg)
			require.NoError(t, err)

			// make sure cache has our entry.
			cachedTLSCfg = f.getClientCreds(tt.args.ctx)
			require.NotNil(t, cachedTLSCfg)
			require.Equal(t, tlsCfg, cachedTLSCfg)
		})
	}
}

func Test_getPodResourceFromRequest(t *testing.T) {
	tests := []struct {
		name       string
		requestURI string
		want       *types.KubernetesResource
	}{
		{
			name:       "pod access endpoint",
			requestURI: "/api/v1/namespaces/default/pods/podName",
			want: &types.KubernetesResource{
				Kind:      types.KindKubePod,
				Namespace: "default",
				Name:      "podName",
			},
		},
		{
			name:       "pod exec endpoint",
			requestURI: "/api/v1/namespaces/default/pods/podName/exec",
			want: &types.KubernetesResource{
				Kind:      types.KindKubePod,
				Namespace: "default",
				Name:      "podName",
			},
		},
		{
			name:       "pod list endpoint",
			requestURI: "/api/v1/namespaces/default/pods/",
			want:       nil,
		},
		{
			name:       "secrets get endpoint",
			requestURI: "/api/v1/namespaces/default/secrets/secretName",
			want:       nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPodResourceFromRequest(tt.requestURI)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_copyImpersonationHeaders(t *testing.T) {
	tests := []struct {
		name        string
		inHeaders   http.Header
		wantHeaders http.Header
	}{
		{
			name: "copy impersonation headers",
			inHeaders: http.Header{
				"Host":                 []string{"example.com"},
				ImpersonateUserHeader:  []string{"user-a"},
				ImpersonateGroupHeader: []string{"kube-group-b"},
			},
			wantHeaders: http.Header{
				ImpersonateUserHeader:  []string{"user-a"},
				ImpersonateGroupHeader: []string{"kube-group-b"},
			},
		},
		{
			name: "don't introduce empty impersonation headers",
			inHeaders: http.Header{
				"Host": []string{"example.com"},
			},
			wantHeaders: http.Header{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := http.Header{}
			copyImpersonationHeaders(dst, tt.inHeaders)
			require.Equal(t, tt.wantHeaders, dst)
		})
	}
}

func Test_authContext_eventClusterMeta(t *testing.T) {
	t.Parallel()
	kubeClusterLabels := map[string]string{
		"label": "value",
	}
	type args struct {
		req *http.Request
		ctx *authContext
	}
	tests := []struct {
		name string
		args args
		want apievents.KubernetesClusterMetadata
	}{
		{
			name: "no headers in request",
			args: args{
				req: &http.Request{
					Header: http.Header{},
				},
				ctx: &authContext{
					kubeClusterName:   "clusterName",
					kubeClusterLabels: kubeClusterLabels,
					kubeGroups:        map[string]struct{}{"kube-group-a": {}, "kube-group-b": {}},
					kubeUsers:         map[string]struct{}{"kube-user-a": {}},
				},
			},
			want: apievents.KubernetesClusterMetadata{
				KubernetesCluster: "clusterName",
				KubernetesLabels:  kubeClusterLabels,
				KubernetesGroups:  []string{"kube-group-a", "kube-group-b"},
				KubernetesUsers:   []string{"kube-user-a"},
			},
		},
		{
			name: "with filter headers in request",
			args: args{
				req: &http.Request{
					Header: http.Header{
						ImpersonateUserHeader:  []string{"kube-user-a"},
						ImpersonateGroupHeader: []string{"kube-group-b", "kube-group-c"},
					},
				},
				ctx: &authContext{
					kubeClusterName:   "clusterName",
					kubeClusterLabels: kubeClusterLabels,
					kubeGroups:        map[string]struct{}{"kube-group-a": {}, "kube-group-b": {}, "kube-group-c": {}},
					kubeUsers:         map[string]struct{}{"kube-user-a": {}, "kube-user-b": {}},
				},
			},
			want: apievents.KubernetesClusterMetadata{
				KubernetesCluster: "clusterName",
				KubernetesLabels:  kubeClusterLabels,
				KubernetesGroups:  []string{"kube-group-b", "kube-group-c"},
				KubernetesUsers:   []string{"kube-user-a"},
			},
		},
		{
			name: "without filter headers in request and multi groups",
			args: args{
				req: &http.Request{
					Header: http.Header{},
				},
				ctx: &authContext{
					kubeClusterName:   "clusterName",
					kubeClusterLabels: kubeClusterLabels,
					kubeGroups:        map[string]struct{}{"kube-group-a": {}, "kube-group-b": {}, "kube-group-c": {}},
					kubeUsers:         map[string]struct{}{"kube-user-a": {}, "kube-user-b": {}},
				},
			},
			want: apievents.KubernetesClusterMetadata{
				KubernetesCluster: "clusterName",
				KubernetesLabels:  kubeClusterLabels,
				KubernetesGroups:  []string{"kube-group-a", "kube-group-b", "kube-group-c"},
				KubernetesUsers:   []string{"kube-user-a", "kube-user-b"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.args.ctx.eventClusterMeta(tt.args.req)
			sort.Strings(got.KubernetesGroups)
			sort.Strings(got.KubernetesGroups)
			require.Equal(t, tt.want, got)
		})
	}
}
