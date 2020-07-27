/*
Copyright 2015 Gravitational, Inc.

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
package service

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
)

type ServiceTestSuite struct {
}

var _ = fmt.Printf
var _ = check.Suite(&ServiceTestSuite{})
var _ = testing.Verbose

func (s *ServiceTestSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests(testing.Verbose())
}

func (s *ServiceTestSuite) TestSelfSignedHTTPS(c *check.C) {
	fileExists := func(fp string) bool {
		_, err := os.Stat(fp)
		if err != nil && os.IsNotExist(err) {
			return false
		}
		return true
	}
	cfg := &Config{
		DataDir:  c.MkDir(),
		Hostname: "example.com",
	}
	err := initSelfSignedHTTPSCert(cfg)
	c.Assert(err, check.IsNil)
	c.Assert(fileExists(cfg.Proxy.TLSCert), check.Equals, true)
	c.Assert(fileExists(cfg.Proxy.TLSKey), check.Equals, true)
}

func (s *ServiceTestSuite) TestMonitor(c *check.C) {
	fakeClock := clockwork.NewFakeClock()

	cfg := MakeDefaultConfig()
	cfg.Clock = fakeClock
	cfg.DataDir = c.MkDir()
	cfg.DiagnosticAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}
	cfg.AuthServers = []utils.NetAddr{{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}}
	cfg.Auth.Enabled = true
	cfg.Auth.StorageConfig.Params["path"] = c.MkDir()
	cfg.Auth.SSHAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}
	cfg.Proxy.Enabled = false
	cfg.SSH.Enabled = false

	process, err := NewTeleport(cfg)
	c.Assert(err, check.IsNil)

	diagAddr, err := process.DiagnosticAddr()
	c.Assert(err, check.IsNil)
	c.Assert(diagAddr, check.NotNil)
	endpoint := fmt.Sprintf("http://%v/readyz", diagAddr.String())

	// Start Teleport and make sure the status is OK.
	go func() {
		c.Assert(process.Run(), check.IsNil)
	}()
	err = waitForStatus(endpoint, http.StatusOK)
	c.Assert(err, check.IsNil)

	// Broadcast a degraded event and make sure Teleport reports it's in a
	// degraded state.
	process.BroadcastEvent(Event{Name: TeleportDegradedEvent, Payload: nil})
	err = waitForStatus(endpoint, http.StatusServiceUnavailable, http.StatusBadRequest)
	c.Assert(err, check.IsNil)

	// Broadcast a OK event, this should put Teleport into a recovering state.
	process.BroadcastEvent(Event{Name: TeleportOKEvent, Payload: nil})
	err = waitForStatus(endpoint, http.StatusBadRequest)
	c.Assert(err, check.IsNil)

	// Broadcast another OK event, Teleport should still be in recovering state
	// because not enough time has passed.
	process.BroadcastEvent(Event{Name: TeleportOKEvent, Payload: nil})
	err = waitForStatus(endpoint, http.StatusBadRequest)
	c.Assert(err, check.IsNil)

	// Advance time past the recovery time and then send another OK event, this
	// should put Teleport into a OK state.
	fakeClock.Advance(defaults.ServerKeepAliveTTL*2 + 1)
	process.BroadcastEvent(Event{Name: TeleportOKEvent, Payload: nil})
	err = waitForStatus(endpoint, http.StatusOK)
	c.Assert(err, check.IsNil)
}

// TestCheckPrincipals checks certificates regeneration only requests
// regeneration when the principals change.
func (s *ServiceTestSuite) TestCheckPrincipals(c *check.C) {
	dataDir := c.MkDir()

	// Create a test auth server to extract the server identity (SSH and TLS
	// certificates).
	testAuthServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir: dataDir,
	})
	c.Assert(err, check.IsNil)
	tlsServer, err := testAuthServer.NewTestTLSServer()
	c.Assert(err, check.IsNil)
	defer tlsServer.Close()

	testConnector := &Connector{
		ServerIdentity: tlsServer.Identity,
	}

	var tests = []struct {
		inPrincipals  []string
		inDNS         []string
		outRegenerate bool
	}{
		// If nothing has been updated, don't regenerate certificate.
		{
			inPrincipals:  []string{},
			inDNS:         []string{},
			outRegenerate: false,
		},
		// Don't regenerate certificate if the node does not know it's own address.
		{
			inPrincipals:  []string{"0.0.0.0"},
			inDNS:         []string{},
			outRegenerate: false,
		},
		// If a new SSH principal is found, regenerate certificate.
		{
			inPrincipals:  []string{"1.1.1.1"},
			inDNS:         []string{},
			outRegenerate: true,
		},
		// If a new TLS DNS name is found, regenerate certificate.
		{
			inPrincipals:  []string{},
			inDNS:         []string{"server.example.com"},
			outRegenerate: true,
		},
		// Don't regenerate certificate if additional principals is already on the
		// certificate.
		{
			inPrincipals:  []string{"test-tls-server"},
			inDNS:         []string{},
			outRegenerate: false,
		},
	}
	for _, tt := range tests {
		ok := checkServerIdentity(testConnector, tt.inPrincipals, tt.inDNS)
		c.Assert(ok, check.Equals, tt.outRegenerate)
	}
}

// TestInitExternalLog verifies that external logging can be used both as a means of
// overriding the local audit event target.  Ideally, this test would also verify
// setup of true external loggers, but at the time of writing there isn't good
// support for setting up fake external logging endpoints.
func (s *ServiceTestSuite) TestInitExternalLog(c *check.C) {
	tts := []struct {
		events []string
		isNil  bool
		isErr  bool
	}{
		// no URIs => no external logger
		{isNil: true},
		// local-only event uri w/o hostname => ok
		{events: []string{"file:///tmp/teleport-test/events"}},
		// local-only event uri w/ localhost => ok
		{events: []string{"file://localhost/tmp/teleport-test/events"}},
		// invalid host parameter => rejected
		{events: []string{"file://example.com/should/fail"}, isErr: true},
		// missing path specifier => rejected
		{events: []string{"file://localhost"}, isErr: true},
	}

	for i, tt := range tts {
		// isErr implies isNil.
		if tt.isErr {
			tt.isNil = true
		}

		cmt := check.Commentf("tt[%v]: %+v", i, tt)

		loggers, err := initExternalLog(services.AuditConfig{
			AuditEventsURI: tt.events,
		})

		if tt.isErr {
			c.Assert(err, check.NotNil, cmt)
		} else {
			c.Assert(err, check.IsNil, cmt)
		}

		if tt.isNil {
			c.Assert(loggers, check.IsNil, cmt)
		} else {
			c.Assert(loggers, check.NotNil, cmt)
		}
	}
}

func waitForStatus(diagAddr string, statusCodes ...int) error {
	tickCh := time.Tick(250 * time.Millisecond)
	timeoutCh := time.After(10 * time.Second)
	for {
		select {
		case <-tickCh:
			resp, err := http.Get(diagAddr)
			if err != nil {
				return trace.Wrap(err)
			}
			resp.Body.Close()
			for _, statusCode := range statusCodes {
				if resp.StatusCode == statusCode {
					return nil
				}
			}
		case <-timeoutCh:
			return trace.BadParameter("timeout waiting for status %v", statusCodes)
		}
	}
}
