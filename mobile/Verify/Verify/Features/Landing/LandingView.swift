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

import Dependencies
import SQLiteData
import SwiftUI
import SwiftUINavigation

/// The root-most view of our app (i.e. where the user initially "lands").
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
			.toolbarVisibility(viewModel.shouldShowToolbar ? .visible : .hidden)
			.toolbar {
				ToolbarItem {
					Menu {
						Button(
							"Delete all clusters",
							systemImage: "trash",
							role: .destructive,
							action: viewModel.userTappedDeleteAllClusters,
						)
					} label: {
						Label("Menu", systemImage: "ellipsis")
					}
				}
			}

			// MARK: Navigation

			.navigationDestination(item: $viewModel.destination.enrollDevice) { deviceEnrollmentViewModel in
				EnrollDeviceView(viewModel: deviceEnrollmentViewModel)
			}
			.sheet(item: $viewModel.destination.cameraScanner, id: \.presentationID) { enrollCameraScannerViewModel in
				EnrollCameraScannerView(viewModel: enrollCameraScannerViewModel)
			}
			.alert($viewModel.destination.notice) { _ in }
			.alert($viewModel.destination.deleteAllClustersAlert) { action in
				switch action {
					case .confirm: Task { await viewModel.userConfirmedDeleteAllClusters() }
					case .none: break
				}
			}

			// MARK: Haptics

			.sensoryFeedback(.success, trigger: viewModel.sensoryFeedbackTrigger)
		}
	}
}

#Preview("Pre-enrollment") {
	LandingView(viewModel: LandingViewModel())
}

#Preview("Post-enrollment") {
	@Previewable @State
	var viewModel = withDependencies {
		$0.defaultDatabase = AppDatabase.makePreviewDatabase()
	} operation: {
		LandingViewModel()
	}

	LandingView(viewModel: viewModel)
}
