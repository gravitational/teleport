// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cryptosuites

import (
	"testing"

	"github.com/gravitational/teleport/lib/cryptosuites/internal/rsa"
)

// PrecomputeKeys sets this package into a mode where a small backlog of RSA keys are
// computed in advance. This should only be enabled if large spikes in RSA key
// computation are expected (e.g. in auth/proxy services when the legacy suite
// is configured). Safe to double-call.
func PrecomputeRSAKeys() {
	rsa.PrecomputeKeys()
}

// PrecomputeRSATestKeys may be called from TestMain to set this package into a
// mode where it will precompute a fixed number of RSA keys and reuse them to
// save on CPU usage.
func PrecomputeRSATestKeys(m *testing.M) {
	rsa.PrecomputeTestKeys(m)
}
