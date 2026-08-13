/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	sharedFixtureMu        sync.Mutex
	sharedFixtureTeardowns []func()
)

func registerSharedFixtureTeardown(fn func()) {
	sharedFixtureMu.Lock()
	defer sharedFixtureMu.Unlock()
	sharedFixtureTeardowns = append(sharedFixtureTeardowns, fn)
}

// teardownSharedFixtures runs the registered teardowns.
// Called once from TestMain.
func teardownSharedFixtures() {
	sharedFixtureMu.Lock()
	defer sharedFixtureMu.Unlock()
	for i := len(sharedFixtureTeardowns) - 1; i >= 0; i-- {
		sharedFixtureTeardowns[i]()
	}
	sharedFixtureTeardowns = nil
}

// dataDirFor returns a data dir for a cluster's file config.
func dataDirFor(t *testing.T, shared bool) string {
	t.Helper()
	if !shared {
		return t.TempDir()
	}
	return sharedTempDir(t)
}

// sharedTempDir returns a temp dir removed by TestMain.
func sharedTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "tsh-shared-fixture")
	require.NoError(t, err)
	registerSharedFixtureTeardown(func() { os.RemoveAll(dir) })
	return dir
}
