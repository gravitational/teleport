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

package desktop

import (
	"io"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	testDirectoryID  directoryID  = 2
	testCompletionID completionID = 999

	testOffset uint64 = 500
	testLength uint32 = 1000

	testDirName  = "test-dir"
	testFilePath = "test/path/test-file.txt"
)

// testDesktop is a dummy desktop used to populate
// audit events for testing
var testDesktop = &types.WindowsDesktopV3{
	ResourceHeader: types.ResourceHeader{
		Metadata: types.Metadata{
			Name:   "test-desktop",
			Labels: map[string]string{"env": "production"},
		},
	},
	Spec: types.WindowsDesktopSpecV3{
		Addr:   "192.168.100.12",
		Domain: "test.example.com",
	},
}

func setup(desktop types.WindowsDesktop) (*tlsca.Identity, *desktopSessionAuditor) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	startTime := time.Now()

	s := &WindowsService{
		clusterName: "test-cluster",
		cfg: WindowsServiceConfig{
			Log:     log,
			Emitter: libevents.NewDiscardEmitter(),
			Heartbeat: HeartbeatConfig{
				HostUUID: "test-host-id",
			},
			Clock: clockwork.NewFakeClockAt(startTime),
		},
		auditCache: newSharedDirectoryAuditCache(),
	}

	id := &tlsca.Identity{
		Username:     "foo",
		Impersonator: "bar",
		MFAVerified:  "mfa-id",
		LoginIP:      "127.0.0.1",
	}

	d := &desktopSessionAuditor{
		clock: s.cfg.Clock,

		sessionID:   "sessionID",
		identity:    id,
		windowsUser: "Administrator",
		desktop:     desktop,

		startTime:          startTime,
		clusterName:        s.clusterName,
		desktopServiceUUID: s.cfg.Heartbeat.HostUUID,

		auditCache: newSharedDirectoryAuditCache(),
	}

	return id, d
}

func TestSessionStartEvent(t *testing.T) {

	id, audit := setup(testDesktop)

	userMeta := id.GetUserMetadata()
	userMeta.Login = "Administrator"
	expected := &events.WindowsDesktopSessionStart{
		Metadata: events.Metadata{
			ClusterName: audit.clusterName,
			Type:        libevents.WindowsDesktopSessionStartEvent,
			Code:        libevents.DesktopSessionStartCode,
			Time:        audit.startTime,
		},
		UserMetadata: userMeta,
		SessionMetadata: events.SessionMetadata{
			SessionID: "sessionID",
			WithMFA:   id.MFAVerified,
		},
		ConnectionMetadata: events.ConnectionMetadata{
			LocalAddr:  id.LoginIP,
			RemoteAddr: testDesktop.GetAddr(),
			Protocol:   libevents.EventProtocolTDP,
		},
		Status: events.Status{
			Success: true,
		},
		WindowsDesktopService: audit.desktopServiceUUID,
		DesktopName:           "test-desktop",
		DesktopAddr:           testDesktop.GetAddr(),
		Domain:                testDesktop.GetDomain(),
		WindowsUser:           "Administrator",
		DesktopLabels:         map[string]string{"env": "production"},
	}

	for _, test := range []struct {
		desc string
		err  error
		exp  func() events.WindowsDesktopSessionStart
	}{
		{
			desc: "success",
			err:  nil,
			exp:  func() events.WindowsDesktopSessionStart { return *expected },
		},
		{
			desc: "failure",
			err:  trace.AccessDenied("access denied"),
			exp: func() events.WindowsDesktopSessionStart {
				e := *expected
				e.Code = libevents.DesktopSessionStartFailureCode
				e.Success = false
				e.Error = "access denied"
				e.UserMessage = "access denied"
				return e
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			startEvent := audit.makeSessionStart(test.err)
			require.Empty(t, cmp.Diff(test.exp(), *startEvent))
		})
	}
}

