//go:build !touchid
// +build !touchid

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

package touchid

var native nativeTID = noopNative{}

type noopNative struct{}

func (noopNative) Diag() (*DiagResult, error) {
	return &DiagResult{}, nil
}

type noopAuthContext struct{}

func (noopAuthContext) Guard(fn func()) error {
	return ErrNotAvailable
}

func (noopAuthContext) Close() {}

func (noopNative) NewAuthContext() AuthContext {
	return noopAuthContext{}
}

func (noopNative) Register(rpID, user string, userHandle []byte) (*CredentialInfo, error) {
	return nil, ErrNotAvailable
}

func (noopNative) Authenticate(actx AuthContext, credentialID string, digest []byte) ([]byte, error) {
	return nil, ErrNotAvailable
}

func (noopNative) FindCredentials(rpID, user string) ([]CredentialInfo, error) {
	return nil, ErrNotAvailable
}

func (noopNative) ListCredentials() ([]CredentialInfo, error) {
	return nil, ErrNotAvailable
}

func (noopNative) DeleteCredential(credentialID string) error {
	return ErrNotAvailable
}

func (noopNative) DeleteNonInteractive(credentialID string) error {
	return ErrNotAvailable
}
