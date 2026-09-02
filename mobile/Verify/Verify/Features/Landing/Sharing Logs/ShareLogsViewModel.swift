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

import Dependencies
import Foundation
import Logging
import Observation
import SwiftUI
import SystemClients

@Observable @MainActor
final class ShareLogsViewModel {
	private let logger = Logger.forType(ShareLogsViewModel.self)
	let supportEmail = "support@goteleport.com"

	// MARK: Mutable State

	// swiftformat:sort:begin
	private var copyEmailButtonResetTask: Task<Void, Never>? = nil
	var copyEmailText: String
	var hapticTrigger = true
	private(set) var logCollectionLoadingState: LoadingState<URL> = .idle
	// swiftformat:sort:end

	weak var delegate: any Delegate? = nil

	// MARK: Dependencies

	@ObservationIgnored
	@Dependency(\.emailComposeClient)
	private var emailComposeClient

	@ObservationIgnored
	@Dependency(\.fileSystemClient)
	private var fileSystemClient

	@ObservationIgnored
	@Dependency(\.continuousClock)
	private var clock

	// MARK: Init

	init(delegate: any Delegate? = nil) {
		self.delegate = delegate
		self.copyEmailText = supportEmail
	}
}

// MARK: - ShareLogsViewModel.Delegate

extension ShareLogsViewModel {
	@MainActor
	protocol Delegate: AnyObject {
		/// Requests the collection of logs and returns the URL to the compressed archive.
		func shareLogsViewModelDidRequestLogCollection(_ viewModel: ShareLogsViewModel) async throws -> URL
	}
}

// MARK: - UI Helpers

extension ShareLogsViewModel {
	var zippedLogsURL: URL? {
		logCollectionLoadingState.value
	}

	var isEmailComposeAvailable: Bool {
		emailComposeClient.isAvailable()
	}
}

// MARK: - User Actions

extension ShareLogsViewModel {
	func userTappedCopyEmailButton() {
		UIPasteboard.general.string = supportEmail
		withAnimation {
			copyEmailText = NSLocalizedString(
				"Copied!",
				comment: "Button text that briefly flashes in place of the original label telling the user they've successfully copied to their clipboard.",
			)
		}
		hapticTrigger.toggle()

		copyEmailButtonResetTask?.cancel()
		copyEmailButtonResetTask = Task {
			try? await clock.sleep(for: .seconds(2))
			if Task.isCancelled { return }
			withAnimation {
				self.copyEmailText = self.supportEmail
			}
		}
	}

	/// Crafts the email
	///
	/// - Note: This doesn't follow the conventional `userTapped` prefix because it breaks from the standard way we
	/// store state, and so we need to send the email draft directly back to the view. For more information on why we're
	/// doing this see <x-source-tag://share-logs-email-destination>
	func makeEmailDraft() async -> EmailDraft? {
		guard
			emailComposeClient.isAvailable(),
			let zippedLogsURL
		else { return nil }

		do {
			let zippedLogsData = try fileSystemClient.readData(zippedLogsURL)
			try Task.checkCancellation()

			@Dependency(\.uuid)
			var uuid

			return EmailDraft(
				id: uuid(),
				recipients: [supportEmail],
				subject: "Teleport Verify Logs",
				body: "Please include a brief description of the issue you'd like us to investigate:\n\n",
				attachments: [
					.init(
						data: zippedLogsData,
						mimeType: "application/zip",
						fileName: zippedLogsURL.lastPathComponent,
					),
				],
			)
		} catch is CancellationError {
			return nil
		} catch {
			logger.warning("Failed to prepare logs for email", error: error)
			return nil
		}
	}
}

// MARK: - Log Collection

extension ShareLogsViewModel {
	func collectLogs() async {
		guard case .idle = logCollectionLoadingState else { return }

		logCollectionLoadingState = .loading
		guard let delegate else {
			logger.warning("No delegate was set when attempting to collect logs.")
			logCollectionLoadingState = .failure(Error.missingDelegate)
			return
		}

		do {
			let compressedLogsURL = try await delegate.shareLogsViewModelDidRequestLogCollection(self)
			try Task.checkCancellation()
			logCollectionLoadingState = .success(compressedLogsURL)
		} catch is CancellationError {
			logCollectionLoadingState = .idle
		} catch {
			logger.warning("Failed to collect log files", error: error)
			logCollectionLoadingState = .failure(error)
		}
	}
}

// MARK: - ShareLogsViewModel.Error

extension ShareLogsViewModel {
	enum Error: Swift.Error {
		case missingDelegate
	}
}

// MARK: - SheetPresentable

extension ShareLogsViewModel: SheetPresentable {
	var presentationID: some Hashable {
		"ShareLogsViewModel"
	}
}
