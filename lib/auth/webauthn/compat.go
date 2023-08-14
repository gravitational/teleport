// Copyright 2023 Gravitational, Inc
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

package webauthn

import wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"

// TODO(codingllama): Delete aliases and vars below once e/ is updated.

type CredentialAssertion = wantypes.CredentialAssertion
type CredentialAssertionResponse = wantypes.CredentialAssertionResponse
type CredentialCreation = wantypes.CredentialCreation
type CredentialCreationResponse = wantypes.CredentialCreationResponse

var (
	CredentialAssertionFromProto         = wantypes.CredentialAssertionFromProto
	CredentialAssertionToProto           = wantypes.CredentialAssertionToProto
	CredentialAssertionResponseFromProto = wantypes.CredentialAssertionResponseFromProto
	CredentialAssertionResponseToProto   = wantypes.CredentialAssertionResponseToProto
)

var (
	CredentialCreationFromProto         = wantypes.CredentialCreationFromProto
	CredentialCreationToProto           = wantypes.CredentialCreationToProto
	CredentialCreationResponseFromProto = wantypes.CredentialCreationResponseFromProto
	CredentialCreationResponseToProto   = wantypes.CredentialCreationResponseToProto
)
