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

import Foundation
import LogBackends
import Logging
import UIKit

/// Owns the singular RotatingFileWriter for the app and vends log handlers that are appropriate for debug vs. release.
final class LoggingController: Sendable {
	private let rotatingFileWriter: RotatingFileWriter?

	init() {
		do {
			let fileManager = FileManager.default
			var logsDirectoryURL = try fileManager.url(
				for: .applicationSupportDirectory,
				in: .userDomainMask,
				appropriateFor: nil,
				create: true,
			).appending(path: "Logs", directoryHint: .isDirectory)

			try fileManager.createDirectory(at: logsDirectoryURL, withIntermediateDirectories: true)

			var resourceValues = URLResourceValues()
			resourceValues.isExcludedFromBackup = true
			try logsDirectoryURL.setResourceValues(resourceValues)

			self.rotatingFileWriter = RotatingFileWriter(
				fileURL: logsDirectoryURL.appending(path: "events.log", directoryHint: .notDirectory),
			)
		} catch {
			print("Failed to initialize persistent logging: \(error)")
			self.rotatingFileWriter = nil
		}
	}

	/// Creates a log handler with differing behavior depending on whether we're in a release or debug build.
	///
	/// In a debug build, we multiplex the logs so that they're written to disk and also output to the debug console
	/// (standard out). Meanwhile, in a release build, we only write to disk.
	///
	/// In the event that writing to disk is unavailable for whatever reason, in debug we still write to the console,
	/// but in release, logging becomes a no-op.
	func makeLogHandler(label: String) -> any LogHandler {
		let fallbackHandler: any LogHandler
		#if DEBUG
			var consoleHandler = ConsoleLogHandler(label: label)
			consoleHandler.logLevel = CommandLine.arguments.contains("--sql-trace") ? .trace : .debug
			fallbackHandler = consoleHandler
		#else
			fallbackHandler = SwiftLogNoOpLogHandler()
		#endif

		guard let rotatingFileWriter else {
			print("Could not create rotating file handler for \(label). Writer was nil.")
			return fallbackHandler
		}

		var rotatingFileHandler = RotatingFileLogHandler(label: label, writer: rotatingFileWriter)
		rotatingFileHandler.logLevel = .info

		#if DEBUG
			return MultiplexLogHandler([rotatingFileHandler, fallbackHandler])
		#else
			return rotatingFileHandler
		#endif
	}

	func flushInBackground() async throws {
		guard let rotatingFileWriter else { return }
		try await rotatingFileWriter.flush()
	}
}
