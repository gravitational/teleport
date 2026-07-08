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
			Group {
				if viewModel.shouldShowPreEnrollmentLanding {
					PreEnrollmentLandingView(onScanQRCodeTapped: viewModel.userTappedOnScanQRCode)
				} else {
					PostEnrollmentLandingView(clusters: viewModel.clusters)
				}
			}

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

#Preview("Landing") {
	@Previewable @State
	var viewModel = LandingViewModel()
	LandingView(viewModel: viewModel)
}
