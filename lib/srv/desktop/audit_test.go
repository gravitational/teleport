/*
Copyright 2021 Gravitational, Inc.

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

package desktop

import (
	"context"
	"crypto/rand"
	"io"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

type testSetup struct {
	s           *WindowsService
	id          *tlsca.Identity
	emitter     *eventstest.MockEmitter
	sessionID   string
	desktopAddr string
	dirID       uint32
	dirName     string
	sendHandler func(m tdp.Message, b []byte)
	recvHandler func(m tdp.Message)
}

func setup() testSetup {
	emitter := &eventstest.MockEmitter{}
	log := logrus.New()
	log.SetOutput(io.Discard)

	s := &WindowsService{
		clusterName: "test-cluster",
		cfg: WindowsServiceConfig{
			Log:     log,
			Emitter: emitter,
			Heartbeat: HeartbeatConfig{
				HostUUID: "test-host-id",
			},
			Clock: clockwork.NewFakeClockAt(time.Now()),
		},
		sdMap: newSharedDirectoryNameMap(log),
	}

	id := &tlsca.Identity{
		Username:     "foo",
		Impersonator: "bar",
		MFAVerified:  "mfa-id",
		ClientIP:     "127.0.0.1",
	}
	sessionID, desktopAddr, dirID, dirName := "session-0", "windows.example.com", uint32(2), "test-dir"

	sendHandler := s.makeTDPSendHandler(context.Background(),
		emitter, func() int64 { return 0 },
		id, sessionID, desktopAddr)
	recvHandler := s.makeTDPReceiveHandler(context.Background(),
		emitter, func() int64 { return 0 },
		id, sessionID, desktopAddr)

	return testSetup{s, id, emitter, sessionID, desktopAddr, dirID, dirName, sendHandler, recvHandler}
}

func TestSessionStartEvent(t *testing.T) {
	su := setup()
	s, id, emitter := su.s, su.id, su.emitter

	desktop := &types.WindowsDesktopV3{
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

	userMeta := id.GetUserMetadata()
	userMeta.Login = "Administrator"
	expected := &events.WindowsDesktopSessionStart{
		Metadata: events.Metadata{
			ClusterName: s.clusterName,
			Type:        libevents.WindowsDesktopSessionStartEvent,
			Code:        libevents.DesktopSessionStartCode,
			Time:        s.cfg.Clock.Now().UTC().Round(time.Millisecond),
		},
		UserMetadata: userMeta,
		SessionMetadata: events.SessionMetadata{
			SessionID: su.sessionID,
			WithMFA:   id.MFAVerified,
		},
		ConnectionMetadata: events.ConnectionMetadata{
			LocalAddr:  id.ClientIP,
			RemoteAddr: desktop.GetAddr(),
			Protocol:   libevents.EventProtocolTDP,
		},
		Status: events.Status{
			Success: true,
		},
		WindowsDesktopService: s.cfg.Heartbeat.HostUUID,
		DesktopName:           "test-desktop",
		DesktopAddr:           desktop.GetAddr(),
		Domain:                desktop.GetDomain(),
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
			s.onSessionStart(
				context.Background(),
				s.cfg.Emitter,
				id,
				s.cfg.Clock.Now().UTC().Round(time.Millisecond),
				"Administrator",
				su.sessionID,
				desktop,
				test.err,
			)

			event := emitter.LastEvent()
			require.NotNil(t, event)

			startEvent, ok := event.(*events.WindowsDesktopSessionStart)
			require.True(t, ok)

			require.Empty(t, cmp.Diff(test.exp(), *startEvent))
		})
	}
}

func TestSessionEndEvent(t *testing.T) {
	su := setup()
	s, id, emitter := su.s, su.id, su.emitter

	desktop := &types.WindowsDesktopV3{
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

	c := clockwork.NewFakeClockAt(time.Now())
	s.cfg.Clock = c
	startTime := s.cfg.Clock.Now().UTC().Round(time.Millisecond)
	c.Advance(30 * time.Second)

	s.onSessionEnd(
		context.Background(),
		s.cfg.Emitter,
		id,
		startTime,
		true,
		"Administrator",
		"sessionID",
		desktop,
	)

	event := emitter.LastEvent()
	require.NotNil(t, event)
	endEvent, ok := event.(*events.WindowsDesktopSessionEnd)
	require.True(t, ok)

	userMeta := id.GetUserMetadata()
	userMeta.Login = "Administrator"
	expected := &events.WindowsDesktopSessionEnd{
		Metadata: events.Metadata{
			ClusterName: s.clusterName,
			Type:        libevents.WindowsDesktopSessionEndEvent,
			Code:        libevents.DesktopSessionEndCode,
		},
		UserMetadata: userMeta,
		SessionMetadata: events.SessionMetadata{
			SessionID: "sessionID",
			WithMFA:   id.MFAVerified,
		},
		WindowsDesktopService: s.cfg.Heartbeat.HostUUID,
		DesktopAddr:           desktop.GetAddr(),
		Domain:                desktop.GetDomain(),
		WindowsUser:           "Administrator",
		DesktopLabels:         map[string]string{"env": "production"},
		StartTime:             startTime,
		EndTime:               c.Now().UTC().Round(time.Millisecond),
		DesktopName:           desktop.GetName(),
		Recorded:              true,
		Participants:          []string{"foo"},
	}
	require.Empty(t, cmp.Diff(expected, endEvent))
}

func TestEmitsRecordingEventsOnSend(t *testing.T) {
	su := setup()
	emitter, handler := su.emitter, su.sendHandler

	// a fake PNG Frame message
	encoded := []byte{byte(tdp.TypePNGFrame), 0x01, 0x02}

	// the handler accepts both the message structure and its encoded form,
	// but our logic only depends on the encoded form, so pass a nil message
	var msg tdp.Message
	handler(msg, encoded)

	e := emitter.LastEvent()
	require.NotNil(t, e)
	dr, ok := e.(*events.DesktopRecording)
	require.True(t, ok)
	require.Equal(t, encoded, dr.Message)
}

func TestSkipsExtremelyLargePNGs(t *testing.T) {
	su := setup()
	emitter, handler := su.emitter, su.sendHandler

	// a fake PNG Frame message, which is way too big to be legitimate
	maliciousPNG := make([]byte, libevents.MaxProtoMessageSizeBytes+1)
	rand.Read(maliciousPNG)
	maliciousPNG[0] = byte(tdp.TypePNGFrame)

	// the handler accepts both the message structure and its encoded form,
	// but our logic only depends on the encoded form, so pass a nil message
	var msg tdp.Message
	handler(msg, maliciousPNG)

	require.Nil(t, emitter.LastEvent())
}

func TestEmitsRecordingEventsOnReceive(t *testing.T) {
	su := setup()
	emitter, handler := su.emitter, su.recvHandler

	msg := tdp.MouseButton{
		Button: tdp.LeftMouseButton,
		State:  tdp.ButtonPressed,
	}
	handler(msg)

	e := emitter.LastEvent()
	require.NotNil(t, e)
	dr, ok := e.(*events.DesktopRecording)
	require.True(t, ok)
	decoded, err := tdp.Decode(dr.Message)
	require.NoError(t, err)
	require.Equal(t, msg, decoded)
}

func TestEmitsClipboardSendEvents(t *testing.T) {
	su := setup()
	s, emitter, handler := su.s, su.emitter, su.recvHandler

	fakeClipboardData := make([]byte, 1024)
	rand.Read(fakeClipboardData)

	start := s.cfg.Clock.Now().UTC()
	msg := tdp.ClipboardData(fakeClipboardData)
	handler(msg)

	e := emitter.LastEvent()
	require.NotNil(t, e)
	cs, ok := e.(*events.DesktopClipboardSend)
	require.True(t, ok)
	require.Equal(t, int32(len(fakeClipboardData)), cs.Length)
	require.Equal(t, su.sessionID, cs.SessionID)
	require.Equal(t, su.desktopAddr, cs.DesktopAddr)
	require.Equal(t, s.clusterName, cs.ClusterName)
	require.Equal(t, start, cs.Time)
}

func TestEmitsClipboardReceiveEvents(t *testing.T) {
	su := setup()
	s, emitter, handler := su.s, su.emitter, su.sendHandler

	fakeClipboardData := make([]byte, 512)
	rand.Read(fakeClipboardData)

	start := s.cfg.Clock.Now().UTC()
	msg := tdp.ClipboardData(fakeClipboardData)
	encoded, err := msg.Encode()
	require.NoError(t, err)
	handler(msg, encoded)

	e := emitter.LastEvent()
	require.NotNil(t, e)
	cs, ok := e.(*events.DesktopClipboardReceive)
	require.True(t, ok)
	require.Equal(t, int32(len(fakeClipboardData)), cs.Length)
	require.Equal(t, su.sessionID, cs.SessionID)
	require.Equal(t, su.desktopAddr, cs.DesktopAddr)
	require.Equal(t, s.clusterName, cs.ClusterName)
	require.Equal(t, start, cs.Time)
}

func TestEmitsDesktopSharedDirectoryStartEvents(t *testing.T) {
	for _, test := range []struct {
		desc        string
		initialized bool
		errCode     int
		succeeded   bool
	}{
		{
			desc:        "directory sharing initialization succeeded",
			initialized: true,
			errCode:     tdp.ErrCodeNil,
			succeeded:   true,
		},
		{
			desc:        "directory sharing initialization failed",
			initialized: true,
			errCode:     tdp.ErrCodeFailed,
			succeeded:   false,
		},
		{
			desc:        "directory name cache somehow became out of sync",
			initialized: false,
			errCode:     tdp.ErrCodeNil,
			succeeded:   true,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			su := setup()

			if test.initialized {
				// Initialize the sdMap by simulating a SharedDirectoryAnnounce
				su.recvHandler(tdp.SharedDirectoryAnnounce{
					DirectoryID: su.dirID,
					Name:        su.dirName,
				})
			}

			// Simulate a successful SharedDirectoryAcknowledge,
			// which should cause a DesktopSharedDirectoryStart
			// to be emitted.
			msg := tdp.SharedDirectoryAcknowledge{
				ErrCode:     uint32(test.errCode),
				DirectoryID: su.dirID,
			}
			encoded, err := msg.Encode()
			require.NoError(t, err)
			su.sendHandler(msg, encoded)

			e := su.emitter.LastEvent()
			require.NotNil(t, e)
			sds, ok := e.(*events.DesktopSharedDirectoryStart)
			require.True(t, ok)
			require.Equal(t, test.succeeded, sds.Succeeded)
			require.Equal(t, su.desktopAddr, sds.DesktopAddr)
			if test.initialized {
				require.Equal(t, su.dirName, sds.DirectoryName)
			} else {
				require.Equal(t, unknownName, sds.DirectoryName)
			}
			require.Equal(t, su.dirID, sds.DirectoryID)

		})
	}
}
