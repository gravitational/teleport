// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package x11

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var parseDisplayTestCases = []struct {
	desc             string
	displayString    string
	expectDisplay    Display
	expectParseError bool
	expectUnixAddr   string
	expectTCPAddr    string
}{
	{
		desc:           "unix socket",
		displayString:  ":10",
		expectDisplay:  Display{DisplayNumber: 10},
		expectUnixAddr: filepath.Join(x11SockDir(), "X10"),
	}, {
		desc:           "unix socket",
		displayString:  "::10",
		expectDisplay:  Display{DisplayNumber: 10},
		expectUnixAddr: filepath.Join(x11SockDir(), "X10"),
	}, {
		desc:           "unix socket",
		displayString:  "unix:10",
		expectDisplay:  Display{HostName: "unix", DisplayNumber: 10},
		expectUnixAddr: filepath.Join(x11SockDir(), "X10"),
	}, {
		desc:           "unix socket with screen number",
		displayString:  "unix:10.1",
		expectDisplay:  Display{HostName: "unix", DisplayNumber: 10, ScreenNumber: 1},
		expectUnixAddr: filepath.Join(x11SockDir(), "X10"),
	}, {
		desc:          "localhost",
		displayString: "localhost:10",
		expectDisplay: Display{HostName: "localhost", DisplayNumber: 10},
		expectTCPAddr: "127.0.0.1:6010",
	}, {
		desc:          "some ip address",
		displayString: "1.2.3.4:10",
		expectDisplay: Display{HostName: "1.2.3.4", DisplayNumber: 10},
		expectTCPAddr: "1.2.3.4:6010",
	}, {
		desc:          "invalid ip address",
		displayString: "1.2.3.4.5:10",
		expectDisplay: Display{HostName: "1.2.3.4.5", DisplayNumber: 10},
	}, {
		desc:             "empty",
		displayString:    "",
		expectParseError: true,
	}, {
		desc:             "no display number",
		displayString:    ":",
		expectParseError: true,
	}, {
		desc:             "negative display number",
		displayString:    ":-10",
		expectParseError: true,
	}, {
		desc:             "negative screen number",
		displayString:    ":10.-1",
		expectParseError: true,
	}, {
		desc:             "invalid characters",
		displayString:    "$(exec ls)",
		expectParseError: true,
	}, {
		desc:             "invalid unix socket",
		displayString:    "/some/socket/without/display",
		expectParseError: true,
	},
}

func TestDisplay(t *testing.T) {
	t.Parallel()

	for _, tc := range parseDisplayTestCases {
		t.Run(tc.desc, func(t *testing.T) {
			display, err := ParseDisplay(tc.displayString)
			if tc.expectParseError == true {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expectDisplay, display)

			unixSock, err := display.unixSocket()
			if tc.expectUnixAddr == "" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectUnixAddr, unixSock.String())
			}

			tcpSock, err := display.tcpSocket()
			if tc.expectTCPAddr == "" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectTCPAddr, tcpSock.String())
			}
		})
	}

	// The unix socket full path test requires its associated
	// unix addr to be discoverable in the file system
	t.Run("unix socket full path", func(t *testing.T) {
		tmpDir := os.TempDir()
		unixSocket := filepath.Join(tmpDir, "org.xquartz.com:0")
		hostName := filepath.Join(tmpDir, "org.xquartz.com")
		_, err := os.Create(unixSocket)
		require.NoError(t, err)

		display, err := ParseDisplay(unixSocket)
		require.NoError(t, err)
		require.Equal(t, Display{HostName: hostName, DisplayNumber: 0}, display)

		unixSock, err := display.unixSocket()
		require.NoError(t, err)
		require.Equal(t, unixSocket, unixSock.String())
	})
}
