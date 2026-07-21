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

	import SwiftUI

	extension FeatureDemo {
		struct DeviceTrustCredentialView: View {
			@Bindable
			var viewModel: DeviceTrustCredentialViewModel

			var body: some View {
				Form {
					descriptionSection
					deviceSection
					actionsSection
					statusSection
					storedCredentialSection
					challengeAndSignatureSection
				}
				.navigationTitle("Device Trust Credential")
				.navigationBarTitleDisplayMode(.inline)
			}
		}
	}

	// MARK: - Sections

	extension FeatureDemo.DeviceTrustCredentialView {
		var descriptionSection: some View {
			Section {
				Text(
					"""
					Creates or loads a separate demo credential, reloads it from Keychain, requests user presence \
					to authorize signing, signs a random challenge, and verifies the signature locally.
					""",
				)
				.font(.callout)
				.foregroundStyle(.secondary)
			}
		}

		var deviceSection: some View {
			Section("Device") {
				HStack {
					Text("Secure Enclave")
					Label(
						viewModel.secureEnclaveAvailable
							? "Available"
							: "Unavailable",
						systemImage: viewModel.secureEnclaveAvailable
							? "checkmark.circle.fill"
							: "xmark.circle.fill",
					)
					.labelIconToTitleSpacing(.small)
					.foregroundStyle(viewModel.secureEnclaveAvailable ? Color.success : Color.danger)
					.frame(maxWidth: .infinity, alignment: .trailing)
				}
			}
		}

		var actionsSection: some View {
			Section {
				runRoundTripButton
				loadCredentialButton
				resetCredentialButton
			} header: {
				Text("Actions")
			} footer: {
				Text(
					"This demo uses a separate Keychain item and never reads or modifies the app's Device Trust credential.",
				)
			}
		}

		var statusSection: some View {
			Section("Status") {
				statusView
			}
		}

		@ViewBuilder
		var storedCredentialSection: some View {
			if let credentialID = viewModel.credentialID {
				Section("Stored Demo Credential") {
					detail("Credential ID", value: credentialID)

					if let fingerprint = viewModel.publicKeyFingerprint {
						detail("Public Key Fingerprint (SHA-256)", value: fingerprint)
					}

					if let publicKey = viewModel.publicKeyDERBase64 {
						detail("Public Key DER (Base64)", value: publicKey)
					}

					if let matched = viewModel.credentialReloadMatched {
						result("Credential persisted correctly", succeeded: matched)
					}
				}
			}
		}

		@ViewBuilder
		var challengeAndSignatureSection: some View {
			if let challenge = viewModel.challengeHex {
				Section("Challenge and Signature") {
					detail("Random 32-byte challenge", value: challenge)

					if let signature = viewModel.signatureDERBase64 {
						detail("ECDSA Signature (DER, Base64-encoded)", value: signature)
					}

					if let verified = viewModel.signatureVerified {
						result("Signature verified with reloaded public key", succeeded: verified)
					}
				}
			}
		}
	}

	// MARK: - Actions

	extension FeatureDemo.DeviceTrustCredentialView {
		var runRoundTripButton: some View {
			Button {
				Task { await viewModel.runRoundTrip() }
			} label: {
				Label("Create, Sign, and Verify", systemImage: "checkmark.shield")
			}
			.disabled(viewModel.isRunning || !viewModel.secureEnclaveAvailable)
		}

		var loadCredentialButton: some View {
			Button {
				viewModel.loadExistingCredential()
			} label: {
				Label("Load Demo Credential", systemImage: "key.viewfinder")
			}
			.disabled(viewModel.isRunning || !viewModel.secureEnclaveAvailable)
		}

		var resetCredentialButton: some View {
			Button {
				viewModel.resetDemoCredential()
			} label: {
				Label {
					Text("Delete Demo Credential")
				} icon: {
					Image(systemName: "trash")
						.foregroundStyle(.danger)
				}
				.tint(.danger)
			}
			.disabled(viewModel.isRunning)
		}
	}

	// MARK: - Status

	extension FeatureDemo.DeviceTrustCredentialView {
		@ViewBuilder
		var statusView: some View {
			switch viewModel.status {
				case .idle:
					status(
						"Ready",
						message: "Create, sign, and verify on a physical device.",
						systemImage: "circle.dotted",
						color: .secondary,
					)

				case let .loading(message):
					HStack(spacing: 12) {
						ProgressView()
						Text(message)
					}

				case let .success(message):
					status(
						"Success",
						message: message,
						systemImage: "checkmark.circle.fill",
						color: .success,
					)

				case .cancelled:
					status(
						"Cancelled",
						message: "No signature was produced.",
						systemImage: "xmark.circle",
						color: .secondary,
					)

				case let .failure(message):
					status(
						"Failed",
						message: message,
						systemImage: "exclamationmark.triangle.fill",
						color: .danger,
					)
			}
		}

		private func status(
			_ title: String,
			message: String,
			systemImage: String,
			color: Color,
		) -> some View {
			HStack(alignment: .top, spacing: 12) {
				Image(systemName: systemImage)
					.foregroundStyle(color)

				VStack(alignment: .leading, spacing: 4) {
					Text(title)
						.fontWeight(.semibold)
					Text(message)
						.font(.callout)
						.foregroundStyle(.secondary)
				}
			}
		}
	}

	// MARK: - Result Rows

	extension FeatureDemo.DeviceTrustCredentialView {
		private func detail(_ title: String, value: String) -> some View {
			VStack(alignment: .leading, spacing: 4) {
				Text(title)
					.font(.caption)
					.foregroundStyle(.secondary)
				Text(value)
					.font(.system(.caption, design: .monospaced))
					.textSelection(.enabled)
			}
		}

		private func result(_ title: String, succeeded: Bool) -> some View {
			Label(
				title,
				systemImage: succeeded ? "checkmark.circle.fill" : "xmark.circle.fill",
			)
			.foregroundStyle(succeeded ? Color.success : Color.danger)
		}
	}

#endif
