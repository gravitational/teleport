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
	"encoding/json"
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/duo-labs/webauthn/protocol"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
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
		log.WithError(err).Warn("WebAuthnWin: failed to check version")
		return &nativeImpl{
			hasCompileSupport: true,
			isAvailable:       false,
		}
	}
	uvPlatform, err := isUVPlatformAuthenticatorAvailable()
	if err != nil {
		// This should not happen if dll exists, however we are fine with
		// to proceed without uvPlatform.
		log.WithError(err).Warn("WebAuthnWin: failed to check isUVPlatformAuthenticatorAvailable")
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
// loginOpts.AuthenticatorAttachment (using auto results in possibilty to select
// either security key or Windows Hello).
// It does not accept username - during passwordless login webauthn.dll provides
// its own dialog with credentials selection.
func (n *nativeImpl) GetAssertion(origin string, in *wanlib.CredentialAssertion, loginOpts *LoginOpts) (*wanlib.CredentialAssertionResponse, error) {
	hwnd, err := getForegroundWindow()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rpid, err := windows.UTF16PtrFromString(in.Response.RelyingPartyID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cd, jsonEncodedCD, err := clientDataToCType(in.Response.Challenge.String(), origin, string(protocol.AssertCeremony))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	opts, err := n.assertOptionsToCType(in.Response, loginOpts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out *webauthnAssertion
	ret, _, err := procWebAuthNAuthenticatorGetAssertion.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(rpid)),
		uintptr(unsafe.Pointer(cd)),
		uintptr(unsafe.Pointer(opts)),
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
	userID := bytesFromCBytes(out.cbUserId, out.pbUserId)
	credential := bytesFromCBytes(out.Credential.cbId, out.Credential.pbId)
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
				ClientDataJSON: jsonEncodedCD,
			},
		},
	}, nil
}

