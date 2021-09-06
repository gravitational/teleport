// Copyright 2021 Gravitational, Inc
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

package webauthn_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
)

func TestCredentialAssertionResponse_json(t *testing.T) {
	resp := &wanlib.CredentialAssertionResponse{
		PublicKeyCredential: wanlib.PublicKeyCredential{
			Credential: wanlib.Credential{
				ID:   base64.RawURLEncoding.EncodeToString([]byte("credentialid")),
				Type: "public-key",
			},
			RawID: []byte("credentialid"),
			Extensions: &wanlib.AuthenticationExtensionsClientOutputs{
				AppID: true,
			},
		},
		AssertionResponse: wanlib.AuthenticatorAssertionResponse{
			AuthenticatorResponse: wanlib.AuthenticatorResponse{
				ClientDataJSON: []byte("clientdatajson"),
			},
			AuthenticatorData: []byte("authdata"),
			Signature:         []byte("signature"),
			UserHandle:        []byte("userhandle"),
		},
	}

	respJSON, err := json.Marshal(resp)
	require.NoError(t, err)

	got := &wanlib.CredentialAssertionResponse{}
	require.NoError(t, json.Unmarshal(respJSON, got))
	if diff := cmp.Diff(resp, got); diff != "" {
		t.Errorf("Unmarshal() mismatch (-want +got):\n%s", diff)
	}
}
