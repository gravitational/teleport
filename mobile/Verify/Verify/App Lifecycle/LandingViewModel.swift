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
import SwiftNavigation

@Observable @MainActor
final class LandingViewModel {
	@CasePathable
	enum Destination {
		case deviceEnrollment(EnrollDeviceViewModel)
		case deepLinkParsingAlert(errorMessage: String)
		case cameraScanner(EnrollCameraScannerViewModel)
	}

	var destination: Destination? = nil
}

// MARK: - User Actions

extension LandingViewModel {
	func userTappedOnScanQRCode() {
		destination = .cameraScanner(EnrollCameraScannerViewModel())
	}
}

// MARK: - Programmatic Navigation

extension LandingViewModel {
	func navigateToDeviceEnrollment(with deepLink: EnrollMobileDeviceDeepLink) {
		destination = .deviceEnrollment(EnrollDeviceViewModel(deepLink: deepLink, delegate: self))
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
