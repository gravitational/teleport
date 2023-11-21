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

//go:generate go run golang.org/x/sys/windows/mkwinsyscall -output zsyscall_windows.go syscall_gen.go

// See https://learn.microsoft.com/en-us/windows/win32/api/webauthn/.

//sys webAuthNGetApiVersionNumber() (ret int, err error) [failretval==0] = WebAuthn.WebAuthNGetApiVersionNumber
// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L895-L897

//sys webAuthNIsUserVerifyingPlatformAuthenticatorAvailable(out *bool) (ret uintptr, err error) [failretval!=0] = WebAuthn.WebAuthNIsUserVerifyingPlatformAuthenticatorAvailable
// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L901

//sys webAuthNAuthenticatorMakeCredential(hwnd syscall.Handle, rp *webauthnRPEntityInformation, user *webauthnUserEntityInformation, pubKeyCredParams *webauthnCoseCredentialParameters, clientData *webauthnClientData, opts *webauthnAuthenticatorMakeCredentialOptions, out **webauthnCredentialAttestation) (ret uintptr, err error) [failretval!=0] = WebAuthn.WebAuthNAuthenticatorMakeCredential
// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L907

//sys freeCredentialAttestation(in *webauthnCredentialAttestation) = WebAuthn.WebAuthNFreeCredentialAttestation
// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L928

//sys webAuthNAuthenticatorGetAssertion(hwnd syscall.Handle, rpID *uint16, clientData *webauthnClientData, opts *webauthnAuthenticatorGetAssertionOptions, out **webauthnAssertion) (ret uintptr, err error) [failretval!=0] = WebAuthn.WebAuthNAuthenticatorGetAssertion
// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L919

//sys freeAssertion(in *webauthnAssertion) = WebAuthn.WebAuthNFreeAssertion
// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L933

//sys webAuthNGetErrorName(in uintptr) (ret uintptr) = WebAuthn.WebAuthNGetErrorName
// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L982

//sys getForegroundWindow() (hwnd syscall.Handle, err error) [failretval==0] = user32.GetForegroundWindow
// https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getforegroundwindow

package webauthnwin
