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

package sftp

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/pkg/sftp"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/session/sftputils"
)

// legacyParseSFTPEvent is the original implementation of event conversion from
// [sftp.Request] to audit log event, before the change to custom types used by
// the SFTP server process.
func legacyParseSFTPEvent(now time.Time, req *sftp.Request, workingDirectory string, reqErr error) (*apievents.SFTP, error) {
	event := &apievents.SFTP{
		Metadata: apievents.Metadata{
			Type: events.SFTPEvent,
			Time: now,
		},
	}

	switch req.Method {
	case sftputils.MethodOpen, sftputils.MethodGet, sftputils.MethodPut:
		if reqErr == nil {
			event.Code = events.SFTPOpenCode
		} else {
			event.Code = events.SFTPOpenFailureCode
		}
		event.Action = apievents.SFTPAction_OPEN
	case sftputils.MethodSetStat:
		if reqErr == nil {
			event.Code = events.SFTPSetstatCode
		} else {
			event.Code = events.SFTPSetstatFailureCode
		}
		event.Action = apievents.SFTPAction_SETSTAT
	case sftputils.MethodList:
		if reqErr == nil {
			event.Code = events.SFTPReaddirCode
		} else {
			event.Code = events.SFTPReaddirFailureCode
		}
		event.Action = apievents.SFTPAction_READDIR
	case sftputils.MethodRemove:
		if reqErr == nil {
			event.Code = events.SFTPRemoveCode
		} else {
			event.Code = events.SFTPRemoveFailureCode
		}
		event.Action = apievents.SFTPAction_REMOVE
	case sftputils.MethodMkdir:
		if reqErr == nil {
			event.Code = events.SFTPMkdirCode
		} else {
			event.Code = events.SFTPMkdirFailureCode
		}
		event.Action = apievents.SFTPAction_MKDIR
	case sftputils.MethodRmdir:
		if reqErr == nil {
			event.Code = events.SFTPRmdirCode
		} else {
			event.Code = events.SFTPRmdirFailureCode
		}
		event.Action = apievents.SFTPAction_RMDIR
	case sftputils.MethodRename:
		if reqErr == nil {
			event.Code = events.SFTPRenameCode
		} else {
			event.Code = events.SFTPRenameFailureCode
		}
		event.Action = apievents.SFTPAction_RENAME
	case sftputils.MethodSymlink:
		if reqErr == nil {
			event.Code = events.SFTPSymlinkCode
		} else {
			event.Code = events.SFTPSymlinkFailureCode
		}
		event.Action = apievents.SFTPAction_SYMLINK
	case sftputils.MethodLink:
		if reqErr == nil {
			event.Code = events.SFTPLinkCode
		} else {
			event.Code = events.SFTPLinkFailureCode
		}
		event.Action = apievents.SFTPAction_LINK
	default:
		return nil, trace.BadParameter("unknown SFTP request %q", req.Method)
	}

	event.Path = req.Filepath
	event.TargetPath = req.Target
	event.Flags = req.Flags
	event.WorkingDirectory = workingDirectory
	if req.Method == sftputils.MethodSetStat {
		attrFlags := req.AttrFlags()
		attrs := req.Attributes()
		event.Attributes = new(apievents.SFTPAttributes)

		if attrFlags.Acmodtime {
			atime := time.Unix(int64(attrs.Atime), 0)
			mtime := time.Unix(int64(attrs.Mtime), 0)
			event.Attributes.AccessTime = &atime
			event.Attributes.ModificationTime = &mtime
		}
		if attrFlags.Permissions {
			perms := uint32(attrs.FileMode().Perm())
			event.Attributes.Permissions = &perms
		}
		if attrFlags.Size {
			event.Attributes.FileSize = &attrs.Size
		}
		if attrFlags.UidGid {
			event.Attributes.UID = &attrs.UID
			event.Attributes.GID = &attrs.GID
		}
	}
	if reqErr != nil {
		// If possible, strip the filename from the error message. The
		// path will be included in audit events already, no need to
		// make the error message longer than it needs to be.
		var pathErr *fs.PathError
		var linkErr *os.LinkError
		if errors.As(reqErr, &pathErr) {
			event.Error = pathErr.Err.Error()
		} else if errors.As(reqErr, &linkErr) {
			event.Error = linkErr.Err.Error()
		} else {
			event.Error = reqErr.Error()
		}
	}

	return event, nil
}

