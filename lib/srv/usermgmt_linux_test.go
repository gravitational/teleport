//go:build linux && cgo
// +build linux,cgo

// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package srv

import (
	"os/user"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestConcurrentUserLookupsDoesNotDeadlock reproduces concurrent use of the
// OS user lookup APIs. Run on Linux with an NSS configuration that loads
// libnss-extrausers to reproduce the deadlock/crash described in
// https://github.com/gravitational/teleport/issues/69662. Requires CGO_ENABLED=1.
func TestConcurrentUserLookupsDoesNotDeadlock(t *testing.T) {
	// Not marked Parallel because this test purposely stresses global C libs.
	backend := &HostUsersProvisioningBackend{}
	startgate := make(chan struct{})
	done := make(chan struct{})
	const goroutines = 1000
	var wg sync.WaitGroup

	// On later versions the race condition can trigger a segfault instead of a deadlock.
	require.NotPanics(t, func() {
		for range goroutines {
			wg.Go(func() {
				<-startgate
				_, _ = backend.Lookup("nonexistent-user")
				_, _ = backend.LookupGroup("nonexistent-group")
				_, _ = backend.LookupGroupByID("99999")
				_, _ = backend.UserGIDs(&user.User{Username: "user", Uid: "1000", Gid: "1000"})
			})
		}

		go func() {
			wg.Wait()
			close(done)
		}()
		close(startgate)
	})

	select {
	case <-done:
		// completed successfully
	case <-time.After(30 * time.Second):
		t.Fatal("concurrent user lookups timed out possible deadlock or crash condition")
	}
}
