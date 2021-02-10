/*
Copyright 2015-2019 Gravitational, Inc.

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

package suite

import (
	"context"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/test"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"

	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

// ServicesTestSuite is an acceptance test suite
// for services. It is used for local implementations and implementations
// using GRPC to guarantee consistency between local and remote services
type ServicesTestSuite struct {
	Access        auth.Access
	CAS           auth.Trust
	PresenceS     auth.Presence
	ProvisioningS auth.Provisioner
	ConfigS       auth.ClusterConfiguration
	EventsS       types.Events
	UsersS        auth.UsersService
	ChangesC      chan interface{}
	Clock         clockwork.FakeClock
}

// ClusterConfig tests cluster configuration
func (s *ServicesTestSuite) ClusterConfig(c *check.C) {
	config, err := types.NewClusterConfig(types.ClusterConfigSpecV3{
		ClientIdleTimeout:     types.NewDuration(17 * time.Second),
		DisconnectExpiredCert: types.NewBool(true),
		ClusterID:             "27",
		SessionRecording:      types.RecordAtProxy,
		Audit: types.AuditConfig{
			Region:           "us-west-1",
			Type:             "dynamodb",
			AuditSessionsURI: "file:///home/log",
			AuditTableName:   "audit_table_name",
			AuditEventsURI:   []string{"dynamodb://audit_table_name", "file:///home/log"},
		},
	})
	c.Assert(err, check.IsNil)

	err = s.ConfigS.SetClusterConfig(config)
	c.Assert(err, check.IsNil)

	gotConfig, err := s.ConfigS.GetClusterConfig()
	c.Assert(err, check.IsNil)
	config.SetResourceID(gotConfig.GetResourceID())
	fixtures.DeepCompare(c, config, gotConfig)
}

// NewServer creates a new server resource
func NewServer(kind, name, addr, namespace string) *services.ServerV2 {
	return &services.ServerV2{
		Kind:    kind,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      name,
			Namespace: namespace,
		},
		Spec: services.ServerSpecV2{
			Addr:       addr,
			PublicAddr: addr,
		},
	}
}

func (s *ServicesTestSuite) ServerCRUD(c *check.C) {
	ctx := context.TODO()
	// SSH service.
	out, err := s.PresenceS.GetNodes(defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	srv := NewServer(services.KindNode, "srv1", "127.0.0.1:2022", defaults.Namespace)
	_, err = s.PresenceS.UpsertNode(srv)
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetNodes(srv.Metadata.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	srv.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, out, []services.Server{srv})

	err = s.PresenceS.DeleteNode(srv.Metadata.Namespace, srv.GetName())
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetNodes(srv.Metadata.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 0)

	// Proxy service.
	out, err = s.PresenceS.GetProxies()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	proxy := NewServer(services.KindProxy, "proxy1", "127.0.0.1:2023", defaults.Namespace)
	c.Assert(s.PresenceS.UpsertProxy(proxy), check.IsNil)

	out, err = s.PresenceS.GetProxies()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	proxy.SetResourceID(out[0].GetResourceID())
	c.Assert(out, check.DeepEquals, []services.Server{proxy})

	err = s.PresenceS.DeleteProxy(proxy.GetName())
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetProxies()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 0)

	// Auth service.
	out, err = s.PresenceS.GetAuthServers()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	auth := NewServer(services.KindAuthServer, "auth1", "127.0.0.1:2025", defaults.Namespace)
	c.Assert(s.PresenceS.UpsertAuthServer(auth), check.IsNil)

	out, err = s.PresenceS.GetAuthServers()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	auth.SetResourceID(out[0].GetResourceID())
	c.Assert(out, check.DeepEquals, []services.Server{auth})

	// Kubernetes service.
	out, err = s.PresenceS.GetKubeServices(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	kube1 := NewServer(services.KindKubeService, "kube1", "10.0.0.1:3026", defaults.Namespace)
	c.Assert(s.PresenceS.UpsertKubeService(ctx, kube1), check.IsNil)
	kube2 := NewServer(services.KindKubeService, "kube2", "10.0.0.2:3026", defaults.Namespace)
	c.Assert(s.PresenceS.UpsertKubeService(ctx, kube2), check.IsNil)

	out, err = s.PresenceS.GetKubeServices(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 2)
	kube1.SetResourceID(out[0].GetResourceID())
	kube2.SetResourceID(out[1].GetResourceID())
	c.Assert(out, check.DeepEquals, []services.Server{kube1, kube2})

	c.Assert(s.PresenceS.DeleteKubeService(ctx, kube1.GetName()), check.IsNil)
	out, err = s.PresenceS.GetKubeServices(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	c.Assert(out, check.DeepEquals, []services.Server{kube2})

	c.Assert(s.PresenceS.DeleteAllKubeServices(ctx), check.IsNil)
	out, err = s.PresenceS.GetKubeServices(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 0)
}

// NewAppServer creates a new application server resource.
func NewAppServer(name string, internalAddr string, publicAddr string) *services.ServerV2 {
	return &services.ServerV2{
		Kind:    services.KindAppServer,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      uuid.New(),
			Namespace: defaults.Namespace,
		},
		Spec: services.ServerSpecV2{
			Apps: []*services.App{
				{
					Name:       name,
					URI:        internalAddr,
					PublicAddr: publicAddr,
				},
			},
		},
	}
}

// AppServerCRUD tests CRUD functionality for services.Server.
func (s *ServicesTestSuite) AppServerCRUD(c *check.C) {
	ctx := context.Background()

	// Create application.
	server := NewAppServer("foo", "http://127.0.0.1:8080", "foo.example.com")

	// Expect not to be returned any applications and trace.NotFound.
	out, err := s.PresenceS.GetAppServers(ctx, defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	// Upsert application.
	_, err = s.PresenceS.UpsertAppServer(ctx, server)
	c.Assert(err, check.IsNil)

	// Check again, expect a single application to be found.
	out, err = s.PresenceS.GetAppServers(ctx, server.GetNamespace())
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	server.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, out, []services.Server{server})

	// Remove the application.
	err = s.PresenceS.DeleteAppServer(ctx, server.Metadata.Namespace, server.GetName())
	c.Assert(err, check.IsNil)

	// Now expect no applications to be returned.
	out, err = s.PresenceS.GetAppServers(ctx, server.Metadata.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 0)
}

func newReverseTunnel(clusterName string, dialAddrs []string) *services.ReverseTunnelV2 {
	return &services.ReverseTunnelV2{
		Kind:    services.KindReverseTunnel,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      clusterName,
			Namespace: defaults.Namespace,
		},
		Spec: services.ReverseTunnelSpecV2{
			ClusterName: clusterName,
			DialAddrs:   dialAddrs,
		},
	}
}

func (s *ServicesTestSuite) ReverseTunnelsCRUD(c *check.C) {
	out, err := s.PresenceS.GetReverseTunnels()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	tunnel := newReverseTunnel("example.com", []string{"example.com:2023"})
	c.Assert(s.PresenceS.UpsertReverseTunnel(tunnel), check.IsNil)

	out, err = s.PresenceS.GetReverseTunnels()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	tunnel.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, out, []services.ReverseTunnel{tunnel})

	err = s.PresenceS.DeleteReverseTunnel(tunnel.Spec.ClusterName)
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetReverseTunnels()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	err = s.PresenceS.UpsertReverseTunnel(newReverseTunnel("", []string{"127.0.0.1:1234"}))
	fixtures.ExpectBadParameter(c, err)

	err = s.PresenceS.UpsertReverseTunnel(newReverseTunnel("example.com", []string{""}))
	fixtures.ExpectBadParameter(c, err)

	err = s.PresenceS.UpsertReverseTunnel(newReverseTunnel("example.com", []string{}))
	fixtures.ExpectBadParameter(c, err)
}

func (s *ServicesTestSuite) NamespacesCRUD(c *check.C) {
	out, err := s.PresenceS.GetNamespaces()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	ns := services.Namespace{
		Kind:    services.KindNamespace,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      defaults.Namespace,
			Namespace: defaults.Namespace,
		},
	}
	err = s.PresenceS.UpsertNamespace(ns)
	c.Assert(err, check.IsNil)
	nsout, err := s.PresenceS.GetNamespace(ns.Metadata.Name)
	c.Assert(err, check.IsNil)
	c.Assert(nsout, check.DeepEquals, &ns)

	err = s.PresenceS.DeleteNamespace(ns.Metadata.Name)
	c.Assert(err, check.IsNil)

	_, err = s.PresenceS.GetNamespace(ns.Metadata.Name)
	fixtures.ExpectNotFound(c, err)
}

func (s *ServicesTestSuite) TunnelConnectionsCRUD(c *check.C) {
	clusterName := "example.com"
	out, err := s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	dt := s.Clock.Now()
	conn, err := services.NewTunnelConnection("conn1", services.TunnelConnectionSpecV2{
		ClusterName:   clusterName,
		ProxyName:     "p1",
		LastHeartbeat: dt,
	})
	c.Assert(err, check.IsNil)

	err = s.PresenceS.UpsertTunnelConnection(conn)
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 1)
	conn.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, out[0], conn)

	out, err = s.PresenceS.GetAllTunnelConnections()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 1)
	fixtures.DeepCompare(c, out[0], conn)

	dt = dt.Add(time.Hour)
	conn.SetLastHeartbeat(dt)

	err = s.PresenceS.UpsertTunnelConnection(conn)
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 1)
	conn.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, out[0], conn)

	err = s.PresenceS.DeleteAllTunnelConnections()
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	err = s.PresenceS.DeleteAllTunnelConnections()
	c.Assert(err, check.IsNil)

	// test delete individual connection
	err = s.PresenceS.UpsertTunnelConnection(conn)
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 1)
	conn.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, out[0], conn)

	err = s.PresenceS.DeleteTunnelConnection(clusterName, conn.GetName())
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)
}

func (s *ServicesTestSuite) RemoteClustersCRUD(c *check.C) {
	clusterName := "example.com"
	out, err := s.PresenceS.GetRemoteClusters()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	rc, err := services.NewRemoteCluster(clusterName)
	c.Assert(err, check.IsNil)

	rc.SetConnectionStatus(teleport.RemoteClusterStatusOffline)

	err = s.PresenceS.CreateRemoteCluster(rc)
	c.Assert(err, check.IsNil)

	err = s.PresenceS.CreateRemoteCluster(rc)
	fixtures.ExpectAlreadyExists(c, err)

	out, err = s.PresenceS.GetRemoteClusters()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 1)
	rc.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, out[0], rc)

	err = s.PresenceS.DeleteAllRemoteClusters()
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetRemoteClusters()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 0)

	// test delete individual connection
	err = s.PresenceS.CreateRemoteCluster(rc)
	c.Assert(err, check.IsNil)

	out, err = s.PresenceS.GetRemoteClusters()
	c.Assert(err, check.IsNil)
	c.Assert(len(out), check.Equals, 1)
	fixtures.DeepCompare(c, out[0], rc)

	err = s.PresenceS.DeleteRemoteCluster(clusterName)
	c.Assert(err, check.IsNil)

	err = s.PresenceS.DeleteRemoteCluster(clusterName)
	fixtures.ExpectNotFound(c, err)
}

// AuthPreference tests authentication preference service
func (s *ServicesTestSuite) AuthPreference(c *check.C) {
	ap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         "local",
		SecondFactor: "otp",
	})
	c.Assert(err, check.IsNil)

	err = s.ConfigS.SetAuthPreference(ap)
	c.Assert(err, check.IsNil)

	gotAP, err := s.ConfigS.GetAuthPreference()
	c.Assert(err, check.IsNil)

	c.Assert(gotAP.GetType(), check.Equals, "local")
	c.Assert(gotAP.GetSecondFactor(), check.Equals, constants.SecondFactorOTP)
}

// Events tests various events variations
func (s *ServicesTestSuite) Events(c *check.C) {
	ctx := context.Background()
	testCases := []EventTest{
		{
			Name: "Cert authority with secrets",
			Kind: services.WatchKind{
				Kind:        services.KindCertAuthority,
				LoadSecrets: true,
			},
			CRUD: func() services.Resource {
				ca := test.NewCA(services.UserCA, "example.com")
				c.Assert(s.CAS.UpsertCertAuthority(ca), check.IsNil)

				out, err := s.CAS.GetCertAuthority(*ca.ID(), true)
				c.Assert(err, check.IsNil)

				c.Assert(s.CAS.DeleteCertAuthority(*ca.ID()), check.IsNil)
				return out
			},
		},
	}
	s.RunEventsTests(c, testCases)

	testCases = []EventTest{
		{
			Name: "Cert authority without secrets",
			Kind: services.WatchKind{
				Kind:        services.KindCertAuthority,
				LoadSecrets: false,
			},
			CRUD: func() services.Resource {
				ca := test.NewCA(services.UserCA, "example.com")
				c.Assert(s.CAS.UpsertCertAuthority(ca), check.IsNil)

				out, err := s.CAS.GetCertAuthority(*ca.ID(), false)
				c.Assert(err, check.IsNil)

				c.Assert(s.CAS.DeleteCertAuthority(*ca.ID()), check.IsNil)
				return out
			},
		},
	}
	s.RunEventsTests(c, testCases)

	testCases = []EventTest{
		{
			Name: "Token",
			Kind: services.WatchKind{
				Kind: services.KindToken,
			},
			CRUD: func() services.Resource {
				expires := time.Now().UTC().Add(time.Hour)
				t, err := services.NewProvisionToken("token",
					teleport.Roles{teleport.RoleAuth, teleport.RoleNode}, expires)
				c.Assert(err, check.IsNil)

				c.Assert(s.ProvisioningS.UpsertToken(ctx, t), check.IsNil)

				token, err := s.ProvisioningS.GetToken(ctx, "token")
				c.Assert(err, check.IsNil)

				c.Assert(s.ProvisioningS.DeleteToken(ctx, "token"), check.IsNil)
				return token
			},
		},
		{
			Name: "Namespace",
			Kind: services.WatchKind{
				Kind: services.KindNamespace,
			},
			CRUD: func() services.Resource {
				ns := services.Namespace{
					Kind:    services.KindNamespace,
					Version: services.V2,
					Metadata: services.Metadata{
						Name:      "testnamespace",
						Namespace: defaults.Namespace,
					},
				}
				err := s.PresenceS.UpsertNamespace(ns)
				c.Assert(err, check.IsNil)

				out, err := s.PresenceS.GetNamespace(ns.Metadata.Name)
				c.Assert(err, check.IsNil)

				err = s.PresenceS.DeleteNamespace(ns.Metadata.Name)
				c.Assert(err, check.IsNil)

				return out
			},
		},
		{
			Name: "Static tokens",
			Kind: services.WatchKind{
				Kind: services.KindStaticTokens,
			},
			CRUD: func() services.Resource {
				staticTokens, err := services.NewStaticTokens(services.StaticTokensSpecV2{
					StaticTokens: []services.ProvisionTokenV1{
						{
							Token:   "tok1",
							Roles:   teleport.Roles{teleport.RoleNode},
							Expires: time.Now().UTC().Add(time.Hour),
						},
					},
				})
				c.Assert(err, check.IsNil)

				err = s.ConfigS.SetStaticTokens(staticTokens)
				c.Assert(err, check.IsNil)

				out, err := s.ConfigS.GetStaticTokens()
				c.Assert(err, check.IsNil)

				err = s.ConfigS.DeleteStaticTokens()
				c.Assert(err, check.IsNil)

				return out
			},
		},
		{
			Name: "Role",
			Kind: services.WatchKind{
				Kind: services.KindRole,
			},
			CRUD: func() services.Resource {
				role, err := services.NewRole("role1", services.RoleSpecV3{
					Options: services.RoleOptions{
						MaxSessionTTL: services.Duration(time.Hour),
					},
					Allow: services.RoleConditions{
						Logins:     []string{"root", "bob"},
						NodeLabels: services.Labels{services.Wildcard: []string{services.Wildcard}},
					},
					Deny: services.RoleConditions{},
				})
				c.Assert(err, check.IsNil)

				err = s.Access.UpsertRole(ctx, role)
				c.Assert(err, check.IsNil)

				out, err := s.Access.GetRole(ctx, role.GetName())
				c.Assert(err, check.IsNil)

				err = s.Access.DeleteRole(ctx, role.GetName())
				c.Assert(err, check.IsNil)

				return out
			},
		},
		{
			Name: "User",
			Kind: services.WatchKind{
				Kind: services.KindUser,
			},
			CRUD: func() services.Resource {
				user := newUser("user1", "admin")
				err := s.UsersS.UpsertUser(user)
				c.Assert(err, check.IsNil)

				out, err := s.UsersS.GetUser(user.GetName(), false)
				c.Assert(err, check.IsNil)

				c.Assert(s.UsersS.DeleteUser(context.TODO(), user.GetName()), check.IsNil)
				return out
			},
		},
		{
			Name: "Node",
			Kind: services.WatchKind{
				Kind: services.KindNode,
			},
			CRUD: func() services.Resource {
				srv := NewServer(services.KindNode, "srv1", "127.0.0.1:2022", defaults.Namespace)

				_, err := s.PresenceS.UpsertNode(srv)
				c.Assert(err, check.IsNil)

				out, err := s.PresenceS.GetNodes(srv.Metadata.Namespace)
				c.Assert(err, check.IsNil)

				err = s.PresenceS.DeleteAllNodes(srv.Metadata.Namespace)
				c.Assert(err, check.IsNil)

				return out[0]
			},
		},
		{
			Name: "Proxy",
			Kind: services.WatchKind{
				Kind: services.KindProxy,
			},
			CRUD: func() services.Resource {
				srv := NewServer(services.KindProxy, "srv1", "127.0.0.1:2022", defaults.Namespace)

				err := s.PresenceS.UpsertProxy(srv)
				c.Assert(err, check.IsNil)

				out, err := s.PresenceS.GetProxies()
				c.Assert(err, check.IsNil)

				err = s.PresenceS.DeleteAllProxies()
				c.Assert(err, check.IsNil)

				return out[0]
			},
		},
		{
			Name: "Tunnel connection",
			Kind: services.WatchKind{
				Kind: services.KindTunnelConnection,
			},
			CRUD: func() services.Resource {
				conn, err := services.NewTunnelConnection("conn1", services.TunnelConnectionSpecV2{
					ClusterName:   "example.com",
					ProxyName:     "p1",
					LastHeartbeat: time.Now().UTC(),
				})
				c.Assert(err, check.IsNil)

				err = s.PresenceS.UpsertTunnelConnection(conn)
				c.Assert(err, check.IsNil)

				out, err := s.PresenceS.GetTunnelConnections("example.com")
				c.Assert(err, check.IsNil)

				err = s.PresenceS.DeleteAllTunnelConnections()
				c.Assert(err, check.IsNil)

				return out[0]
			},
		},
		{
			Name: "Reverse tunnel",
			Kind: services.WatchKind{
				Kind: services.KindReverseTunnel,
			},
			CRUD: func() services.Resource {
				tunnel := newReverseTunnel("example.com", []string{"example.com:2023"})
				c.Assert(s.PresenceS.UpsertReverseTunnel(tunnel), check.IsNil)

				out, err := s.PresenceS.GetReverseTunnels()
				c.Assert(err, check.IsNil)

				err = s.PresenceS.DeleteReverseTunnel(tunnel.Spec.ClusterName)
				c.Assert(err, check.IsNil)

				return out[0]
			},
		},
		{
			Name: "Remote cluster",
			Kind: services.WatchKind{
				Kind: services.KindRemoteCluster,
			},
			CRUD: func() services.Resource {
				rc, err := services.NewRemoteCluster("example.com")
				rc.SetConnectionStatus(teleport.RemoteClusterStatusOffline)
				c.Assert(err, check.IsNil)
				c.Assert(s.PresenceS.CreateRemoteCluster(rc), check.IsNil)

				out, err := s.PresenceS.GetRemoteClusters()
				c.Assert(err, check.IsNil)

				err = s.PresenceS.DeleteRemoteCluster(rc.GetName())
				c.Assert(err, check.IsNil)

				return out[0]
			},
		},
	}
	s.RunEventsTests(c, testCases)

	// Namespace with a name
	testCases = []EventTest{
		{
			Name: "Namespace with a name",
			Kind: services.WatchKind{
				Kind: services.KindNamespace,
				Name: "shmest",
			},
			CRUD: func() services.Resource {
				ns := services.Namespace{
					Kind:    services.KindNamespace,
					Version: services.V2,
					Metadata: services.Metadata{
						Name:      "shmest",
						Namespace: defaults.Namespace,
					},
				}
				err := s.PresenceS.UpsertNamespace(ns)
				c.Assert(err, check.IsNil)

				out, err := s.PresenceS.GetNamespace(ns.Metadata.Name)
				c.Assert(err, check.IsNil)

				err = s.PresenceS.DeleteNamespace(ns.Metadata.Name)
				c.Assert(err, check.IsNil)

				return out
			},
		},
	}
	s.RunEventsTests(c, testCases)
}

func (s *ServicesTestSuite) RunEventsTests(c *check.C, testCases []EventTest) {
	ctx := context.TODO()
	w, err := s.EventsS.NewWatcher(ctx, services.Watch{
		Kinds: eventsTestKinds(testCases),
	})
	c.Assert(err, check.IsNil)
	defer w.Close()

	select {
	case event := <-w.Events():
		c.Assert(event.Type, check.Equals, backend.OpInit)
	case <-w.Done():
		c.Fatalf("Watcher exited with error %v", w.Error())
	case <-time.After(2 * time.Second):
		c.Fatalf("Timeout waiting for init event")
	}

	// filter out all events that could have been inserted
	// by the initialization routines
skiploop:
	for {
		select {
		case event := <-w.Events():
			log.Debugf("Skipping pre-test event: %v", event)
			continue skiploop
		default:
			break skiploop
		case <-w.Done():
			c.Fatalf("Watcher exited with error %v", w.Error())
		}
	}

	for _, tc := range testCases {
		c.Logf("test case %q", tc.Name)
		resource := tc.CRUD()

		test.ExpectResource(c, w, 3*time.Second, resource)

		meta := resource.GetMetadata()
		header := &services.ResourceHeader{
			Kind:    resource.GetKind(),
			SubKind: resource.GetSubKind(),
			Version: resource.GetVersion(),
			Metadata: services.Metadata{
				Name:      meta.Name,
				Namespace: meta.Namespace,
			},
		}
		// delete events don't have IDs yet
		header.SetResourceID(0)
		test.ExpectDeleteResource(c, w, 3*time.Second, header)
	}
}

type EventTest struct {
	Name string
	Kind services.WatchKind
	CRUD func() services.Resource
}

func eventsTestKinds(tests []EventTest) []services.WatchKind {
	out := make([]services.WatchKind, len(tests))
	for i, tc := range tests {
		out[i] = tc.Kind
	}
	return out
}

func newUser(name string, roles ...string) types.User {
	return &types.UserV2{
		Kind:    types.KindUser,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: types.UserSpecV2{
			Roles: roles,
		},
	}
}
