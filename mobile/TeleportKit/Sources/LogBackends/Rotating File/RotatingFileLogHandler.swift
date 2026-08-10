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

/// A log handler that writes to a rotating file on disk.
public struct RotatingFileLogHandler: LogHandler {
	public let label: String

	@Dependency(\.date.now)
	private var now

//	private let writer: RotatingFileWriter
//
//	public init(label: String, writer: RotatingFileWriter) {
//		self.label = label
//		self.writer = writer
//	}

	// MARK: LogHandler Conformance

	public var logLevel: Logger.Level = .info

	public var metadata: Logger.Metadata = [:]

	public subscript(metadataKey key: String) -> Logger.Metadata.Value? {
		get { metadata[key] }
		set { metadata[key] = newValue }
	}

	public func log(event: LogEvent) {
		let logMessage = LogFormatter.format(label: label, event: event, handlerMetadata: metadata, timestamp: now)
		// TODO: Write to file using the file writer
		// writer.enqueue(record: Data("\(output)\n".utf8))
	}
}
