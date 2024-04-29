//go:build darwin || linux

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package uds

import (
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCreds(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	l, err := net.Listen("unix", sockPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, l.Close())
	})

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := net.Dial("unix", sockPath)
		assert.NoError(t, err)
		t.Cleanup(func() {
			_ = conn.Close()
		})
	}()

	conn, err := l.Accept()
	require.NoError(t, err)

	// Wait for the connection to be established.
	wg.Wait()

	creds, err := GetCreds(conn)
	require.NoError(t, err)

	// Compare to the current process.
	assert.Equal(t, os.Getpid(), creds.PID)
	assert.Equal(t, os.Getuid(), creds.UID)
	assert.Equal(t, os.Getgid(), creds.GID)
}
