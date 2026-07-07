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
						Icon(systemName: "viewfinder")
							.padding(.top, .xlarge)
						titleBlock
						instructionSteps
						scanQRCodeButton
					}
				}
				.scrollBounceBehavior(.basedOnSize)
			}
			.padding(.horizontal)
			.background(Color.Background.depth3)

			// MARK: Navigation

			.navigationDestination(item: $viewModel.destination.enrollDevice) { deviceEnrollmentViewModel in
				EnrollDeviceView(viewModel: deviceEnrollmentViewModel)
			}
			.sheet(item: $viewModel.destination.cameraScanner, id: \.presentationID) { enrollCameraScannerViewModel in
				EnrollCameraScannerView(viewModel: enrollCameraScannerViewModel)
			}
			.alert(
				item: $viewModel.destination.deepLinkParsingAlert,
				title: { errorMessage in
					Text(errorMessage)
				},
				actions: { _ in
					Button("OK") {}
				},
			)

			// MARK: Haptics

			.sensoryFeedback(.success, trigger: viewModel.sensoryFeedbackTrigger)
		}
	}
}

// MARK: - Subviews

extension LandingView {
	private var titleBlock: some View {
		VStack(spacing: .small) {
			Text("Scan QR code")
				.font(.title2)
				.fontWeight(.semibold)
			Text("Enroll this device for your cluster by scanning the QR code in your web browser")
				.multilineTextAlignment(.center)
				.foregroundStyle(Color.Foreground.slightlyMuted)
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
			viewModel.userTappedOnScanQRCode()
		} label: {
			Text("Scan QR Code")
				.frame(maxWidth: .infinity)
		}
		.buttonStyle(.primary)
		.controlSize(.large)
	}

	private func instructionStep(stepNumber: UInt, @ViewBuilder label: () -> some View) -> some View {
		Label {
			label()
				.font(.callout)
				.foregroundStyle(Color.Foreground.slightlyMuted)
		} icon: {
			Image(systemName: "\(stepNumber).circle.fill")
				.foregroundStyle(.tint.opacity(0.8))
		}
		.frame(maxWidth: .infinity, alignment: .leading)
		.padding()
		.background(
			RoundedRectangle(cornerRadius: .small)
				.fill(Color.Background.depth2),
		)
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