func TestSessionEndEvent(t *testing.T) {

	id, audit := setup(testDesktop)

	audit.clock.(clockwork.FakeClock).Advance(30 * time.Second)

	endEvent := audit.makeSessionEnd(true)

	userMeta := id.GetUserMetadata()
	userMeta.Login = "Administrator"
	expected := &events.WindowsDesktopSessionEnd{
		Metadata: events.Metadata{
			ClusterName: audit.clusterName,
			Type:        libevents.WindowsDesktopSessionEndEvent,
			Code:        libevents.DesktopSessionEndCode,
		},
		UserMetadata: userMeta,
		SessionMetadata: events.SessionMetadata{
			SessionID: "sessionID",
			WithMFA:   id.MFAVerified,
		},
		WindowsDesktopService: audit.desktopServiceUUID,
		DesktopAddr:           testDesktop.GetAddr(),
		Domain:                testDesktop.GetDomain(),
		WindowsUser:           "Administrator",
		DesktopLabels:         map[string]string{"env": "production"},
		StartTime:             audit.startTime,
		EndTime:               audit.clock.Now().UTC(),
		DesktopName:           testDesktop.GetName(),
		Recorded:              true,
		Participants:          []string{"foo"},
	}
	require.Empty(t, cmp.Diff(expected, endEvent))
}

func TestDesktopSharedDirectoryStartEvent(t *testing.T) {
	for _, test := range []struct {
		name string
		// sendsAnnounce determines whether a SharedDirectoryAnnounce is sent.
		sendsAnnounce bool
		// errCode is the error code in the simulated SharedDirectoryAcknowledge
		errCode uint32
		// expected returns the event we expect to be emitted by modifying baseEvent
		// (which is passed in from the test body below).
		expected func(baseEvent *events.DesktopSharedDirectoryStart) *events.DesktopSharedDirectoryStart
	}{
		{
			// when everything is working as expected
			name:          "typical operation",
			sendsAnnounce: true,
			errCode:       tdp.ErrCodeNil,
			expected: func(baseEvent *events.DesktopSharedDirectoryStart) *events.DesktopSharedDirectoryStart {
				return baseEvent
			},
		},
		{
			// the announce operation failed
			name:          "announce failed",
			sendsAnnounce: true,
			errCode:       tdp.ErrCodeFailed,
			expected: func(baseEvent *events.DesktopSharedDirectoryStart) *events.DesktopSharedDirectoryStart {
				baseEvent.Metadata.Code = libevents.DesktopSharedDirectoryStartFailureCode
				return baseEvent
			},
		},
		{
			// should never happen but just in case
			name:          "directory name unknown",
			sendsAnnounce: false,
			errCode:       tdp.ErrCodeNil,
			expected: func(baseEvent *events.DesktopSharedDirectoryStart) *events.DesktopSharedDirectoryStart {
				baseEvent.Metadata.Code = libevents.DesktopSharedDirectoryStartFailureCode
				baseEvent.DirectoryName = "unknown"
				return baseEvent
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			id, audit := setup(testDesktop)

			if test.sendsAnnounce {
				// SharedDirectoryAnnounce initializes the nameCache.
				audit.onSharedDirectoryAnnounce(tdp.SharedDirectoryAnnounce{
					DirectoryID: uint32(testDirectoryID),
					Name:        testDirName,
				})
			}

			// SharedDirectoryAcknowledge causes the event to be emitted
			startEvent := audit.makeSharedDirectoryStart(tdp.SharedDirectoryAcknowledge{
				DirectoryID: uint32(testDirectoryID),
				ErrCode:     test.errCode,
			})

			baseEvent := &events.DesktopSharedDirectoryStart{
				Metadata: events.Metadata{
					Type:        libevents.DesktopSharedDirectoryStartEvent,
					Code:        libevents.DesktopSharedDirectoryStartCode,
					ClusterName: audit.clusterName,
					Time:        audit.clock.Now().UTC(),
				},
				UserMetadata: id.GetUserMetadata(),
				SessionMetadata: events.SessionMetadata{
					SessionID: audit.sessionID,
					WithMFA:   id.MFAVerified,
				},
				ConnectionMetadata: events.ConnectionMetadata{
					LocalAddr:  id.LoginIP,
					RemoteAddr: audit.desktop.GetAddr(),
					Protocol:   libevents.EventProtocolTDP,
				},
				Status:        statusFromErrCode(test.errCode),
				DesktopAddr:   audit.desktop.GetAddr(),
				DirectoryName: testDirName,
				DirectoryID:   uint32(testDirectoryID),
			}

			expected := test.expected(baseEvent)
			require.Empty(t, cmp.Diff(expected, startEvent))
		})
	}
}

