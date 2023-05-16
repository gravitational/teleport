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

package webauthnwin

import (
	"encoding/base64"
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
)

var (
	modWebAuthn = windows.NewLazySystemDLL("WebAuthn.dll")

	// For reference, see
	// https://learn.microsoft.com/en-us/windows/win32/api/webauthn/.
	procWebAuthNGetApiVersionNumber                           = modWebAuthn.NewProc("WebAuthNGetApiVersionNumber")
	procWebAuthNIsUserVerifyingPlatformAuthenticatorAvailable = modWebAuthn.NewProc("WebAuthNIsUserVerifyingPlatformAuthenticatorAvailable")
	procWebAuthNAuthenticatorMakeCredential                   = modWebAuthn.NewProc("WebAuthNAuthenticatorMakeCredential")
	procWebAuthNFreeCredentialAttestation                     = modWebAuthn.NewProc("WebAuthNFreeCredentialAttestation")
	procWebAuthNAuthenticatorGetAssertion                     = modWebAuthn.NewProc("WebAuthNAuthenticatorGetAssertion")
	procWebAuthNFreeAssertion                                 = modWebAuthn.NewProc("WebAuthNFreeAssertion")
	procWebAuthNGetErrorName                                  = modWebAuthn.NewProc("WebAuthNGetErrorName")

	modUser32               = windows.NewLazySystemDLL("user32.dll")
	procGetForegroundWindow = modUser32.NewProc("GetForegroundWindow")
)

var native nativeWebauthn = newNativeImpl()

// nativeImpl keeps diagnostic informations about windows webauthn support.
type nativeImpl struct {
	webauthnAPIVersion int
	hasCompileSupport  bool
	isAvailable        bool
	hasPlatformUV      bool
}

// newNativeImpl creates nativeImpl which contains diagnostics info.
// Diagnostics are safe to cache because dll isn't something that
// could change during program invocation.
// Client will be always created, even if dll is missing on system.
func newNativeImpl() *nativeImpl {
	v, err := checkIfDLLExistsAndGetAPIVersionNumber()
	if err != nil {
		log.WithError(err).Debug("WebAuthnWin: failed to check version")
		return &nativeImpl{
			hasCompileSupport: true,
			isAvailable:       false,
		}
	}
	uvPlatform, err := isUVPlatformAuthenticatorAvailable()
	if err != nil {
		// This should not happen if dll exists, however we are fine with
		// to proceed without uvPlatform.
		log.WithError(err).Debug("WebAuthnWin: failed to check isUVPlatformAuthenticatorAvailable")
	}

	return &nativeImpl{
		webauthnAPIVersion: v,
		hasCompileSupport:  true,
		hasPlatformUV:      uvPlatform,
		isAvailable:        v > 0,
	}
}

func (n *nativeImpl) CheckSupport() CheckSupportResult {
	return CheckSupportResult{
		HasCompileSupport:  n.hasCompileSupport,
		IsAvailable:        n.isAvailable,
		HasPlatformUV:      n.hasPlatformUV,
		WebAuthnAPIVersion: n.webauthnAPIVersion,
	}
}

// GetAssertion calls WebAuthNAuthenticatorGetAssertion endpoint from
// webauthn.dll and returns CredentialAssertionResponse.
// It interacts with both FIDO2 and Windows Hello depending on
// opts.AuthenticatorAttachment (using auto results in possibilty to select
// either security key or Windows Hello).
// It does not accept username - during passwordless login webauthn.dll provides
// its own dialog with credentials selection.
func (n *nativeImpl) GetAssertion(origin string, in *getAssertionRequest) (*wanlib.CredentialAssertionResponse, error) {
	hwnd, err := getForegroundWindow()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out *webauthnAssertion
	ret, _, err := procWebAuthNAuthenticatorGetAssertion.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(in.rpID)),
		uintptr(unsafe.Pointer(in.clientData)),
		uintptr(unsafe.Pointer(in.opts)),
		uintptr(unsafe.Pointer(&out)),
	)
	if ret != 0 {
		return nil, trace.Wrap(getErrorNameOrLastErr(ret, err))
	}
	if out == nil {
		return nil, errors.New("unexpected nil response from GetAssertion")
	}

	// Note that we need to copy bytes out of `out` if we want to free object.
	// That's why bytesFromCBytes is used.
	// We don't care about free error so ignore it explicitly.
	defer func() { _ = freeAssertion(out) }()

	authData := bytesFromCBytes(out.cbAuthenticatorData, out.pbAuthenticatorData)
	signature := bytesFromCBytes(out.cbSignature, out.pbSignature)
	userID := bytesFromCBytes(out.cbUserID, out.pbUserID)
	credential := bytesFromCBytes(out.Credential.cbID, out.Credential.pbID)
	credType := windows.UTF16PtrToString(out.Credential.pwszCredentialType)

	return &wanlib.CredentialAssertionResponse{
		PublicKeyCredential: wanlib.PublicKeyCredential{
			RawID: credential,
			Credential: wanlib.Credential{
				ID:   base64.RawURLEncoding.EncodeToString(credential),
				Type: credType,
			},
		},
		AssertionResponse: wanlib.AuthenticatorAssertionResponse{
			AuthenticatorData: authData,
			Signature:         signature,
			UserHandle:        userID,
			AuthenticatorResponse: wanlib.AuthenticatorResponse{
				ClientDataJSON: in.jsonEncodedClientData,
			},
		},
	}, nil
}

