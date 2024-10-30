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

package tdp

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	authproto "github.com/gravitational/teleport/api/client/proto"
	wanpb "github.com/gravitational/teleport/api/types/webauthn"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
)

func TestEncodeDecode(t *testing.T) {
	for _, m := range []Message{
		MouseMove{X: 1, Y: 2},
		MouseButton{Button: MiddleMouseButton, State: ButtonPressed},
		KeyboardButton{KeyCode: 1, State: ButtonPressed},
		func() Message {
			img := image.NewNRGBA(image.Rect(5, 5, 10, 10))
			for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
				for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
					img.Set(x, y, color.NRGBA{1, 2, 3, 4})
				}
			}
			return PNGFrame{Img: img}
		}(),
		ClientScreenSpec{Width: 123, Height: 456},
		ClientUsername{Username: "admin"},
		MouseWheel{Axis: HorizontalWheelAxis, Delta: -123},
		Error{Message: "An error occurred"},
	} {
		t.Run(fmt.Sprintf("%T", m), func(t *testing.T) {
			buf, err := m.Encode()
			require.NoError(t, err)

			out, err := Decode(buf)
			require.NoError(t, err)

			require.Empty(t, cmp.Diff(m, out, cmpopts.IgnoreUnexported(PNGFrame{})))
		})
	}
}

func FuzzDecode(f *testing.F) {
	corpus := []string{
		"0",
		"\x02",
		"\x1b\xff\xff\x800",
		"\x1b\xff\xff\xff\xeb",
		"\nn\x00\x00\x00\x04  {}",
		"\v00000000\x00\x00\x00\x00",
		"\nn\x00\x00\x00\x04 { }000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
	}

	for _, s := range corpus {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, buf []byte) {
		require.NotPanics(t, func() {
			// decode random buffer
			msg, err := Decode(buf)
			if err != nil {
				return
			}

			// test that we can encode the message back:
			buf2, err := msg.Encode()
			require.NoError(t, err)
			require.NotNil(t, buf2)

			// decode the new buffer. it must be equal to the original msg.
			msg2, err := Decode(buf2)
			require.NoError(t, err)
			require.Equalf(t, msg, msg2, "mismatch for message %v", buf)

			// encode another time.
			// after encoding, it must match the second buffer identically.
			// this isn't the case for the first buffer, as there can be trailing bytes after the message.
			buf3, err := msg2.Encode()
			require.NoError(t, err)
			require.NotNil(t, buf3)
			require.Equal(t, buf2, buf3)
		})
	})
}

func TestBadDecode(t *testing.T) {
	// 254 is an unknown message type.
	_, err := Decode([]byte{254})
	require.Error(t, err)
}

func TestMFA(t *testing.T) {
	var buff bytes.Buffer
	c := NewConn(&fakeConn{Buffer: &buff})

	mfaWant := &MFA{
		Type: defaults.WebsocketMFAChallenge[0],
		MFAAuthenticateChallenge: &client.MFAAuthenticateChallenge{
			WebauthnChallenge: &wantypes.CredentialAssertion{
				Response: wantypes.PublicKeyCredentialRequestOptions{
					Challenge:      []byte("challenge"),
					Timeout:        10,
					RelyingPartyID: "teleport",
					AllowedCredentials: []wantypes.CredentialDescriptor{
						{
							Type:         "public-key",
							CredentialID: []byte("credential id"),
							Transport:    []protocol.AuthenticatorTransport{protocol.USB},
						},
					},
					UserVerification: "discouraged",
					Extensions: wantypes.AuthenticationExtensions{
						"ext1": "value1",
					},
				},
			},
		},
	}
	err := c.WriteMessage(mfaWant)
	require.NoError(t, err)

	mt, err := buff.ReadByte()
	require.NoError(t, err)
	require.Equal(t, TypeMFA, MessageType(mt))

	mfaGot, err := DecodeMFAChallenge(&buff)
	require.NoError(t, err)
	require.Equal(t, mfaWant, mfaGot)

	respWant := &MFA{
		Type: defaults.WebsocketMFAChallenge[0],
		MFAAuthenticateResponse: &authproto.MFAAuthenticateResponse{
			Response: &authproto.MFAAuthenticateResponse_Webauthn{
				Webauthn: &wanpb.CredentialAssertionResponse{
					Type:  "public-key",
					RawId: []byte("credential id"),
					Response: &wanpb.AuthenticatorAssertionResponse{
						ClientDataJson:    []byte("client data json"),
						AuthenticatorData: []byte("authenticator data"),
						Signature:         []byte("signature"),
						UserHandle:        []byte("user handle"),
					},
					Extensions: &wanpb.AuthenticationExtensionsClientOutputs{
						AppId: true,
					},
				},
			},
		},
	}
	err = c.WriteMessage(respWant)
	require.NoError(t, err)
	respGot, err := c.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, respWant, respGot)
}

func TestIsNonFatalErr(t *testing.T) {
	// Test that nil returns false
	require.False(t, IsNonFatalErr(nil))
	// Test that any other error returns false
	require.False(t, IsNonFatalErr(errors.New("some other error")))
}