func TestDesktopSharedDirectoryReadEvent(t *testing.T) {
	for _, test := range []struct {
		name string
		// sendsAnnounce determines whether a SharedDirectoryAnnounce is sent.
		sendsAnnounce bool
		// sendsReq determines whether a SharedDirectoryReadRequest is sent.
		sendsReq bool
		// errCode is the error code in the simulated SharedDirectoryReadResponse
		errCode uint32
		// expected returns the event we expect to be emitted by modifying baseEvent
		// (which is passed in from the test body below).
		expected func(baseEvent *events.DesktopSharedDirectoryRead) *events.DesktopSharedDirectoryRead
	}{
		{
			// when everything is working as expected
			name:          "typical operation",
			sendsAnnounce: true,
			sendsReq:      true,
			errCode:       tdp.ErrCodeNil,
			expected: func(baseEvent *events.DesktopSharedDirectoryRead) *events.DesktopSharedDirectoryRead {
				return baseEvent
			},
		},
		{
			// the read operation failed
			name:          "read failed",
			sendsAnnounce: true,
			sendsReq:      true,
			errCode:       tdp.ErrCodeFailed,
			expected: func(baseEvent *events.DesktopSharedDirectoryRead) *events.DesktopSharedDirectoryRead {
				baseEvent.Metadata.Code = libevents.DesktopSharedDirectoryWriteFailureCode
				return baseEvent
			},
		},
		{
			// should never happen but just in case
			name:          "directory name unknown",
			sendsAnnounce: false,
			sendsReq:      true,
			errCode:       tdp.ErrCodeNil,
			expected: func(baseEvent *events.DesktopSharedDirectoryRead) *events.DesktopSharedDirectoryRead {
				baseEvent.Metadata.Code = libevents.DesktopSharedDirectoryReadFailureCode
				baseEvent.DirectoryName = "unknown"
				return baseEvent
			},
		},
		{
			// should never happen but just in case
			name:          "request info unknown",
			sendsAnnounce: true,
			sendsReq:      false,
			errCode:       tdp.ErrCodeNil,
			expected: func(baseEvent *events.DesktopSharedDirectoryRead) *events.DesktopSharedDirectoryRead {
				baseEvent.Metadata.Code = libevents.DesktopSharedDirectoryReadFailureCode

				// resorts to default values for these
				baseEvent.DirectoryID = 0
				baseEvent.Offset = 0

				// sets "unknown" for these
				baseEvent.Path = "unknown"
				// we can't retrieve the directory name because we don't have the directoryID
				baseEvent.DirectoryName = "unknown"

				return baseEvent
			},
		},
		{
			// should never happen but just in case
			name:          "directory name and request info unknown",
			sendsAnnounce: false,
			sendsReq:      false,
			errCode:       tdp.ErrCodeNil,
			expected: func(baseEvent *events.DesktopSharedDirectoryRead) *events.DesktopSharedDirectoryRead {
				baseEvent.Metadata.Code = libevents.DesktopSharedDirectoryReadFailureCode

				// resorts to default values for these
				baseEvent.DirectoryID = 0
				baseEvent.Offset = 0

				// sets "unknown" for these
				baseEvent.Path = "unknown"
				baseEvent.DirectoryName = "unknown"

				return baseEvent
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			id, audit := setup(testDesktop)

			if test.sendsAnnounce {
				// SharedDirectoryAnnounce initializes the name cache
				audit.onSharedDirectoryAnnounce(tdp.SharedDirectoryAnnounce{
					DirectoryID: uint32(testDirectoryID),
					Name:        testDirName,
				})
			}

			if test.sendsReq {
				// SharedDirectoryReadRequest initializes the readRequestCache.
				audit.onSharedDirectoryReadRequest(tdp.SharedDirectoryReadRequest{
					CompletionID: uint32(testCompletionID),
					DirectoryID:  uint32(testDirectoryID),
					Path:         testFilePath,
					Offset:       testOffset,
					Length:       testLength,
				})
			}

			// SharedDirectoryReadResponse causes the event to be emitted.
			readEvent := audit.makeSharedDirectoryReadResponse(tdp.SharedDirectoryReadResponse{
				CompletionID:   uint32(testCompletionID),
				ErrCode:        test.errCode,
				ReadDataLength: testLength,
				ReadData:       []byte{}, // irrelevant in this context
			})

			baseEvent := &events.DesktopSharedDirectoryRead{
				Metadata: events.Metadata{
					Type:        libevents.DesktopSharedDirectoryReadEvent,
					Code:        libevents.DesktopSharedDirectoryReadCode,
					ClusterName: audit.clusterName,
					Time:        audit.clock.Now().UTC(),
				},
				UserMetadata: id.GetUserMetadata(),
				SessionMetadata: events.SessionMetadata{
					SessionID: audit.sessionID,
					WithMFA:   id.MFAVerified,
				},
				ConnectionMetadata: events.ConnectionMetadata{
					LocalAddr:  id.LoginIP,
					RemoteAddr: audit.desktop.GetAddr(),
					Protocol:   libevents.EventProtocolTDP,
				},
				Status:        statusFromErrCode(test.errCode),
				DesktopAddr:   audit.desktop.GetAddr(),
				DirectoryName: testDirName,
				DirectoryID:   uint32(testDirectoryID),
				Path:          testFilePath,
				Length:        testLength,
				Offset:        testOffset,
			}

			require.Empty(t, cmp.Diff(test.expected(baseEvent), readEvent))
		})
	}
}

