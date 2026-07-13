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
import Observation
import OSLog
import SQLiteData
import SwiftNavigation

@Observable @MainActor
final class LandingViewModel {
	static let logger = Logger.forType(LandingView.self)

	// swiftformat:sort
	@CasePathable
	enum Destination {
		case cameraScanner(EnrollCameraScannerViewModel)
		case deleteAllClustersAlert(AlertState<DeleteAllClustersAlertAction>)
		case enrollDevice(EnrollDeviceViewModel)
		case notice(AlertState<Void>)
	}

	@ObservationIgnored
	@Dependency(\.openURL)
	var openURL

	@ObservationIgnored
	@Dependency(\.defaultDatabase)
	var database

	@ObservationIgnored
	@FetchAll
	var clusters: [Cluster]

	var destination: Destination? = nil
	var sensoryFeedbackTrigger = false
}

// MARK: - LandingViewModel.DeleteAllClustersAlertAction

extension LandingViewModel {
	enum DeleteAllClustersAlertAction {
		case confirm
	}
}

// MARK: - UI Helper

extension LandingViewModel {
	var shouldShowPreEnrollmentLanding: Bool {
		clusters.isEmpty
	}

	var shouldShowToolbar: Bool {
		!shouldShowPreEnrollmentLanding
	}
}

// MARK: - User Actions

extension LandingViewModel {
	func userTappedOnScanQRCode() {
		destination = .cameraScanner(EnrollCameraScannerViewModel(delegate: self))
	}

	func userTapped(onCluster cluster: Cluster) async {
		if let url = cluster.url {
			await openURL(url)
		} else {
			destination = .notice(AlertState(
				title: {
					TextState("Bad URL")
				},
				message: {
					TextState("Could not build a valid HTTPS URL for \(cluster.host):\(String(cluster.port))")
				},
			))
		}
	}

	func userDeletedClusters(at indexSet: IndexSet) async {
		let idsToDelete = clusters.values(at: indexSet).map(\.id)
		await deleteClusters {
			Cluster
				.delete()
				.where { idsToDelete.contains($0.id) }
		}
	}

	func userTappedDeleteAllClusters() {
		let alertState = AlertState<DeleteAllClustersAlertAction> {
			TextState("Are you sure you want to unenroll your device from all clusters?")
		} actions: {
			ButtonState(role: .destructive, action: .confirm) { TextState("Confirm") }
			ButtonState(role: .cancel) { TextState("Cancel") }
		} message: {
			TextState("This action cannot be undone. You will need to re-enroll this device with each cluster.")
		}
		destination = .deleteAllClustersAlert(alertState)
	}

	func userConfirmedDeleteAllClusters() async {
		await deleteClusters { Cluster.delete() }
	}
}

// MARK: - Programmatic Navigation

extension LandingViewModel {
	func navigateToDeviceEnrollment(with deepLink: EnrollMobileDeviceDeepLink) {
		destination = .enrollDevice(EnrollDeviceViewModel(deepLink: deepLink, delegate: self))
	}

	func showParserError(errorMessage: String) {
		destination = .notice(AlertState(
			title: {
				TextState(errorMessage)
			},
		))
	}
}

// MARK: - EnrollDeviceViewModel.Delegate

extension LandingViewModel: EnrollDeviceViewModel.Delegate {
	func enrollDeviceViewModelDidCancelOperation(_ viewModel: EnrollDeviceViewModel) {
		destination = nil
	}

	func enrollDeviceViewModelDidEnrollCluster(_ viewModel: EnrollDeviceViewModel) {
		destination = nil
	}
}

// MARK: - EnrollCameraScannerViewModel.Delegate

extension LandingViewModel: EnrollCameraScannerViewModel.Delegate {
	func enrollCameraScannerViewModel(
		_ viewModel: EnrollCameraScannerViewModel,
		didReceiveEnrollMobileDeviceDeepLink deepLink: EnrollMobileDeviceDeepLink,
	) {
		sensoryFeedbackTrigger.toggle()
		destination = .enrollDevice(EnrollDeviceViewModel(deepLink: deepLink, delegate: self))
	}
}

// MARK: - Private Helpers

extension LandingViewModel {
	/// A helper function that encapsulates running a cluster deletion and showing an error upon failure.
	private func deleteClusters(using deleteOperation: @Sendable () -> DeleteOf<Cluster>) async {
		do {
			try await database.write { db in
				try deleteOperation().execute(db)
			}
		} catch {
			Self.logger.warning("Failed to delete clusters: \(error)")
			destination = .notice(AlertState(
				title: {
					TextState("Could Not Delete Clusters")
				},
				message: {
					TextState("An error occurred when trying to deregister the cluster from your device.")
				},
			))
		}
	}
}
