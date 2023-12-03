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
	"encoding/json"
	"strings"
	"syscall"
	"unicode/utf16"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/gravitational/trace"

	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

func assertOptionsToCType(in wantypes.PublicKeyCredentialRequestOptions, loginOpts *LoginOpts) (*webauthnAuthenticatorGetAssertionOptions, error) {
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

func rpToCType(in wantypes.RelyingPartyEntity) (*webauthnRPEntityInformation, error) {
	if in.ID == "" {
		return nil, trace.BadParameter("missing RelyingPartyEntity.Id")
	}
	if in.Name == "" {
		return nil, trace.BadParameter("missing RelyingPartyEntity.Name")
	}
	id, err := utf16PtrFromString(in.ID)
	if err != nil {
		return nil, err
	}
	name, err := utf16PtrFromString(in.Name)
	if err != nil {
		return nil, err
	}
	return &webauthnRPEntityInformation{
		dwVersion: 1,
		pwszID:    id,
		pwszName:  name,
	}, nil
}

func userToCType(in wantypes.UserEntity) (*webauthnUserEntityInformation, error) {
	if len(in.ID) == 0 {
		return nil, trace.BadParameter("missing UserEntity.Id")
	}
	if in.Name == "" {
		return nil, trace.BadParameter("missing UserEntity.Name")
	}

	name, err := utf16PtrFromString(in.Name)
	if err != nil {
		return nil, err
	}
	var displayName *uint16
	if in.DisplayName != "" {
		displayName, err = utf16PtrFromString(in.DisplayName)
		if err != nil {
			return nil, err
		}
	}
	return &webauthnUserEntityInformation{
		dwVersion:       1,
		cbID:            uint32(len(in.ID)),
		pbID:            &in.ID[0],
		pwszName:        name,
		pwszDisplayName: displayName,
	}, nil
}

func credParamToCType(in []wantypes.CredentialParameter) (*webauthnCoseCredentialParameters, error) {
	if len(in) == 0 {
		return nil, trace.BadParameter("missing CredentialParameter")
	}
	out := make([]webauthnCoseCredentialParameter, 0, len(in))
	for _, c := range in {
		pwszCredentialType, err := utf16PtrFromString(string(c.Type))
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
		return nil, nil, trace.BadParameter("missing ClientData.Challenge")
	}
	if origin == "" {
		return nil, nil, trace.BadParameter("missing ClientData.Origin")
	}
	algID, err := utf16PtrFromString("SHA-256")
	if err != nil {
		return nil, nil, err
	}
	type clientDataJSON struct {
		Type      string `json:"type"`
		Challenge string `json:"challenge"`
		Origin    string `json:"origin"`
	}
	cd := clientDataJSON{
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
		pwszHashAlgID:    algID,
	}, jsonCD, nil
}

func credentialsExToCType(in []wantypes.CredentialDescriptor) (*webauthnCredentialList, error) {
	exCredList := make([]*webauthnCredentialEX, 0, len(in))
	for _, e := range in {
		if e.Type == "" {
			return nil, trace.BadParameter("missing CredentialDescriptor.Type")
		}
		if len(e.CredentialID) == 0 {
			return nil, trace.BadParameter("missing CredentialDescriptor.CredentialID")
		}
		pwszCredentialType, err := utf16PtrFromString(string(e.Type))
		if err != nil {
			return nil, err
		}
		exCredList = append(exCredList, &webauthnCredentialEX{
			dwVersion:          1,
			cbID:               uint32(len(e.CredentialID)),
			pbID:               &e.CredentialID[0],
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
			out |= 0x1
		case protocol.NFC:
			out |= 0x2
		case protocol.BLE:
			out |= 0x4
		case protocol.Internal:
			out |= 0x10
		}
	}
	return out
}

func attachmentToCType(in protocol.AuthenticatorAttachment) uint32 {
	// Mapped based on:
	// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L493-L496
	switch in {
	case protocol.Platform:
		return webauthnAttachmentPlatform
	case protocol.CrossPlatform:
		return webauthnAttachmentCrossPlatform
	default:
		return webauthnAttachmentAny
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
		return webauthnUserVerificationRequired
	case protocol.VerificationPreferred:
		return webauthnUserVerificationPreferred
	case protocol.VerificationDiscouraged:
		return webauthnUserVerificationDiscouraged
	default:
		return webauthnUserVerificationAny
	}
}

func requirePreferResidentKey(in wantypes.AuthenticatorSelection) (requireRK bool, preferRK bool) {
	switch in.ResidentKey {
	case protocol.ResidentKeyRequirementRequired:
		return true, false
	case protocol.ResidentKeyRequirementPreferred:
		return false, true
	case protocol.ResidentKeyRequirementDiscouraged:
		return false, false
	default:
		if in.RequireResidentKey != nil && *in.RequireResidentKey {
			return true, false
		}
		return false, false
	}
}

func makeCredOptionsToCType(in wantypes.PublicKeyCredentialCreationOptions) (*webauthnAuthenticatorMakeCredentialOptions, error) {
	exCredList, err := credentialsExToCType(in.CredentialExcludeList)
	if err != nil {
		return nil, err
	}

	requiredRK, preferRK := requirePreferResidentKey(in.AuthenticatorSelection)
	return &webauthnAuthenticatorMakeCredentialOptions{
		// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L36-L97
		// contains information about different versions.
		// We can set newest version and it still works on older APIs.
		dwVersion:                         5,
		dwTimeoutMilliseconds:             uint32(in.Timeout),
		dwAuthenticatorAttachment:         attachmentToCType(in.AuthenticatorSelection.AuthenticatorAttachment),
		dwAttestationConveyancePreference: conveyancePreferenceToCType(in.Attestation),
		bRequireResidentKey:               boolToUint32(requiredRK),
		dwUserVerificationRequirement:     userVerificationToCType(in.AuthenticatorSelection.UserVerification),
		pExcludeCredentialList:            exCredList,
		bPreferResidentKey:                boolToUint32(preferRK),
	}, nil
}

func boolToUint32(in bool) uint32 {
	if in {
		return 1
	}
	return 0
}

// utf16PtrFromString is copied from golang.org/x/sys/windows because we want
// to test conversions on linux machines also.
func utf16PtrFromString(s string) (*uint16, error) {
	utf16FromString := func(s string) ([]uint16, error) {
		if strings.IndexByte(s, 0) != -1 {
			return nil, syscall.EINVAL
		}
		return utf16.Encode([]rune(s + "\x00")), nil
	}
	a, err := utf16FromString(s)
	if err != nil {
		return nil, err
	}
	return &a[0], nil
}
