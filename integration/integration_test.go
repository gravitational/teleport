/*
Copyright 2016 Gravitational, Inc.

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
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/defaults"

	"gopkg.in/check.v1"
)

type IntSuite struct{}

// bootstrap check
func TestIntegrations(t *testing.T) { check.TestingT(t) }

var _ = check.Suite(&IntSuite{})

func (s *IntSuite) SetUpSuite(c *check.C) {
	testVal := time.Duration(time.Millisecond * 10)
	defaults.ReverseTunnelsRefreshPeriod = testVal
	defaults.ReverseTunnelAgentReconnectPeriod = testVal
	defaults.ReverseTunnelAgentHeartbeatPeriod = testVal

	native.SetTestKeys()
}

func (s *IntSuite) TestEverything(c *check.C) {
	cl := NewInstance("client", 5000)
	sr := NewInstance("server", 6000)

	fatalIf(sr.Create(cl, false))
	fatalIf(cl.Create(sr, true))

	fatalIf(sr.Start())
	fatalIf(cl.Start())

	fmt.Println("Sleeping for 11 seconds")
	time.Sleep(time.Second * 11)

	sr.SSH([]string{"ls", "/"}, "127.0.0.1", 5000)

	cl.Stop()
	sr.Stop()
}
