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

package webauthntypes_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	wanpb "github.com/gravitational/teleport/api/types/webauthn"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

func TestConversionFromProto_nils(t *testing.T) {
	// The objective of this test is not to check for correct conversions; those
	// are already checked elsewhere by the various flows that require them.
	// What we want here is to make sure that malformed protos won't make us
	// panic. If a malformed message makes it through, validation will catch it
	// downstream.

	tests := []struct {
		name string
		fn   func()
	}{
		{
			name: "CredentialAssertion nil",
			fn: func() {
				wantypes.CredentialAssertionFromProto(nil)
			},
		},
		{
			name: "CredentialAssertion empty",
			fn: func() {
				wantypes.CredentialAssertionFromProto(&wanpb.CredentialAssertion{})
			},
		},
		{
			name: "CredentialAssertion.PublicKey empty",
			fn: func() {
				wantypes.CredentialAssertionFromProto(&wanpb.CredentialAssertion{
					PublicKey: &wanpb.PublicKeyCredentialRequestOptions{},
				})
			},
		},
		{
			name: "CredentialAssertion.PublicKey slice elements nil",
			fn: func() {
				wantypes.CredentialAssertionFromProto(&wanpb.CredentialAssertion{
					PublicKey: &wanpb.PublicKeyCredentialRequestOptions{
						AllowCredentials: []*wanpb.CredentialDescriptor{
							{}, nil, {},
						},
					},
				})
			},
		},
		{
			name: "CredentialAssertionResponse nil",
			fn: func() {
				wantypes.CredentialAssertionResponseFromProto(nil)
			},
		},
		{
			name: "CredentialAssertionResponse empty",
			fn: func() {
				wantypes.CredentialAssertionResponseFromProto(&wanpb.CredentialAssertionResponse{})
			},
		},
		{
			name: "CredentialAssertionResponse.Response empty",
			fn: func() {
				wantypes.CredentialAssertionResponseFromProto(&wanpb.CredentialAssertionResponse{
					Response: &wanpb.AuthenticatorAssertionResponse{},
				})
			},
		},
		{
			name: "CredentialCreation nil",
			fn: func() {
				wantypes.CredentialCreationFromProto(nil)
			},
		},
		{
			name: "CredentialCreation empty",
			fn: func() {
				wantypes.CredentialCreationFromProto(&wanpb.CredentialCreation{})
			},
		},
		{
			name: "CredentialCreation.PublicKey empty",
			fn: func() {
				wantypes.CredentialCreationFromProto(&wanpb.CredentialCreation{
					PublicKey: &wanpb.PublicKeyCredentialCreationOptions{},
				})
			},
		},
		{
			name: "CredentialCreation.PublicKey slice elements nil",
			fn: func() {
				wantypes.CredentialCreationFromProto(&wanpb.CredentialCreation{
					PublicKey: &wanpb.PublicKeyCredentialCreationOptions{
						CredentialParameters: []*wanpb.CredentialParameter{
							{}, nil, {},
						},
						ExcludeCredentials: []*wanpb.CredentialDescriptor{
							{}, nil, {},
						},
					},
				})
			},
		},
		{
			name: "CredentialCreationResponse nil",
			fn: func() {
				wantypes.CredentialCreationResponseFromProto(nil)
			},
		},
		{
			name: "CredentialCreationResponse empty",
			fn: func() {
				wantypes.CredentialCreationResponseFromProto(&wanpb.CredentialCreationResponse{})
			},
		},
		{
			name: "CredentialCreationResponse.Response empty",
			fn: func() {
				wantypes.CredentialCreationResponseFromProto(&wanpb.CredentialCreationResponse{
					Response: &wanpb.AuthenticatorAttestationResponse{},
				})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Absence of panics is good enough for us.
			require.NotPanics(t, test.fn)
		})
	}
}
