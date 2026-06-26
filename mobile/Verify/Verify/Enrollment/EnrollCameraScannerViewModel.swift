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
import OSLog

@Observable @MainActor
final class EnrollCameraScannerViewModel {
	private static let logger = Logger.forType(EnrollCameraScannerViewModel.self)
	weak var delegate: (any Delegate)? = nil

	init(delegate: (any Delegate)? = nil) {
		self.delegate = delegate
	}
}

// MARK: - EnrollCameraScannerViewModel.Delegate

extension EnrollCameraScannerViewModel {
	protocol Delegate: AnyObject {
		func enrollCameraScannerViewModel(
			_ viewModel: EnrollCameraScannerViewModel,
			didReceiveEnrollMobileDeviceDeepLink deepLink: EnrollMobileDeviceDeepLink,
		)
	}
}

// MARK: - Scanner Actions

extension EnrollCameraScannerViewModel {
	func validateScannedCode(_ payload: String) -> EnrollMobileDeviceDeepLink? {
		Self.logger.debug("Validating scanned QR code: \(payload)")
		do {
			guard let url = URL(string: payload) else {
				return nil
			}
			let deepLink = try DeepLink(from: url)
			guard case let .enrollMobileDevice(enrollMobileDeviceDeepLink) = deepLink else {
				return nil
			}
			return enrollMobileDeviceDeepLink
		} catch {
			Self.logger.debug("\(payload) did not pass validation")
			return nil
		}
	}

	func didReceive(deepLink: EnrollMobileDeviceDeepLink) {
		Self.logger.info("Scanned deep link: \(deepLink.debugDescription)")
		delegate?.enrollCameraScannerViewModel(self, didReceiveEnrollMobileDeviceDeepLink: deepLink)
	}
}

// MARK: - Navigation Helpers

extension EnrollCameraScannerViewModel {
	/// For the purposes of presentation, there is no distinction between instances of EnrollCameraScannerViewModel,
	/// so we vend this constant presentation ID to express that to SwiftUI.
	var presentationID: String {
		"EnrollCameraScannerViewModel"
	}
}
