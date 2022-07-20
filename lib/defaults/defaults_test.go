/*
Copyright 2016 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package defaults

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestMakeAddr(t *testing.T) {
	addr := makeAddr("example.com", 3022)
	if addr == nil {
		t.Fatal("makeAddr failed")
	}
	if addr.FullAddress() != "tcp://example.com:3022" {
		t.Fatalf("makeAddr did not make a correct address. Got: %v", addr.FullAddress())
	}
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
		if actual == nil {
			t.Fatalf("Expected '%v' got nil", expected)
		} else if actual.FullAddress() != expected {
			t.Errorf("Expected '%v' got '%v'", expected, actual.FullAddress())
		}
	}
}

func TestSearchSessionRange(t *testing.T) {
	baseFakeTime, err := time.Parse(time.RFC3339, "2019-10-12T07:20:50Z")
	require.NoError(t, err)
	for _, tc := range []struct {
		name     string
		fromUTC  string
		toUTC    string
		wantFrom string
		wantTo   string
		err      error
	}{
		{
			name:     "base case",
			fromUTC:  "2019-10-12T07:20:50Z",
			toUTC:    "2019-10-12T07:20:50Z",
			wantFrom: "2019-10-12T07:20:50Z",
			wantTo:   "2019-10-12T07:20:50Z",
		},
		{
			name:     "missing from",
			toUTC:    "2019-10-12T07:20:50Z",
			wantFrom: "2019-10-11T07:20:50Z",
			wantTo:   "2019-10-12T07:20:50Z",
		},
		{
			name:     "missing to",
			fromUTC:  "2019-10-12T07:20:50Z",
			wantFrom: "2019-10-12T07:20:50Z",
			wantTo:   "2019-10-12T07:20:50Z",
		},
		{
			name:    "invalid from",
			fromUTC: "this is not a time",
			err:     trace.BadParameter("failed to parse session listing start time: expected format %s, got this is not a time.", time.RFC3339),
		}, {
			name:  "invalid to",
			toUTC: "this is also not a time",
			err:   trace.BadParameter("failed to parse session listing end time: expected format %s, got this is also not a time.", time.RFC3339),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotFrom, gotTo, err := SearchSessionRange(clockwork.NewFakeClockAt(baseFakeTime), tc.fromUTC, tc.toUTC)
			if tc.err != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantFrom, gotFrom.Format(time.RFC3339))
			require.Equal(t, tc.wantTo, gotTo.Format(time.RFC3339))

		})
	}
}
