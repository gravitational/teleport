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

package proxy

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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
	"go.opentelemetry.io/otel"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/transport"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
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

func fakeClusterFeatures() proto.Features {
	return proto.Features{
		Kubernetes: true,
	}
}

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
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
		sites: map[string]reversetunnelclient.RemoteSite{
			"remote": mockRemoteSite{name: "remote"},
			"local":  mockRemoteSite{name: "local"},
		},
	}
	f := &Forwarder{
		log: logrus.NewEntry(logrus.New()),
		cfg: ForwarderConfig{
			ClusterName:       "local",
			CachingAuthClient: ap,
			TracerProvider:    otel.GetTracerProvider(),
			tracer:            otel.Tracer(teleport.ComponentKube),
			ClusterFeatures:   fakeClusterFeatures,
			KubeServiceType:   ProxyService,
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
		tunnel            reversetunnelclient.Server
		kubeServers       []types.KubeServer
		activeRequests    []string

		wantCtx     *authContext
		wantErr     bool
		wantAuthErr bool
	}{
		{
			desc:              "local user and cluster with active access request",
			user:              authz.LocalUser{},
			roleKubeGroups:    []string{"kube-group-a", "kube-group-b"},
			routeToCluster:    "local",
			kubernetesCluster: "local",
			haveKubeCreds:     true,
			tunnel:            tun,
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
			desc:              "local user and cluster",
			user:              authz.LocalUser{},
			roleKubeGroups:    []string{"kube-group-a", "kube-group-b"},
			routeToCluster:    "local",
			kubernetesCluster: "local",
			haveKubeCreds:     true,
			tunnel:            tun,
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
			desc:              "local user and cluster, no kubeconfig",
			user:              authz.LocalUser{},
			roleKubeGroups:    []string{"kube-group-a", "kube-group-b"},
			routeToCluster:    "local",
			kubernetesCluster: "local",
			haveKubeCreds:     false,
			tunnel:            tun,
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
			desc:              "remote user and local cluster",
			user:              authz.RemoteUser{},
			roleKubeGroups:    []string{"kube-group-a", "kube-group-b"},
			routeToCluster:    "local",
			kubernetesCluster: "local",
			haveKubeCreds:     true,
			tunnel:            tun,
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
			desc:              "remote user and local cluster with active request id",
			user:              authz.RemoteUser{},
			roleKubeGroups:    []string{"kube-group-a", "kube-group-b"},
			routeToCluster:    "local",
			kubernetesCluster: "local",
			haveKubeCreds:     true,
			tunnel:            tun,
			activeRequests:    activeAccessRequests,
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
			desc:              "kube users passed in request",
			user:              authz.LocalUser{},
			roleKubeUsers:     []string{"kube-user-a", "kube-user-b"},
			roleKubeGroups:    []string{"kube-group-a", "kube-group-b"},
			routeToCluster:    "local",
			kubernetesCluster: "local",
			haveKubeCreds:     true,
			tunnel:            tun,
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
			desc:              "local user and cluster, no tunnel",
			user:              authz.LocalUser{},
			roleKubeGroups:    []string{"kube-group-a", "kube-group-b"},
			routeToCluster:    "local",
			kubernetesCluster: "local",
			haveKubeCreds:     true,
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
					Expires:           certExpiration,
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
				URL:        &url.URL{},
				TLS: &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{
						{
							Subject: pkix.Name{
								CommonName:   username,
								Organization: []string{"example"},
							},
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
				require.Equal(t, tt.wantAuthErr, trace.IsAccessDenied(err))
				return
			}
			require.NoError(t, err)
			err = f.authorize(context.Background(), gotCtx)
			require.NoError(t, err)

			require.Empty(t, cmp.Diff(gotCtx, tt.wantCtx,
				cmp.AllowUnexported(authContext{}, teleportClusterClient{}, apiResource{}),
				cmpopts.IgnoreFields(authContext{}, "clientIdleTimeout", "sessionTTL", "Context", "recordingConfig", "disconnectExpiredCert", "kubeCluster", "apiResource"),
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
		isProxy       bool
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
		{
			desc:         "no existing impersonation headers, proxy service",
			kubeUsers:    []string{"kube-user-a"},
			kubeGroups:   []string{"kube-group-a", "kube-group-b"},
			isProxy:      true,
			inHeaders:    http.Header{},
			wantHeaders:  http.Header{},
			errAssertion: require.NoError,
		},
		{
			desc:       "existing impersonation headers, proxy service",
			kubeUsers:  []string{"kube-user-a"},
			kubeGroups: []string{"kube-group-a", "kube-group-b"},
			isProxy:    true,
			inHeaders: http.Header{
				ImpersonateGroupHeader: []string{"kube-group-a"},
			},
			wantHeaders: http.Header{
				ImpersonateGroupHeader: []string{"kube-group-a"},
			},
			errAssertion: require.NoError,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			var kubeCreds kubeCreds
			if !tt.isProxy {
				kubeCreds = &staticKubeCreds{}
			}
			err := setupImpersonationHeaders(
				logrus.NewEntry(logrus.New()),
				&clusterSession{
					kubeAPICreds: kubeCreds,
					authContext: authContext{
						kubeUsers:       utils.StringsSet(tt.kubeUsers),
						kubeGroups:      utils.StringsSet(tt.kubeGroups),
						teleportCluster: teleportClusterClient{isRemote: tt.remoteCluster},
					},
				},
				tt.inHeaders,
			)
			tt.errAssertion(t, err)

			if err == nil {
				// Sort header values to get predictable ordering.
				for _, vals := range tt.inHeaders {
					sort.Strings(vals)
				}
				require.Empty(t, cmp.Diff(tt.inHeaders, tt.wantHeaders))
			}
		},
		)
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
		// getClientCreds requires sessions to be valid for at least 1 minute
		sessionTTL: 2 * time.Minute,
	}
}

// TestKubeFwdHTTPProxyEnv ensures that Teleport only respects the `[HTTP(S)|NO]_PROXY`
// env variables when dialing directly to the EKS cluster and doesn't respect
// them when dialing via reverse tunnel to other Teleport services.
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

	checkTransportProxyDirectDial := func(rt http.RoundTripper) http.RoundTripper {
		tr, ok := rt.(*http.Transport)
		require.True(t, ok)
		require.NotNil(t, tr.Proxy, "kube forwarder should take into account HTTPS_PROXY env when dialing to kubernetes API")
		return rt
	}

	checkTransportProxIndirectDialer := func(rt http.RoundTripper) http.RoundTripper {
		tr, ok := rt.(*http.Transport)
		require.True(t, ok)
		require.Nil(t, tr.Proxy, "kube forwarder should not take into account HTTPS_PROXY env when dialing over tunnel")
		return rt
	}

	t.Setenv("HTTP_PROXY", "example.com:9999")
	t.Setenv("HTTPS_PROXY", "example.com:9999")

	for _, test := range []struct {
		name      string
		rtBuilder func(t *testing.T) http.RoundTripper
		checkFunc func(t *testing.T, req *http.Request)
	}{
		{
			name: "newDirectTransports",
			rtBuilder: func(t *testing.T) http.RoundTripper {
				rt, err := newDirectTransport("test", &tls.Config{
					InsecureSkipVerify: true,
				},
					&transport.Config{
						WrapTransport: checkTransportProxyDirectDial,
					})
				require.NoError(t, err)
				return rt
			},
		},
		{
			name: "newTransport",
			rtBuilder: func(t *testing.T) http.RoundTripper {
				h2HTTPTransport, err := newH2Transport(&tls.Config{
					InsecureSkipVerify: true,
				}, nil)
				require.NoError(t, err)
				h2Transport, err := wrapTransport(h2HTTPTransport, &transport.Config{
					WrapTransport: checkTransportProxIndirectDialer,
				})
				require.NoError(t, err)
				return h2Transport
			},
		},
	} {

		f.clusterDetails = map[string]*kubeDetails{
			"local": {
				kubeCreds: &staticKubeCreds{
					targetAddr: mockKubeAPI.URL,
					tlsConfig:  mockKubeAPI.TLS,
					transport:  test.rtBuilder(t),
				},
			},
		}

		authCtx.kubeClusterName = "local"
		sess, err := f.newClusterSession(ctx, authCtx)
		require.NoError(t, err)
		t.Cleanup(sess.close)

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
		require.NoError(t, resp.Body.Close())
	}
	require.Equal(t, uint32(2), atomic.LoadUint32(&kubeAPICallCount))
}

func newMockForwader(ctx context.Context, t *testing.T) *Forwarder {
	clock := clockwork.NewFakeClock()
	cachedTransport, err := ttlmap.New(defaults.ClientCacheSize, ttlmap.Clock(clock))
	require.NoError(t, err)
	csrClient, err := newMockCSRClient(clock)
	require.NoError(t, err)

	return &Forwarder{
		log:    logrus.NewEntry(logrus.New()),
		router: httprouter.New(),
		cfg: ForwarderConfig{
			Keygen:            testauthority.New(),
			AuthClient:        csrClient,
			CachingAuthClient: mockAccessPoint{},
			Clock:             clock,
			Context:           ctx,
			TracerProvider:    otel.GetTracerProvider(),
			tracer:            otel.Tracer(teleport.ComponentKube),
			ClusterFeatures:   fakeClusterFeatures,
		},
		activeRequests:  make(map[string]context.Context),
		ctx:             ctx,
		cachedTransport: cachedTransport,
	}
}

// mockCSRClient to intercept ProcessKubeCSR requests, record them and return a
// stub response.
type mockCSRClient struct {
	auth.ClientI

	clock           clockwork.Clock
	ca              *tlsca.CertAuthority
	gotCSR          auth.KubeCSR
	lastCert        *x509.Certificate
	leafClusterName string
}

func newMockCSRClient(clock clockwork.Clock) (*mockCSRClient, error) {
	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	if err != nil {
		return nil, err
	}
	return &mockCSRClient{ca: ca, clock: clock}, nil
}

func (c *mockCSRClient) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
	if id.DomainName == c.leafClusterName {
		return &types.CertAuthorityV2{
			Kind:    types.KindCertAuthority,
			Version: types.V3,
			Metadata: types.Metadata{
				Name: "local",
			},
			Spec: types.CertAuthoritySpecV2{
				Type:        types.HostCA,
				ClusterName: c.leafClusterName,
				ActiveKeys: types.CAKeySet{
					TLS: []*types.TLSKeyPair{{Cert: []byte(fixtures.TLSCACertPEM)}},
				},
			},
		}, nil
	}
	return nil, trace.NotFound("cluster not found")
}

func (c *mockCSRClient) ProcessKubeCSR(csr auth.KubeCSR) (*auth.KubeCSRResponse, error) {
	c.gotCSR = csr

	x509CSR, err := tlsca.ParseCertificateRequestPEM(csr.CSR)
	if err != nil {
		return nil, err
	}
	caCSR := tlsca.CertificateRequest{
		Clock:     c.clock,
		PublicKey: x509CSR.PublicKey.(crypto.PublicKey),
		Subject:   x509CSR.Subject,
		// getClientCreds requires sessions to be valid for at least 1 minute
		NotAfter: c.clock.Now().Add(2 * time.Minute),
		DNSNames: x509CSR.DNSNames,
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

// mockRemoteSite is a reversetunnelclient.RemoteSite implementation with hardcoded
// name, because there's no easy way to construct a real
// reversetunnelclient.RemoteSite.
type mockRemoteSite struct {
	reversetunnelclient.RemoteSite
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

func (ap mockAccessPoint) GetClusterNetworkingConfig(context.Context) (types.ClusterNetworkingConfig, error) {
	return ap.netConfig, nil
}

func (ap mockAccessPoint) GetSessionRecordingConfig(context.Context) (types.SessionRecordingConfig, error) {
	return ap.recordingConfig, nil
}

func (ap mockAccessPoint) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	return ap.authPref, nil
}

func (ap mockAccessPoint) GetKubernetesServers(ctx context.Context) ([]types.KubeServer, error) {
	return ap.kubeServers, nil
}

func (ap mockAccessPoint) GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error) {
	var cas []types.CertAuthority
	for _, ca := range ap.cas {
		cas = append(cas, ca)
	}
	return cas, nil
}

func (ap mockAccessPoint) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
	return ap.cas[id.DomainName], nil
}

type mockRevTunnel struct {
	reversetunnelclient.Server

	sites map[string]reversetunnelclient.RemoteSite
}

func (t mockRevTunnel) GetSite(name string) (reversetunnelclient.RemoteSite, error) {
	s, ok := t.sites[name]
	if !ok {
		return nil, trace.NotFound("remote site %q not found", name)
	}
	return s, nil
}

func (t mockRevTunnel) GetSites() ([]reversetunnelclient.RemoteSite, error) {
	var sites []reversetunnelclient.RemoteSite
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
		router:         httprouter.New(),
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
				TracerProvider:    otel.GetTracerProvider(),
				tracer:            otel.Tracer(teleport.ComponentKube),
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

func TestKubernetesLicenseEnforcement(t *testing.T) {
	t.Parallel()
	// kubeMock is a Kubernetes API mock for the session tests.
	kubeMock, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })

	tests := []struct {
		name          string
		features      proto.Features
		assertErrFunc require.ErrorAssertionFunc
	}{
		{
			name: "kubernetes agent is licensed",
			features: proto.Features{
				Kubernetes: true,
			},
			assertErrFunc: require.NoError,
		},
		{
			name: "kubernetes isn't licensed",
			features: proto.Features{
				Kubernetes: false,
			},
			assertErrFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(tt, err)
				var kubeErr *kubeerrors.StatusError
				require.ErrorAs(tt, err, &kubeErr)
				require.Equal(tt, int32(http.StatusForbidden), kubeErr.ErrStatus.Code)
				require.Equal(tt, metav1.StatusReasonForbidden, kubeErr.ErrStatus.Reason)
				require.Equal(tt, "Teleport cluster is not licensed for Kubernetes", kubeErr.ErrStatus.Message)
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// creates a Kubernetes service with a configured cluster pointing to mock api server
			testCtx := SetupTestContext(
				context.Background(),
				t,
				TestConfig{
					Clusters: []KubeClusterConfig{{Name: kubeCluster, APIEndpoint: kubeMock.URL}},
					ClusterFeatures: func() proto.Features {
						return tt.features
					},
				},
			)
			// close tests
			t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

			_, _ = testCtx.CreateUserAndRole(
				testCtx.Context,
				t,
				username,
				RoleSpec{
					Name:       roleName,
					KubeUsers:  roleKubeUsers,
					KubeGroups: roleKubeGroups,
				})

			// generate a kube client with user certs for auth
			client, _ := testCtx.GenTestKubeClientTLSCert(
				t,
				username,
				kubeCluster,
			)

			_, err := client.CoreV1().Pods(metav1.NamespaceDefault).List(context.Background(), metav1.ListOptions{})
			tt.assertErrFunc(t, err)
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

func TestForwarderTLSConfigCAs(t *testing.T) {
	clusterName := "leaf"

	// Create a cert pool with the cert from fixtures.TLSCACertPEM
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM([]byte(fixtures.TLSCACertPEM))

	// create the tls config used by the forwarder
	originalTLSConfig := &tls.Config{}
	// create the auth server mock client
	clock := clockwork.NewFakeClock()
	cl, err := newMockCSRClient(clock)
	require.NoError(t, err)
	cl.leafClusterName = clusterName

	f := &Forwarder{
		cfg: ForwarderConfig{
			Keygen:            testauthority.New(),
			AuthClient:        cl,
			TracerProvider:    otel.GetTracerProvider(),
			tracer:            otel.Tracer(teleport.ComponentKube),
			KubeServiceType:   ProxyService,
			CachingAuthClient: cl,
			ConnTLSConfig:     originalTLSConfig,
		},
		log: logrus.NewEntry(logrus.New()),
		ctx: context.Background(),
	}
	// generate tlsConfig for the leaf cluster
	tlsConfig, err := f.getTLSConfigForLeafCluster(clusterName)
	require.NoError(t, err)
	// ensure that the tlsConfig is a clone of the originalTLSConfig
	require.NotSame(t, originalTLSConfig, tlsConfig, "expected tlsConfig to be different from originalTLSConfig")
	// ensure that the tlsConfig has the certPool as the RootCAs
	require.True(t, tlsConfig.RootCAs.Equal(certPool), "expected root CAs to be equal to certPool")

	// generate tlsConfig for the local cluster
	_, localTLSConfig, err := f.newLocalClusterTransport(clusterName)
	require.NoError(t, err)
	// ensure that the localTLSConfig is a clone of the originalTLSConfig
	require.NotSame(t, originalTLSConfig, localTLSConfig, "expected localTLSConfig pointer to be different from originalTLSConfig")
	// ensure that the localTLSConfig doesn't have the certPool as the RootCAs
	require.False(t, localTLSConfig.RootCAs.Equal(certPool), "root CAs should not include certPool")
}
