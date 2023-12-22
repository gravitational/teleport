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

const (
	// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L493-L496
	webauthnAttachmentAny           = uint32(0)
	webauthnAttachmentPlatform      = uint32(1)
	webauthnAttachmentCrossPlatform = uint32(2)

	// https://github.com/microsoft/webauthn/blob/7ab979cc833bfab9a682ed51761309db57f56c8c/webauthn.h#L498-L501
	webauthnUserVerificationAny         = uint32(0)
	webauthnUserVerificationRequired    = uint32(1)
	webauthnUserVerificationPreferred   = uint32(2)
	webauthnUserVerificationDiscouraged = uint32(3)
)

type webauthnRPEntityInformation struct {
	dwVersion uint32
	// Identifier for the RP. This field is required.
	pwszID *uint16
	// Contains the friendly name of the Relying Party, such as
	// "Acme Corporation", "Widgets Inc" or "Awesome Site".
	// This field is required.
	pwszName *uint16

	// RP icon (previously pwszIcon).
	// This field is kept just to keep size of struct valid.
	_ *uint16
}

type webauthnUserEntityInformation struct {
	dwVersion uint32
	// Identifier for the User. This field is required.
	cbID uint32
	pbID *byte
	// Contains a detailed name for this account, such as
	// "john.p.smith@example.com".
	// It holds the Teleport user name.
	pwszName *uint16
	// User icon (previously pwszIcon).
	// This field is kept just to keep size of struct valid.
	_ *uint16
	// For User: Contains the friendly name associated with the user account by the Relying Party, such as "John P. Smith".
	pwszDisplayName *uint16
}

type webauthnCoseCredentialParameters struct {
	cCredentialParameters uint32
	pCredentialParameters *webauthnCoseCredentialParameter
}

type webauthnCoseCredentialParameter struct {
	dwVersion uint32
	// Well-known credential type specifying a credential to create.
	pwszCredentialType *uint16
	// Well-known COSE algorithm specifying the algorithm to use for the credential.
	lAlg int32
}

type webauthnAuthenticatorMakeCredentialOptions struct {
	dwVersion             uint32
	dwTimeoutMilliseconds uint32
	// For excluding credentials use pExcludeCredentialList.
	// This field is kept just to keep size of struct valid.
	_ webauthnCredentials
	// Optional extensions to parse when performing the operation.
	// Right now not supported by Teleport.
	_ webauthnExtensions
	// Optional. Platform vs Cross-Platform Authenticators.
	dwAuthenticatorAttachment uint32
	// Optional. Require key to be resident or not. Defaulting to FALSE.
	bRequireResidentKey uint32
	// User Verification Requirement.
	dwUserVerificationRequirement uint32
	// Attestation Conveyance Preference.
	dwAttestationConveyancePreference uint32
	// Reserved for future Use
	_ uint32

	//
	// The following fields have been added in WEBAUTHN_AUTHENTICATOR_MAKE_CREDENTIAL_OPTIONS_VERSION_2
	//

	// Cancellation Id - Optional - See WebAuthNGetCancellationId
	// This field is kept just to keep size of struct valid.
	_ *GUID

	//
	// The following fields have been added in WEBAUTHN_AUTHENTICATOR_MAKE_CREDENTIAL_OPTIONS_VERSION_3
	//

	// Exclude Credential List. If present, "CredentialList" will be ignored.
	pExcludeCredentialList *webauthnCredentialList

	//
	// The following fields have been added in WEBAUTHN_AUTHENTICATOR_MAKE_CREDENTIAL_OPTIONS_VERSION_4
	//

	// Enterprise Attestation
	// This field is kept just to keep size of struct valid.
	_ uint32
	// Large Blob Support: none, required or preferred
	// NTE_INVALID_PARAMETER when large blob required or preferred and
	//   bRequireResidentKey isn't set to TRUE
	// This field is kept just to keep size of struct valid.
	_ uint32
	// Optional. Prefer key to be resident. Defaulting to FALSE. When TRUE,
	// overrides the above bRequireResidentKey.
	bPreferResidentKey uint32

	//
	// The following fields have been added in WEBAUTHN_AUTHENTICATOR_MAKE_CREDENTIAL_OPTIONS_VERSION_5
	//

	// Optional. BrowserInPrivate Mode. Defaulting to FALSE.
	// This field is kept just to keep size of struct valid.
	_ uint32
}

type webauthnCredentials struct {
	_ uint32
	_ *webauthnCredential
}

//nolint:unused // TODO: remove when linter runs on windows build tag
type webauthnCredential struct {
	dwVersion uint32
	// Size of pbID.
	cbID uint32
	pbID *byte
	// Well-known credential type specifying what this particular credential is.
	pwszCredentialType *uint16
}

//nolint:unused // This struct is kept just to keep size of struct valid.
type webauthnExtension struct {
	pwszExtensionIdentifier *uint16
	cbExtension             uint32
	pvExtension             *byte
}

//nolint:unused // This struct is kept just to keep size of struct valid.
type webauthnExtensions struct {
	cExtensions uint32
	pExtensions *webauthnExtension
}

type webauthnCredentialEX struct {
	dwVersion uint32
	// Size of pbID.
	cbID uint32
	// Unique ID for this particular credential.
	pbID *byte
	// Well-known credential type specifying what this particular credential is.
	pwszCredentialType *uint16
	// Transports. 0 means no transport restrictions.
	dwTransports uint32
}
type webauthnCredentialList struct {
	cCredentials  uint32
	ppCredentials **webauthnCredentialEX
}

