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

//go:generate go run golang.org/x/sys/windows/mkwinsyscall -output syscall_windows.go syscall_gen.go

//sys webAuthNGetApiVersionNumber() (ret int, err error) [failretval==0] = WebAuthn.WebAuthNGetApiVersionNumber
// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L895-L897

//sys webAuthNIsUserVerifyingPlatformAuthenticatorAvailable(out *bool) (ret uintptr, err error) [failretval!=0] = WebAuthn.WebAuthNIsUserVerifyingPlatformAuthenticatorAvailable
// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L901

//sys webAuthNAuthenticatorMakeCredential(hwnd syscall.Handle, rp *webauthnRPEntityInformation, user *webauthnUserEntityInformation, pubKeyCredParams *webauthnCoseCredentialParameters, clientData *webauthnClientData, opts *webauthnAuthenticatorMakeCredentialOptions, out **webauthnCredentialAttestation) (ret uintptr, err error) [failretval!=0] = WebAuthn.WebAuthNAuthenticatorMakeCredential
// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L907

//sys webAuthNFreeCredentialAttestation(in *webauthnCredentialAttestation) = WebAuthn.WebAuthNFreeCredentialAttestation
// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L928

package webauthnwin
