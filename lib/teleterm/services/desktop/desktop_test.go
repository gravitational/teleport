// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package desktop

import (
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

var desktopURI = uri.NewClusterURI("foo").AppendWindowsDesktop("bar")
var login = "admin"

func TestSetDirectory(t *testing.T) {
	path := t.TempDir()
	session, err := NewSession(desktopURI, login)
	require.NoError(t, err)

	err = session.SetSharedDirectory(path)
	require.NoError(t, err)

	err = session.SetSharedDirectory("any_path")
	require.True(t, trace.IsAlreadyExists(err))
}

func TestGetDirectory(t *testing.T) {
	path := t.TempDir()
	session, err := NewSession(desktopURI, login)
	require.NoError(t, err)

	_, err = session.GetDirectoryAccess()
	require.True(t, trace.IsNotFound(err))

	err = session.SetSharedDirectory(path)
	require.NoError(t, err)

	access, err := session.GetDirectoryAccess()
	require.NoError(t, err)
	_, err = access.Stat("")
	require.NoError(t, err)
}

func TestTDPBInBandMFA_MessageExchange(t *testing.T) {
	t.Parallel()

	clientPipe, serverPipe := net.Pipe()
	defer clientPipe.Close()
	defer serverPipe.Close()

	go func() {
		conn := tdp.NewConn(serverPipe, tdp.DecoderAdapter(tdpb.DecodePermissive), tdpb.WarningConstructor)

		msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if _, ok := msg.(*tdpb.ClientHello); !ok {
			return
		}

		if err := conn.WriteMessage((*tdpb.AuthPrompt)(newAuthPrompt())); err != nil {
			return
		}

		resp, err := conn.ReadMessage()
		if err != nil {
			return
		}
		mfaResp, ok := resp.(*tdpb.MFAPromptResponse)
		if !ok {
			return
		}
		if (*tdpbv1.MFAPromptResponse)(mfaResp).GetReference().GetChallengeName() != "test-challenge" {
			return
		}

		if err := conn.WriteMessage(&tdpb.SessionEstablishing{}); err != nil {
			return
		}
		if err := conn.WriteMessage(&tdpb.ServerHello{ClipboardEnabled: true}); err != nil {
			return
		}
	}()

	conn := tdp.NewConn(clientPipe, tdp.DecoderAdapter(tdpb.DecodePermissive), tdpb.WarningConstructor)

	if err := conn.WriteMessage(newTestClientHello()); err != nil {
		t.Fatalf("failed to send ClientHello: %v", err)
	}

	msg, err := conn.ReadMessage()
	require.NoError(t, err)

	authPrompt, ok := msg.(*tdpb.AuthPrompt)
	require.True(t, ok, "expected AuthPrompt, got %T", msg)
	require.NotNil(t, (*tdpbv1.AuthPrompt)(authPrompt).GetMfaPrompt())

	if err := conn.WriteMessage(
		(*tdpb.MFAPromptResponse)(tdpbv1.MFAPromptResponse_builder{
			Reference: &tdpbv1.MFAPromptResponseReference{
				ChallengeName: "test-challenge",
			},
		}.Build()),
	); err != nil {
		t.Fatalf("failed to send MFAPromptResponse: %v", err)
	}

	msg, err = conn.ReadMessage()
	require.NoError(t, err)
	if _, ok := msg.(*tdpb.SessionEstablishing); !ok {
		t.Fatalf("expected SessionEstablishing, got %T", msg)
	}

	msg, err = conn.ReadMessage()
	require.NoError(t, err)
	serverHello, ok := msg.(*tdpb.ServerHello)
	require.True(t, ok, "expected ServerHello, got %T", msg)
	require.True(t, serverHello.ClipboardEnabled)
}

func TestTDPBNoMFA_MessageExchange(t *testing.T) {
	t.Parallel()

	clientPipe, serverPipe := net.Pipe()
	defer clientPipe.Close()
	defer serverPipe.Close()

	go func() {
		conn := tdp.NewConn(serverPipe, tdp.DecoderAdapter(tdpb.DecodePermissive), tdpb.WarningConstructor)

		if _, err := conn.ReadMessage(); err != nil {
			return
		}

		if err := conn.WriteMessage(&tdpb.SessionEstablishing{}); err != nil {
			return
		}
		if err := conn.WriteMessage(&tdpb.ServerHello{ClipboardEnabled: true}); err != nil {
			return
		}
	}()

	conn := tdp.NewConn(clientPipe, tdp.DecoderAdapter(tdpb.DecodePermissive), tdpb.WarningConstructor)

	if err := conn.WriteMessage(newTestClientHello()); err != nil {
		t.Fatalf("failed to send ClientHello: %v", err)
	}

	msg, err := conn.ReadMessage()
	require.NoError(t, err)
	if _, ok := msg.(*tdpb.SessionEstablishing); !ok {
		t.Fatalf("expected SessionEstablishing, got %T", msg)
	}

	msg, err = conn.ReadMessage()
	require.NoError(t, err)
	serverHello, ok := msg.(*tdpb.ServerHello)
	require.True(t, ok, "expected ServerHello, got %T", msg)
	require.True(t, serverHello.ClipboardEnabled)
}

func TestTDPBLegacyWDS_Alert(t *testing.T) {
	t.Parallel()

	clientPipe, serverPipe := net.Pipe()
	defer clientPipe.Close()
	defer serverPipe.Close()

	go func() {
		conn := tdp.NewConn(serverPipe, tdp.DecoderAdapter(tdpb.DecodePermissive), tdpb.WarningConstructor)

		_, err := conn.ReadMessage()
		if err != nil {
			return
		}

		if err := conn.WriteMessage(&tdpb.Alert{
			Message:  "in-band MFA not supported",
			Severity: tdpbv1.AlertSeverity_ALERT_SEVERITY_WARNING,
		}); err != nil {
			return
		}
	}()

	conn := tdp.NewConn(clientPipe, tdp.DecoderAdapter(tdpb.DecodePermissive), tdpb.WarningConstructor)

	if err := conn.WriteMessage(newTestClientHello()); err != nil {
		t.Fatalf("failed to send ClientHello: %v", err)
	}

	msg, err := conn.ReadMessage()
	require.NoError(t, err)
	alert, ok := msg.(*tdpb.Alert)
	require.True(t, ok, "expected Alert, got %T", msg)
	require.Equal(t, "in-band MFA not supported", alert.Message)
}

func newAuthPrompt() *tdpbv1.AuthPrompt {
	return &tdpbv1.AuthPrompt{
		Prompt: &tdpbv1.AuthPrompt_MfaPrompt{
			MfaPrompt: &tdpbv1.MFAPrompt{},
		},
	}
}

func newTestClientHello() *tdpb.ClientHello {
	return &tdpb.ClientHello{
		Username: login,
		ScreenSpec: &tdpbv1.ClientScreenSpec{
			Width:  1920,
			Height: 1080,
		},
	}
}
