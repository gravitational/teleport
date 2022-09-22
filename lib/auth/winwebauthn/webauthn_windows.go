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

//go:build windows
// +build windows

package winwebauthn

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/duo-labs/webauthn/protocol"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

var (
	modWebAuthn = windows.NewLazySystemDLL("WebAuthn.dll")

	procWebAuthNGetApiVersionNumber                           = modWebAuthn.NewProc("WebAuthNGetApiVersionNumber")
	procWebAuthNIsUserVerifyingPlatformAuthenticatorAvailable = modWebAuthn.NewProc("WebAuthNIsUserVerifyingPlatformAuthenticatorAvailable")
	procWebAuthNAuthenticatorMakeCredential                   = modWebAuthn.NewProc("WebAuthNAuthenticatorMakeCredential")
	procWebAuthNFreeCredentialAttestation                     = modWebAuthn.NewProc("WebAuthNFreeCredentialAttestation")
	procWebAuthNAuthenticatorGetAssertion                     = modWebAuthn.NewProc("WebAuthNAuthenticatorGetAssertion")
	procWebAuthNFreeAssertion                                 = modWebAuthn.NewProc("WebAuthNFreeAssertion")
	procWebAuthNGetErrorName                                  = modWebAuthn.NewProc("WebAuthNGetErrorName")

	moduser32               = windows.NewLazySystemDLL("user32.dll")
	procGetForegroundWindow = moduser32.NewProc("GetForegroundWindow")
)

type client struct {
	version           int
	hasCompileSupport bool
	isAvailable       bool
	hasPlatformUV     bool
}

// initClient creates client which contains diagnostics info.
// Diagnostics are safe to cache because dll isn't something that
// could change during program invocation.
// Client will be alaways created, even if dll is missing on system.
func initClient() *client {
	v, err := checkIfDLLExistsAndGetAPIVersionNumber()
	if err != nil {
		log.WithError(err).Warn("Windows webauthn failed to check version")
		return &client{
			hasCompileSupport: true,
			isAvailable:       false,
		}
	}
	uvPlatform, err := isUVPlatformAuthenticatorAvailable()
	if err != nil {
		// This should not happen if dll exists, however we are fine with
		// to proceed without uvPlatform.
		log.WithError(err).Warn("Windows webauthn failed to check isUVPlatformAuthenticatorAvailable")
	}

	return &client{
		version:           v,
		hasCompileSupport: true,
		hasPlatformUV:     uvPlatform,
		isAvailable:       v > 0 && v <= 4,
	}
}

const (
	apiVersion1 = 1
	apiVersion2 = 2
	apiVersion3 = 3
	apiVersion4 = 4
)

func (c client) CheckSupport() CheckSupportResult {
	return CheckSupportResult{
		HasCompileSupport: c.hasCompileSupport,
		IsAvailable:       c.isAvailable,
		HasPlatformUV:     c.hasPlatformUV,
		APIVersion:        c.version,
	}
}

// GetAssertion calls WebAuthNAuthenticatorGetAssertion endpoiont from
// webauthn.dll and returns CredentialAssertionResponse
func (c client) GetAssertion(origin string, in protocol.PublicKeyCredentialRequestOptions, loginOpts *LoginOpts) (*wanlib.CredentialAssertionResponse, error) {
	hwnd, err := getForegroundWindow()
	if err != nil {
		return nil, err
	}
	pwszRpId, err := windows.UTF16PtrFromString(in.RelyingPartyID)
	if err != nil {
		return nil, err
	}
	cd, jsonEncodedCd, err := clientDataToCType(in.Challenge.String(), origin, string(protocol.AssertCeremony))
	if err != nil {
		return nil, err
	}
	opts, err := c.assertOptionsToCType(in, loginOpts)
	if err != nil {
		return nil, err
	}
	var out *_WEBAUTHN_ASSERTION
	ret, _, err := procWebAuthNAuthenticatorGetAssertion.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(pwszRpId)),
		uintptr(unsafe.Pointer(cd)),
		uintptr(unsafe.Pointer(opts)),
		uintptr(unsafe.Pointer(&out)),
	)
	if err != syscall.Errno(0) {
		return nil, err
	}
	if ret != 0 {
		return nil, getErrorName(hresult(ret), ret)
	}
	if out == nil {
		return nil, fmt.Errorf("unexpected nil response from GetAssertion")
	}
	defer freeAssertion(out)

	authData := bytePtrToByte(out.cbAuthenticatorData, out.pbAuthenticatorData)
	signiture := bytePtrToByte(out.cbSignature, out.pbSignature)
	userID := bytePtrToByte(out.cbUserId, out.pbUserId)
	credential := bytePtrToByte(out.Credential.cbId, out.Credential.pbId)
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
			Signature:         signiture,
			UserHandle:        userID,
			AuthenticatorResponse: wanlib.AuthenticatorResponse{
				ClientDataJSON: jsonEncodedCd,
			},
		},
	}, nil
}