//nolint:unused // TODO: remove when linter runs on windows build tag
type webauthnCredentialAttestation struct {
	dwVersion uint32
	// Attestation format type
	pwszFormatType *uint16
	// Size of cbAuthenticatorData.
	cbAuthenticatorData uint32
	// Authenticator data that was created for this credential.
	pbAuthenticatorData *byte
	// Size of CBOR encoded attestation information
	// 0 => encoded as CBOR null value.
	cbAttestation uint32
	// Encoded CBOR attestation information
	pbAttestation           *byte
	dwAttestationDecodeType uint32
	// Following depends on the dwAttestationDecodeType
	//  WEBAUTHN_ATTESTATION_DECODE_NONE
	//      NULL - not able to decode the CBOR attestation information
	//  WEBAUTHN_ATTESTATION_DECODE_COMMON
	//      PWEBAUTHN_COMMON_ATTESTATION;
	pvAttestationDecode *byte
	// The CBOR encoded Attestation Object to be returned to the RP.
	cbAttestationObject uint32
	pbAttestationObject *byte
	// The CredentialId bytes extracted from the Authenticator Data.
	// Used by Edge to return to the RP.
	cbCredentialID uint32
	pbCredentialID *byte

	//
	// Following fields have been added in WEBAUTHN_CREDENTIAL_ATTESTATION_VERSION_2
	//

	Extensions webauthnExtensions

	//
	// Following fields have been added in WEBAUTHN_CREDENTIAL_ATTESTATION_VERSION_3
	//

	// One of the WEBAUTHN_CTAP_TRANSPORT_* bits will be set corresponding to
	// the transport that was used.
	dwUsedTransport uint32

	//
	// Following fields have been added in WEBAUTHN_CREDENTIAL_ATTESTATION_VERSION_4
	//

	bEpAtt              uint32
	bLargeBlobSupported uint32
	bResidentKey        uint32
}

type webauthnClientData struct {
	dwVersion uint32
	// Size of the pbClientDataJSON field.
	cbClientDataJSON uint32
	// UTF-8 encoded JSON serialization of the client data.
	pbClientDataJSON *byte
	// Hash algorithm ID used to hash the pbClientDataJSON field.
	pwszHashAlgID *uint16
}

type webauthnAuthenticatorGetAssertionOptions struct {
	dwVersion uint32
	// Time that the operation is expected to complete within.
	// This is used as guidance, and can be overridden by the platform.
	dwTimeoutMilliseconds uint32
	// Allowed Credentials List.
	// This field is kept just to keep size of struct valid.
	_ webauthnCredentials
	// Optional extensions to parse when performing the operation.
	// Right now not supported by Teleport.
	_ webauthnExtensions
	// Optional. Platform vs Cross-Platform Authenticators.
	dwAuthenticatorAttachment uint32
	// User Verification Requirement.
	dwUserVerificationRequirement uint32
	// Flags
	// This field is kept just to keep size of struct valid.
	_ uint32

	//
	// The following fields have been added in WEBAUTHN_AUTHENTICATOR_GET_ASSERTION_OPTIONS_VERSION_2
	//

	// Optional identifier for the U2F AppId. Converted to UTF8 before being hashed. Not lower cased.
	//nolint:unused // TODO(tobiaszheller): rm nolint when support for U2FappID is added
	pwszU2fAppID *uint16
	// If the following is non-NULL, then, set to TRUE if the above pwszU2fAppid was used instead of
	// PCWSTR pwszRpId;
	//nolint:unused // TODO(tobiaszheller): rm nolint when support for U2FappID is added
	pbU2fAppID uint32

	//
	// The following fields have been added in WEBAUTHN_AUTHENTICATOR_GET_ASSERTION_OPTIONS_VERSION_3
	//

	// Cancellation Id - Optional - See WebAuthNGetCancellationId
	// This field is kept just to keep size of struct valid.
	_ *GUID

	//
	// The following fields have been added in WEBAUTHN_AUTHENTICATOR_GET_ASSERTION_OPTIONS_VERSION_4
	//

	// Allow Credential List. If present, "CredentialList" will be ignored.
	pAllowCredentialList *webauthnCredentialList

	//
	// The following fields have been added in WEBAUTHN_AUTHENTICATOR_GET_ASSERTION_OPTIONS_VERSION_5
	//
	// This field is kept just to keep size of struct valid.
	_ uint32
	// Size of pbCredLargeBlob
	// This field is kept just to keep size of struct valid.
	_ uint32
	// This field is kept just to keep size of struct valid.
	_ *byte
}

//nolint:unused // TODO: remove when linter runs on windows build tag
type webauthnAssertion struct {
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
	Credential webauthnCredential

	// Size of User ID
	cbUserID uint32
	// UserID
	pbUserID *byte

	//
	// Following fields have been added in WEBAUTHN_ASSERTION_VERSION_2
	//

	Extensions webauthnExtensions

	// Size of pbCredLargeBlob
	cbCredLargeBlob       uint32
	pbCredLargeBlob       *byte
	dwCredLargeBlobStatus uint32
}

//nolint:unused // TODO: remove when linter runs on windows build tag
type webauthnX5C struct {
	// Length of X.509 encoded certificate
	cbData uint32
	// X.509 DER encoded certificate bytes
	pbData *byte
}

type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}
