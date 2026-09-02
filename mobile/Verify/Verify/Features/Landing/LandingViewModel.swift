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
import Logging
import Observation
import SQLiteData
import SwiftNavigation

@Observable @MainActor
final class LandingViewModel {
	private let logger = Logger.forType(LandingView.self)

	// swiftformat:sort
	@CasePathable
	enum Destination {
		case cameraScanner(EnrollCameraScannerViewModel)
		case enrollDevice(EnrollDeviceViewModel)
		case forgetAllClustersAlert
		case notice(title: String, message: String? = nil)
		case shareLogs(ShareLogsViewModel)
		#if DEBUG
			case debug(DebugViewModel)
		#endif
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

	weak var delegate: any Delegate? = nil
	var destination: Destination? = nil
	var sensoryFeedbackTrigger = false
}

// MARK: - LandingViewModel.Delegate

extension LandingViewModel {
	@MainActor
	protocol Delegate: AnyObject {
		/// Requests the collection of logs and returns the URL to the compressed archive.
		func landingViewModelDidRequestLogCollection(_ viewModel: LandingViewModel) async throws -> URL
		func landingViewModelDidRequestTemporaryLogDeletion(_ viewModel: LandingViewModel)
	}
}

// MARK: - UI Helpers

extension LandingViewModel {
	var shouldShowPreEnrollmentLanding: Bool {
		clusters.isEmpty
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
			destination = .notice(
				title: "Bad URL",
				message: "Could not build a valid HTTPS URL for \(cluster.host):\(String(cluster.port))",
			)
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

	func userTappedShareLogsButton() {
		destination = .shareLogs(ShareLogsViewModel(delegate: self))
	}

	func userDismissedShareLogsSheet() {
		delegate?.landingViewModelDidRequestTemporaryLogDeletion(self)
	}

	func userTappedForgetAllClusters() {
		destination = .forgetAllClustersAlert
	}

	func userConfirmedForgetAllClusters() async {
		await deleteClusters { Cluster.delete() }
	}

	#if DEBUG
		func userTappedOnDebugButton() {
			destination = .debug(DebugViewModel())
		}
	#endif
}

// MARK: - ShareLogsViewModel.Delegate

extension LandingViewModel: ShareLogsViewModel.Delegate {
	func shareLogsViewModelDidRequestLogCollection(_ viewModel: ShareLogsViewModel) async throws -> URL {
		guard let delegate else {
			logger.warning("No delegate was set when attempting to collect logs.")
			throw Error.missingDelegate
		}

		return try await delegate.landingViewModelDidRequestLogCollection(self)
	}
}

// MARK: - LandingViewModel.Error

extension LandingViewModel {
	enum Error: Swift.Error {
		case missingDelegate
	}
}

// MARK: - Programmatic Navigation

extension LandingViewModel {
	func navigateToDeviceEnrollment(with deepLink: EnrollMobileDeviceDeepLink) {
		destination = .enrollDevice(EnrollDeviceViewModel(deepLink: deepLink, delegate: self))
	}

	func showParserError(errorMessage: String) {
		destination = .notice(title: errorMessage)
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
			logger.warning("Failed to forget clusters", error: error)
			destination = .notice(
				title: "Could Not Forget Clusters",
				message: "An error occurred when trying to forget the cluster on this device.",
			)
		}
	}
}
