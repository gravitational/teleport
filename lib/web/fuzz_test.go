/*
Copyright 2022 Gravitational, Inc.

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

package web

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	apiproto "github.com/gravitational/teleport/api/client/proto"
	wanpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

func FuzzTdpMFACodecDecodeChallenge(f *testing.F) {
	allowedCreds := wanpb.CredentialDescriptor{
		Type: "public-key",
		Id:   []byte{0x02, 0x02, 0x02, 0x02},
	}
	extensions := wanpb.AuthenticationExtensionsClientInputs{AppId: "id"}
	jsonData, err := json.Marshal(&apiproto.MFAAuthenticateChallenge{
		WebauthnChallenge: &wanpb.CredentialAssertion{
			PublicKey: &wanpb.PublicKeyCredentialRequestOptions{
				Challenge:        []byte{0xAA, 0xAA, 0xAA, 0xAA},
				TimeoutMs:        int64(120),
				RpId:             "RelyingPartyID",
				AllowCredentials: []*wanpb.CredentialDescriptor{&allowedCreds},
				Extensions:       &extensions,
				UserVerification: "verification",
			},
		},
	})
	require.NoError(f, err)
	var normalBuf bytes.Buffer
	var maxSizeBuf bytes.Buffer
	// add initial bytes for protocol
	_, err = normalBuf.Write([]byte{byte(tdp.TypeMFA), []byte(defaults.WebsocketWebauthnChallenge)[0]})
	require.NoError(f, err)
	_, err = maxSizeBuf.Write([]byte{byte(tdp.TypeMFA), []byte(defaults.WebsocketWebauthnChallenge)[0]})
	require.NoError(f, err)
	// Write the length using BigEndian encoding
	require.NoError(f, binary.Write(&normalBuf, binary.BigEndian, uint32(len(jsonData))))
	require.NoError(f, binary.Write(&maxSizeBuf, binary.BigEndian, uint32(math.MaxUint32)))
	// Write the JSON data itself
	_, err = normalBuf.Write(jsonData)
	require.NoError(f, err)
	_, err = maxSizeBuf.Write(jsonData)
	require.NoError(f, err)

	f.Add(normalBuf.Bytes())
	f.Add(maxSizeBuf.Bytes())
	f.Add([]byte{0xa, 0x6e, 0x0, 0x0, 0x0, 0x4, 0x6e, 0x75, 0x6c, 0x6c}) // nil json unmarshal without error

	f.Fuzz(func(t *testing.T, buf []byte) {
		require.NotPanics(t, func() {
			codec := tdpMFACodec{}
			_, _ = codec.decodeChallenge(buf, "")
		})
	})
}

func FuzzTdpMFACodecDecodeResponse(f *testing.F) {
	var normalBuf bytes.Buffer
	var maxSizeBuf bytes.Buffer
	// add initial bytes for protocol
	_, err := normalBuf.Write([]byte{byte(tdp.TypeMFA), []byte(defaults.WebsocketWebauthnChallenge)[0]})
	require.NoError(f, err)
	_, err = maxSizeBuf.Write([]byte{byte(tdp.TypeMFA), []byte(defaults.WebsocketWebauthnChallenge)[0]})
	require.NoError(f, err)
	mfaData := []byte("fake-data")
	// Write the length using BigEndian encoding
	require.NoError(f, binary.Write(&normalBuf, binary.BigEndian, uint32(len(mfaData))))
	require.NoError(f, binary.Write(&maxSizeBuf, binary.BigEndian, uint32(math.MaxUint32)))
	// add data field
	_, err = normalBuf.Write(mfaData)
	require.NoError(f, err)
	_, err = maxSizeBuf.Write(mfaData)
	require.NoError(f, err)

	f.Add(normalBuf.Bytes())
	f.Add(maxSizeBuf.Bytes())

	f.Fuzz(func(t *testing.T, buf []byte) {
		require.NotPanics(t, func() {
			codec := tdpMFACodec{}
			_, _ = codec.decodeResponse(buf, "")
		})
	})
}
