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
	app        services.Server

	closeContext context.Context
	closeFunc    context.CancelFunc
	message      string
	hostport     string
	testhttp     *httptest.Server
}

var _ = check.Suite(&Suite{})

func TestApp(t *testing.T) { check.TestingT(t) }

func (s *Suite) SetUpSuite(c *check.C) {
	var err error

	utils.InitLoggerForTests(testing.Verbose())

	s.clock = clockwork.NewFakeClockAt(time.Now())

	// Create Auth Server.
	s.authServer, err = auth.NewTestAuthServer(auth.TestAuthServerConfig{
		ClusterName: "localhost",
		Dir:         c.MkDir(),
	})
	c.Assert(err, check.IsNil)
	s.tlsServer, err = s.authServer.NewTestTLSServer()
	c.Assert(err, check.IsNil)

	// Create a client with a machine role of RoleApp.
	s.authClient, err = s.tlsServer.NewClient(auth.TestBuiltin(teleport.RoleApp))
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
	s.app = &services.ServerV2{
		Kind:    services.KindApp,
		Version: services.V2,
		Metadata: services.Metadata{
			Namespace: defaults.Namespace,
			Name:      "foo",
			Labels:    staticLabels,
		},
		Spec: services.ServerSpecV2{
			Protocol:     services.ServerSpecV2_HTTPS,
			InternalAddr: s.hostport,
			PublicAddr:   "foo.example.com",
			CmdLabels:    services.LabelsToV2(dynamicLabels),
			Version:      teleport.Version,
		},
	}

	s.appServer, err = New(context.Background(), &Config{
		Clock:       s.clock,
		AccessPoint: s.authClient,
		GetRotation: testRotationGetter,
		App:         s.app,
	})
	c.Assert(err, check.IsNil)

	s.appServer.Start()
	err = s.appServer.heartbeat.ForceSend(time.Second)
	c.Assert(err, check.IsNil)
}

func (s *Suite) TearDownTest(c *check.C) {
	err := s.appServer.Close()
	c.Assert(err, check.IsNil)

	s.testhttp.Close()
}

// TestStart makes sure after the server has started a correct services.App
// has been created.
func (s *Suite) TestStart(c *check.C) {
	// Fetch the services.App that the service heartbeat.
	apps, err := s.authServer.AuthServer.GetApps(context.Background(), defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(apps, check.HasLen, 1)
	app := apps[0]

	// Check that the services.App that was heartbeat is correct. For example,
	// check that the dynamic labels have been evaluated.
	c.Assert(app.GetName(), check.Equals, "foo")
	c.Assert(app.GetInternalAddr(), check.Equals, s.hostport)
	c.Assert(app.GetPublicAddr(), check.Equals, "foo.example.com")
	c.Assert(app.GetLabels(), check.DeepEquals, map[string]string{
		"bar": "baz",
	})
	dynamicLabel, ok := app.GetCmdLabels()["qux"]
	c.Assert(ok, check.Equals, true)
	c.Assert(dynamicLabel.GetResult(), check.Equals, "4")

	// Check the expiry time is correct.
	c.Assert(s.clock.Now().Before(app.Expiry()), check.Equals, true)
	c.Assert(s.clock.Now().Add(2*defaults.ServerAnnounceTTL).After(app.Expiry()), check.Equals, true)
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

// TestHandleConnection makes sure the application server forwards
// connections to the target host and accurately keeps track of connections.
func (s *Suite) TestHandleConnection(c *check.C) {
	// Create a net.Pipe. The "server" end will be passed to the application
	// server while the client end will be used by the http.Client to read/write
	// a request.
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	// Process the connection.
	go s.appServer.HandleConnection(serverConn)

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

func testRotationGetter(role teleport.Role) (*services.Rotation, error) {
	return &services.Rotation{}, nil
}
