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

#if DEBUG

	import CryptoKit
	import Foundation
	import Observation
	import Security

	extension FeatureDemo {
		@Observable @MainActor
		final class DeviceTrustCredentialViewModel {
			private(set) var status: DeviceTrustCredentialDemoStatus = .idle
			private(set) var credentialID: String? = nil
			private(set) var publicKeyFingerprint: String? = nil
			private(set) var publicKeyDERBase64: String? = nil
			private(set) var credentialReloadMatched: Bool? = nil
			private(set) var challengeHex: String? = nil
			private(set) var signatureDERBase64: String? = nil
			private(set) var signatureVerified: Bool? = nil

			@ObservationIgnored
			private let credentialClient: DeviceTrustCredentialClient

			init() {
				self.credentialClient = DeviceTrustCredentialClient.value(location: .demo)
			}

			var secureEnclaveAvailable: Bool {
				SecureEnclave.isAvailable
			}

			var isRunning: Bool {
				if case .loading = status {
					true
				} else {
					false
				}
			}
		}
	}

	// MARK: - User Actions

	extension FeatureDemo.DeviceTrustCredentialViewModel {
		/// Exercises persistence, Secure Enclave signing, and verification using the public key loaded from storage.
		func runRoundTrip() async {
			guard !isRunning else {
				return
			}

			resetResults()

			do {
				status = .loading("Creating or loading the demo credential…")
				let initialCredential = try credentialClient.loadOrCreate()
				show(initialCredential)

				status = .loading("Loading the credential back from Keychain…")
				let reloadedCredential = try credentialClient.load()
				credentialReloadMatched = initialCredential == reloadedCredential

				guard credentialReloadMatched == true else {
					throw DeviceTrustCredentialDemoError.credentialChangedDuringReload
				}

				let challenge = Self.makeRandomChallenge()
				challengeHex = Self.hex(challenge)

				status = .loading("Requesting user presence to authorize signing…")
				let signature = try await credentialClient.signChallenge(challenge, .authentication)
				signatureDERBase64 = signature.base64EncodedString()

				status = .loading("Verifying the signature with the reloaded public key…")
				let verified = try Self.verify(
					signature: signature,
					for: challenge,
					using: reloadedCredential.publicKeyDER,
				)
				signatureVerified = verified

				guard verified else {
					throw DeviceTrustCredentialDemoError.signatureRejected
				}

				status = .success(
					"The Secure Enclave key signed the challenge, and the reloaded public key verified the signature.",
				)
			} catch DeviceTrustCredentialError.signingAuthorizationCancelled {
				// Cancellation is expected control flow. Nothing is sent and no failure is shown.
				status = .cancelled
			} catch {
				status = .failure(Self.message(for: error))
			}
		}

		/// Loads the existing demo credential without creating one or showing a user-presence prompt.
		func loadExistingCredential() {
			guard !isRunning else {
				return
			}

			resetResults()
			status = .loading("Loading the existing demo credential from Keychain…")

			do {
				let credential = try credentialClient.load()
				show(credential)
				status = .success("Loaded the demo credential without requesting signing authorization.")
			} catch {
				status = .failure(Self.message(for: error))
			}
		}

		/// Deletes only the debug demo's Keychain record, bypassing the production credential client.
		func resetDemoCredential() {
			guard !isRunning else {
				return
			}

			resetResults()
			status = .loading("Deleting the demo credential from Keychain…")

			do {
				// Keep the storage location explicit here so this debug escape hatch cannot affect the app credential.
				try DeviceTrustCredentialKeychain(location: .demo).delete()
				status = .success("Deleted the demo credential. The app's Device Trust credential was not touched.")
			} catch {
				status = .failure(Self.message(for: error))
			}
		}
	}

	// MARK: - Presented Results

	extension FeatureDemo.DeviceTrustCredentialViewModel {
		private func resetResults() {
			credentialID = nil
			publicKeyFingerprint = nil
			publicKeyDERBase64 = nil
			credentialReloadMatched = nil
			challengeHex = nil
			signatureDERBase64 = nil
			signatureVerified = nil
		}

		private func show(_ credential: DeviceTrustCredential) {
			credentialID = credential.id
			publicKeyFingerprint = Self.hex(
				Data(SHA256.hash(data: credential.publicKeyDER)),
				separator: ":",
			)
			publicKeyDERBase64 = credential.publicKeyDER.base64EncodedString()
		}
	}

	// MARK: - Challenge and Signature

	extension FeatureDemo.DeviceTrustCredentialViewModel {
		private static func makeRandomChallenge() -> Data {
			var generator = SystemRandomNumberGenerator()
			return Data(
				(0 ..< 32).map { _ in
					UInt8.random(in: UInt8.min ... UInt8.max, using: &generator)
				},
			)
		}

		private static func verify(
			signature: Data,
			for challenge: Data,
			using publicKeyDER: Data,
		) throws -> Bool {
			let publicKey = try P256.Signing.PublicKey(derRepresentation: publicKeyDER)
			let signature = try P256.Signing.ECDSASignature(derRepresentation: signature)
			return publicKey.isValidSignature(signature, for: challenge)
		}

		private static func hex(_ data: Data, separator: String = "") -> String {
			data.map { String(format: "%02X", $0) }.joined(separator: separator)
		}
	}

	// MARK: - Error Presentation

	extension FeatureDemo.DeviceTrustCredentialViewModel {
		private static func message(for error: any Error) -> String {
			guard let error = error as? DeviceTrustCredentialError else {
				return error.localizedDescription
			}

			switch error {
				case .secureEnclaveUnavailable:
					return "The Secure Enclave is unavailable. Run this demo on a supported physical device."

				case .notFound:
					return "No demo credential exists yet. Create, sign, and verify to create one."

				case .invalidStoredCredential:
					return "Keychain contains a credential that this Secure Enclave cannot restore."

				case .emptyChallenge:
					return "The signing API received an empty challenge."

				case .keyCreationFailed:
					return "CryptoKit could not create the Secure Enclave signing key."

				case .accessControlCreationFailed:
					return "Security.framework could not create the key's access-control policy."

				case .signingAuthorizationCancelled, .signingAuthorizationFailed, .signingFailed:
					return operationMessage(for: error)

				case let .keychain(status):
					let detail = SecCopyErrorMessageString(status, nil) as String? ?? "Unknown Keychain error"
					return "Keychain returned OSStatus \(status): \(detail)"
			}
		}

		private static func operationMessage(for error: DeviceTrustCredentialError) -> String {
			switch error {
				case .signingAuthorizationCancelled:
					"Signing authorization was cancelled."

				case .signingAuthorizationFailed:
					"LocalAuthentication could not confirm user presence."

				case .signingFailed:
					"CryptoKit could not sign with the stored Secure Enclave key."

				default:
					preconditionFailure("Only signing authorization and signing errors belong here")
			}
		}
	}

	// MARK: - Supporting Types

	extension FeatureDemo.DeviceTrustCredentialViewModel {
		enum DeviceTrustCredentialDemoStatus {
			case idle
			case loading(String)
			case success(String)
			case cancelled
			case failure(String)
		}

		private enum DeviceTrustCredentialDemoError: LocalizedError {
			case credentialChangedDuringReload
			case signatureRejected

			var errorDescription: String? {
				switch self {
					case .credentialChangedDuringReload:
						"The credential loaded from Keychain did not match the credential returned by loadOrCreate."

					case .signatureRejected:
						"CryptoKit rejected the signature produced by the Secure Enclave."
				}
			}
		}
	}

#endif