func TestDesktopSharedDirectoryWriteEvent(t *testing.T) {
	for _, test := range []struct {
		name string
		// sendsAnnounce determines whether a SharedDirectoryAnnounce is sent.
		sendsAnnounce bool
		// sendsReq determines whether a SharedDirectoryWriteRequest is sent.
		sendsReq bool
		// errCode is the error code in the simulated SharedDirectoryWriteResponse
		errCode uint32
		// expected returns the event we expect to be emitted by modifying baseEvent
		// (which is passed in from the test body below).
		expected func(baseEvent *events.DesktopSharedDirectoryWrite) *events.DesktopSharedDirectoryWrite
	}{
		{
			// when everything is working as expected
			name:          "typical operation",
			sendsAnnounce: true,
			sendsReq:      true,
			errCode:       tdp.ErrCodeNil,
			expected: func(baseEvent *events.DesktopSharedDirectoryWrite) *events.DesktopSharedDirectoryWrite {
				return baseEvent
			},
		},
		{
			// the Write operation failed
			name:          "write failed",
			sendsAnnounce: true,
			sendsReq:      true,
			errCode:       tdp.ErrCodeFailed,
			expected: func(baseEvent *events.DesktopSharedDirectoryWrite) *events.DesktopSharedDirectoryWrite {
				baseEvent.Metadata.Code = libevents.DesktopSharedDirectoryWriteFailureCode
				return baseEvent
			},
		},
		{
			// should never happen but just in case
			name:          "directory name unknown",
			sendsAnnounce: false,
			sendsReq:      true,
			errCode:       tdp.ErrCodeNil,
			expected: func(baseEvent *events.DesktopSharedDirectoryWrite) *events.DesktopSharedDirectoryWrite {
				baseEvent.Metadata.Code = libevents.DesktopSharedDirectoryWriteFailureCode
				baseEvent.DirectoryName = "unknown"
				return baseEvent
			},
		},
		{
			// should never happen but just in case
			name:          "request info unknown",
			sendsAnnounce: true,
			sendsReq:      false,
			errCode:       tdp.ErrCodeNil,
			expected: func(baseEvent *events.DesktopSharedDirectoryWrite) *events.DesktopSharedDirectoryWrite {
				baseEvent.Metadata.Code = libevents.DesktopSharedDirectoryWriteFailureCode

				// resorts to default values for these
				baseEvent.DirectoryID = 0
				baseEvent.Offset = 0

				// sets "unknown" for these
				baseEvent.Path = "unknown"
				// we can't retrieve the directory name because we don't have the directoryID
				baseEvent.DirectoryName = "unknown"

				return baseEvent
			},
		},
		{
			// should never happen but just in case
			name:          "directory name and request info unknown",
			sendsAnnounce: false,
			sendsReq:      false,
			errCode:       tdp.ErrCodeNil,
			expected: func(baseEvent *events.DesktopSharedDirectoryWrite) *events.DesktopSharedDirectoryWrite {
				baseEvent.Metadata.Code = libevents.DesktopSharedDirectoryWriteFailureCode

				// resorts to default values for these
				baseEvent.DirectoryID = 0
				baseEvent.Offset = 0

				// sets "unknown" for these
				baseEvent.Path = "unknown"
				baseEvent.DirectoryName = "unknown"

				return baseEvent
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			id, audit := setup(testDesktop)

			if test.sendsAnnounce {
				// SharedDirectoryAnnounce initializes the nameCache.
				audit.onSharedDirectoryAnnounce(tdp.SharedDirectoryAnnounce{
					DirectoryID: uint32(testDirectoryID),
					Name:        testDirName,
				})
			}

			if test.sendsReq {
				// SharedDirectoryWriteRequest initializes the writeRequestCache.
				audit.onSharedDirectoryWriteRequest(tdp.SharedDirectoryWriteRequest{
					CompletionID:    uint32(testCompletionID),
					DirectoryID:     uint32(testDirectoryID),
					Path:            testFilePath,
					Offset:          testOffset,
					WriteDataLength: testLength,
				})
			}

			// SharedDirectoryWriteResponse causes the event to be emitted.
			writeEvent := audit.makeSharedDirectoryWriteResponse(tdp.SharedDirectoryWriteResponse{
				CompletionID: uint32(testCompletionID),
				ErrCode:      test.errCode,
				BytesWritten: testLength,
			})

			baseEvent := &events.DesktopSharedDirectoryWrite{
				Metadata: events.Metadata{
					Type:        libevents.DesktopSharedDirectoryWriteEvent,
					Code:        libevents.DesktopSharedDirectoryWriteCode,
					ClusterName: audit.clusterName,
					Time:        audit.clock.Now().UTC(),
				},
				UserMetadata: id.GetUserMetadata(),
				SessionMetadata: events.SessionMetadata{
					SessionID: audit.sessionID,
					WithMFA:   id.MFAVerified,
				},
				ConnectionMetadata: events.ConnectionMetadata{
					LocalAddr:  id.LoginIP,
					RemoteAddr: audit.desktop.GetAddr(),
					Protocol:   libevents.EventProtocolTDP,
				},
				Status:        statusFromErrCode(test.errCode),
				DesktopAddr:   audit.desktop.GetAddr(),
				DirectoryName: testDirName,
				DirectoryID:   uint32(testDirectoryID),
				Path:          testFilePath,
				Length:        testLength,
				Offset:        testOffset,
			}

			require.Empty(t, cmp.Diff(test.expected(baseEvent), writeEvent))
		})
	}
}

