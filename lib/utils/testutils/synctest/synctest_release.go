//go:build go1.25

// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package synctest

import (
	"testing"
	"testing/synctest" //nolint:depguard // this package wraps synctest
)

// Test runs the provided function in a synctest bubble if synctest is
// supported, otherwise skips the test. To support older versions of Go, the
// function might be called in a subtest of the provided test.
func Test(t *testing.T, f func(*testing.T)) {
	// this version of Go (1.25+) is recent enough
	synctest.Test(t, f)
}

// Wait blocks until every goroutine in the bubble is durably blocked. See
// [synctest.Wait] for details. Wait will panic if called from outside a bubble.
func Wait() {
	synctest.Wait()
}