// TDP messages must have size limits in order to prevent attacks that
// soak up system memory. At the same time, exceeding such size limits shouldn't
// kill a user's running session, or else that becomes a DoS attack vector.
// To this end, TestSizeLimitsAreNonFatal checks that exceeding size limits causes
// only non-fatal errors.
//
// An exception to this rule is a long ClientUsername, which can't be used in a DoS
// attack (because there's no way for the RDP server to send a message that's translated
// into a too-long ClientUsername). The best UX in this case is to send a fatal error
// letting them know that the username was too long.
func TestSizeLimitsAreNonFatal(t *testing.T) {
	for _, test := range []struct {
		name  string
		msg   Message
		fatal bool
	}{
		{
			name: "rejects long ClientUsername as fatal",
			msg: ClientUsername{
				Username: string(bytes.Repeat([]byte("a"), windowsMaxUsernameLength+1)),
			},
			fatal: true,
		},
		{
			name:  "rejects long Clipboard",
			msg:   ClipboardData(bytes.Repeat([]byte("a"), maxClipboardDataLength+1)),
			fatal: false,
		},
		{
			name: "rejects long Error",
			msg: Error{
				Message: string(bytes.Repeat([]byte("a"), tdpMaxAlertMessageLength+1)),
			},
			fatal: false,
		},
		{
			name: "rejects long Alert",
			msg: Alert{
				Message: string(bytes.Repeat([]byte("a"), tdpMaxAlertMessageLength+1)),
			},
			fatal: false,
		},
		{
			name: "rejects long SharedDirectoryAnnounce",
			msg: SharedDirectoryAnnounce{
				Name: string(bytes.Repeat([]byte("a"), windowsMaxUsernameLength+1)),
			},
			fatal: false,
		},
		{
			name: "rejects long SharedDirectoryInfoRequest",
			msg: SharedDirectoryInfoRequest{
				Path: string(bytes.Repeat([]byte("a"), tdpMaxPathLength+1)),
			},
			fatal: false,
		},
		{
			name: "rejects long SharedDirectoryCreateRequest",
			msg: SharedDirectoryCreateRequest{
				Path: string(bytes.Repeat([]byte("a"), tdpMaxPathLength+1)),
			},
			fatal: false,
		},
		{
			name: "rejects long SharedDirectoryDeleteRequest",
			msg: SharedDirectoryDeleteRequest{
				Path: string(bytes.Repeat([]byte("a"), tdpMaxPathLength+1)),
			},
			fatal: false,
		},
		{
			name: "rejects long SharedDirectoryListRequest",
			msg: SharedDirectoryListRequest{
				Path: string(bytes.Repeat([]byte("a"), tdpMaxPathLength+1)),
			},
			fatal: false,
		},
		{
			name: "rejects long SharedDirectoryReadRequest",
			msg: SharedDirectoryReadRequest{
				Path: string(bytes.Repeat([]byte("a"), tdpMaxPathLength+1)),
			},
			fatal: false,
		},
		{
			name: "rejects long SharedDirectoryReadResponse",
			msg: SharedDirectoryReadResponse{
				ReadDataLength: tdpMaxFileReadWriteLength + 1,
			},
			fatal: false,
		},
		{
			name: "rejects long SharedDirectoryWriteRequest",
			msg: SharedDirectoryWriteRequest{
				WriteDataLength: tdpMaxFileReadWriteLength + 1,
			},
			fatal: false,
		},
		{
			name: "rejects long SharedDirectoryMoveRequest",
			msg: SharedDirectoryMoveRequest{
				OriginalPath: string(bytes.Repeat([]byte("a"), tdpMaxPathLength+1)),
			},
			fatal: false,
		},
		{
			name: "rejects long SharedDirectoryInfoResponse",
			msg: SharedDirectoryInfoResponse{
				CompletionID: 0,
				ErrCode:      0,
				Fso: FileSystemObject{
					Path: string(bytes.Repeat([]byte("a"), tdpMaxPathLength+1)),
				},
			},
			fatal: false,
		},
		{
			name: "rejects long SharedDirectoryCreateResponse",
			msg: SharedDirectoryCreateResponse{
				CompletionID: 0,
				ErrCode:      0,
				Fso: FileSystemObject{
					Path: string(bytes.Repeat([]byte("a"), tdpMaxPathLength+1)),
				},
			},
			fatal: false,
		},
		{
			name: "rejects long SharedDirectoryListResponse",
			msg: SharedDirectoryListResponse{
				CompletionID: 0,
				ErrCode:      0,
				FsoList: []FileSystemObject{{
					Path: string(bytes.Repeat([]byte("a"), tdpMaxPathLength+1)),
				}},
			},
			fatal: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			bytes, err := test.msg.Encode()
			require.NoError(t, err)
			_, err = Decode(bytes)
			require.True(t, trace.IsLimitExceeded(err))
			require.Equal(t, test.fatal, IsFatalErr(err))
			require.Equal(t, !test.fatal, IsNonFatalErr(err))
		})
	}
}