// fillReadRequestCache is a helper function that fills an entry's readRequestCache up with entryMaxItems.
func fillReadRequestCache(cache *sharedDirectoryAuditCache, did directoryID) {
	cache.Lock()
	defer cache.Unlock()

	for i := 0; i < maxAuditCacheItems; i++ {
		cache.readRequestCache[completionID(i)] = readRequestInfo{
			directoryID: did,
		}
	}
}

// TestDesktopSharedDirectoryStartEventAuditCacheMax tests that a
// failed DesktopSharedDirectoryStart is emitted when the shared
// directory audit cache is full.
func TestDesktopSharedDirectoryStartEventAuditCacheMax(t *testing.T) {

	id, audit := setup(testDesktop)

	// Set the audit cache entry to the maximum allowable size
	fillReadRequestCache(&audit.auditCache, testDirectoryID)

	// Send a SharedDirectoryAnnounce
	startEvent := audit.onSharedDirectoryAnnounce(tdp.SharedDirectoryAnnounce{
		DirectoryID: uint32(testDirectoryID),
		Name:        testDirName,
	})
	require.NotNil(t, startEvent)

	// Expect the audit cache to emit a failed DesktopSharedDirectoryStart
	// with a status detailing the security problem.
	expected := &events.DesktopSharedDirectoryStart{
		Metadata: events.Metadata{
			Type:        libevents.DesktopSharedDirectoryStartEvent,
			Code:        libevents.DesktopSharedDirectoryStartFailureCode,
			ClusterName: audit.clusterName,
			Time:        audit.clock.Now().UTC(),
		},
		UserMetadata: id.GetUserMetadata(),
		SessionMetadata: events.SessionMetadata{
			SessionID: audit.sessionID,
			WithMFA:   id.MFAVerified,
		},
		ConnectionMetadata: events.ConnectionMetadata{
			LocalAddr:  id.LoginIP,
			RemoteAddr: audit.desktop.GetAddr(),
			Protocol:   libevents.EventProtocolTDP,
		},
		Status: events.Status{
			Success:     false,
			Error:       "audit cache exceeded maximum size",
			UserMessage: "Teleport failed the request and terminated the session as a security precaution",
		},
		DesktopAddr:   audit.desktop.GetAddr(),
		DirectoryName: testDirName,
		DirectoryID:   uint32(testDirectoryID),
	}

	require.Empty(t, cmp.Diff(expected, startEvent))
}