// MakeCredential calls WebAuthNAuthenticatorMakeCredential endpoiont from
// webauthn.dll and returns CredentialCreationResponse
func (c client) MakeCredential(origin string, in protocol.PublicKeyCredentialCreationOptions) (*wanlib.CredentialCreationResponse, error) {
	hwnd, err := getForegroundWindow()
	if err != nil {
		return nil, err
	}
	rp, err := rpToCType(in.RelyingParty)
	if err != nil {
		return nil, err
	}
	u, err := userToCType(in.User)
	if err != nil {
		return nil, err
	}
	credParam, err := credParamToCType(in.Parameters)
	if err != nil {
		return nil, err
	}
	cd, jsonEncodedCd, err := clientDataToCType(in.Challenge.String(), origin, string(protocol.CreateCeremony))
	if err != nil {
		return nil, err
	}
	opts, err := c.makeCredOptionsToCType(in)
	if err != nil {
		return nil, err
	}
	var out *_WEBAUTHN_CREDENTIAL_ATTESTATION
	ret, _, err := procWebAuthNAuthenticatorMakeCredential.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(rp)),
		uintptr(unsafe.Pointer(u)),
		uintptr(unsafe.Pointer(credParam)),
		uintptr(unsafe.Pointer(cd)),
		uintptr(unsafe.Pointer(opts)),
		uintptr(unsafe.Pointer(&out)),
	)
	if err != syscall.Errno(0) {
		return nil, err
	}
	if ret != 0 {
		return nil, getErrorName(hresult(ret), ret)
	}
	if out == nil {
		return nil, fmt.Errorf("unexpected nil response from MakeCredential")
	}

	defer freeCredentialAttestation(out)

	credential := bytePtrToByte(out.CbCredentialId, out.PbCredentialId)

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
				ClientDataJSON: jsonEncodedCd,
			},
			AttestationObject: bytePtrToByte(out.CbAttestationObject, out.PbAttestationObject),
		},
	}, nil
}

func freeCredentialAttestation(in *_WEBAUTHN_CREDENTIAL_ATTESTATION) error {
	_, _, err := procWebAuthNFreeCredentialAttestation.Call(
		uintptr(unsafe.Pointer(in)),
	)
	if err != syscall.Errno(0) {
		return err
	}
	return nil
}

func freeAssertion(in *_WEBAUTHN_ASSERTION) error {
	_, _, err := procWebAuthNFreeAssertion.Call(
		uintptr(unsafe.Pointer(in)),
	)
	if err != syscall.Errno(0) {
		return err
	}
	return nil
}

type hresult int32

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
	ret, _, err := procWebAuthNGetApiVersionNumber.Call()
	if err != syscall.Errno(0) {
		return 0, err
	}
	return int(ret), nil
}

func getErrorName(in hresult, originCode uintptr) error {
	ret, _, err := procWebAuthNGetErrorName.Call(
		uintptr(in),
	)
	if err != syscall.Errno(0) {
		return fmt.Errorf("could not check error name for %x because of: %x", ret, err)
	}
	errString := windows.UTF16PtrToString((*uint16)(unsafe.Pointer(ret)))
	return fmt.Errorf("Webauthn err for code %v: %s", originCode, errString)
}

