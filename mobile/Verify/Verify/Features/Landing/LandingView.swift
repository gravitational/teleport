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
				Image(.logo)
					.resizable()
					.scaledToFit()
					.frame(maxWidth: .infinity, maxHeight: 44, alignment: .leading)
				if viewModel.shouldShowPreEnrollmentLanding {
					PreEnrollmentLandingView(
						onScanQRCodeTapped: viewModel.userTappedOnScanQRCode,
					)
				} else {
					PostEnrollmentLandingView(
						clusters: viewModel.clusters,
						didTapOnCluster: viewModel.userTapped(onCluster:),
						didDeleteClustersAtIndex: viewModel.userDeletedClusters(at:),
					)
				}
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
			.alert($viewModel.destination.notice) { _ in }

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
