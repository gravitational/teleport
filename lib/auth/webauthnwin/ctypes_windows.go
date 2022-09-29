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

import "golang.org/x/sys/windows"

type webauthnRPEntityInformation struct {
	dwVersion uint32
	// Identifier for the RP. This field is required.
	pwszId *uint16
	// Contains the friendly name of the Relying Party, such as
	// "Acme Corporation", "Widgets Inc" or "Awesome Site".
	// This field is required.
	pwszName *uint16
	// Optional URL pointing to RP's logo.
	pwszIcon *uint16
}

type webauthnUserEntityInformation struct {
	dwVersion uint32
	// Identifier for the User. This field is required.
	cbId uint32
	pbId *byte
	// Contains a detailed name for this account, such as
	// "john.p.smith@example.com".
	// It holds the Teleport user name.
	pwszName *uint16
	// Optional URL that can be used to retrieve an image containing the user's current avatar,
	// or a data URI that contains the image data.
	pwszIcon *uint16
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
	// Credentials used for exclusion.
	CredentialList webauthnCredentials
	// Optional extensions to parse when performing the operation.
	Extensions webauthnExtenstions
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
	pExcludeCredentialList *webauthnCredentialList

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

type webauthnCredentials struct {
	cCredentials uint32
	pCredentials *webauthnCredential
}

type webauthnCredential struct {
	dwVersion uint32
	// Size of pbID.
	cbId uint32
	pbId *byte
	// Well-known credential type specifying what this particular credential is.
	pwszCredentialType *uint16
}

type webauthnExtension struct {
	pwszExtensionIdentifier *uint16
	cbExtension             uint32
	pvExtension             *byte
}
type webauthnExtenstions struct {
	cExtensions uint32
	pExtensions *webauthnExtension
}

type webauthnCredentialEX struct {
	dwVersion uint32
	// Size of pbID.
	cbId uint32
	// Unique ID for this particular credential.
	pbId *byte
	// Well-known credential type specifying what this particular credential is.
	pwszCredentialType *uint16
	// Transports. 0 means no transport restrictions.
	dwTransports uint32
}
type webauthnCredentialList struct {
	cCredentials  uint32
	ppCredentials **webauthnCredentialEX
}

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
	cbCredentialId uint32
	pbCredentialId *byte

	//
	// Following fields have been added in WEBAUTHN_CREDENTIAL_ATTESTATION_VERSION_2
	//

	Extensions webauthnExtenstions

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
	pwszHashAlgId *uint16
}

type webauthnAuthenticatorGetAssertionOptions struct {
	dwVersion uint32
	// Time that the operation is expected to complete within.
	// This is used as guidance, and can be overridden by the platform.
	dwTimeoutMilliseconds uint32
	// Allowed Credentials List.
	CredentialList webauthnCredentials
	// Optional extensions to parse when performing the operation.
	Extensions webauthnExtenstions
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
	pAllowCredentialList *webauthnCredentialList

	//
	// The following fields have been added in WEBAUTHN_AUTHENTICATOR_GET_ASSERTION_OPTIONS_VERSION_5
	//

	dwCredLargeBlobOperation uint32
	// Size of pbCredLargeBlob
	cbCredLargeBlob uint32
	pbCredLargeBlob *byte
}

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

	// Size of User Id
	cbUserId uint32
	// UserId
	pbUserId *byte

	//
	// Following fields have been added in WEBAUTHN_ASSERTION_VERSION_2
	//

	Extensions webauthnExtenstions

	// Size of pbCredLargeBlob
	cbCredLargeBlob       uint32
	pbCredLargeBlob       *byte
	dwCredLargeBlobStatus uint32
}

type webauthnX5C struct {
	// Length of X.509 encoded certificate
	cbData uint32
	// X.509 DER encoded certificate bytes
	pbData *byte
}

type webauthnCommonAttestation struct {
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
	pX5c *webauthnX5C

	// Following are also set for TPM
	pwszVer    *uint16 // "2.0"
	cbCertInfo uint32
	pbCertInfo *byte
	cbPubArea  uint32
	pbPubArea  *byte
}
