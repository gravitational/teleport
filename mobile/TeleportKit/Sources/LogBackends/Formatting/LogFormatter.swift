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
import Logging

/// LogFormatter doesn't currently maintain any local state, so it's simplest to implement it as an enum with static
/// functions.
enum LogFormatter {
	private static let timestampFormatter = Date.ISO8601FormatStyle(
		dateSeparator: .dash,
		dateTimeSeparator: .space,
		timeSeparator: .colon,
		timeZoneSeparator: .colon,
		includingFractionalSeconds: true,
		timeZone: .autoupdatingCurrent,
	)

	/// Formats the log message
	/// - Parameters:
	///   - label: The label of the logger
	///   - event: The event to log
	///   - handlerMetadata: The metadata for the `LogHandler` (note: events carry their own metadata and this formatter
	/// 	automatically coalesces them with the `LogHandler` metadata)
	///   - timestamp: The time at which the logging event took place
	/// - Returns: A formatted log message suitable for output to a debug console or writing to a file
	static func format(
		label: String,
		event: LogEvent,
		handlerMetadata: Logger.Metadata,
		timestamp: Date,
	) -> String {
		let metadata = handlerMetadata.merging(event.metadata ?? [:]) { _, eventValue in eventValue }
		var components: [String] = [
			timestampFormatter.format(timestamp),
			event.level.formatted,
			"[\(label)]",
			"\(event.file):\(event.line)",
			"|",
			"\(event.message)",
		]

		let formattedMetadata = metadata.formatted
		if !formattedMetadata.isEmpty {
			components.append(formattedMetadata)
		}

		if let error = event.error {
			components.append("❌ error=\(error)")
		}

		return components.joined(separator: " ")
	}
}
