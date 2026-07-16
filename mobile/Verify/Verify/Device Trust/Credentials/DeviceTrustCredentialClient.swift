// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see http://www.gnu.org/licenses/

import Dependencies
import DependenciesMacros
import Foundation

/// Provides the Device Trust credential operations needed by enrollment and authentication ceremonies.
///
/// Feature tests should override this dependency directly. They do not need to imitate Keychain or Secure Enclave.
@DependencyClient
struct DeviceTrustCredentialClient {
	/// Enrollment operation: loads the existing credential or creates one only when it is genuinely absent.
	var loadOrCreate: @Sendable () throws -> DeviceTrustCredential

	/// Authentication operation: loads the existing credential without ever creating a replacement.
	var load: @Sendable () throws -> DeviceTrustCredential

	/// Signs the raw Teleport challenge after a user-presence check and returns an ASN.1 DER ECDSA signature.
	///
	/// Callers should treat ``DeviceTrustCredentialError/authenticationCancelled`` as a no-op. All other errors
	/// describe an operation that failed and should follow the feature's normal error handling.
	var signChallenge: @Sendable (
		_ challenge: Data,
		_ purpose: DeviceTrustChallengePurpose,
	) async throws -> Data
}

/// The public portion of a Device Trust credential.
///
/// The private key is deliberately absent. It remains in the Secure Enclave and can only be used through
/// ``DeviceTrustCredentialClient/signChallenge``.
struct DeviceTrustCredential: Equatable {
	/// An opaque identifier generated when the credential is first created.
	let id: String

	/// The P-256 public key encoded as PKIX ASN.1 DER for Teleport's enrollment protocol.
	let publicKeyDER: Data
}

/// Identifies why Teleport needs a signature so the system authentication prompt can explain the request.
enum DeviceTrustChallengePurpose {
	case enrollment
	case authentication
}

/// Stable failures that callers can turn into user-facing recovery actions.
///
/// Apple framework errors are mapped here so `CFError`, `NSError`, and other non-Sendable values do not escape the
/// dependency boundary.
enum DeviceTrustCredentialError: Error, Equatable {
	/// The current device cannot create or use Secure Enclave keys.
	case secureEnclaveUnavailable

	/// Authentication asked for a credential, but this app has not created one.
	case notFound

	/// Keychain contained a record that could not be decoded or restored by this Secure Enclave.
	case invalidStoredCredential

	/// Teleport supplied no bytes to sign.
	case emptyChallenge

	/// The Secure Enclave could not create a new signing key.
	case keyCreationFailed

	/// Security.framework could not construct the access-control policy used by the key.
	case accessControlCreationFailed

	/// The user or system cancelled the presence prompt.
	case authenticationCancelled

	/// The user-presence check failed without being cancelled.
	case authenticationFailed

	/// CryptoKit could not sign the challenge with the stored Secure Enclave key.
	case signingFailed

	/// Security.framework returned an error while accessing Keychain.
	case keychain(status: Int32)
}
