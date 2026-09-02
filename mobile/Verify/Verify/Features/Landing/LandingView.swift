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

			// MARK: Toolbar

			.toolbar {
				ToolbarItem {
					Menu {
						Button(
							"Forget All Clusters",
							systemImage: "trash",
							role: .destructive,
							action: viewModel.userTappedForgetAllClusters,
						)
						Button(
							"Share Logs",
							systemImage: "scroll",
							action: viewModel.userTappedShareLogsButton,
						)
						#if DEBUG
							Divider()
							Button(
								"Debug",
								systemImage: "apple.terminal",
								action: viewModel.userTappedOnDebugButton,
							)
						#endif
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
			.sheet(item: $viewModel.destination.shareLogs, id: \.presentationID, onDismiss: viewModel.userDismissedShareLogsSheet) { shareLogsViewModel in
				ShareLogsView(viewModel: shareLogsViewModel)
			}
			.alert(
				item: $viewModel.destination.notice,
				title: { notice in
					Text(notice.title)
				},
				actions: { _ in
					Button("OK") {}
				},
				message: { notice in
					if let message = notice.message {
						Text(message)
					}
				},
			)
			.alert(
				"Are you sure you want to forget all clusters?",
				isPresented: Binding($viewModel.destination.forgetAllClustersAlert),
				actions: {
					Button("Forget All Clusters", role: .destructive) {
						Task {
							await viewModel.userConfirmedForgetAllClusters()
						}
					}
					Button("Cancel", role: .cancel) {}
				},
				message: {
					Text(
						"""
						This action cannot be undone. You will still be able to authenticate using this device until \
						you remove it from your trusted devices in Account Settings in the web UI.
						""",
					)
				},
			)

			// MARK: Haptics

			.sensoryFeedback(.success, trigger: viewModel.sensoryFeedbackTrigger)

			// MARK: Debug

			// swiftformat:disable indent
			#if DEBUG
			.sheet(item: $viewModel.destination.debug, id: \.presentationID) { debugViewModel in
				DebugView(viewModel: debugViewModel)
			}
			#endif
			// swiftformat:enable indent
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
