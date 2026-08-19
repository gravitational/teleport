//go:build linux && cgo

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

package user

import (
	"os/user"
	"sync"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// TestConcurrentUserCalls uses property testing to assert that any randomly
// selected set of concurrent os/user wrapper calls do not panic and finish.
func TestConcurrentUserCalls(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		const goroutines = 1000

		// This test runs both operations on users that exist and
		// users that are randomly generated and _could_ exist.
		u, err := Current()
		if err != nil {
			t.Skipf("user.Current unavailable: %v", err)
		}

		g, err := user.LookupGroupId(u.Gid)
		if err != nil {
			t.Skipf("user.LookupGroupId unavailable: %v", err)
		}

		var wg sync.WaitGroup
		start := make(chan struct{})

		idgen := rapid.StringMatching(`0|[1-9][0-9]{0,8}`)
		stringgen := rapid.String()

		for range goroutines {
			// To maximize the chances of collisions queue up all operations beforehand,
			// where applicable draw the rapid inputs upfront instead of during the concurrent
			// calls.
			switch rapid.IntRange(0, 9).Draw(t, "op") {
			case 0:
				wg.Go(func() {
					<-start
					_, _ = Lookup(u.Username)
				})
			case 1:
				wg.Go(func() {
					<-start
					_, _ = LookupId(u.Uid)
				})
			case 2:
				wg.Go(func() {
					<-start
					_, _ = LookupGroup(g.Name)
				})
			case 3:
				wg.Go(func() {
					<-start
					_, _ = LookupGroupId(g.Gid)
				})
			case 4:
				wg.Go(func() {
					<-start
					_, _ = Current()
				})
			case 5:
				wg.Go(func() {
					<-start
					_, _ = GroupIds(u)
				})
			case 6:
				username := stringgen.Draw(t, "username")
				wg.Go(func() {
					<-start
					_, _ = Lookup(username)
				})
			case 7:
				uid := idgen.Draw(t, "uid")
				wg.Go(func() {
					<-start
					_, _ = LookupId(uid)
				})
			case 8:
				groupname := stringgen.Draw(t, "group")
				wg.Go(func() {
					<-start
					_, _ = LookupGroup(groupname)
				})
			case 9:
				gid := idgen.Draw(t, "gid")
				wg.Go(func() {
					<-start
					_, _ = LookupGroupId(gid)
				})
			}
		}

		// release all goroutines at once
		close(start)

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// finished
		case <-time.After(30 * time.Second):
			t.Fatal("concurrent user calls timed out")
		}
	})
}
