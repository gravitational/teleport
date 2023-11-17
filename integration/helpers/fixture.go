// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helpers

import (
	"os"
	"os/user"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	HostID = "00000000-0000-0000-0000-000000000000"
	Site   = "local-site"
)

type Fixture struct {
	Me *user.User

	// Priv/pub pair to avoid re-generating it
	Priv []byte
	Pub  []byte

	// Log defines the test-specific logger
	Log utils.Logger
}

func NewFixture(t *testing.T) *Fixture {
	fixture := &Fixture{}

	var err error
	fixture.Priv, fixture.Pub, err = testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	// Find AllocatePortsNum free listening ports to use.
	fixture.Me, err = user.Current()
	require.NoError(t, err)

	// close & re-open stdin because 'go test' runs with os.stdin connected to /dev/null
	stdin, err := os.Open("/dev/tty")
	if err == nil {
		os.Stdin.Close()
		os.Stdin = stdin
	}

	t.Cleanup(func() {
		// restore os.Stdin to its original condition: connected to /dev/null
		os.Stdin.Close()
		os.Stdin, err = os.Open("/dev/null")
		require.NoError(t, err)
	})

	return fixture
}

// NewTeleportWithConfig is a helper function that will create a running
// Teleport instance with the passed in user, instance secrets, and Teleport
// configuration.
func (s *Fixture) NewTeleportWithConfig(t *testing.T, logins []string, instanceSecrets []*InstanceSecrets, teleportConfig *servicecfg.Config) *TeleInstance {
	teleport := s.NewTeleportInstance(t)

	// use passed logins, but use suite's default login if nothing was passed
	if len(logins) == 0 {
		logins = []string{s.Me.Username}
	}
	for _, login := range logins {
		teleport.AddUser(login, []string{login})
	}

	// create a new teleport instance with passed in configuration
	if err := teleport.CreateEx(t, instanceSecrets, teleportConfig); err != nil {
		t.Fatalf("Unexpected response from CreateEx: %v", trace.DebugReport(err))
	}

	if err := teleport.Start(); err != nil {
		t.Fatalf("Unexpected response from Start: %v", trace.DebugReport(err))
	}

	return teleport
}

func (s *Fixture) NewTeleportInstance(t *testing.T) *TeleInstance {
	return NewInstance(t, s.DefaultInstanceConfig(t))
}

func (s *Fixture) DefaultInstanceConfig(t *testing.T) InstanceConfig {
	cfg := InstanceConfig{
		ClusterName: Site,
		HostID:      HostID,
		NodeName:    Host,
		Priv:        s.Priv,
		Pub:         s.Pub,
		Log:         s.Log,
	}
	cfg.Listeners = StandardListenerSetup(t, &cfg.Fds)
	return cfg
}
