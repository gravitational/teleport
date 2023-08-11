//go:build !libfido2
// +build !libfido2

// Copyright 2022 Gravitational, Inc
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

package webauthncli

import (
	"context"
	"errors"

	"github.com/gravitational/teleport/api/client/proto"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

var errFIDO2Unavailable = errors.New("FIDO2 unavailable in current build")

// isLibfido2Enabled returns true if libfido2 is available in the current build.
func isLibfido2Enabled() bool {
	return false
}

func fido2Login(
	ctx context.Context,
	origin string, assertion *wantypes.CredentialAssertion, prompt LoginPrompt, opts *LoginOpts,
) (*proto.MFAAuthenticateResponse, string, error) {
	return nil, "", errFIDO2Unavailable
}

func fido2Register(
	ctx context.Context,
	origin string, cc *wantypes.CredentialCreation, prompt RegisterPrompt,
) (*proto.MFARegisterResponse, error) {
	return nil, errFIDO2Unavailable
}
