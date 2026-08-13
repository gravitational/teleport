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

/// A simple print-based log handler primarily used for live debugging sessions.
public struct ConsoleLogHandler: LogHandler {
	public let label: String

	private let timestampFormmater = Date.ISO8601FormatStyle(
		dateSeparator: .dash,
		dateTimeSeparator: .space,
		timeSeparator: .colon,
		timeZoneSeparator: .colon,
		includingFractionalSeconds: true,
		timeZone: .current,
	)

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
		let timestamp = timestampFormmater.format(.now)
		let metadataToLog = metadata.merging(event.metadata ?? [:]) { _, new in new }

		let output = "\(timestamp) \(event.level.formatted) \(event.file):\(event.line) | \(event.message) \(metadataToLog.formatted)"

		print(output)
	}
}
