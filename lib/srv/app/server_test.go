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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

type Suite struct {
	clock        clockwork.FakeClock
	dataDir      string
	authServer   *auth.TestAuthServer
	tlsServer    *auth.TestTLSServer
	authClient   *auth.Client
	appServer    *Server
	server       types.Server
	hostCertPool *x509.CertPool

	hostUUID          string
	closeContext      context.Context
	closeFunc         context.CancelFunc
	message           string
	hostport          string
	testhttp          *httptest.Server
	clientCertificate tls.Certificate

	user types.User
	role types.Role
}

var _ = check.Suite(&Suite{})

func TestApp(t *testing.T) { check.TestingT(t) }

func (s *Suite) SetUpSuite(c *check.C) {
	s.clock = clockwork.NewFakeClock()
	s.dataDir = c.MkDir()

	var err error
	// Create Auth Server.
	s.authServer, err = auth.NewTestAuthServer(auth.TestAuthServerConfig{
		ClusterName: "root.example.com",
		Dir:         s.dataDir,
		Clock:       s.clock,
	})
	c.Assert(err, check.IsNil)
	s.tlsServer, err = s.authServer.NewTestTLSServer()
	c.Assert(err, check.IsNil)

	// Create user and role.
	s.user, s.role, err = auth.CreateUserAndRole(s.tlsServer.Auth(), "foo", []string{"foo-login"})
	c.Assert(err, check.IsNil)

	// Grant the user's role access to the application label "bar: baz".
	s.role.SetAppLabels(services.Allow, types.Labels{"bar": []string{"baz"}})
	err = s.tlsServer.Auth().UpsertRole(context.Background(), s.role)
	c.Assert(err, check.IsNil)

	rootCA, err := s.tlsServer.Auth().GetCertAuthority(types.CertAuthID{
		Type:       types.HostCA,
		DomainName: "root.example.com",
	}, false)
	c.Assert(err, check.IsNil)
	s.hostCertPool, err = services.CertPool(rootCA)
	c.Assert(err, check.IsNil)
}

func (s *Suite) TearDownSuite(c *check.C) {
	err := s.tlsServer.Close()
	c.Assert(err, check.IsNil)
}

func (s *Suite) SetUpTest(c *check.C) {
	s.closeContext, s.closeFunc = context.WithCancel(context.Background())

	// Create a in-memory HTTP server that will respond with a UUID. This value
	// will be checked in the client later to ensure a connection was made.
	s.message = uuid.New()

	s.testhttp = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, s.message)
		s.closeFunc()
	}))
	s.testhttp.Config.TLSConfig = &tls.Config{Time: s.clock.Now}
	s.testhttp.Start()

	// Extract the hostport that the in-memory HTTP server is running on.
	u, err := url.Parse(s.testhttp.URL)
	c.Assert(err, check.IsNil)
	s.hostport = u.Host

	// Create a services.App that will be used for each test.
	staticLabels := map[string]string{
		"bar": "baz",
	}
	dynamicLabels := map[string]types.CommandLabel{
		"qux": &types.CommandLabelV2{
			Period:  types.NewDuration(time.Second),
			Command: []string{"expr", "1", "+", "3"},
		},
	}
	s.hostUUID = uuid.New()
	s.server = &types.ServerV2{
		Kind:    types.KindAppServer,
		Version: types.V2,
		Metadata: types.Metadata{
			Namespace: defaults.Namespace,
			Name:      s.hostUUID,
		},
		Spec: types.ServerSpecV2{
			Version: teleport.Version,
			Apps: []*types.App{
				{
					Name:          "foo",
					URI:           s.testhttp.URL,
					PublicAddr:    "foo.example.com",
					StaticLabels:  staticLabels,
					DynamicLabels: types.LabelsToV2(dynamicLabels),
				},
			},
		},
	}

	// Create a client with a machine role of RoleApp.
	s.authClient, err = s.tlsServer.NewClient(auth.TestServerID(teleport.RoleApp, s.hostUUID))
	c.Assert(err, check.IsNil)

	serverIdentity, err := auth.NewServerIdentity(s.authServer.AuthServer, s.hostUUID, teleport.RoleApp)
	c.Assert(err, check.IsNil)
	tlsConfig, err := serverIdentity.TLSConfig(nil)
	c.Assert(err, check.IsNil)
	tlsConfig.Time = s.clock.Now

	// Generate certificate for user.
	privateKey, publicKey, err := s.tlsServer.Auth().GenerateKeyPair("")
	c.Assert(err, check.IsNil)
	certificate, err := s.tlsServer.Auth().GenerateUserAppTestCert(auth.AppTestCertRequest{
		PublicKey:   publicKey,
		Username:    s.user.GetName(),
		TTL:         1 * time.Hour,
		PublicAddr:  "foo.example.com",
		ClusterName: "root.example.com",
	})
	c.Assert(err, check.IsNil)
	s.clientCertificate, err = tls.X509KeyPair(certificate, privateKey)
	c.Assert(err, check.IsNil)

	// Make sure the upload directory is created.
	err = os.MkdirAll(filepath.Join(
		s.dataDir, teleport.LogsDir, teleport.ComponentUpload,
		events.StreamingLogsDir, defaults.Namespace,
	), 0755)
	c.Assert(err, check.IsNil)

	authorizer, err := auth.NewAuthorizer("cluster-name", s.authClient, s.authClient, s.authClient)
	c.Assert(err, check.IsNil)

	s.appServer, err = New(context.Background(), &Config{
		Clock:        s.clock,
		DataDir:      s.dataDir,
		AccessPoint:  s.authClient,
		AuthClient:   s.authClient,
		TLSConfig:    tlsConfig,
		CipherSuites: utils.DefaultCipherSuites(),
		Authorizer:   authorizer,
		GetRotation:  testRotationGetter,
		Server:       s.server,
		OnHeartbeat:  func(err error) {},
	})
	c.Assert(err, check.IsNil)

	s.appServer.Start()
	err = s.appServer.ForceHeartbeat()
	c.Assert(err, check.IsNil)
}

