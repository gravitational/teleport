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

public import Logging
import OSLog

public struct OSLogBackend: LogHandler {
	private let osLogger: os.Logger

	public init(subsystem: String, category: String) {
		self.osLogger = Logger(subsystem: subsystem, category: category)
	}

	// MARK: LogHandler Conformance

	public var logLevel: Logging.Logger.Level = .info

	public var metadata: Logging.Logger.Metadata = [:]

	public subscript(metadataKey key: String) -> Logging.Logger.Metadata.Value? {
		get { metadata[key] }
		set { metadata[key] = newValue }
	}

	public func log(event: LogEvent) {
		var metadataToLog = metadata
		if let eventMetadata = event.metadata {
			// Always favor metadata provided by the event, not the one supplied as base to the logger.
			metadataToLog.merge(eventMetadata, uniquingKeysWith: { $1 })
		}

		osLogger.log(level: event.level.osLogLevel, "\(event.message)\(metadataToLog.formattedByNewLines())")
	}
}

// MARK: - Private Heleprs

extension Logging.Logger.Level {
	var osLogLevel: OSLogType {
		switch self {
			case .trace:
					.debug
			case .debug:
					.debug
			case .info:
					.info
			case .notice:
					.default
			case .warning:
					.error
			case .error:
					.error
			case .critical:
					.fault
		}
	}
}
