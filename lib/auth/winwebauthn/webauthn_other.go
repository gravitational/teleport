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

package winwebauthn

import (
	"context"
	"errors"

	"github.com/gravitational/teleport/api/client/proto"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
)

var errUnavailable = errors.New("windows webauthn unavailable in current build")

func login(
	ctx context.Context,
	origin string, assertion *wanlib.CredentialAssertion, opts *LoginOpts,
) (*proto.MFAAuthenticateResponse, string, error) {
	return nil, "", errUnavailable
}

func register(
	ctx context.Context,
	origin string, cc *wanlib.CredentialCreation,
) (*proto.MFARegisterResponse, error) {
	return nil, errUnavailable
}

func isAvailable() bool {
	return false
}

func checkSupport() (*CheckSupportResult, error) {
	return &CheckSupportResult{
		HasCompileSupport: false,
	}, nil
}
