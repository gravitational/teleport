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

package helpers

import (
	"log/slog"
	"os"
	"os/user"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/service/servicecfg"
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
	Log *slog.Logger
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
		Logger:      s.Log,
	}
	cfg.Listeners = StandardListenerSetup(t, &cfg.Fds)
	return cfg
}
