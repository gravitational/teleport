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

import CryptoKit
import Foundation
import LocalAuthentication
import OSLog
import Security

extension DeviceTrustCredentialClient {
	static let liveValue = value(location: .app)

	static func value(location: DeviceTrustCredentialKeychain.Location) -> Self {
		let store = SecureEnclaveCredentialStore(
			keychain: DeviceTrustCredentialKeychain(location: location),
		)
		return Self(
			loadOrCreate: {
				try store.loadOrCreate()
			},
			load: {
				try store.load()
			},
			signChallenge: { challenge, purpose in
				try await store.signChallenge(challenge, purpose: purpose)
			},
		)
	}
}

/// Implements Device Trust using two pieces of device-local storage.
///
/// The Secure Enclave owns the actual private key. Keychain stores only CryptoKit's encrypted representation of that
/// key together with Teleport's opaque credential ID. The representation cannot be used to recover the private-key
/// bytes and only the Secure Enclave that created it can restore the key.
private final class SecureEnclaveCredentialStore: Sendable {
	private let keychain: DeviceTrustCredentialKeychain
	private let logger = Logger.forType(SecureEnclaveCredentialStore.self)

	init(keychain: DeviceTrustCredentialKeychain) {
		self.keychain = keychain
	}

	/// Loads the configured location's existing credential or creates one when no record exists.
	///
	/// A corrupt record or unexpected Keychain error is never treated as absence. Silently replacing a credential in
	/// those cases would leave Teleport enrolled with a public key that the app can no longer use.
	func loadOrCreate() throws -> DeviceTrustCredential {
		try ensureSecureEnclaveIsAvailable()

		do {
			let credential = try loadCredential()
			logger.debug("Loaded existing Device Trust credential")
			return credential
		} catch DeviceTrustCredentialError.notFound {
			// Absence is the only condition under which enrollment may create a credential.
		}

		let privateKey = try createPrivateKey()
		let storedCredential = StoredDeviceTrustCredential(
			id: UUID().uuidString.lowercased(),
			privateKeyRepresentation: privateKey.dataRepresentation,
		)

		do {
			// This is intentionally an add, not an update: an existing identity must never be overwritten.
			try keychain.insert(storedCredential)
		} catch DeviceTrustCredentialError.keychain(status: errSecDuplicateItem) {
			// Another creator won the race. Discard this new handle and use the credential that became canonical.
			logger.debug("Another caller stored a Device Trust credential; loading it")
			return try loadCredential()
		}

		logger.debug("Created Device Trust credential")
		return makeCredential(from: storedCredential, privateKey: privateKey)
	}

	/// Loads an existing credential for authentication without creating or rotating key material.
	func load() throws -> DeviceTrustCredential {
		try ensureSecureEnclaveIsAvailable()
		return try loadCredential()
	}

	/// Uses the existing private key to sign a raw Teleport challenge after confirming user presence.
	func signChallenge(
		_ challenge: Data,
		purpose: DeviceTrustChallengePurpose,
	) async throws -> Data {
		guard !challenge.isEmpty else {
			throw DeviceTrustCredentialError.emptyChallenge
		}

		try ensureSecureEnclaveIsAvailable()

		// Load the record before prompting so a missing or inaccessible credential remains a Keychain error.
		let storedCredential = try keychain.load()
		let context = LAContext()
		context.localizedReason = purpose.localizedReason
		let privateKey = try restorePrivateKey(
			from: storedCredential,
			authenticationContext: context,
		)

		try await authenticateUser(using: context, for: purpose)
		defer { context.invalidate() }

		do {
			// The Data overload hashes the challenge with SHA-256 once. DER is the format Teleport's Go verifier expects.
			return try privateKey.signature(for: challenge).derRepresentation
		} catch {
			logger.error("Could not sign the Device Trust challenge")
			throw DeviceTrustCredentialError.signingFailed
		}
	}

	// MARK: - Loading and creation

	private func ensureSecureEnclaveIsAvailable() throws {
		guard SecureEnclave.isAvailable else {
			throw DeviceTrustCredentialError.secureEnclaveUnavailable
		}
	}

	private func loadCredential() throws -> DeviceTrustCredential {
		let storedCredential = try keychain.load()
		let privateKey = try restorePrivateKey(from: storedCredential)
		return makeCredential(from: storedCredential, privateKey: privateKey)
	}

