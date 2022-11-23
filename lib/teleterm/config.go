// Copyright 2021 Gravitational, Inc
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

package teleterm

import (
	"os"
	"syscall"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// Config describes teleterm configuration
type Config struct {
	// Addr is the bind address for the server
	Addr string
	// ShutdownSignals is the set of captured signals that cause server shutdown.
	ShutdownSignals []os.Signal
	// HomeDir is the directory to store cluster profiles
	HomeDir string
	// Directory containing certs used to create secure gRPC connection with daemon service
	CertsDir string
	// InsecureSkipVerify is an option to skip HTTPS cert check
	InsecureSkipVerify bool
	// ListeningC propagates the address on which the gRPC server listens. Mostly useful in tests, as
	// the Electron app gets the server port from stdout.
	ListeningC chan<- utils.NetAddr
}

// CheckAndSetDefaults checks and sets default config values.
func (c *Config) CheckAndSetDefaults() error {
	if c.HomeDir == "" {
		return trace.BadParameter("missing home directory")
	}

	if c.CertsDir == "" {
		return trace.BadParameter("missing certs directory")
	}

	if c.Addr == "" {
		return trace.BadParameter("missing network address")
	}

	addr, err := utils.ParseAddr(c.Addr)
	if err != nil {
		return trace.Wrap(err)
	}

	if !(addr.Network() == "unix" || addr.Network() == "tcp") {
		return trace.BadParameter("network address should start with unix:// or tcp:// or be empty (tcp:// is used in that case)")
	}

	if len(c.ShutdownSignals) == 0 {
		// If ShutdownSignals is empty, the service will be immediately shut down on start as
		// Signal.Notify relays all signals if it's given no specific signals to watch for.
		c.ShutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
	}

	return nil
}
