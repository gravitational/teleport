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
import Foundation
import LogBackends
import Logging
import Synchronization
import UIKit
import UniformTypeIdentifiers

/// Owns the singular RotatingFileWriter for the app and vends log handlers that are appropriate for debug vs. release.
final class LoggingController: Sendable {
	private let rotatingFileWriter: RotatingFileWriter?
	private let temporaryLogsURL: Mutex<URL?> = Mutex(nil)

	init() {
		do {
			var logDirectoryURL = try Self.logDirectoryURL()
			try FileManager.default.createDirectory(at: logDirectoryURL, withIntermediateDirectories: true)

			var resourceValues = URLResourceValues()
			resourceValues.isExcludedFromBackup = true
			try logDirectoryURL.setResourceValues(resourceValues)

			self.rotatingFileWriter = RotatingFileWriter(
				fileURL: logDirectoryURL.appending(path: "events.log", directoryHint: .notDirectory),
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

	/// Compresses the active log file along with any archives log files into a ZIP archive and returns the ZIP
	/// archive's URL.
	func compressLogFiles() async throws -> URL {
		try await rotatingFileWriter?.flush()
		let logDirectoryURL = try Self.logDirectoryURL()
		return try compressLogs(at: logDirectoryURL)
	}

	func clearTemporaryLogs() {
		temporaryLogsURL.withLock { temporaryLogsURL in
			if let temporaryLogsURL {
				try? FileManager.default.removeItem(at: temporaryLogsURL)
			}
			temporaryLogsURL = nil
		}
	}
}

// MARK: - LoggingController.Error

extension LoggingController {
	enum Error: Swift.Error {
		case failedToRetrieveCompressedLogURL
	}
}

// MARK: - Private Helpers

extension LoggingController {
	static func logDirectoryURL() throws -> URL {
		try FileManager.default.url(
			for: .applicationSupportDirectory,
			in: .userDomainMask,
			appropriateFor: nil,
			create: true,
		).appending(path: "Logs", directoryHint: .isDirectory)
	}

	private func compressLogs(at url: URL) throws -> URL {
		@Dependency(\.date.now)
		var now

		let timestamp = now.formatted(Self.zipFileTimestampFormat)

		var zippedLogsURL: URL? = nil
		var fileCoordinatorError: NSError? = nil
		var fileCopyError: any Swift.Error? = nil

		// Though poorly named, calling `coordinate` with a directory URL and a `.forUploading` option is the canonical
		// way to zip a directory on iOS. It's an old API, but it checks out.
		NSFileCoordinator().coordinate(
			readingItemAt: url,
			options: .forUploading,
			error: &fileCoordinatorError,
		) { ephemeralZipURL in
			// The file at ephemeralZipURL will be deleted once we leave this closure, so we need to copy it out
			let temporaryZipURL = FileManager
				.default
				.temporaryDirectory
				.appending(path: "teleport-logs-\(timestamp).zip")
			do {
				try? FileManager.default.removeItem(at: temporaryZipURL)
				try FileManager.default.copyItem(at: ephemeralZipURL, to: temporaryZipURL)
				zippedLogsURL = temporaryZipURL
			} catch {
				fileCopyError = error
			}
		}

		// Rethrow any errors we ran into while trying to compress the logs
		if let fileCoordinatorError {
			throw fileCoordinatorError
		}
		if let fileCopyError {
			throw fileCopyError
		}
		guard let zippedLogsURL else {
			// In practice this should never happen, but we need to satisfy the compiler
			throw Error.failedToRetrieveCompressedLogURL
		}

		temporaryLogsURL.withLock { $0 = zippedLogsURL }
		return zippedLogsURL
	}

	private static let zipFileTimestampFormat = Date.ISO8601FormatStyle(
		dateSeparator: .omitted,
		dateTimeSeparator: .standard,
		timeSeparator: .omitted,
		timeZoneSeparator: .omitted,
		includingFractionalSeconds: false,
		timeZone: .autoupdatingCurrent,
	)
}