	/// Restores CryptoKit's handle to the existing Secure Enclave key.
	///
	/// Restoring a handle is not the same as exporting the private key. If restoration fails, the record remains in
	/// place so callers can require deliberate re-enrollment rather than silently changing device identity.
	private func restorePrivateKey(
		from storedCredential: StoredDeviceTrustCredential,
		authenticationContext: LAContext? = nil,
	) throws -> SecureEnclave.P256.Signing.PrivateKey {
		do {
			return try SecureEnclave.P256.Signing.PrivateKey(
				dataRepresentation: storedCredential.privateKeyRepresentation,
				authenticationContext: authenticationContext,
			)
		} catch {
			logger.error("Could not restore the stored Device Trust credential")
			throw DeviceTrustCredentialError.invalidStoredCredential
		}
	}

	/// Evaluates the key's user-presence policy through LocalAuthentication before asking CryptoKit to sign.
	///
	/// `LAContext` reports cancellation as a direct `LAError`, so callers can ignore it without relying on the
	/// undocumented error wrappers produced by CryptoKit. The same authenticated context is reused for signing.
	private func authenticateUser(
		using context: LAContext,
		for purpose: DeviceTrustChallengePurpose,
	) async throws {
		let accessControl = try makeAccessControl()

		let authenticated: Bool
		do {
			authenticated = try await context.evaluateAccessControl(
				accessControl,
				operation: .useKeySign,
				localizedReason: purpose.localizedReason,
			)
		} catch let error as LAError {
			switch error.code {
				case .appCancel, .systemCancel, .userCancel:
					throw DeviceTrustCredentialError.authenticationCancelled

				default:
					throw DeviceTrustCredentialError.authenticationFailed
			}
		} catch is CancellationError {
			throw DeviceTrustCredentialError.authenticationCancelled
		} catch {
			throw DeviceTrustCredentialError.authenticationFailed
		}

		guard authenticated else {
			throw DeviceTrustCredentialError.authenticationFailed
		}

		// Only the explicit LocalAuthentication call may present UI. CryptoKit must reuse this authorization or fail.
		context.interactionNotAllowed = true
	}

	/// Creates a P-256 signing key whose private operations require local user presence.
	private func createPrivateKey() throws -> SecureEnclave.P256.Signing.PrivateKey {
		let accessControl = try makeAccessControl()

		do {
			return try SecureEnclave.P256.Signing.PrivateKey(accessControl: accessControl)
		} catch {
			logger.error("Could not create a Secure Enclave Device Trust key")
			throw DeviceTrustCredentialError.keyCreationFailed
		}
	}

	/// Builds the policy enforced by Security.framework whenever the private key signs.
	///
	/// - `WhenPasscodeSetThisDeviceOnly` prevents migration and invalidates the credential if the passcode is removed.
	/// - `privateKeyUsage` applies the policy to private-key operations rather than key creation or public-key reads.
	/// - `userPresence` accepts Face ID, Touch ID, or the device passcode.
	private func makeAccessControl() throws -> SecAccessControl {
		var error: Unmanaged<CFError>? = nil
		guard
			let accessControl = SecAccessControlCreateWithFlags(
				kCFAllocatorDefault,
				kSecAttrAccessibleWhenPasscodeSetThisDeviceOnly,
				[.privateKeyUsage, .userPresence],
				&error,
			)
		else {
			if let error {
				logger.error("Could not create the Device Trust access-control policy: \(error.takeRetainedValue())")
			}

			throw DeviceTrustCredentialError.accessControlCreationFailed
		}

		return accessControl
	}

	private func makeCredential(
		from storedCredential: StoredDeviceTrustCredential,
		privateKey: SecureEnclave.P256.Signing.PrivateKey,
	) -> DeviceTrustCredential {
		DeviceTrustCredential(
			id: storedCredential.id,
			publicKeyDER: privateKey.publicKey.derRepresentation,
		)
	}
}

extension DeviceTrustChallengePurpose {
	fileprivate var localizedReason: String {
		switch self {
			case .enrollment:
				"Confirm your presence to enroll this device with Teleport."

			case .authentication:
				"Confirm your presence to verify this device with Teleport."
		}
	}
}