func isUVPlatformAuthenticatorAvailable() (bool, error) {
	var out uint32
	ret, _, err := procWebAuthNIsUserVerifyingPlatformAuthenticatorAvailable.Call(
		uintptr(unsafe.Pointer(&out)),
	)
	if err != syscall.Errno(0) {
		return false, err
	}
	if ret != 0 {
		return false, getErrorName(hresult(ret), ret)
	}
	return out == 1, nil
}

func (c client) assertOptionsToCType(in protocol.PublicKeyCredentialRequestOptions, loginOpts *LoginOpts) (*_WEBAUTHN_AUTHENTICATOR_GET_ASSERTION_OPTIONS, error) {
	allowCredList, err := credentialsExToCType(in.AllowedCredentials)
	if err != nil {
		return nil, err
	}
	var dwVersion uint32
	switch c.version {
	case apiVersion1, apiVersion2:
		dwVersion = 4
	case apiVersion3:
		dwVersion = 5
	case apiVersion4:
		dwVersion = 6
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

	return &_WEBAUTHN_AUTHENTICATOR_GET_ASSERTION_OPTIONS{
		dwVersion:                     dwVersion,
		dwTimeoutMilliseconds:         uint32(in.Timeout),
		dwAuthenticatorAttachment:     dwAuthenticatorAttachment,
		dwUserVerificationRequirement: userVerificationToCType(in.UserVerification),
		// TODO(tobiaszheller): check if we need to support U2fAppId.
		// pwszU2fAppId: ,
		pAllowCredentialList: allowCredList,
	}, nil
}

func rpToCType(in protocol.RelyingPartyEntity) (*_WEBAUTHN_RP_ENTITY_INFORMATION, error) {
	if in.ID == "" {
		return nil, fmt.Errorf("missing RelyingPartyEntity.Id")
	}
	if in.Name == "" {
		return nil, fmt.Errorf("missing RelyingPartyEntity.Name")
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
	return &_WEBAUTHN_RP_ENTITY_INFORMATION{
		dwVersion: 1,
		pwszId:    id,
		pwszName:  name,
		pwszIcon:  icon,
	}, nil
}

func userToCType(in protocol.UserEntity) (*_WEBAUTHN_USER_ENTITY_INFORMATION, error) {
	if len(in.ID) == 0 {
		return nil, fmt.Errorf("missing UserEntity.Id")
	}
	if in.Name == "" {
		return nil, fmt.Errorf("missing UserEntity.Name")
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
	return &_WEBAUTHN_USER_ENTITY_INFORMATION{
		dwVersion:       1,
		cbId:            uint32(len(in.ID)),
		pbId:            &in.ID[0],
		pwszName:        name,
		pwszDisplayName: displayName,
		pwszIcon:        icon,
	}, nil
}

func credParamToCType(in []protocol.CredentialParameter) (*_WEBAUTHN_COSE_CREDENTIAL_PARAMETERS, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("missing CredentialParameter")
	}
	out := make([]_WEBAUTHN_COSE_CREDENTIAL_PARAMETER, 0, len(in))
	for _, c := range in {
		pwszCredentialType, err := windows.UTF16PtrFromString(string(c.Type))
		if err != nil {
			return nil, err
		}
		out = append(out, _WEBAUTHN_COSE_CREDENTIAL_PARAMETER{
			dwVersion:          1,
			pwszCredentialType: pwszCredentialType,
			lAlg:               int32(c.Algorithm),
		})
	}
	return &_WEBAUTHN_COSE_CREDENTIAL_PARAMETERS{
		cCredentialParameters: uint32(len(out)),
		pCredentialParameters: &out[0],
	}, nil
}

func clientDataToCType(challenge, origin, cdType string) (*_WEBAUTHN_CLIENT_DATA, []byte, error) {
	if challenge == "" {
		return nil, nil, fmt.Errorf("missing ClientData.Challenge")
	}
	if origin == "" {
		return nil, nil, fmt.Errorf("missing ClientData.Origin")
	}
	algId, err := windows.UTF16PtrFromString("SHA-256")
	if err != nil {
		return nil, nil, err
	}
	cd := clientDataJson{
		Type:        cdType,
		Challenge:   challenge,
		Origin:      origin,
		CrossOrigin: false,
	}
	bb, err := json.Marshal(cd)
	if err != nil {
		return nil, nil, err
	}
	return &_WEBAUTHN_CLIENT_DATA{
		dwVersion:        1,
		cbClientDataJSON: uint32(len(bb)),
		pbClientDataJSON: &bb[0],
		pwszHashAlgId:    algId,
	}, bb, nil

}

func credentialsExToCType(in []protocol.CredentialDescriptor) (*_WEBAUTHN_CREDENTIAL_LIST, error) {
	exCredList := make([]*_WEBAUTHN_CREDENTIAL_EX, 0, len(in))
	for _, e := range in {
		if e.Type == "" {
			return nil, fmt.Errorf("missing CredentialDescriptor.Type")
		}
		if len(e.CredentialID) == 0 {
			return nil, fmt.Errorf("missing CredentialDescriptor.CredentialID")
		}
		pwszCredentialType, err := windows.UTF16PtrFromString(string(e.Type))
		if err != nil {
			return nil, err
		}
		exCredList = append(exCredList, &_WEBAUTHN_CREDENTIAL_EX{
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
	return &_WEBAUTHN_CREDENTIAL_LIST{
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
		// WEBAUTHN_CTAP_TRANSPORT_USB         0x00000001
		// WEBAUTHN_CTAP_TRANSPORT_NFC         0x00000002
		// WEBAUTHN_CTAP_TRANSPORT_BLE         0x00000004
		// WEBAUTHN_CTAP_TRANSPORT_TEST        0x00000008
		// WEBAUTHN_CTAP_TRANSPORT_INTERNAL    0x00000010
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
	// WEBAUTHN_AUTHENTICATOR_ATTACHMENT_ANY                               0
	// WEBAUTHN_AUTHENTICATOR_ATTACHMENT_PLATFORM                          1
	// WEBAUTHN_AUTHENTICATOR_ATTACHMENT_CROSS_PLATFORM                    2
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
	// WEBAUTHN_ATTESTATION_CONVEYANCE_PREFERENCE_ANY                      0
	// WEBAUTHN_ATTESTATION_CONVEYANCE_PREFERENCE_NONE                     1
	// WEBAUTHN_ATTESTATION_CONVEYANCE_PREFERENCE_INDIRECT                 2
	// WEBAUTHN_ATTESTATION_CONVEYANCE_PREFERENCE_DIRECT                   3
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
	// WEBAUTHN_USER_VERIFICATION_REQUIREMENT_ANY                          0
	// WEBAUTHN_USER_VERIFICATION_REQUIREMENT_REQUIRED                     1
	// WEBAUTHN_USER_VERIFICATION_REQUIREMENT_PREFERRED                    2
	// WEBAUTHN_USER_VERIFICATION_REQUIREMENT_DISCOURAGED                  3
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

func (c client) makeCredOptionsToCType(in protocol.PublicKeyCredentialCreationOptions) (*_WEBAUTHN_AUTHENTICATOR_MAKE_CREDENTIAL_OPTIONS, error) {
	exCredList, err := credentialsExToCType(in.CredentialExcludeList)
	if err != nil {
		return nil, err
	}

	var dwVersion uint32
	switch c.version {
	case apiVersion1, apiVersion2:
		dwVersion = 3
	case apiVersion3:
		dwVersion = 4
	case apiVersion4:
		dwVersion = 5
	}

	// TODO (tobiaszheller): version of duo-labs/webatuhn used in teleport does
	// not support PreferResidentKey field. We cannot easily update version due
	// to dependency issues.
	var bPreferResidentKey uint32
	return &_WEBAUTHN_AUTHENTICATOR_MAKE_CREDENTIAL_OPTIONS{
		dwVersion:                         dwVersion,
		dwTimeoutMilliseconds:             uint32(in.Timeout),
		dwAuthenticatorAttachment:         attachmentToCType(in.AuthenticatorSelection.AuthenticatorAttachment),
		dwAttestationConveyancePreference: conveyancePreferenceToCType(in.Attestation),
		bRequireResidentKey:               requireResidentKeyToCType(in.AuthenticatorSelection.RequireResidentKey),
		dwUserVerificationRequirement:     userVerificationToCType(in.AuthenticatorSelection.UserVerification),
		pExcludeCredentialList:            exCredList,
		bPreferResidentKey:                bPreferResidentKey,
	}, nil
}

type _WEBAUTHN_RP_ENTITY_INFORMATION struct {
	dwVersion uint32
	// Identifier for the RP. This field is required.
	pwszId *uint16
	// Contains the friendly name of the Relying Party, such as "Acme Corporation", "Widgets Inc" or "Awesome Site".
	// This field is required.
	pwszName *uint16
	// Optional URL pointing to RP's logo.
	pwszIcon *uint16
}

type _WEBAUTHN_USER_ENTITY_INFORMATION struct {
	dwVersion uint32
	// Identifier for the User. This field is required.
	cbId uint32
	pbId *byte
	// Contains a detailed name for this account, such as "john.p.smith@example.com".
	pwszName *uint16
	// Optional URL that can be used to retrieve an image containing the user's current avatar,
	// or a data URI that contains the image data.
	pwszIcon *uint16
	// For User: Contains the friendly name associated with the user account by the Relying Party, such as "John P. Smith".
	pwszDisplayName *uint16
}

type _WEBAUTHN_COSE_CREDENTIAL_PARAMETERS struct {
	cCredentialParameters uint32
	pCredentialParameters *_WEBAUTHN_COSE_CREDENTIAL_PARAMETER
}

type _WEBAUTHN_COSE_CREDENTIAL_PARAMETER struct {
	dwVersion uint32
	// Well-known credential type specifying a credential to create.
	pwszCredentialType *uint16
	// Well-known COSE algorithm specifying the algorithm to use for the credential.
	lAlg int32
}

type _WEBAUTHN_AUTHENTICATOR_MAKE_CREDENTIAL_OPTIONS struct {
	dwVersion             uint32
	dwTimeoutMilliseconds uint32
	// Credentials used for exclusion.
	CredentialList _WEBAUTHN_CREDENTIALS
	// Optional extensions to parse when performing the operation.
	Extensions _WEBAUTHN_EXTENSIONS
	// Optional. Platform vs Cross-Platform Authenticators.
	dwAuthenticatorAttachment uint32
	// Optional. Require key to be resident or not. Defaulting to FALSE.
	bRequireResidentKey uint32
	// User Verification Requirement.
	dwUserVerificationRequirement uint32
	// Attestation Conveyance Preference.
	dwAttestationConveyancePreference uint32
	// Reserved for future Use
	dwFlags uint32

	//
	// The following fields have been added in WEBAUTHN_AUTHENTICATOR_MAKE_CREDENTIAL_OPTIONS_VERSION_2
	//

	// Cancellation Id - Optional - See WebAuthNGetCancellationId
	pCancellationId *windows.GUID

	//
	// The following fields have been added in WEBAUTHN_AUTHENTICATOR_MAKE_CREDENTIAL_OPTIONS_VERSION_3
	//

	// Exclude Credential List. If present, "CredentialList" will be ignored.
	pExcludeCredentialList *_WEBAUTHN_CREDENTIAL_LIST

	//
	// The following fields have been added in WEBAUTHN_AUTHENTICATOR_MAKE_CREDENTIAL_OPTIONS_VERSION_4
	//

	// Enterprise Attestation
	dwEnterpriseAttestation uint32
	// Large Blob Support: none, required or preferred
	// NTE_INVALID_PARAMETER when large blob required or preferred and
	//   bRequireResidentKey isn't set to TRUE
	dwLargeBlobSupport uint32
	// Optional. Prefer key to be resident. Defaulting to FALSE. When TRUE,
	// overrides the above bRequireResidentKey.
	bPreferResidentKey uint32

	//
	// The following fields have been added in WEBAUTHN_AUTHENTICATOR_MAKE_CREDENTIAL_OPTIONS_VERSION_5
	//

	// Optional. BrowserInPrivate Mode. Defaulting to FALSE.
	bBrowserInPrivateMode uint32
}

type _WEBAUTHN_CREDENTIALS struct {
	cCredentials uint32
	pCredentials *_WEBAUTHN_CREDENTIAL
}

type _WEBAUTHN_CREDENTIAL struct {
	dwVersion uint32
	// Size of pbID.
	cbId uint32
	pbId *byte
	// Well-known credential type specifying what this particular credential is.
	pwszCredentialType *uint16
}

type _WEBAUTHN_EXTENSION struct {
	pwszExtensionIdentifier *uint16
	cbExtension             uint32
	pvExtension             *byte
}
type _WEBAUTHN_EXTENSIONS struct {
	cExtensions uint32
	pExtensions *_WEBAUTHN_EXTENSION
}

type _WEBAUTHN_CREDENTIAL_EX struct {
	dwVersion uint32
	// Size of pbID.
	cbId uint32
	// Unique ID for this particular credential.
	pbId *byte
	// Well-known credential type specifying what this particular credential is.
	pwszCredentialType *uint16
	// Transports. 0 implies no transport restrictions.
	dwTransports uint32
}
type _WEBAUTHN_CREDENTIAL_LIST struct {
	cCredentials  uint32
	ppCredentials **_WEBAUTHN_CREDENTIAL_EX
}

type _WEBAUTHN_CREDENTIAL_ATTESTATION struct {
	DwVersion uint32
	// Attestation format type
	PwszFormatType *uint16
	// Size of CbAuthenticatorData.
	CbAuthenticatorData uint32
	// Authenticator data that was created for this credential.
	PbAuthenticatorData *byte
	// Size of CBOR encoded attestation information
	//0 => encoded as CBOR null value.
	CbAttestation uint32
	//Encoded CBOR attestation information
	PbAttestation           *byte
	DwAttestationDecodeType uint32
	// Following depends on the dwAttestationDecodeType
	//  WEBAUTHN_ATTESTATION_DECODE_NONE
	//      NULL - not able to decode the CBOR attestation information
	//  WEBAUTHN_ATTESTATION_DECODE_COMMON
	//      PWEBAUTHN_COMMON_ATTESTATION;
	PvAttestationDecode *byte
	// The CBOR encoded Attestation Object to be returned to the RP.
	CbAttestationObject uint32
	PbAttestationObject *byte
	// The CredentialId bytes extracted from the Authenticator Data.
	// Used by Edge to return to the RP.
	CbCredentialId uint32
	PbCredentialId *byte

	//
	// Following fields have been added in WEBAUTHN_CREDENTIAL_ATTESTATION_VERSION_2
	//

	Extensions _WEBAUTHN_EXTENSIONS

	//
	// Following fields have been added in WEBAUTHN_CREDENTIAL_ATTESTATION_VERSION_3
	//

	// One of the WEBAUTHN_CTAP_TRANSPORT_* bits will be set corresponding to
	// the transport that was used.
	DwUsedTransport uint32

	//
	// Following fields have been added in WEBAUTHN_CREDENTIAL_ATTESTATION_VERSION_4
	//

	EpAtt              uint32
	LargeBlobSupported uint32
	ResidentKey        uint32
}

type _WEBAUTHN_CLIENT_DATA struct {
	dwVersion uint32
	// Size of the pbClientDataJSON field.
	cbClientDataJSON uint32
	// UTF-8 encoded JSON serialization of the client data.
	pbClientDataJSON *byte
	// Hash algorithm ID used to hash the pbClientDataJSON field.
	pwszHashAlgId *uint16
}

type _WEBAUTHN_AUTHENTICATOR_GET_ASSERTION_OPTIONS struct {
	dwVersion uint32
	// Time that the operation is expected to complete within.
	// This is used as guidance, and can be overridden by the platform.
	dwTimeoutMilliseconds uint32
	// Allowed Credentials List.
	CredentialList _WEBAUTHN_CREDENTIALS
	// Optional extensions to parse when performing the operation.
	Extensions _WEBAUTHN_EXTENSIONS
	// Optional. Platform vs Cross-Platform Authenticators.
	dwAuthenticatorAttachment uint32
	// User Verification Requirement.
	dwUserVerificationRequirement uint32
	// Flags
	dwFlags uint32

	//
	// The following fields have been added in WEBAUTHN_AUTHENTICATOR_GET_ASSERTION_OPTIONS_VERSION_2
	//

	// Optional identifier for the U2F AppId. Converted to UTF8 before being hashed. Not lower cased.
	pwszU2fAppId *uint16
	// If the following is non-NULL, then, set to TRUE if the above pwszU2fAppid was used instead of
	// PCWSTR pwszRpId;
	pbU2fAppId uint32

	//
	// The following fields have been added in WEBAUTHN_AUTHENTICATOR_GET_ASSERTION_OPTIONS_VERSION_3
	//

	// Cancellation Id - Optional - See WebAuthNGetCancellationId
	pCancellationId *windows.GUID

	//
	// The following fields have been added in WEBAUTHN_AUTHENTICATOR_GET_ASSERTION_OPTIONS_VERSION_4
	//

	// Allow Credential List. If present, "CredentialList" will be ignored.
	pAllowCredentialList *_WEBAUTHN_CREDENTIAL_LIST

	//
	// The following fields have been added in WEBAUTHN_AUTHENTICATOR_GET_ASSERTION_OPTIONS_VERSION_5
	//

	dwCredLargeBlobOperation uint32
	// Size of pbCredLargeBlob
	cbCredLargeBlob uint32
	pbCredLargeBlob *byte
}

type _WEBAUTHN_ASSERTION struct {
	dwVersion uint32

	// Size of cbAuthenticatorData.
	cbAuthenticatorData uint32
	// Authenticator data that was created for this assertion.
	pbAuthenticatorData *byte

	// Size of pbSignature.
	cbSignature uint32
	// Signature that was generated for this assertion.
	pbSignature *byte

	// Credential that was used for this assertion.
	Credential _WEBAUTHN_CREDENTIAL

	// Size of User Id
	cbUserId uint32
	// UserId
	pbUserId *byte

	//
	// Following fields have been added in WEBAUTHN_ASSERTION_VERSION_2
	//

	Extensions _WEBAUTHN_EXTENSIONS

	// Size of pbCredLargeBlob
	cbCredLargeBlob       uint32
	pbCredLargeBlob       *byte
	dwCredLargeBlobStatus uint32
}

type _WEBAUTHN_X5C struct {
	// Length of X.509 encoded certificate
	cbData uint32
	// X.509 encoded certificate bytes
	pbData *byte
}

type _WEBAUTHN_COMMON_ATTESTATION struct {
	dwVersion uint32

	// Hash and Padding Algorithm
	//
	// The following won't be set for "fido-u2f" which assumes "ES256".
	pwszAlg *uint16
	lAlg    int32 // COSE algorithm

	// Signature that was generated for this attestation.
	cbSignature uint32
	pbSignature *byte

	// Following is set for Full Basic Attestation. If not, set then, this is Self Attestation.
	// Array of X.509 DER encoded certificates. The first certificate is the signer, leaf certificate.
	cX5c uint32
	pX5c *_WEBAUTHN_X5C

	// Following are also set for tpm
	pwszVer    *uint16 // L"2.0"
	cbCertInfo uint32
	pbCertInfo *byte
	cbPubArea  uint32
	pbPubArea  *byte
}

type clientDataJson struct {
	Type        string `json:"type"`
	Challenge   string `json:"challenge"`
	Origin      string `json:"origin"`
	CrossOrigin bool   `json:"cross_origin,omitempty"`
}

func boolToUint32(in bool) uint32 {
	if in {
		return 1
	}
	return 0
}

func bytePtrToByte(size uint32, p *byte) []byte {
	if p == nil {
		return nil
	}
	if *p == 0 {
		return nil
	}

	return unsafe.Slice(p, size)
}

func getForegroundWindow() (hwnd syscall.Handle, err error) {
	r0, _, err := procGetForegroundWindow.Call()
	if err != syscall.Errno(0) {
		return syscall.InvalidHandle, err
	}
	return syscall.Handle(r0), nil
}
