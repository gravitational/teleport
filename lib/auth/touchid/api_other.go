//go:build !touchid
// +build !touchid

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

package touchid

var native nativeTID = noopNative{}

type noopNative struct{}

func (noopNative) Diag() (*DiagResult, error) {
	return &DiagResult{}, nil
}

func (noopNative) Register(rpID, user string, userHandle []byte) (*CredentialInfo, error) {
	return nil, ErrNotAvailable
}

func (noopNative) Authenticate(credentialID string, digest []byte) ([]byte, error) {
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
