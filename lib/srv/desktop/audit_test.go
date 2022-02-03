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
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func setup() (*WindowsService, *tlsca.Identity, *libevents.MockEmitter) {
	emitter := &libevents.MockEmitter{}
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
		},
	}

	id := &tlsca.Identity{
		Username:     "foo",
		Impersonator: "bar",
		MFAVerified:  "mfa-id",
		ClientIP:     "127.0.0.1",
	}

	return s, id, emitter
}

func TestSessionStartEvent(t *testing.T) {
	s, id, emitter := setup()

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

	expected := &events.WindowsDesktopSessionStart{
		Metadata: events.Metadata{
			ClusterName: s.clusterName,
			Type:        libevents.WindowsDesktopSessionStartEvent,
			Code:        libevents.DesktopSessionStartCode,
		},
		UserMetadata: id.GetUserMetadata(),
		SessionMetadata: events.SessionMetadata{
			SessionID: "sessionID",
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
				id,
				"Administrator",
				"sessionID",
				desktop,
				test.err,
			)

			event := emitter.LastEvent()
			require.NotNil(t, event)

			startEvent, ok := event.(*events.WindowsDesktopSessionStart)
			require.True(t, ok)

			require.Empty(t, cmp.Diff(test.exp(), *startEvent,
				cmpopts.IgnoreFields(events.Metadata{}, "Time"),
			))
		})
	}
}

func TestSessionEndEvent(t *testing.T) {
	s, id, emitter := setup()

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

	s.onSessionEnd(
		context.Background(),
		id,
		"Administrator",
		"sessionID",
		desktop,
	)

	event := emitter.LastEvent()
	require.NotNil(t, event)
	endEvent, ok := event.(*events.WindowsDesktopSessionEnd)
	require.True(t, ok)

	expected := &events.WindowsDesktopSessionEnd{
		Metadata: events.Metadata{
			ClusterName: s.clusterName,
			Type:        libevents.WindowsDesktopSessionEndEvent,
			Code:        libevents.DesktopSessionEndCode,
		},
		UserMetadata: id.GetUserMetadata(),
		SessionMetadata: events.SessionMetadata{
			SessionID: "sessionID",
			WithMFA:   id.MFAVerified,
		},
		WindowsDesktopService: s.cfg.Heartbeat.HostUUID,
		DesktopAddr:           desktop.GetAddr(),
		Domain:                desktop.GetDomain(),
		WindowsUser:           "Administrator",
		DesktopLabels:         map[string]string{"env": "production"},
	}
	require.Empty(t, cmp.Diff(expected, endEvent))
}
