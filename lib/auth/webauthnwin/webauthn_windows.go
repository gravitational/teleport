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
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"syscall"
	"unsafe"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"

	"github.com/gravitational/teleport"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
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
	n := &nativeImpl{
		hasCompileSupport: true,
	}

	logger := slog.With(teleport.ComponentKey, "WebAuthnWin")
	ctx := context.Background()

	// Explicitly loading the module avoids a panic when calling DLL functions if
	// the DLL is missing.
	// https://github.com/gravitational/teleport/issues/36851
	if err := modWebAuthn.Load(); err != nil {
		logger.DebugContext(ctx, "failed to load WebAuthn.dll (it's likely missing)", "error", err)
		return n
	}
	// Load WebAuthNGetApiVersionNumber explicitly too, it avoids a panic on some
	// Windows Server 2019 installs.
	if err := procWebAuthNGetApiVersionNumber.Find(); err != nil {
		logger.DebugContext(ctx, "failed to load WebAuthNGetApiVersionNumber", "error", err)
		return n
	}

	v, err := webAuthNGetApiVersionNumber()
	if err != nil {
		logger.DebugContext(ctx, "failed to check version", "error", err)
		return n
	}
	n.webauthnAPIVersion = v
	n.isAvailable = v > 0

	if !n.isAvailable {
		return n
	}

	n.hasPlatformUV, err = isUVPlatformAuthenticatorAvailable()
	if err != nil {
		// This should not happen if dll exists, however we are fine with
		// to proceed without uvPlatform.
		logger.DebugContext(ctx, "failed to check isUVPlatformAuthenticatorAvailable", "error", err)
	}

	return n
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
func (n *nativeImpl) GetAssertion(origin string, in *getAssertionRequest) (*wantypes.CredentialAssertionResponse, error) {
	hwnd, err := getForegroundWindow()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out *webauthnAssertion
	ret, err := webAuthNAuthenticatorGetAssertion(hwnd, in.rpID, in.clientData, in.opts, &out)
	if ret != 0 {
		return nil, trace.Wrap(getErrorNameOrLastErr(ret, err))
	}
	if out == nil {
		return nil, errors.New("unexpected nil response from GetAssertion")
	}

	// Note that we need to copy bytes out of `out` if we want to free object.
	// That's why bytesFromCBytes is used.
	defer freeAssertion(out)

	authData := bytesFromCBytes(out.cbAuthenticatorData, out.pbAuthenticatorData)
	signature := bytesFromCBytes(out.cbSignature, out.pbSignature)
	userID := bytesFromCBytes(out.cbUserID, out.pbUserID)
	credential := bytesFromCBytes(out.Credential.cbID, out.Credential.pbID)
	credType := windows.UTF16PtrToString(out.Credential.pwszCredentialType)

	return &wantypes.CredentialAssertionResponse{
		PublicKeyCredential: wantypes.PublicKeyCredential{
			RawID: credential,
			Credential: wantypes.Credential{
				ID:   base64.RawURLEncoding.EncodeToString(credential),
				Type: credType,
			},
		},
		AssertionResponse: wantypes.AuthenticatorAssertionResponse{
			AuthenticatorData: authData,
			Signature:         signature,
			UserHandle:        userID,
			AuthenticatorResponse: wantypes.AuthenticatorResponse{
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
func (n *nativeImpl) MakeCredential(origin string, in *makeCredentialRequest) (*wantypes.CredentialCreationResponse, error) {
	hwnd, err := getForegroundWindow()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out *webauthnCredentialAttestation
	ret, err := webAuthNAuthenticatorMakeCredential(
		hwnd, in.rp, in.user, in.credParameters, in.clientData, in.opts, &out)
	if ret != 0 {
		return nil, trace.Wrap(getErrorNameOrLastErr(ret, err))
	}
	if out == nil {
		return nil, errors.New("unexpected nil response from MakeCredential")
	}

	// Note that we need to copy bytes out of `out` if we want to free object.
	// That's why bytesFromCBytes is used.
	defer freeCredentialAttestation(out)

	credential := bytesFromCBytes(out.cbCredentialID, out.pbCredentialID)

	return &wantypes.CredentialCreationResponse{
		PublicKeyCredential: wantypes.PublicKeyCredential{
			Credential: wantypes.Credential{
				ID:   base64.RawURLEncoding.EncodeToString(credential),
				Type: string(protocol.PublicKeyCredentialType),
			},
			RawID: credential,
		},
		AttestationResponse: wantypes.AuthenticatorAttestationResponse{
			AuthenticatorResponse: wantypes.AuthenticatorResponse{
				ClientDataJSON: in.jsonEncodedClientData,
			},
			AttestationObject: bytesFromCBytes(out.cbAttestationObject, out.pbAttestationObject),
		},
	}, nil
}

func getErrorNameOrLastErr(in uintptr, lastError error) error {
	ret := webAuthNGetErrorName(in)
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
	var out bool
	ret, err := webAuthNIsUserVerifyingPlatformAuthenticatorAvailable(&out)
	if err != nil {
		return false, getErrorNameOrLastErr(ret, err)
	}
	return out, nil
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
