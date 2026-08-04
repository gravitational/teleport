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
public import Logging

/// Features:
/// - OSLog-style timestamp formatting
/// - Color circle emoji indicators for log levels
/// - Metadata support
/// - Label (category) display
public struct SimpleLogHandler: LogHandler {
	public let label: String

	private let formatter: DateFormatter = {
		let formatter = DateFormatter()
		formatter.dateFormat = "yyyy-MM-dd HH:mm:ss.SSS XXXXX"
		formatter.locale = Locale(identifier: "en_US_POSIX")
		return formatter
	}()

	public init(label: String) {
		self.label = label
	}

	// MARK: LogHandler Conformance

	public var logLevel: Logging.Logger.Level = .info

	public var metadata: Logging.Logger.Metadata = [:]

	public subscript(metadataKey key: String) -> Logging.Logger.Metadata.Value? {
		get { metadata[key] }
		set { metadata[key] = newValue }
	}

	public func log(event: LogEvent) {
		let timestamp = formatter.string(from: Date())
		let indicator = levelIndicator(for: event.level)
		let levelText = levelString(for: event.level)
		let metadataToLog = metadata.merging(event.metadata ?? [:]) { _, new in new }

		var output = "\(timestamp) \(indicator) \(levelText) [\(label)] \(event.message)\(metadataToLog.formattedByNewLines())"

		print(output)
	}
}

// MARK: - Private Helpers

extension SimpleLogHandler {
	private func levelIndicator(for level: Logger.Level) -> String {
		switch level {
			case .trace:
				"⚪️"
			case .debug:
				"🔵"
			case .info, .notice:
				"🟢"
			case .warning:
				"🟡"
			case .error, .critical:
				"🔴"
		}
	}

	private func levelString(for level: Logger.Level) -> String {
		switch level {
			case .trace:
				"TRACE"
			case .debug:
				"DEBUG"
			case .info:
				"INFO"
			case .notice:
				"NOTICE"
			case .warning:
				"WARNING"
			case .error:
				"ERROR"
			case .critical:
				"CRITICAL"
		}
	}
}

// MARK: - LoggingSystem Bootstrap Helper

extension LoggingSystem {
	public static func bootstrapSimpleLogStyle(defaultLogLevel: Logger.Level = .info) {
		LoggingSystem.bootstrap { label in
			var handler = SimpleLogHandler(label: label)
			handler.logLevel = defaultLogLevel
			return handler
		}
	}
}
