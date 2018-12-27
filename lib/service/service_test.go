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

	"strconv"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
)

type ServiceTestSuite struct {
}

var _ = fmt.Printf
var _ = check.Suite(&ServiceTestSuite{})

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

	ports, err := utils.GetFreeTCPPorts(2)
	c.Assert(err, check.IsNil)
	authPort, err := strconv.Atoi(ports.Pop())
	c.Assert(err, check.IsNil)
	authAddr, err := utils.ParseHostPortAddr("127.0.0.1", authPort)
	c.Assert(err, check.IsNil)
	diagPort, err := strconv.Atoi(ports.Pop())
	c.Assert(err, check.IsNil)
	diagAddr, err := utils.ParseHostPortAddr("127.0.0.1", diagPort)
	c.Assert(err, check.IsNil)

	endpoint := fmt.Sprintf("http://%v/readyz", diagAddr.String())

	cfg := MakeDefaultConfig()
	cfg.Clock = fakeClock
	cfg.DataDir = c.MkDir()
	cfg.DiagnosticAddr = *diagAddr
	cfg.AuthServers = []utils.NetAddr{*authAddr}
	cfg.Auth.Enabled = true
	cfg.Auth.StorageConfig.Params["path"] = c.MkDir()
	cfg.Proxy.Enabled = false
	cfg.SSH.Enabled = false

	process, err := NewTeleport(cfg)
	c.Assert(err, check.IsNil)

	// Start Teleport and make sure the status is OK.
	go process.Run()
	err = waitForStatus(endpoint, http.StatusOK)
	c.Assert(err, check.IsNil)

	// Broadcast a degraded event and make sure Teleport reports it's in a
	// degraded state.
	process.BroadcastEvent(Event{Name: TeleportDegradedEvent, Payload: nil})
	err = waitForStatus(endpoint, http.StatusServiceUnavailable)
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

func waitForStatus(diagAddr string, statusCode int) error {
	tickCh := time.Tick(250 * time.Millisecond)
	timeoutCh := time.After(10 * time.Second)
	for {
		select {
		case <-tickCh:
			resp, err := http.Get(diagAddr)
			if err != nil {
				return trace.Wrap(err)
			}
			if resp.StatusCode == statusCode {
				return nil
			}
		case <-timeoutCh:
			return trace.BadParameter("timeout waiting for status %v", statusCode)
		}
	}
}
