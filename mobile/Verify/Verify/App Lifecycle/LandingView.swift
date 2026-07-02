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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

import SwiftUI
import SwiftUINavigation

struct LandingView: View {
	@Bindable
	var viewModel: LandingViewModel

	var body: some View {
		NavigationStack {
			VStack(spacing: .zero) {
				Image("logo")
					.resizable()
					.scaledToFit()
					.frame(maxWidth: .infinity, maxHeight: 44, alignment: .leading)
				ScrollView {
					VStack(spacing: .large) {
						titleBlock
						instructionSteps
						scanQRCodeButton
					}
				}
				.scrollBounceBehavior(.basedOnSize)
			}
			.padding(.horizontal)

			// MARK: Navigation

			.navigationDestination(
				item: $viewModel.destination.deviceEnrollment,
				destination: { deviceEnrollmentViewModel in
					EnrollDeviceView(viewModel: deviceEnrollmentViewModel)
				},
			)
			.alert(
				item: $viewModel.destination.deepLinkParsingAlert,
				title: { errorMessage in
					Text(errorMessage)
				},
				actions: { _ in
					Button("OK") {}
				},
			)
		}
	}
}

// MARK: - Subviews

extension LandingView {
	private var titleBlock: some View {
		VStack(spacing: .small) {
			Text("Scan QR code")
				.font(.title)
				.padding(.top, .xlarge)
			Text("Enroll this device for your cluster by scanning the QR code in your web browser")
				.font(.callout)
				.multilineTextAlignment(.center)
				.foregroundStyle(.secondary)
		}
	}

	private var instructionSteps: some View {
		VStack(spacing: .medium) {
			instructionStep(stepNumber: 1) {
				Text(
					"""
					In the Teleport Web UI, go to Profile Dropdown \
					\(rightArrow) Account Settings \
					\(rightArrow) Enroll Trusted Device.
					""",
				)
			}
			instructionStep(stepNumber: 2) {
				Text("Click on \"Enroll Device\" to display the QR code for device enrollment.")
			}
			instructionStep(stepNumber: 3) {
				Text("Tap the \"Scan QR Code\" button.")
			}
		}
	}

	private var scanQRCodeButton: some View {
		Button {
			print("scanning has not been built yet")
		} label: {
			Text("Scan QR Code")
				.padding(.vertical, .xsmall)
				.frame(maxWidth: .infinity)
		}
		.buttonStyle(.borderedProminent)
	}

	private func instructionStep(stepNumber: UInt, @ViewBuilder label: () -> some View) -> some View {
		Label {
			label()
		} icon: {
			Image(systemName: "\(stepNumber).circle.fill")
				.foregroundStyle(.tint.opacity(0.8))
		}
		.frame(maxWidth: .infinity, alignment: .leading)
		.padding()
		.background(RoundedRectangle(cornerRadius: .small).fill(.background).strokeBorder(.separator))
	}

	private var rightArrow: Image {
		Image(systemName: "arrow.right")
	}
}

#Preview("In ContentView") {
	@Previewable @State
	var viewModel = LandingViewModel()
	LandingView(viewModel: viewModel)
}
