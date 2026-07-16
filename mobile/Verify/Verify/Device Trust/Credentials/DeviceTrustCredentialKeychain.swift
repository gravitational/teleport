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

import Foundation
import OSLog
import Security

/// The versioned value stored as one Keychain item.
///
/// Keeping the credential ID and encrypted key representation in a single record ensures they are created and loaded
/// together. `privateKeyRepresentation` is an opaque, encrypted Secure Enclave handle—not raw private-key bytes.
struct StoredDeviceTrustCredential: Codable {
	fileprivate static let currentVersion = 1

	fileprivate let version: Int
	let id: String
	let privateKeyRepresentation: Data

	init(id: String, privateKeyRepresentation: Data) {
		self.version = Self.currentVersion
		self.id = id
		self.privateKeyRepresentation = privateKeyRepresentation
	}
}

/// Persists the Device Trust credential record in the Verify app's default Keychain access group.
///
/// This type knows nothing about signing or key creation. Its entire responsibility is to atomically load or insert
/// the opaque record used by ``SecureEnclaveCredentialStore``.
struct DeviceTrustCredentialKeychain {
	private static let service = "com.gravitational.teleport.verify.device-trust"
	private static let account = "credential.v1"

	private let logger = Logger.forType(DeviceTrustCredentialKeychain.self)

	/// Loads and validates the existing versioned record.
	func load() throws -> StoredDeviceTrustCredential {
		var query = Self.baseQuery
		query[kSecReturnData] = true
		query[kSecMatchLimit] = kSecMatchLimitOne

		var item: CFTypeRef? = nil
		let status = SecItemCopyMatching(query as CFDictionary, &item)
		switch status {
			case errSecSuccess:
				guard let data = item as? Data else {
					throw DeviceTrustCredentialError.invalidStoredCredential
				}

				return try decode(data)

			case errSecItemNotFound:
				throw DeviceTrustCredentialError.notFound

			default:
				logger.error("Could not load Device Trust credential from Keychain: \(status)")
				throw DeviceTrustCredentialError.keychain(status: status)
		}
	}

	/// Adds a new record without replacing an existing credential.
	///
	/// `errSecDuplicateItem` is intentionally returned to the caller, which reloads the record that won a concurrent
	/// creation race.
	func insert(_ storedCredential: StoredDeviceTrustCredential) throws {
		let data: Data
		do {
			let encoder = PropertyListEncoder()
			encoder.outputFormat = .binary
			data = try encoder.encode(storedCredential)
		} catch {
			throw DeviceTrustCredentialError.keyCreationFailed
		}

		var attributes = Self.baseQuery
		// Match the Secure Enclave key's lifetime: local to this device and invalid once its passcode is removed.
		attributes[kSecAttrAccessible] = kSecAttrAccessibleWhenPasscodeSetThisDeviceOnly
		attributes[kSecAttrSynchronizable] = false
		attributes[kSecValueData] = data

		let status = SecItemAdd(attributes as CFDictionary, nil)
		guard status == errSecSuccess else {
			logger.error("Could not store Device Trust credential in Keychain: \(status)")
			throw DeviceTrustCredentialError.keychain(status: status)
		}
	}

	private func decode(_ data: Data) throws -> StoredDeviceTrustCredential {
		do {
			let storedCredential = try PropertyListDecoder().decode(StoredDeviceTrustCredential.self, from: data)
			guard
				storedCredential.version == StoredDeviceTrustCredential.currentVersion,
				UUID(uuidString: storedCredential.id) != nil,
				!storedCredential.privateKeyRepresentation.isEmpty
			else {
				throw DeviceTrustCredentialError.invalidStoredCredential
			}

			return storedCredential
		} catch let error as DeviceTrustCredentialError {
			throw error
		} catch {
			logger.error("Could not decode the stored Device Trust credential")
			throw DeviceTrustCredentialError.invalidStoredCredential
		}
	}

	/// The stable service/account pair identifies exactly one app-wide Device Trust credential.
	///
	/// No access group is specified, so Security.framework uses the Verify app's default signed access group.
	private static var baseQuery: [CFString: Any] {
		[
			kSecClass: kSecClassGenericPassword,
			kSecAttrService: service,
			kSecAttrAccount: account,
		]
	}
}
