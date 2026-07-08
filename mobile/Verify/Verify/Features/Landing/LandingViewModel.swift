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

import Observation
import SQLiteData
import SwiftNavigation

@Observable @MainActor
final class LandingViewModel {
	// swiftformat:sort
	@CasePathable
	enum Destination {
		case cameraScanner(EnrollCameraScannerViewModel)
		case deepLinkParsingAlert(errorMessage: String)
		case enrollDevice(EnrollDeviceViewModel)
	}

	@ObservationIgnored
	@FetchAll
	var clusters: [Cluster]

	var destination: Destination? = nil
	var sensoryFeedbackTrigger = false
}

// MARK: - UI Helper

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
}

// MARK: - Programmatic Navigation

extension LandingViewModel {
	func navigateToDeviceEnrollment(with deepLink: EnrollMobileDeviceDeepLink) {
		destination = .enrollDevice(EnrollDeviceViewModel(deepLink: deepLink, delegate: self))
	}

	func showParserError(errorMessage: String) {
		destination = .deepLinkParsingAlert(errorMessage: errorMessage)
	}
}

// MARK: - EnrollDeviceViewModel.Delegate

extension LandingViewModel: EnrollDeviceViewModel.Delegate {
	func enrollDeviceViewModelDidCancelOperation(_ viewModel: EnrollDeviceViewModel) {
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
