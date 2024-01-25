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

package helpers

import (
	"crypto/x509"
	"sync"
	"testing"
	_ "unsafe"
)

//go:linkname x509_systemRootsMu crypto/x509.systemRootsMu
//go:linkname x509_systemRoots crypto/x509.systemRoots

// These variables are used in x509 to cache the system cert pool after
// calculating it once; this is not a public interface and it's liable to even
// crash if the details of crypto/x509 are changed. An alternative to this would
// be to use x509.SetFallbackRoots and GODEBUG=x509usefallbackroots=1, but that
// can only be done once and requires a go:debug directive, so it will require
// the cooperation of the entire package's tests.
var (
	x509_systemRootsMu sync.RWMutex
	x509_systemRoots   *x509.CertPool
)

// OverrideSystemRoots overrides the system certificate pool for the remainder
// of the test. The test must not be parallel, and OverrideSystemRoots will
// panic if called outside of a test binary.
func OverrideSystemRoots(t *testing.T, pool *x509.CertPool) {
	if !testing.Testing() {
		panic("SetSystemRoots was called outside of a test binary")
	}
	// panic if the test is parallel or when the test is made parallel
	t.Setenv("_OverrideSystemRoots_called", "1")

	// ensure that systemRoots is already primed, or the next call to
	// x509.SystemCertPool() will overwrite it
	_, _ = x509.SystemCertPool()

	// the pool we store in systemRoots must not be changed
	pool = pool.Clone()

	x509_systemRootsMu.Lock()
	previousSystemRoots := x509_systemRoots
	x509_systemRoots = pool
	x509_systemRootsMu.Unlock()

	t.Cleanup(func() {
		x509_systemRootsMu.Lock()
		if x509_systemRoots != pool {
			t.Log("x509.systemRoots was left modified after OverrideSystemRoots")
			t.Fail() // continue execution but fail the test
		}
		x509_systemRoots = previousSystemRoots
		x509_systemRootsMu.Unlock()
	})
}