// MakeCredential calls WebAuthNAuthenticatorMakeCredential endpoint from
// webauthn.dll and returns CredentialCreationResponse.
// It interacts with both FIDO2 and Windows Hello depending on
// wanlib.CredentialCreation (using auto starts with Windows Hello but there is
// option to select other devices).
// Windows Hello keys are always resident.
func (n *nativeImpl) MakeCredential(origin string, in *wanlib.CredentialCreation) (*wanlib.CredentialCreationResponse, error) {
	hwnd, err := getForegroundWindow()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rp, err := rpToCType(in.Response.RelyingParty)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u, err := userToCType(in.Response.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	credParam, err := credParamToCType(in.Response.Parameters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cd, jsonEncodedCD, err := clientDataToCType(in.Response.Challenge.String(), origin, string(protocol.CreateCeremony))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	opts, err := n.makeCredOptionsToCType(in.Response)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out *webauthnCredentialAttestation
	ret, _, err := procWebAuthNAuthenticatorMakeCredential.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(rp)),
		uintptr(unsafe.Pointer(u)),
		uintptr(unsafe.Pointer(credParam)),
		uintptr(unsafe.Pointer(cd)),
		uintptr(unsafe.Pointer(opts)),
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

	credential := bytesFromCBytes(out.cbCredentialId, out.pbCredentialId)

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
				ClientDataJSON: jsonEncodedCD,
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

func (n *nativeImpl) assertOptionsToCType(in protocol.PublicKeyCredentialRequestOptions, loginOpts *LoginOpts) (*webauthnAuthenticatorGetAssertionOptions, error) {
	allowCredList, err := credentialsExToCType(in.AllowedCredentials)
	if err != nil {
		return nil, err
	}

	var dwAuthenticatorAttachment uint32
	if loginOpts != nil {
		switch loginOpts.AuthenticatorAttachment {
		case AttachmentPlatform:
			dwAuthenticatorAttachment = 1
		case AttachmentCrossPlatform:
			dwAuthenticatorAttachment = 2
		}
	}

	return &webauthnAuthenticatorGetAssertionOptions{
		// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L36-L97
		// contains information about different versions.
		// We can set newest version and it still works on older APIs.
		dwVersion:                     6,
		dwTimeoutMilliseconds:         uint32(in.Timeout),
		dwAuthenticatorAttachment:     dwAuthenticatorAttachment,
		dwUserVerificationRequirement: userVerificationToCType(in.UserVerification),
		// TODO(tobiaszheller): support U2fAppId.
		pAllowCredentialList: allowCredList,
	}, nil
}

func rpToCType(in protocol.RelyingPartyEntity) (*webauthnRPEntityInformation, error) {
	if in.ID == "" {
		return nil, errors.New("missing RelyingPartyEntity.Id")
	}
	if in.Name == "" {
		return nil, errors.New("missing RelyingPartyEntity.Name")
	}
	id, err := windows.UTF16PtrFromString(in.ID)
	if err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(in.Name)
	if err != nil {
		return nil, err
	}
	var icon *uint16
	if in.Icon != "" {
		icon, err = windows.UTF16PtrFromString(in.Icon)
		if err != nil {
			return nil, err
		}
	}
	return &webauthnRPEntityInformation{
		dwVersion: 1,
		pwszId:    id,
		pwszName:  name,
		pwszIcon:  icon,
	}, nil
}

func userToCType(in protocol.UserEntity) (*webauthnUserEntityInformation, error) {
	if len(in.ID) == 0 {
		return nil, errors.New("missing UserEntity.Id")
	}
	if in.Name == "" {
		return nil, errors.New("missing UserEntity.Name")
	}

	name, err := windows.UTF16PtrFromString(in.Name)
	if err != nil {
		return nil, err
	}
	var displayName *uint16
	if in.DisplayName != "" {
		displayName, err = windows.UTF16PtrFromString(in.DisplayName)
		if err != nil {
			return nil, err
		}
	}
	var icon *uint16
	if in.Icon != "" {
		icon, err = windows.UTF16PtrFromString(in.Icon)
		if err != nil {
			return nil, err
		}
	}
	return &webauthnUserEntityInformation{
		dwVersion:       1,
		cbId:            uint32(len(in.ID)),
		pbId:            &in.ID[0],
		pwszName:        name,
		pwszDisplayName: displayName,
		pwszIcon:        icon,
	}, nil
}

func credParamToCType(in []protocol.CredentialParameter) (*webauthnCoseCredentialParameters, error) {
	if len(in) == 0 {
		return nil, errors.New("missing CredentialParameter")
	}
	out := make([]webauthnCoseCredentialParameter, 0, len(in))
	for _, c := range in {
		pwszCredentialType, err := windows.UTF16PtrFromString(string(c.Type))
		if err != nil {
			return nil, err
		}
		out = append(out, webauthnCoseCredentialParameter{
			dwVersion:          1,
			pwszCredentialType: pwszCredentialType,
			lAlg:               int32(c.Algorithm),
		})
	}
	return &webauthnCoseCredentialParameters{
		cCredentialParameters: uint32(len(out)),
		pCredentialParameters: &out[0],
	}, nil
}

func clientDataToCType(challenge, origin, cdType string) (*webauthnClientData, []byte, error) {
	if challenge == "" {
		return nil, nil, errors.New("missing ClientData.Challenge")
	}
	if origin == "" {
		return nil, nil, errors.New("missing ClientData.Origin")
	}
	algId, err := windows.UTF16PtrFromString("SHA-256")
	if err != nil {
		return nil, nil, err
	}
	type clientDataJson struct {
		Type      string `json:"type"`
		Challenge string `json:"challenge"`
		Origin    string `json:"origin"`
	}
	cd := clientDataJson{
		Type:      cdType,
		Challenge: challenge,
		Origin:    origin,
	}
	jsonCD, err := json.Marshal(cd)
	if err != nil {
		return nil, nil, err
	}
	return &webauthnClientData{
		dwVersion:        1,
		cbClientDataJSON: uint32(len(jsonCD)),
		pbClientDataJSON: &jsonCD[0],
		pwszHashAlgId:    algId,
	}, jsonCD, nil

}

func credentialsExToCType(in []protocol.CredentialDescriptor) (*webauthnCredentialList, error) {
	exCredList := make([]*webauthnCredentialEX, 0, len(in))
	for _, e := range in {
		if e.Type == "" {
			return nil, errors.New("missing CredentialDescriptor.Type")
		}
		if len(e.CredentialID) == 0 {
			return nil, errors.New("missing CredentialDescriptor.CredentialID")
		}
		pwszCredentialType, err := windows.UTF16PtrFromString(string(e.Type))
		if err != nil {
			return nil, err
		}
		exCredList = append(exCredList, &webauthnCredentialEX{
			dwVersion:          1,
			cbId:               uint32(len(e.CredentialID)),
			pbId:               &e.CredentialID[0],
			pwszCredentialType: pwszCredentialType,
			dwTransports:       transportsToCType(e.Transport),
		})
	}

	if len(exCredList) == 0 {
		return nil, nil
	}
	return &webauthnCredentialList{
		cCredentials:  uint32(len(exCredList)),
		ppCredentials: &exCredList[0],
	}, nil
}

func transportsToCType(in []protocol.AuthenticatorTransport) uint32 {
	if len(in) == 0 {
		return 0
	}
	var out uint32
	for _, at := range in {
		// Mappped based on:
		// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L249-L254
		switch at {
		case protocol.USB:
			out += 0x1
		case protocol.NFC:
			out += 0x2
		case protocol.BLE:
			out += 0x4
		case protocol.Internal:
			out += 0x10
		}
	}
	return out
}

func attachmentToCType(in protocol.AuthenticatorAttachment) uint32 {
	// Mapped based on:
	// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L493-L496
	switch in {
	case protocol.Platform:
		return 1
	case protocol.CrossPlatform:
		return 2
	default:
		return 0
	}
}

func conveyancePreferenceToCType(in protocol.ConveyancePreference) uint32 {
	// Mapped based on:
	// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L503-L506
	switch in {
	case protocol.PreferNoAttestation:
		return 1
	case protocol.PreferIndirectAttestation:
		return 2
	case protocol.PreferDirectAttestation:
		return 3
	default:
		return 0
	}
}

func userVerificationToCType(in protocol.UserVerificationRequirement) uint32 {
	// Mapped based on:
	// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L498-L501
	switch in {
	case protocol.VerificationRequired:
		return 1
	case protocol.VerificationPreferred:
		return 2
	case protocol.VerificationDiscouraged:
		return 3
	default:
		return 0
	}
}

func requireResidentKeyToCType(in *bool) uint32 {
	if in == nil {
		return 0
	}
	return boolToUint32(*in)
}

func (n *nativeImpl) makeCredOptionsToCType(in protocol.PublicKeyCredentialCreationOptions) (*webauthnAuthenticatorMakeCredentialOptions, error) {
	exCredList, err := credentialsExToCType(in.CredentialExcludeList)
	if err != nil {
		return nil, err
	}

	// TODO (tobiaszheller): teleport server right now does not support
	// preferResidentKey.
	var bPreferResidentKey uint32
	return &webauthnAuthenticatorMakeCredentialOptions{
		// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L36-L97
		// contains information about different versions.
		// We can set newest version and it still works on older APIs.
		dwVersion:                         5,
		dwTimeoutMilliseconds:             uint32(in.Timeout),
		dwAuthenticatorAttachment:         attachmentToCType(in.AuthenticatorSelection.AuthenticatorAttachment),
		dwAttestationConveyancePreference: conveyancePreferenceToCType(in.Attestation),
		bRequireResidentKey:               requireResidentKeyToCType(in.AuthenticatorSelection.RequireResidentKey),
		dwUserVerificationRequirement:     userVerificationToCType(in.AuthenticatorSelection.UserVerification),
		pExcludeCredentialList:            exCredList,
		bPreferResidentKey:                bPreferResidentKey,
	}, nil
}

func boolToUint32(in bool) uint32 {
	if in {
		return 1
	}
	return 0
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
