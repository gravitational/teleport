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

/// The root of our app's view model tree.
@Observable @MainActor
final class VerifyAppModel {
	static var logger = Logger.forType(VerifyAppModel.self)

	// MARK: Child View Models

	let landingViewModel = LandingViewModel()
}

// MARK: - Deep Link Handling

// We keep the deep link handing here at the root-most level of the app because from here we have a bird's eye view of
// the whole app and can manipulate any pieces we need.

extension VerifyAppModel {
	func openDeepLink(_ url: URL) {
		do {
			switch try DeepLink(from: url) {
				case let .enrollMobileDevice(deepLink):
					Self.logger.debug("Correctly parsed deep link from \(url): \(String(describing: deepLink))")
					landingViewModel.navigateToDeviceEnrollment(with: deepLink)
			}
		} catch {
			Self.logger.warning("Failed to parse deep link \"\(url)\", error: \(String(describing: error))")
			landingViewModel.showParserError(errorMessage: error.localizedDescription)
		}
	}
}
