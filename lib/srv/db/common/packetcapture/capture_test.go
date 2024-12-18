// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package packetcapture

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCapture(runCommand func(name string, arg ...string) ([]byte, error)) *Capture {
	clock := clockwork.NewFakeClockAt(time.Date(2024, time.December, 4, 0, 0, 0, 0, time.UTC))
	testCapture := NewCapture(clock)
	testCapture.runCommand = runCommand

	testCapture.AddPacket(ClientToTeleport, []byte("hello server, this is client"))
	clock.Advance(time.Second)

	testCapture.AddPacket(TeleportToServer, []byte("hello server, this is Teleport"))
	clock.Advance(time.Second)

	testCapture.AddPacket(ServerToTeleport, []byte("hello Teleport, this is server"))
	clock.Advance(time.Second)

	testCapture.AddPacket(TeleportToClient, []byte("hello client, this is server (actually Teleport, wink)"))
	clock.Advance(time.Second)

	return testCapture
}

func TestCapture_SaveAsText(t *testing.T) {
	testCapture := newTestCapture(func(name string, arg ...string) ([]byte, error) {
		assert.FailNow(t, "runCommand shouldn't be called for SaveAsText")
		return nil, trace.BadParameter("not expected")
	})

	tmp := t.TempDir()
	fileOut := filepath.Join(tmp, "trace.txt")

	err := testCapture.SaveAsText(fileOut)
	require.NoError(t, err)

	out, err := os.ReadFile(fileOut)
	require.NoError(t, err)
	want := `Timestamp: 2024-12-04 00:00:00 +0000 UTC
Direction: Client->Teleport

00000000  68 65 6c 6c 6f 20 73 65  72 76 65 72 2c 20 74 68  |hello server, th|
00000010  69 73 20 69 73 20 63 6c  69 65 6e 74              |is is client|


Timestamp: 2024-12-04 00:00:01 +0000 UTC
Direction: Teleport->Server

00000000  68 65 6c 6c 6f 20 73 65  72 76 65 72 2c 20 74 68  |hello server, th|
00000010  69 73 20 69 73 20 54 65  6c 65 70 6f 72 74        |is is Teleport|


Timestamp: 2024-12-04 00:00:02 +0000 UTC
Direction: Server->Teleport

00000000  68 65 6c 6c 6f 20 54 65  6c 65 70 6f 72 74 2c 20  |hello Teleport, |
00000010  74 68 69 73 20 69 73 20  73 65 72 76 65 72        |this is server|


Timestamp: 2024-12-04 00:00:03 +0000 UTC
Direction: Teleport->Client

00000000  68 65 6c 6c 6f 20 63 6c  69 65 6e 74 2c 20 74 68  |hello client, th|
00000010  69 73 20 69 73 20 73 65  72 76 65 72 20 28 61 63  |is is server (ac|
00000020  74 75 61 6c 6c 79 20 54  65 6c 65 70 6f 72 74 2c  |tually Teleport,|
00000030  20 77 69 6e 6b 29                                 | wink)|


`
	require.Equal(t, want, string(out))
}

