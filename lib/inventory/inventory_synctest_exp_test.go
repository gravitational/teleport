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

//go:build !go1.25 && goexperiment.synctest

package inventory

import (
	"testing"
	"testing/synctest"
)

// TODO(espadolini): DELETE IN v21 or after the oldest supported Teleport
// version is on go 1.25

func maybeSynctest(t *testing.T, fn func(*testing.T)) {
	// in go 1.24 with the synctest experiment there's no integrated support for
	// t.Cleanup callbacks to run before exiting the bubble, but if we run
	// things in a subtest we get the same behavior, albeit with everything in a
	// subtest, which is ugly but functional
	synctest.Run(func() { t.Run("goexperiment.synctest", fn) })
}