// TestDesktopSharedDirectoryReadEventAuditCacheMax tests that a
// failed DesktopSharedDirectoryRead is generated when the shared
// directory audit cache is full.
func TestDesktopSharedDirectoryReadEventAuditCacheMax(t *testing.T) {

	id, audit := setup(testDesktop)

	// Send a SharedDirectoryAnnounce
	audit.onSharedDirectoryAnnounce(tdp.SharedDirectoryAnnounce{
		DirectoryID: uint32(testDirectoryID),
		Name:        testDirName,
	})

	// Set the audit cache entry to the maximum allowable size
	fillReadRequestCache(&audit.auditCache, testDirectoryID)

	// SharedDirectoryReadRequest should cause a failed audit event.
	readEvent := audit.onSharedDirectoryReadRequest(tdp.SharedDirectoryReadRequest{
		CompletionID: uint32(testCompletionID),
		DirectoryID:  uint32(testDirectoryID),
		Path:         testFilePath,
		Offset:       testOffset,
		Length:       testLength,
	})
	require.NotNil(t, readEvent)

	expected := &events.DesktopSharedDirectoryRead{
		Metadata: events.Metadata{
			Type:        libevents.DesktopSharedDirectoryReadEvent,
			Code:        libevents.DesktopSharedDirectoryReadFailureCode,
			ClusterName: audit.clusterName,
			Time:        audit.clock.Now().UTC(),
		},
		UserMetadata: id.GetUserMetadata(),
		SessionMetadata: events.SessionMetadata{
			SessionID: audit.sessionID,
			WithMFA:   id.MFAVerified,
		},
		ConnectionMetadata: events.ConnectionMetadata{
			LocalAddr:  id.LoginIP,
			RemoteAddr: audit.desktop.GetAddr(),
			Protocol:   libevents.EventProtocolTDP,
		},
		Status: events.Status{
			Success:     false,
			Error:       "audit cache exceeded maximum size",
			UserMessage: "Teleport failed the request and terminated the session as a security precaution",
		},
		DesktopAddr:   audit.desktop.GetAddr(),
		DirectoryName: testDirName,
		DirectoryID:   uint32(testDirectoryID),
		Path:          testFilePath,
		Length:        testLength,
		Offset:        testOffset,
	}

	require.Empty(t, cmp.Diff(expected, readEvent))
}

// TestDesktopSharedDirectoryWriteEventAuditCacheMax tests that a
// failed DesktopSharedDirectoryWrite is generated when the shared
// directory audit cache is full.
func TestDesktopSharedDirectoryWriteEventAuditCacheMax(t *testing.T) {

	id, audit := setup(testDesktop)

	audit.onSharedDirectoryAnnounce(tdp.SharedDirectoryAnnounce{
		DirectoryID: uint32(testDirectoryID),
		Name:        testDirName,
	})

	fillReadRequestCache(&audit.auditCache, testDirectoryID)

	writeEvent := audit.onSharedDirectoryWriteRequest(tdp.SharedDirectoryWriteRequest{
		CompletionID:    uint32(testCompletionID),
		DirectoryID:     uint32(testDirectoryID),
		Path:            testFilePath,
		Offset:          testOffset,
		WriteDataLength: testLength,
	})
	require.NotNil(t, writeEvent, "audit event should have been generated")

	expected := &events.DesktopSharedDirectoryWrite{
		Metadata: events.Metadata{
			Type:        libevents.DesktopSharedDirectoryWriteEvent,
			Code:        libevents.DesktopSharedDirectoryWriteFailureCode,
			ClusterName: audit.clusterName,
			Time:        audit.clock.Now().UTC(),
		},
		UserMetadata: id.GetUserMetadata(),
		SessionMetadata: events.SessionMetadata{
			SessionID: audit.sessionID,
			WithMFA:   id.MFAVerified,
		},
		ConnectionMetadata: events.ConnectionMetadata{
			LocalAddr:  id.LoginIP,
			RemoteAddr: audit.desktop.GetAddr(),
			Protocol:   libevents.EventProtocolTDP,
		},
		Status: events.Status{
			Success:     false,
			Error:       "audit cache exceeded maximum size",
			UserMessage: "Teleport failed the request and terminated the session as a security precaution",
		},
		DesktopAddr:   audit.desktop.GetAddr(),
		DirectoryName: testDirName,
		DirectoryID:   uint32(testDirectoryID),
		Path:          testFilePath,
		Length:        testLength,
		Offset:        testOffset,
	}

	require.Empty(t, cmp.Diff(expected, writeEvent))
}

