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
				Text("Open the Camera app to scan a QR code in the Web UI.")
					.font(.title)
					.fontWeight(.medium)
					.multilineTextAlignment(.center)
					.frame(maxHeight: .infinity)
			}
			.padding(.horizontal)
			.navigationDestination(item: $viewModel.destination.deviceEnrollment, destination: { deviceEnrollmentViewModel in
				EnrollMobileDeviceView(viewModel: deviceEnrollmentViewModel)
			})
			.alert(item: $viewModel.destination.failedToParseDeepLink, title: { errorMessage in
				Text(errorMessage)
			}, actions: { _ in
				Button("OK") {}
			})
		}
	}
}

#Preview("In ContentView") {
	@Previewable @State
	var viewModel = LandingViewModel()
	LandingView(viewModel: viewModel)
}
