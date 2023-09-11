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

//go:build !windows
// +build !windows

package webauthnwin

import (
	"errors"

	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

var native nativeWebauthn = noopNative{}

var errUnavailable = errors.New("windows webauthn unavailable in current build")

type noopNative struct{}

func (n noopNative) CheckSupport() CheckSupportResult {
	return CheckSupportResult{
		HasCompileSupport: false,
	}
}

func (n noopNative) GetAssertion(origin string, in *getAssertionRequest) (*wantypes.CredentialAssertionResponse, error) {
	return nil, errUnavailable
}

func (n noopNative) MakeCredential(origin string, in *makeCredentialRequest) (*wantypes.CredentialCreationResponse, error) {
	return nil, errUnavailable
}
