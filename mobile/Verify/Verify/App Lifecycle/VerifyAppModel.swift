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
import LogBackends
import Logging
import Observation
import UIKit

/// The root of our app's view model tree.
@Observable @MainActor
final class VerifyAppModel {
	private let logger = Logger(label: "VerifyAppModel")
	private let loggingController: LoggingController

	// MARK: Child View Models

	let landingViewModel: LandingViewModel

	init(loggingController: LoggingController) {
		self.loggingController = loggingController
		self.landingViewModel = LandingViewModel()
		landingViewModel.delegate = self
	}

	func cleanUpBeforeBackgrounding() {
		beginBackgroundTask(named: "flush-logs") {
			try await self.loggingController.flushInBackground()
		}
	}
}

// MARK: - Deep Link Handling

// We keep the deep link handing here at the root-most level of the app because from here we have a bird's eye view of
// the whole app and can manipulate any pieces we need.

extension VerifyAppModel {
	func openDeepLink(_ url: URL) {
		do {
			switch try DeepLink(from: url) {
				case let .enrollMobileDevice(deepLink):
					logger.debug("Correctly parsed deep link", metadata: deepLink.logMetadata)
					landingViewModel.navigateToDeviceEnrollment(with: deepLink)
			}
		} catch {
			logger.warning(
				"Failed to parse deep link",
				error: error,
				metadata: ["scannedURL": "\(url, sensitivity: .sensitive)"],
			)
			landingViewModel.showParserError(errorMessage: error.localizedDescription)
		}
	}
}

// MARK: - Background Work

extension VerifyAppModel {
	func beginBackgroundTask(named name: String, task: @escaping @Sendable () async throws -> Void) {
		var backgroundTaskID: UIBackgroundTaskIdentifier = .invalid
		func endBackgroundTask() {
			UIApplication.shared.endBackgroundTask(backgroundTaskID)
			backgroundTaskID = .invalid
		}

		backgroundTaskID = UIApplication.shared.beginBackgroundTask(
			withName: name,
			expirationHandler: endBackgroundTask,
		)

		Task {
			defer { endBackgroundTask() }
			do {
				try await task()
			} catch {
				print("Failed to complete background task \"\(name)\": \(error)")
			}
		}
	}
}

// MARK: - LandingViewModel.Delegate

extension VerifyAppModel: LandingViewModel.Delegate {
	func landingViewModelDidRequestLogCollection(_ viewModel: LandingViewModel) async throws -> URL {
		try await loggingController.compressLogFiles()
	}

	func landingViewModelDidRequestTemporaryLogDeletion(_ viewModel: LandingViewModel) {
		loggingController.clearTemporaryLogs()
	}
}
