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
import SwiftUI

struct AuthenticateDeviceView: View {
	let viewModel: AuthenticateDeviceViewModel
	var body: some View {
		VStack(spacing: .medium) {
			ScrollView {
				VStack(spacing: .small) {
					Icon(systemName: "lock")
						.frame(maxWidth: 80)
						.padding(.bottom, .small)
						.padding(.top, .xxlarge)
					titleBlock
					if let failureMessage = viewModel.failureMessage {
						errorBlock(message: failureMessage)
					}
				}
				.multilineTextAlignment(.center)
				.frame(maxHeight: .infinity, alignment: .center)
			}
			.scrollBounceBehavior(.basedOnSize)
			primaryButton
			cancelButton
		}
		.padding()
		.background(Color.Background.depth3)
		.onDisappear(perform: viewModel.onDisappear)
	}
}

// MARK: - Subviews

extension AuthenticateDeviceView {
	@ViewBuilder
	var titleBlock: some View {
		Text("Authorize Device")
			.font(.title2)
			.fontWeight(.semibold)
		Text("You are authorizing this device for “\(viewModel.hostname)”")
			.foregroundStyle(Color.Foreground.slightlyMuted)
	}

	var primaryButton: some View {
		Button(action: viewModel.userTappedPrimaryButton) {
			Group {
				switch viewModel.primaryButtonState {
					case .loading:
						Label(
							"Loading…",
							systemImage: "progress.indicator",
						)
						.labelStyle(.iconOnly)
						.symbolEffect(
							.variableColor.iterative,
							options: .repeat(.continuous),
							isActive: true,
						)
					case .authorize:
						Text("Authorize")
					case .retry:
						Text("Retry")
					case .returnToWeb:
						Text("Return to Web")
					case .done:
						Text("Done")
				}
			}
			.frame(maxWidth: .infinity)
		}
		.buttonStyle(.primary)
		.controlSize(.large)
	}

	var cancelButton: some View {
		Button(role: .cancel, action: viewModel.userTappedCancel) {
			Text("Cancel").frame(maxWidth: .infinity)
		}
		.buttonStyle(.bordered)
		.controlSize(.large)
		.disabled(!viewModel.isCancelButtonEnabled)
	}

	func errorBlock(message: String) -> some View {
		Text(message)
			.font(.callout)
			.foregroundStyle(.danger)
	}
}

#Preview {
	AuthenticateDeviceView(
		viewModel: AuthenticateDeviceViewModel(
			deepLink: AuthenticateDeviceDeepLink(
				hostname: "example.com",
				port: Cluster.defaultPort,
				webTokenID: "dummy-id",
				webToken: "dummy-token",
				redirectURI: "https://example.com/some-resource",
			),
		),
	)
}
