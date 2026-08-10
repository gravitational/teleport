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
import SystemClients
import Testing
@testable import LogBackends

struct RotatingFileWriterTests {
	@Test
	func `flush appends all previously enqueued records in order`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let writer = makeWriter(fileURL: fileURL)

			writer.enqueue(record: Data("first\n".utf8))
			writer.enqueue(record: Data("second\n".utf8))
			try await writer.flush()

			let contents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(contents == "first\nsecond\n")
		}
	}

	@Test
	func `handler sends its formatted record to the shared writer`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let writer = makeWriter(fileURL: fileURL)
			let handler = withDependencies {
				$0.date.now = Date(timeIntervalSince1970: 0)
			} operation: {
				RotatingFileLogHandler(label: "test.logger", writer: writer)
			}

			handler.log(event: LogEvent(
				level: .info,
				message: "hello from the handler",
				metadata: nil,
				source: nil,
				file: "Module/File.swift",
				function: "run()",
				line: 42,
			))
			try await writer.flush()

			let contents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(contents?.contains("hello from the handler") == true)
		}
	}
}

private func makeWriter(fileURL: URL) -> RotatingFileWriter {
	withDependencies {
		$0.fileSystemClient = FileSystemClient.liveValue
	} operation: {
		RotatingFileWriter(fileURL: fileURL)
	}
}
