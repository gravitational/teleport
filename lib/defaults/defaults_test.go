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

package defaults

import (
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestMakeAddr(t *testing.T) {
	addr := makeAddr("example.com", 3022)
	require.NotNil(t, addr)
	require.Equal(t, "tcp://example.com:3022", addr.FullAddress())
}

func TestDefaultAddresses(t *testing.T) {
	table := map[string]*utils.NetAddr{
		"tcp://0.0.0.0:3025":   AuthListenAddr(),
		"tcp://127.0.0.1:3025": AuthConnectAddr(),
		"tcp://0.0.0.0:3023":   ProxyListenAddr(),
		"tcp://0.0.0.0:3080":   ProxyWebListenAddr(),
		"tcp://0.0.0.0:3022":   SSHServerListenAddr(),
		"tcp://0.0.0.0:3024":   ReverseTunnelListenAddr(),
	}
	for expected, actual := range table {
		require.NotNil(t, actual)
		require.Equal(t, expected, actual.FullAddress())
	}
}

func TestSearchSessionRange(t *testing.T) {
	baseFakeTime, err := time.Parse(time.RFC3339, "2019-10-12T07:20:50Z")
	require.NoError(t, err)
	for _, tc := range []struct {
		name     string
		fromUTC  string
		toUTC    string
		since    string
		wantFrom string
		wantTo   string
		err      bool
	}{
		{
			name:     "base case",
			fromUTC:  "2019-10-12",
			toUTC:    "2019-10-12",
			wantFrom: "2019-10-12",
			wantTo:   "2019-10-12",
		},
		{
			name:     "missing from",
			toUTC:    "2019-10-12",
			wantFrom: "2019-10-11",
			wantTo:   "2019-10-12",
		},
		{
			name:     "missing to",
			fromUTC:  "2019-10-12",
			wantFrom: "2019-10-12",
			wantTo:   "2019-10-12",
		},
		{
			name:    "invalid from",
			fromUTC: "this is not a time",
			err:     true,
		},
		{
			name:  "invalid to",
			toUTC: "this is also not a time",
			err:   true,
		},
		{
			name:    "from after to",
			fromUTC: "2019-11-12",
			toUTC:   "2019-10-12",
			err:     true,
		},
		{
			name:    "to in the future",
			fromUTC: "2020-11-12",
			toUTC:   "2019-10-12",
			err:     true,
		},
		{
			name:    "since and from/to specified",
			fromUTC: "2020-11-12",
			toUTC:   "2019-10-12",
			since:   "10d",
			err:     true,
		},
		{
			name:     "since specified",
			wantTo:   "2019-10-12",
			wantFrom: "2019-10-11",
			since:    "24h",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotFrom, gotTo, err := SearchSessionRange(clockwork.NewFakeClockAt(baseFakeTime), tc.fromUTC, tc.toUTC, tc.since)
			if tc.err {
				require.True(t, trace.IsBadParameter(err))
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantFrom, gotFrom.Format(TshTctlSessionListTimeFormat))
			require.Equal(t, tc.wantTo, gotTo.Format(TshTctlSessionListTimeFormat))

		})
	}
}

func TestReadableDatabaseProtocol(t *testing.T) {
	require.Equal(t, "Microsoft SQL Server", fmt.Sprint(ReadableDatabaseProtocol(ProtocolSQLServer)))
	require.Equal(t, "unknown", fmt.Sprint(ReadableDatabaseProtocol("unknown")))
}