func (s *Suite) TearDownTest(c *check.C) {
	err := s.authClient.Close()
	c.Assert(err, check.IsNil)

	err = s.appServer.Close()
	c.Assert(err, check.IsNil)

	s.testhttp.Close()

	err = s.tlsServer.Auth().DeleteAllAppServers(context.Background(), defaults.Namespace)
	c.Assert(err, check.IsNil)
}

// TestStart makes sure that after the server has started, a correct services.App
// has been created.
func (s *Suite) TestStart(c *check.C) {
	// Fetch the services.App that the service heartbeat.
	servers, err := s.authServer.AuthServer.GetAppServers(context.Background(), defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(servers, check.HasLen, 1)
	server := servers[0]

	// Check that the services.Server sent via heartbeat is correct. For example,
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

// TestHandleConnection verifies that requests with valid certificates are forwarded.
func (s *Suite) TestHandleConnection(c *check.C) {
	pr, pw := net.Pipe()
	defer pw.Close()
	defer pr.Close()

	// Create an HTTP client authenticated with the user's credentials. This acts
	// like the proxy does.
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return pr, nil
			},
			TLSClientConfig: &tls.Config{
				// RootCAs is a pool of host certificates used to verify the identity of
				// the server this client is connecting to.
				RootCAs: s.hostCertPool,
				// Certificates is the user's application specific certificate.
				Certificates: []tls.Certificate{s.clientCertificate},
				// Time defines the time anchor for certificate validation
				Time: s.clock.Now,
			},
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)

	// Handle the connection in another goroutine.
	go func() {
		s.appServer.HandleConnection(pw)
		wg.Done()
	}()

	// Issue request.
	resp, err := httpClient.Get("https://" + teleport.APIDomain)
	c.Assert(err, check.IsNil)

	// Check response.
	buf, err := ioutil.ReadAll(resp.Body)
	c.Assert(err, check.IsNil)
	err = resp.Body.Close()
	c.Assert(err, check.IsNil)
	c.Assert(strings.TrimSpace(string(buf)), check.Equals, s.message)

	// Context will close because of the net.Pipe, expect a context canceled
	// error here.
	err = s.appServer.Close()
	c.Assert(err, check.NotNil)

	// Wait for the application server to actually stop serving before
	// closing the test. This will make sure the server removes the listeners
	wg.Wait()
}

// TestAuthorize verifies that only authorized requests are handled.
func (s *Suite) TestAuthorize(c *check.C) {
}

// TestGetConfigForClient verifies that only the CAs of the requested cluster are returned.
func (s *Suite) TestGetConfigForClient(c *check.C) {
}

// TestRewriteRequest verifies that requests are rewritten to include JWT headers.
func (s *Suite) TestRewriteRequest(c *check.C) {
}

// TestRewriteResponse verifies that responses are rewritten if rewrite rules are specified.
func (s *Suite) TestRewriteResponse(c *check.C) {
}

// TestSessionClose makes sure sessions are closed after the given session time period.
func (s *Suite) TestSessionClose(c *check.C) {
}

func testRotationGetter(role teleport.Role) (*types.Rotation, error) {
	return &types.Rotation{}, nil
}
