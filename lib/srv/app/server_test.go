/*
Copyright 2020 Gravitational, Inc.

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

package app

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"

	"gopkg.in/check.v1"
)

type Suite struct {
	clock      clockwork.Clock
	authServer *auth.TestAuthServer
	tlsServer  *auth.TestTLSServer
	authClient *auth.Client
	appServer  *Server
	server     services.Server

	hostUUID     string
	closeContext context.Context
	closeFunc    context.CancelFunc
	message      string
	hostport     string
	testhttp     *httptest.Server
	cert         []byte
}

var _ = check.Suite(&Suite{})

func TestApp(t *testing.T) { check.TestingT(t) }

func (s *Suite) SetUpSuite(c *check.C) {
	var err error

	utils.InitLoggerForTests(testing.Verbose())

	s.clock = clockwork.NewFakeClockAt(time.Now())

	// Create Auth Server.
	s.authServer, err = auth.NewTestAuthServer(auth.TestAuthServerConfig{
		ClusterName: "root.example.com",
		Dir:         c.MkDir(),
	})
	c.Assert(err, check.IsNil)
	s.tlsServer, err = s.authServer.NewTestTLSServer()
	c.Assert(err, check.IsNil)

	// Create a client with a machine role of RoleApp.
	s.authClient, err = s.tlsServer.NewClient(auth.TestBuiltin(teleport.RoleApp))
	c.Assert(err, check.IsNil)

	// Create user and role.
	user, role, err := auth.CreateUserAndRole(s.tlsServer.Auth(), "foo", []string{"foo-login"})
	c.Assert(err, check.IsNil)

	// Give the users role application label "bar: baz".
	role.SetAppLabels(services.Allow, services.Labels{"bar": []string{"baz"}})
	err = s.tlsServer.Auth().UpsertRole(context.Background(), role)
	c.Assert(err, check.IsNil)

	// Generate certificate for user.
	_, public, err := s.tlsServer.Auth().GenerateKeyPair("")
	c.Assert(err, check.IsNil)
	_, s.cert, err = s.tlsServer.Auth().GenerateUserTestCerts(public, user.GetName(), 1*time.Hour, teleport.CertificateFormatStandard, "")
	c.Assert(err, check.IsNil)
}

func (s *Suite) TearDownSuite(c *check.C) {
	err := s.authClient.Close()
	c.Assert(err, check.IsNil)

	err = s.tlsServer.Close()
	c.Assert(err, check.IsNil)
}

func (s *Suite) SetUpTest(c *check.C) {
	s.closeContext, s.closeFunc = context.WithCancel(context.Background())

	// Create a in-memory HTTP server that will respond with a UUID. This value
	// will be checked in the client later to ensure a connection was made.
	s.message = uuid.New()
	s.testhttp = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, s.message)
		s.closeFunc()
	}))

	// Extract the hostport that the in-memory HTTP server is running on.
	u, err := url.Parse(s.testhttp.URL)
	c.Assert(err, check.IsNil)
	s.hostport = u.Host

	// Create a services.App that will be used for each tests.
	staticLabels := map[string]string{
		"bar": "baz",
	}
	dynamicLabels := map[string]services.CommandLabel{
		"qux": &services.CommandLabelV2{
			Period:  services.NewDuration(time.Second),
			Command: []string{"expr", "1", "+", "3"},
		},
	}
	s.hostUUID = uuid.New()
	s.server = &services.ServerV2{
		Kind:    services.KindApp,
		Version: services.V2,
		Metadata: services.Metadata{
			Namespace: defaults.Namespace,
			Name:      s.hostUUID,
		},
		Spec: services.ServerSpecV2{
			Protocol: services.ServerSpecV2_HTTPS,
			Version:  teleport.Version,
			Apps: []*services.App{
				&services.App{
					Name:          "foo",
					URI:           s.testhttp.URL,
					PublicAddr:    "foo.example.com",
					StaticLabels:  staticLabels,
					DynamicLabels: services.LabelsToV2(dynamicLabels),
				},
			},
		},
	}

	s.appServer, err = New(context.Background(), &Config{
		Clock:       s.clock,
		AccessPoint: s.authClient,
		GetRotation: testRotationGetter,
		Server:      s.server,
	})
	c.Assert(err, check.IsNil)

	s.appServer.Start()
	err = s.appServer.ForceHeartbeat()
	c.Assert(err, check.IsNil)
}

func (s *Suite) TearDownTest(c *check.C) {
	err := s.appServer.Close()
	c.Assert(err, check.IsNil)

	s.testhttp.Close()

	err = s.tlsServer.Auth().DeleteAllApps(context.Background(), defaults.Namespace)
	c.Assert(err, check.IsNil)
}

// TestStart makes sure after the server has started a correct services.App
// has been created.
func (s *Suite) TestStart(c *check.C) {
	// Fetch the services.App that the service heartbeat.
	servers, err := s.authServer.AuthServer.GetApps(context.Background(), defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(servers, check.HasLen, 1)
	server := servers[0]

	// Check that the services.Server that was heartbeat is correct. For example,
	// check that the dynamic labels have been evaluated.
	c.Assert(server.GetApps(), check.HasLen, 1)
	app := server.GetApps()[0]

	c.Assert(app.Name, check.Equals, "foo")
	c.Assert(app.URI, check.Equals, s.testhttp.URL)
	c.Assert(app.PublicAddr, check.Equals, "foo.example.com")
	c.Assert(app.StaticLabels, check.DeepEquals, map[string]string{
		"bar": "baz",
	})
	dynamicLabel, ok := app.DynamicLabels["qux"]
	c.Assert(ok, check.Equals, true)
	c.Assert(dynamicLabel.GetResult(), check.Equals, "4")

	// Check the expiry time is correct.
	c.Assert(s.clock.Now().Before(server.Expiry()), check.Equals, true)
	c.Assert(s.clock.Now().Add(2*defaults.ServerAnnounceTTL).After(server.Expiry()), check.Equals, true)
}

// TestWaitStop makes sure the server will block and unlock.
func (s *Suite) TestWaitStop(c *check.C) {
	// Make sure that wait will block while the server is running.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		s.appServer.Wait()
		cancel()
	}()
	select {
	case <-ctx.Done():
		c.Fatalf("Wait failed to block.")
	case <-time.After(250 * time.Millisecond):
	}

	// Close should unblock wait.
	err := s.appServer.Close()
	c.Assert(err, check.IsNil)
	err = s.appServer.Wait()
	c.Assert(err, check.Equals, context.Canceled)
}

// TestForwardConnection makes sure the application server forwards
// connections to the target host and accurately keeps track of connections.
func (s *Suite) TestForwardConnection(c *check.C) {
	// Create a net.Pipe. The "server" end will be passed to the application
	// server while the client end will be used by the http.Client to read/write
	// a request.
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	// Process the connection.
	go s.appServer.ForwardConnection(serverConn, s.testhttp.URL)

	// Perform a simple HTTP GET against the application server.
	httpTransport := &http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			return clientConn, nil
		},
	}
	httpClient := http.Client{
		Transport: httpTransport,
	}
	resp, err := httpClient.Get("http://foo.example.com")
	c.Assert(err, check.IsNil)
	defer resp.Body.Close()

	// Make sure the response contains the UUID that the server replied with earlier.
	body, err := ioutil.ReadAll(resp.Body)
	c.Assert(err, check.IsNil)
	c.Assert(strings.TrimSpace(string(body)), check.Equals, s.message)

	// Application server is proxying a single connection, count should be 1.
	c.Assert(s.appServer.activeConnections(), check.Equals, int64(1))

	// Break connection from client side.
	clientConn.Close()

	// Wait a few seconds for the count to go down to 0.
	ticker := time.Tick(50 * time.Millisecond)
	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-ticker:
			if s.appServer.activeConnections() == 0 {
				return
			}
		case <-timeout:
			c.Fatalf("Timed out waiting for active connection count to come to 0.")
		}
	}
}

// TestVerifyCertificate checks that certificates not signed by a known CA are rejected.
func (s *Suite) TestVerifyCertificate(c *check.C) {
	// Create another independent cluster acme.example.com.
	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		ClusterName: "acme.example.com",
		Dir:         c.MkDir(),
	})
	c.Assert(err, check.IsNil)
	tlsServer, err := authServer.NewTestTLSServer()
	c.Assert(err, check.IsNil)
	defer tlsServer.Close()

	// Create user and role in acme.example.com.
	user, _, err := auth.CreateUserAndRole(tlsServer.Auth(), "foo", []string{"foo-login"})
	c.Assert(err, check.IsNil)

	// Generate certificate for user in acme.example.com
	_, public, err := s.tlsServer.Auth().GenerateKeyPair("")
	c.Assert(err, check.IsNil)
	_, cert, err := tlsServer.Auth().GenerateUserTestCerts(public, user.GetName(), 1*time.Hour, teleport.CertificateFormatStandard, "")
	c.Assert(err, check.IsNil)

	var tests = []struct {
		desc          string
		inCertificate []byte
		outError      bool
	}{
		{
			desc:          "invalid cert, issued to different cluster",
			inCertificate: cert,
			outError:      true,
		},
		{
			desc:          "valid cert, issued to same cluster",
			inCertificate: s.cert,
			outError:      false,
		},
	}
	for _, tt := range tests {
		identity, _, err := s.appServer.verifyCertificate(tt.inCertificate)
		c.Assert(err != nil, check.Equals, tt.outError, check.Commentf(tt.desc))
		if tt.outError == true {
			continue
		}
		c.Assert(identity.Username, check.Equals, "foo", check.Commentf(tt.desc))
		c.Assert(identity.Groups, check.DeepEquals, []string{"user:foo"}, check.Commentf(tt.desc))
	}
}

// TestCheckAccess verifies that the "CheckAccess" function correctly builds
// and applies a services.AccessChecker when attempting to access an application.
func (s *Suite) TestCheckAccess(c *check.C) {
	// Create an application with label "bar: baz" and an application with
	// "qux: quxx". The user created by the suite has role with label "bar: baz"
	// so only "app-001" should be available.
	server := &services.ServerV2{
		Kind:    services.KindApp,
		Version: services.V2,
		Metadata: services.Metadata{
			Namespace: defaults.Namespace,
			Name:      uuid.New(),
		},
		Spec: services.ServerSpecV2{
			Hostname: "server-001",
			Protocol: services.ServerSpecV2_HTTPS,
			Version:  teleport.Version,
			Apps: []*services.App{
				&services.App{
					Name:       "app-001",
					URI:        "http://127.0.0.1:8080",
					PublicAddr: "app-001.example.com",
					StaticLabels: map[string]string{
						"bar": "baz",
					},
				},
				&services.App{
					Name:       "app-002",
					URI:        "http://127.0.0.1:8080",
					PublicAddr: "app-002.example.com",
					StaticLabels: map[string]string{
						"qux": "quxx",
					},
				},
			},
		},
	}
	_, err := s.tlsServer.Auth().UpsertApp(context.Background(), server)
	c.Assert(err, check.IsNil)

	var tests = []struct {
		desc          string
		inCertificate []byte
		inAddress     string
		outError      bool
	}{
		{
			desc:          "allowed access, label match",
			inCertificate: s.cert,
			inAddress:     "app-001.example.com",
			outError:      false,
		},
		{
			desc:          "denied access, labels do not match",
			inCertificate: s.cert,
			inAddress:     "app-002.example.com",
			outError:      true,
		},
	}
	for _, tt := range tests {
		_, err := s.appServer.CheckAccess(context.Background(), tt.inCertificate, tt.inAddress)
		c.Assert(err != nil, check.Equals, tt.outError, check.Commentf(tt.desc))
	}
}

// TestCheckAccessTrustedCluster verifies that the "CheckAccess" function
// correctly maps a role then builds and applies a services.AccessChecker
// that the access checker to the mapped role.
func (s *Suite) TestCheckAccessTrustedCluster(c *check.C) {
	// Create leaf cluster.
	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		ClusterName: "leaf.example.com",
		Dir:         c.MkDir(),
	})
	c.Assert(err, check.IsNil)
	tlsServer, err := authServer.NewTestTLSServer()
	c.Assert(err, check.IsNil)
	defer tlsServer.Close()

	// Create a client with a machine role of RoleApp.
	authClient, err := tlsServer.NewClient(auth.TestBuiltin(teleport.RoleApp))
	c.Assert(err, check.IsNil)
	defer authClient.Close()

	// Create user and role in leaf cluster.
	_, role, err := auth.CreateUserAndRole(tlsServer.Auth(), "bar", []string{"bar-login"})
	c.Assert(err, check.IsNil)

	// Give the role application label "qux: quxx".
	role.SetAppLabels(services.Allow, services.Labels{"qux": []string{"quxx"}})
	err = tlsServer.Auth().UpsertRole(context.Background(), role)
	c.Assert(err, check.IsNil)

	// Fetch root CA and update role mapping in to map "user: foo" role to
	// "user: bar" role.
	rootCA, err := s.tlsServer.Auth().GetCertAuthority(services.CertAuthID{
		Type:       services.UserCA,
		DomainName: "root.example.com",
	}, false)
	c.Assert(err, check.IsNil)
	rootCA.SetRoleMap(services.RoleMap{
		services.RoleMapping{
			Remote: "user:foo",
			Local:  []string{"user:bar"},
		},
	})

	// Add both CA into leaf backend, this should effectively create a trusted
	// cluster relationship.
	err = tlsServer.Auth().UpsertCertAuthority(rootCA)
	c.Assert(err, check.IsNil)

	// Create an application with label "bar: baz" and an application with
	// "qux: quxx". The user created by the suite will map to a user with role
	// with label "qux: quxx" so only "app-002" should be available.
	server := &services.ServerV2{
		Kind:    services.KindApp,
		Version: services.V2,
		Metadata: services.Metadata{
			Namespace: defaults.Namespace,
			Name:      uuid.New(),
		},
		Spec: services.ServerSpecV2{
			Hostname: "server-001",
			Protocol: services.ServerSpecV2_HTTPS,
			Version:  teleport.Version,
			Apps: []*services.App{
				&services.App{
					Name:       "app-001",
					URI:        "http://127.0.0.1:8080",
					PublicAddr: "app-001.example.com",
					StaticLabels: map[string]string{
						"bar": "baz",
					},
				},
				&services.App{
					Name:       "app-002",
					URI:        "http://127.0.0.1:8080",
					PublicAddr: "app-002.example.com",
					StaticLabels: map[string]string{
						"qux": "quxx",
					},
				},
			},
		},
	}

	// Create the handler in the leaf cluster.
	appServer, err := New(context.Background(), &Config{
		Clock:       s.clock,
		AccessPoint: authClient,
		GetRotation: testRotationGetter,
		Server:      server,
	})
	c.Assert(err, check.IsNil)
	appServer.Start()
	err = appServer.ForceHeartbeat()
	c.Assert(err, check.IsNil)
	defer appServer.Close()

	var tests = []struct {
		desc          string
		inCertificate []byte
		inAddress     string
		outError      bool
	}{
		{
			desc:          "denied access, mapped to role where labels do not match",
			inCertificate: s.cert,
			inAddress:     "app-001.example.com",
			outError:      true,
		},
		{
			desc:          "allowed access, mapped to role where label match",
			inCertificate: s.cert,
			inAddress:     "app-002.example.com",
			outError:      false,
		},
	}
	for _, tt := range tests {
		_, err := appServer.CheckAccess(context.Background(), tt.inCertificate, tt.inAddress)
		c.Assert(err != nil, check.Equals, tt.outError, check.Commentf(tt.desc))
	}
}

func testRotationGetter(role teleport.Role) (*services.Rotation, error) {
	return &services.Rotation{}, nil
}