// MakeCredential calls WebAuthNAuthenticatorMakeCredential endpoint from
// webauthn.dll and returns CredentialCreationResponse.
// It interacts with both FIDO2 and Windows Hello depending on opts
// (using auto starts with Windows Hello but there is
// option to select other devices).
// Windows Hello keys are always resident.
func (n *nativeImpl) MakeCredential(origin string, in *makeCredentialRequest) (*wanlib.CredentialCreationResponse, error) {
	hwnd, err := getForegroundWindow()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out *webauthnCredentialAttestation
	ret, _, err := procWebAuthNAuthenticatorMakeCredential.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(in.rp)),
		uintptr(unsafe.Pointer(in.user)),
		uintptr(unsafe.Pointer(in.credParameters)),
		uintptr(unsafe.Pointer(in.clientData)),
		uintptr(unsafe.Pointer(in.opts)),
		uintptr(unsafe.Pointer(&out)),
	)
	if ret != 0 {
		return nil, trace.Wrap(getErrorNameOrLastErr(ret, err))
	}
	if out == nil {
		return nil, errors.New("unexpected nil response from MakeCredential")
	}

	// Note that we need to copy bytes out of `out` if we want to free object.
	// That's why bytesFromCBytes is used.
	// We don't care about free error so ignore it explicitly.
	defer func() { _ = freeCredentialAttestation(out) }()

	credential := bytesFromCBytes(out.cbCredentialID, out.pbCredentialID)

	return &wanlib.CredentialCreationResponse{
		PublicKeyCredential: wanlib.PublicKeyCredential{
			Credential: wanlib.Credential{
				ID:   base64.RawURLEncoding.EncodeToString(credential),
				Type: string(protocol.PublicKeyCredentialType),
			},
			RawID: credential,
		},
		AttestationResponse: wanlib.AuthenticatorAttestationResponse{
			AuthenticatorResponse: wanlib.AuthenticatorResponse{
				ClientDataJSON: in.jsonEncodedClientData,
			},
			AttestationObject: bytesFromCBytes(out.cbAttestationObject, out.pbAttestationObject),
		},
	}, nil
}

func freeCredentialAttestation(in *webauthnCredentialAttestation) error {
	_, _, err := procWebAuthNFreeCredentialAttestation.Call(
		uintptr(unsafe.Pointer(in)),
	)
	if err != syscall.Errno(0) {
		return err
	}
	return nil
}

func freeAssertion(in *webauthnAssertion) error {
	_, _, err := procWebAuthNFreeAssertion.Call(
		uintptr(unsafe.Pointer(in)),
	)
	if err != syscall.Errno(0) {
		return err
	}
	return nil
}

// checkIfDLLExistsAndGetAPIVersionNumber checks if dll exists and tries to load
// it's version via API call. This function makes sure to not panic if dll is
// missing.
func checkIfDLLExistsAndGetAPIVersionNumber() (int, error) {
	if err := modWebAuthn.Load(); err != nil {
		return 0, err
	}
	if err := procWebAuthNGetApiVersionNumber.Find(); err != nil {
		return 0, err
	}
	// This is the only API call of Windows Webauthn API that returns non-zero
	// value when everything went fine.
	// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L895-L897
	ret, _, err := procWebAuthNGetApiVersionNumber.Call()
	if ret == 0 && err != syscall.Errno(0) {
		return 0, err
	}
	return int(ret), nil
}

func getErrorNameOrLastErr(in uintptr, lastError error) error {
	ret, _, _ := procWebAuthNGetErrorName.Call(
		uintptr(int32(in)),
	)
	if ret == 0 {
		if lastError != syscall.Errno(0) {
			return fmt.Errorf("webauthn error code %v and syscall err: %v", in, lastError)
		}
		return fmt.Errorf("webauthn error code %v", in)
	}
	errString := windows.UTF16PtrToString((*uint16)(unsafe.Pointer(ret)))
	return fmt.Errorf("webauthn error code %v: %v", in, errString)
}

func isUVPlatformAuthenticatorAvailable() (bool, error) {
	var out uint32
	ret, _, err := procWebAuthNIsUserVerifyingPlatformAuthenticatorAvailable.Call(
		uintptr(unsafe.Pointer(&out)),
	)
	if ret != 0 {
		return false, getErrorNameOrLastErr(ret, err)
	}
	return out == 1, nil
}

// bytesFromCBytes gets slice of bytes from C type and copies it to new slice
// so that it won't interfere when main objects is free.
func bytesFromCBytes(size uint32, p *byte) []byte {
	if p == nil {
		return nil
	}
	if *p == 0 {
		return nil
	}
	tmp := unsafe.Slice(p, size)
	out := make([]byte, len(tmp))
	copy(out, tmp)
	return out
}

func getForegroundWindow() (hwnd syscall.Handle, err error) {
	r0, _, err := procGetForegroundWindow.Call()
	if err != syscall.Errno(0) {
		return syscall.InvalidHandle, err
	}
	return syscall.Handle(r0), nil
}
