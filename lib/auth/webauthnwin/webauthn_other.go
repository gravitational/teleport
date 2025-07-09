//go:build !windows
// +build !windows

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