// TestAuditCacheLifecycle confirms that the audit cache operates correctly
// in response to protocol events.
func TestAuditCacheLifecycle(t *testing.T) {
	_, audit := setup(testDesktop)

	// SharedDirectoryAnnounce initializes the nameCache.
	audit.onSharedDirectoryAnnounce(tdp.SharedDirectoryAnnounce{
		DirectoryID: uint32(testDirectoryID),
		Name:        testDirName,
	})

	// Confirm that audit cache is in the expected state.
	require.Equal(t, 1, audit.auditCache.totalItems())
	name, ok := audit.auditCache.GetName(testDirectoryID)
	require.True(t, ok)
	require.Equal(t, directoryName(testDirName), name)
	_, ok = audit.auditCache.TakeReadRequestInfo(testCompletionID)
	require.False(t, ok)
	_, ok = audit.auditCache.TakeWriteRequestInfo(testCompletionID)
	require.False(t, ok)

	// A SharedDirectoryReadRequest should add a corresponding entry in the readRequestCache.
	audit.onSharedDirectoryReadRequest(tdp.SharedDirectoryReadRequest{
		CompletionID: uint32(testCompletionID),
		DirectoryID:  uint32(testDirectoryID),
		Path:         testFilePath,
		Offset:       testOffset,
		Length:       testLength,
	})
	require.Equal(t, 2, audit.auditCache.totalItems())

	// A SharedDirectoryWriteRequest should add a corresponding entry in the writeRequestCache.
	audit.onSharedDirectoryWriteRequest(tdp.SharedDirectoryWriteRequest{
		CompletionID:    uint32(testCompletionID),
		DirectoryID:     uint32(testDirectoryID),
		Path:            testFilePath,
		Offset:          testOffset,
		WriteDataLength: testLength,
	})
	require.Equal(t, 3, audit.auditCache.totalItems())

	// Check that the readRequestCache was properly filled out.
	require.Contains(t, audit.auditCache.readRequestCache, testCompletionID)

	// Check that the writeRequestCache was properly filled out.
	require.Contains(t, audit.auditCache.writeRequestCache, testCompletionID)

	// SharedDirectoryReadResponse should cause the entry in the readRequestCache to be cleaned up.
	audit.makeSharedDirectoryReadResponse(tdp.SharedDirectoryReadResponse{
		CompletionID:   uint32(testCompletionID),
		ErrCode:        tdp.ErrCodeNil,
		ReadDataLength: testLength,
		ReadData:       []byte{}, // irrelevant in this context
	})
	require.Equal(t, 2, audit.auditCache.totalItems())

	// SharedDirectoryWriteResponse should cause the entry in the writeRequestCache to be cleaned up.
	audit.makeSharedDirectoryWriteResponse(tdp.SharedDirectoryWriteResponse{
		CompletionID: uint32(testCompletionID),
		ErrCode:      tdp.ErrCodeNil,
		BytesWritten: testLength,
	})
	require.Equal(t, 1, audit.auditCache.totalItems())

	// Check that the readRequestCache was properly cleaned up.
	require.NotContains(t, audit.auditCache.readRequestCache, testCompletionID)

	// Check that the writeRequestCache was properly cleaned up.
	require.NotContains(t, audit.auditCache.writeRequestCache, testCompletionID)
}
