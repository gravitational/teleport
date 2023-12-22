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
