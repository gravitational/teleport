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

package mysql

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestFetchMySQLVersion(t *testing.T) {
	t.Parallel()

	// Handshake message, version: 5.5.5-10.8.2-MariaDB-1:10.8.2+maria~focal
	versionMsg := []byte{0x6d, 0x00, 0x00, 0x00, 0x0a, 0x35, 0x2e, 0x35, 0x2e, 0x35, 0x2d, 0x31, 0x30, 0x2e, 0x38,
		0x2e, 0x32, 0x2d, 0x4d, 0x61, 0x72, 0x69, 0x61, 0x44, 0x42, 0x2d, 0x31, 0x3a, 0x31, 0x30, 0x2e, 0x38, 0x2e,
		0x32, 0x2b, 0x6d, 0x61, 0x72, 0x69, 0x61, 0x7e, 0x66, 0x6f, 0x63, 0x61, 0x6c, 0x00, 0x04, 0x00, 0x00, 0x00,
		0x36, 0x47, 0x42, 0x54, 0x62, 0x6e, 0x22, 0x5a, 0x00, 0xfe, 0xff, 0x2d, 0x02, 0x00, 0xff, 0xc1, 0x15, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x1d, 0x00, 0x00, 0x00, 0x23, 0x3c, 0x46, 0x2a, 0x3e, 0x38, 0x40, 0x76, 0x56,
		0x28, 0x63, 0x3d, 0x00, 0x6d, 0x79, 0x73, 0x71, 0x6c, 0x5f, 0x6e, 0x61, 0x74, 0x69, 0x76, 0x65, 0x5f, 0x70,
		0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x00}

	// ERR package with message: Host '70.144.128.44' is blocked because of many connection errors; unblock with 'mariadb-admin flush-hosts'
	errMsg := []byte{0x6e, 0x00, 0x00, 0x01, 0xff, 0x69, 0x04, 0x48, 0x6f, 0x73, 0x74, 0x20, 0x27, 0x37, 0x30, 0x2e,
		0x31, 0x34, 0x34, 0x2e, 0x31, 0x32, 0x38, 0x2e, 0x34, 0x34, 0x27, 0x20, 0x69, 0x73, 0x20, 0x62, 0x6c, 0x6f,
		0x63, 0x6b, 0x65, 0x64, 0x20, 0x62, 0x65, 0x63, 0x61, 0x75, 0x73, 0x65, 0x20, 0x6f, 0x66, 0x20, 0x6d, 0x61,
		0x6e, 0x79, 0x20, 0x63, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x20, 0x65, 0x72, 0x72, 0x6f,
		0x72, 0x73, 0x3b, 0x20, 0x75, 0x6e, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x20, 0x77, 0x69, 0x74, 0x68, 0x20, 0x27,
		0x6d, 0x61, 0x72, 0x69, 0x61, 0x64, 0x62, 0x2d, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x20, 0x66, 0x6c, 0x75, 0x73,
		0x68, 0x2d, 0x68, 0x6f, 0x73, 0x74, 0x73, 0x27}

	tests := []struct {
		name     string
		message  []byte
		assertFn func(t *testing.T, version string, err error)
	}{
		{
			name:    "versions success",
			message: versionMsg,
			assertFn: func(t *testing.T, version string, err error) {
				require.NoError(t, err)
				require.Equal(t, "5.5.5-10.8.2-MariaDB-1:10.8.2+maria~focal", version)
			},
		},
		{
			name:    "returned error",
			message: errMsg,
			assertFn: func(t *testing.T, version string, err error) {
				require.Error(t, err)
				require.Equal(t, "Host '70.144.128.44' is blocked because of many connection errors; unblock with 'mariadb-admin flush-hosts'", err.Error())
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeMySQL, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)

			t.Cleanup(func() {
				require.NoError(t, fakeMySQL.Close())
			})

			errs := make(chan error)

			go func() {
				conn, err := fakeMySQL.Accept()
				if err != nil {
					errs <- err
					return
				}

				n, err := io.Copy(conn, bytes.NewReader(tt.message))
				if err != nil {
					// Close connection to avoid leak, ignore error as we already have different error at that point.
					_ = conn.Close()
					errs <- err
					return
				}

				if int(n) != len(tt.message) {
					_ = conn.Close()
					errs <- trace.Errorf("failed to copy whole message: expected %d, copied %d", len(tt.message), n)
					return
				}

				if err := conn.Close(); err != nil {
					errs <- err
					return
				}

				errs <- nil
			}()

			srvAddr := fakeMySQL.Addr().String()
			ctx := context.Background()

			version, err := FetchMySQLVersion(ctx, &types.DatabaseV3{
				Spec: types.DatabaseSpecV3{
					URI: srvAddr,
				},
			})

			tt.assertFn(t, version, err)
			require.NoError(t, <-errs)
		})
	}
}

func TestFetchMySQLVersionDoesntBlock(t *testing.T) {
	t.Parallel()

	fakeMySQL, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, fakeMySQL.Close())
	})

	serverWait := make(chan struct{})
	t.Cleanup(func() {
		close(serverWait)
	})

	go func() {
		conn, err := fakeMySQL.Accept()
		if err == nil {
			t.Cleanup(func() {
				conn.Close()
			})
		}

		// accept connection and just wait for timeout.
		<-serverWait
	}()

	srvAddr := fakeMySQL.Addr().String()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	fetchVerErrs := make(chan error)

	go func() {
		// Passed context should be canceled and i/o timeout should be returned.
		_, err := FetchMySQLVersion(ctx, &types.DatabaseV3{
			Spec: types.DatabaseSpecV3{
				URI: srvAddr,
			},
		})

		fetchVerErrs <- err
	}()

	select {
	case err := <-fetchVerErrs:
		require.Error(t, err)
		require.Contains(t, err.Error(), "i/o timeout")
	case <-time.After(5 * time.Second):
		require.FailNow(t, "connection should return before")
	}
}
