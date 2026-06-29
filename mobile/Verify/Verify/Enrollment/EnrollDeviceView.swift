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

struct EnrollDeviceView: View {
	var viewModel: EnrollDeviceViewModel

	var body: some View {
		VStack(spacing: .medium) {
			ScrollView {
				VStack(spacing: .small) {
					Icon(systemName: "ipad.and.iphone")
						.frame(maxWidth: 80)
						.padding(.bottom, .small)
						.padding(.top, .xxlarge)
					Text("Enroll Your Device")
						.font(.title2)
						.fontWeight(.semibold)
					Text(
						"""
						To finish enrolling this device, approve the request from \
						your account settings on another device.
						""",
					)
					.foregroundStyle(.secondary)
				}
				.multilineTextAlignment(.center)
				.frame(maxHeight: .infinity, alignment: .center)
			}
			.scrollBounceBehavior(.basedOnSize)
			Button {
				Task { await viewModel.requestEnrollToken() }
			} label: {
				Group {
					if viewModel.attempt.isLoading {
						Label(
							"Request in progress",
							systemImage: "progress.indicator",
						)
						.labelStyle(.iconOnly)
						.symbolEffect(
							.variableColor.iterative,
							options: .repeat(.continuous),
							isActive: true,
						)
					} else {
						Text("Request Now")
					}
				}
				.frame(maxWidth: .infinity)
			}
			.buttonStyle(.primary)
			.controlSize(.large)
			.animation(.easeInOut, value: viewModel.attempt.isLoading)
			.disabled(viewModel.attempt.isLoading)

			Button(role: .cancel, action: viewModel.userTappedCancel) {
				Text("Cancel").frame(maxWidth: .infinity)
			}
			.buttonStyle(.bordered)
			.controlSize(.large)
			.disabled(viewModel.attempt.isLoading)
		}
		.padding()
		.frame(maxWidth: .infinity, maxHeight: .infinity)
		.background(Color(.systemGroupedBackground))
		.sheet(
			isPresented: Binding(
				get: {
					switch viewModel.attempt {
						case .idle: false
						default: true
					}
				},
				set: { if !$0 { viewModel.attempt = .idle } },
			),
		) {
			EnrollRequestStatusView(
				attempt: viewModel.attempt,
				onDismiss: { viewModel.attempt = .idle },
			)
			.presentationDetents([.medium])
			.interactiveDismissDisabled(viewModel.attempt.isLoading)
		}
	}
}

#Preview {
	EnrollDeviceView(
		viewModel: EnrollDeviceViewModel(
			deepLink: EnrollMobileDeviceDeepLink(
				hostname: "localhost",
				port: 1234,
				enrollPairingToken: "pairing-token",
			),
		),
	)
}