func TestSFTPEventMatchesLegacy(t *testing.T) {
	// sftp protocol constants
	const (
		sshFileXferAttrSize        = 0x00000001
		sshFileXferAttrUIDGID      = 0x00000002
		sshFileXferAttrPermissions = 0x00000004
		sshFileXferAttrACmodTime   = 0x00000008
	)

	inputs := []struct {
		req              *sftp.Request
		workingDirectory string
		reqErr           error
	}{
		{&sftp.Request{
			Method:   sftputils.MethodGet,
			Filepath: "/fp",
		}, "/mywd", nil},
		{&sftp.Request{
			Method:   sftputils.MethodPut,
			Filepath: "/fp",
			Flags:    42,
		}, "/mywd", nil},
		{&sftp.Request{
			Method:   sftputils.MethodPut,
			Filepath: "/fp",
		}, "/mywd", &fs.PathError{Path: "/fp", Err: errors.New("lmao")}},
		{&sftp.Request{
			Method:   sftputils.MethodRemove,
			Filepath: "/fp",
		}, "/mywd", nil},
		{&sftp.Request{
			Method:   sftputils.MethodLink,
			Filepath: "/fp",
			Target:   "/fp2",
		}, "/mywd", &os.LinkError{Old: "/fp", New: "/fp2", Err: errors.New("lmao")}},
		{&sftp.Request{
			Method:   sftputils.MethodSetStat,
			Filepath: "/fp",
			Flags:    sshFileXferAttrACmodTime,
			Attrs:    []byte{0x1, 0x23, 0x45, 0x67, 0x12, 0x34, 0x56, 0x78},
		}, "/mywd", nil},
		{&sftp.Request{
			Method:   sftputils.MethodSetStat,
			Filepath: "/fp",
			Flags:    sshFileXferAttrSize,
			Attrs:    []byte{0x1, 0x23, 0x45, 0x67, 0x12, 0x34, 0x56, 0x78},
		}, "/mywd", nil},
		{&sftp.Request{
			Method:   sftputils.MethodSetStat,
			Filepath: "/fp",
			Flags:    sshFileXferAttrPermissions,
			Attrs:    []byte{0, 0, 0o7, 0o55},
		}, "/mywd", nil},
	}

	for _, input := range inputs {
		sftpEvent, err := sftputils.ParseSFTPEvent(input.req, input.workingDirectory, input.reqErr)
		require.NoError(t, err)

		newEvent, err := SFTPEventToProto(sftpEvent)
		require.NoError(t, err)

		legacyEvent, err := legacyParseSFTPEvent(time.Unix(0, sftpEvent.Time).UTC(), input.req, input.workingDirectory, input.reqErr)
		require.NoError(t, err)

		requireEqualEvents(t, legacyEvent, newEvent)
	}
}

func TestSFTPSummaryEventRoundtrip(t *testing.T) {
	type testCase struct {
		source   *sftputils.SFTPSummaryEvent
		expected *apievents.SFTPSummary
	}

	testCases := []testCase{
		{
			source: &sftputils.SFTPSummaryEvent{
				Time: 1777386995863701123,
			},
			expected: &apievents.SFTPSummary{
				Metadata: apievents.Metadata{
					Type: "sftp_summary",
					Code: "TS021I",
					Time: time.Date(2026, 4, 28, 14, 36, 35, 863701123, time.UTC),
				},
			},
		},
		{
			source: &sftputils.SFTPSummaryEvent{
				Stats: []sftputils.SFTPSummaryEventFileTransferStat{},
			},
			expected: &apievents.SFTPSummary{
				Metadata: apievents.Metadata{
					Type: "sftp_summary",
					Code: "TS021I",
					Time: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				FileTransferStats: []*apievents.FileTransferStat(nil),
			},
		},
		{
			source: &sftputils.SFTPSummaryEvent{
				Time: 1777386995863701123,
				Stats: []sftputils.SFTPSummaryEventFileTransferStat{
					{
						Path:    "foo",
						Read:    12345,
						Written: 0,
					}, {
						Path:    "/bar/baz",
						Read:    0,
						Written: 678,
					},
				},
			},
			expected: &apievents.SFTPSummary{
				Metadata: apievents.Metadata{
					Type: "sftp_summary",
					Code: "TS021I",
					Time: time.Date(2026, 4, 28, 14, 36, 35, 863701123, time.UTC),
				},
				FileTransferStats: []*apievents.FileTransferStat{
					{
						Path:         "foo",
						BytesRead:    12345,
						BytesWritten: 0,
					},
					{
						Path:         "/bar/baz",
						BytesRead:    0,
						BytesWritten: 678,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		j, err := json.Marshal(tc.source)
		require.NoError(t, err)
		var unmarshaled *sftputils.SFTPSummaryEvent
		require.NoError(t, json.Unmarshal(j, &unmarshaled))
		require.NotNil(t, unmarshaled)
		actual := SFTPSummaryEventToProto(unmarshaled)
		requireEqualEvents(t, tc.expected, actual)
	}
}

func requireEqualEvents(t testing.TB, expected, actual apievents.AuditEvent) {
	t.Helper()
	e, err := events.ToEventFields(expected)
	require.NoError(t, err)
	a, err := events.ToEventFields(actual)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(e, a))
}
