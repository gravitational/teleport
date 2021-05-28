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

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/regular"
	"github.com/siddontang/go-log/log"
	"gopkg.in/check.v1"
)

func extractPort(svr *httptest.Server) (int, error) {
	u, err := url.Parse(svr.URL)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(u.Port())
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *IntSuite) TestPortForwarding(c *check.C) {
	s.setUpTest(c)
	defer s.tearDownTest(c)

	testCases := []struct {
		desc          string
		mode          regular.SSHPortForwardingMode
		expectSuccess bool
	}{
		{
			desc:          "Enabled (All)",
			mode:          regular.SSHPortForwardingModeAll,
			expectSuccess: true,
		}, {
			desc:          "Enabled (Local)",
			mode:          regular.SSHPortForwardingModeLocal,
			expectSuccess: true,
		}, {
			desc:          "Disabled",
			mode:          regular.SSHPortForwardingModeNone,
			expectSuccess: false,
		},
	}

	for _, tt := range testCases {
		doTest := func() {
			// Given a running teleport instance with port forwarding
			// permissions set per the test case

			recCfg, err := types.NewSessionRecordingConfig(types.SessionRecordingConfigSpecV2{
				Mode: services.RecordOff,
			})
			c.Assert(err, check.IsNil)

			cfg := s.defaultServiceConfig()
			cfg.Auth.Enabled = true
			cfg.Auth.SessionRecordingConfig = recCfg
			cfg.Proxy.Enabled = true
			cfg.Proxy.DisableWebService = false
			cfg.Proxy.DisableWebInterface = true
			cfg.SSH.Enabled = true
			cfg.SSH.AllowTCPForwarding = tt.mode

			teleport := s.newTeleportWithConfig(c, nil, nil, cfg)
			defer teleport.StopAll()

			site := teleport.GetSiteAPI(Site)

			// ...and a running dummy server
			remoteSvr := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("Hello, World"))
				}))
			defer remoteSvr.Close()

			// ... and a client connection that was launched with port
			// forwarding enabled to that dummy server
			localPort := ports.PopInt()
			remotePort, err := extractPort(remoteSvr)
			c.Assert(err, check.IsNil)

			nodeSSHPort := teleport.GetPortSSHInt()
			cl, err := teleport.NewClient(ClientConfig{
				Login:   s.me.Username,
				Cluster: Site,
				Host:    Host,
				Port:    nodeSSHPort,
			})
			c.Assert(err, check.IsNil)
			cl.Config.LocalForwardPorts = []client.ForwardedPort{
				{
					SrcIP:    "127.0.0.1",
					SrcPort:  localPort,
					DestHost: "localhost",
					DestPort: remotePort,
				},
			}
			term := NewTerminal(250)
			cl.Stdout = term
			cl.Stdin = term

			log.Warnf("Launching SSH session to %d", nodeSSHPort)

			sshSessionCtx, sshSessionCancel := context.WithCancel(context.Background())
			go cl.SSH(sshSessionCtx, []string{}, false)
			defer sshSessionCancel()

			deadline := time.Now().Add(5 * time.Second)
			for {
				ss, err := site.GetSessions(defaults.Namespace)
				c.Assert(err, check.IsNil)
				if len(ss) > 0 {
					break
				}
				c.Assert(time.Now().After(deadline), check.Equals, false)
				time.Sleep(100 * time.Millisecond)
			}

			// When everything is *finally* set up, and I attempt to use the
			// forwarded connection
			localURL := fmt.Sprintf("http://%s:%d/", "localhost", localPort)
			r, err := http.Get(localURL)

			if r != nil {
				r.Body.Close()
			}

			if tt.expectSuccess {
				log.Warnf("Checking for success")
				c.Assert(err, check.IsNil)
				c.Assert(r, check.NotNil)
			} else {
				log.Warnf("Checking for failure: %v", err)
				c.Assert(err, check.NotNil)
			}
		}
		doTest()
	}
}