func TestCapture_SaveToPCAP(t *testing.T) {
	tmp := t.TempDir()
	fileOut := path.Join(tmp, "trace.pcap")

	runCommandCount := 0

	testCapture := newTestCapture(func(progName string, args ...string) ([]byte, error) {
		runCommandCount++

		// verify command being run as well as its arguments.
		switch runCommandCount {
		case 1:
			want := []string{
				"-D", "-t", "%Y-%m-%dT%H:%M:%S.%fZ", "-l", "1", "-4", "1.1.1.1,2.2.2.2", "-T", "1111,1111",
				fileOut + ".001.hex",
				fileOut + ".001",
			}
			assert.Equal(t, want, args)
			assert.Equal(t, text2pcapBin, progName)

			hexData, err := os.ReadFile(fileOut + ".001.hex")
			require.NoError(t, err)
			require.Equal(t, `I 2024-12-04T00:00:00Z
00000000  68 65 6c 6c 6f 20 73 65  72 76 65 72 2c 20 74 68  |hello server, th|
00000010  69 73 20 69 73 20 63 6c  69 65 6e 74              |is is client|

O 2024-12-04T00:00:03Z
00000000  68 65 6c 6c 6f 20 63 6c  69 65 6e 74 2c 20 74 68  |hello client, th|
00000010  69 73 20 69 73 20 73 65  72 76 65 72 20 28 61 63  |is is server (ac|
00000020  74 75 61 6c 6c 79 20 54  65 6c 65 70 6f 72 74 2c  |tually Teleport,|
00000030  20 77 69 6e 6b 29                                 | wink)|

`, string(hexData))

		case 2:
			want := []string{
				"-D", "-t", "%Y-%m-%dT%H:%M:%S.%fZ", "-l", "1", "-4", "2.2.2.2,3.3.3.3", "-T", "1111,1111",
				fileOut + ".002.hex",
				fileOut + ".002",
			}
			assert.Equal(t, want, args)
			assert.Equal(t, text2pcapBin, progName)

			hexData, err := os.ReadFile(fileOut + ".002.hex")
			require.NoError(t, err)
			require.Equal(t, `I 2024-12-04T00:00:01Z
00000000  68 65 6c 6c 6f 20 73 65  72 76 65 72 2c 20 74 68  |hello server, th|
00000010  69 73 20 69 73 20 54 65  6c 65 70 6f 72 74        |is is Teleport|

O 2024-12-04T00:00:02Z
00000000  68 65 6c 6c 6f 20 54 65  6c 65 70 6f 72 74 2c 20  |hello Teleport, |
00000010  74 68 69 73 20 69 73 20  73 65 72 76 65 72        |this is server|

`, string(hexData))
		case 3:
			assert.Equal(t, mergecapBin, progName)
			want := []string{"-w",
				fileOut,
				fileOut + ".001",
				fileOut + ".002",
			}
			assert.Equal(t, want, args)
		default:
			assert.Fail(t, "unexpected number of commands")
		}

		// return no error.
		return []byte("all is fine"), nil
	})

	err := testCapture.SaveToPCAP(fileOut, 1111)
	require.NoError(t, err)

	// verify all temp files have been deleted.
	entries, err := os.ReadDir(tmp)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestCapture_WriteTo(t *testing.T) {
	testCapture := newTestCapture(func(name string, arg ...string) ([]byte, error) {
		assert.FailNow(t, "runCommand shouldn't be called for SaveAsText")
		return nil, trace.BadParameter("not expected")
	})

	var buf bytes.Buffer

	count, err := testCapture.WriteTo(&buf)
	require.NoError(t, err)
	require.Equal(t, int64(1060), count)

	want := `Timestamp: 2024-12-04 00:00:00 +0000 UTC
Direction: Client->Teleport

00000000  68 65 6c 6c 6f 20 73 65  72 76 65 72 2c 20 74 68  |hello server, th|
00000010  69 73 20 69 73 20 63 6c  69 65 6e 74              |is is client|


Timestamp: 2024-12-04 00:00:01 +0000 UTC
Direction: Teleport->Server

00000000  68 65 6c 6c 6f 20 73 65  72 76 65 72 2c 20 74 68  |hello server, th|
00000010  69 73 20 69 73 20 54 65  6c 65 70 6f 72 74        |is is Teleport|


Timestamp: 2024-12-04 00:00:02 +0000 UTC
Direction: Server->Teleport

00000000  68 65 6c 6c 6f 20 54 65  6c 65 70 6f 72 74 2c 20  |hello Teleport, |
00000010  74 68 69 73 20 69 73 20  73 65 72 76 65 72        |this is server|


Timestamp: 2024-12-04 00:00:03 +0000 UTC
Direction: Teleport->Client

00000000  68 65 6c 6c 6f 20 63 6c  69 65 6e 74 2c 20 74 68  |hello client, th|
00000010  69 73 20 69 73 20 73 65  72 76 65 72 20 28 61 63  |is is server (ac|
00000020  74 75 61 6c 6c 79 20 54  65 6c 65 70 6f 72 74 2c  |tually Teleport,|
00000030  20 77 69 6e 6b 29                                 | wink)|


`
	require.Equal(t, want, buf.String())
}
