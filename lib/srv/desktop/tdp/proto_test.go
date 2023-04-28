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

package tdp

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	authproto "github.com/gravitational/teleport/api/client/proto"
	wantypes "github.com/gravitational/teleport/api/types/webauthn"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
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
	var corpus = []string{
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

func TestRejectsLongUsername(t *testing.T) {
	const lengthTooLong = 4096

	b := &bytes.Buffer{}
	b.WriteByte(byte(TypeClientUsername))
	binary.Write(b, binary.BigEndian, uint32(lengthTooLong))
	b.Write(bytes.Repeat([]byte("a"), lengthTooLong))

	_, err := Decode(b.Bytes())
	require.True(t, trace.IsBadParameter(err))
}

var encodedFrame []byte

func BenchmarkEncodePNG(b *testing.B) {
	b.StopTimer()
	frames := loadBitmaps(b)
	b.StartTimer()
	var err error
	for i := 0; i < b.N; i++ {
		fi := i % len(frames)
		encodedFrame, err = frames[fi].Encode()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func loadBitmaps(b *testing.B) []PNG2Frame {
	b.Helper()

	f, err := os.Open(filepath.Join("testdata", "png_frames.json"))
	require.NoError(b, err)
	defer f.Close()

	enc := PNGEncoder()

	var result []PNG2Frame
	type record struct {
		Top, Left, Right, Bottom int
		Pix                      []byte
	}
	s := bufio.NewScanner(f)
	for s.Scan() {
		var r record
		require.NoError(b, json.Unmarshal(s.Bytes(), &r))

		img := image.NewNRGBA(image.Rectangle{
			Min: image.Pt(r.Left, r.Top),
			Max: image.Pt(r.Right, r.Bottom),
		})
		copy(img.Pix, r.Pix)
		result = append(result, NewPNG(img, enc))
	}
	require.NoError(b, s.Err())
	return result
}

func TestMFA(t *testing.T) {
	var buff bytes.Buffer
	c := NewConn(&fakeConn{Buffer: &buff})

	mfaWant := &MFA{
		Type: defaults.WebsocketWebauthnChallenge[0],
		MFAAuthenticateChallenge: &client.MFAAuthenticateChallenge{
			WebauthnChallenge: &wanlib.CredentialAssertion{
				Response: protocol.PublicKeyCredentialRequestOptions{
					Challenge:      []byte("challenge"),
					Timeout:        10,
					RelyingPartyID: "teleport",
					AllowedCredentials: []protocol.CredentialDescriptor{
						{
							Type:         "public-key",
							CredentialID: []byte("credential id"),
							Transport:    []protocol.AuthenticatorTransport{protocol.USB},
						},
					},
					UserVerification: "discouraged",
					Extensions: protocol.AuthenticationExtensions{
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
		Type: defaults.WebsocketWebauthnChallenge[0],
		MFAAuthenticateResponse: &authproto.MFAAuthenticateResponse{
			Response: &authproto.MFAAuthenticateResponse_Webauthn{
				Webauthn: &wantypes.CredentialAssertionResponse{
					Type:  "public-key",
					RawId: []byte("credential id"),
					Response: &wantypes.AuthenticatorAssertionResponse{
						ClientDataJson:    []byte("client data json"),
						AuthenticatorData: []byte("authenticator data"),
						Signature:         []byte("signature"),
						UserHandle:        []byte("user handle"),
					},
					Extensions: &wantypes.AuthenticationExtensionsClientOutputs{
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
