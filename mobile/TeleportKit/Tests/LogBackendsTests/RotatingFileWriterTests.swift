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

			writer.enqueue(logMessage: "first")
			writer.enqueue(logMessage: "second")
			try await writer.flush()

			let contents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(contents == "firstsecond")
		}
	}

	@Test
	func `records enqueued after a completed flush are processed`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let writer = makeWriter(fileURL: fileURL)

			writer.enqueue(logMessage: "first")
			try await writer.flush()

			writer.enqueue(logMessage: "second")
			try await writer.flush()

			let contents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(contents == "firstsecond")
		}
	}

	@Test
	func `writing creates missing parent directories`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "nested/logs/events.log")
			let writer = makeWriter(fileURL: fileURL)

			writer.enqueue(logMessage: "record")
			try await writer.flush()

			let contents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(contents == "record")
		}
	}

	@Test
	func `writing appends to an existing active file`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			try Data("existing".utf8).write(to: fileURL)
			let writer = makeWriter(fileURL: fileURL)

			writer.enqueue(logMessage: "new")
			try await writer.flush()

			let contents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(contents == "existingnew")
		}
	}

	@Test
	func `an append reaching the exact size limit remains in the active file`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let archiveURL = directoryURL.appending(path: "events.1.log")
			try Data("123456789012".utf8).write(to: fileURL)
			let writer = makeWriter(fileURL: fileURL, maximumFileSize: 16)

			writer.enqueue(logMessage: "3456")
			try await writer.flush()

			let contents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(contents == "1234567890123456")
			#expect(!FileManager.default.fileExists(atPath: archiveURL.path))
		}
	}

	@Test
	func `a record matching the exact file size limit remains unchanged`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let archiveURL = directoryURL.appending(path: "events.1.log")
			let writer = makeWriter(fileURL: fileURL, maximumFileSize: 16)
			let record = "1234567890123456"

			writer.enqueue(logMessage: record)
			try await writer.flush()

			let contents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(contents == record)
			#expect(!FileManager.default.fileExists(atPath: archiveURL.path))
		}
	}

	@Test
	func `a record exceeding the size limit is truncated`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let archiveURL = directoryURL.appending(path: "events.1.log")
			let writer = makeWriter(fileURL: fileURL, maximumFileSize: 16)
			let record = "12345678901234567"
			let truncatedRecord = "1… [truncated]"

			writer.enqueue(logMessage: record)
			try await writer.flush()

			let activeContents = try? String(contentsOf: fileURL, encoding: .utf8)
			let archiveContents = try? String(contentsOf: archiveURL, encoding: .utf8)
			#expect(activeContents == "")
			#expect(archiveContents == truncatedRecord)
		}
	}

	@Test
	func `an append exceeding the size limit rotates the active file first`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let archiveURL = directoryURL.appending(path: "events.1.log")
			try Data("123456789012".utf8).write(to: fileURL)
			let writer = makeWriter(fileURL: fileURL, maximumFileSize: 16)

			writer.enqueue(logMessage: "34567")
			try await writer.flush()

			let activeContents = try? String(contentsOf: fileURL, encoding: .utf8)
			let archiveContents = try? String(contentsOf: archiveURL, encoding: .utf8)
			#expect(activeContents == "34567")
			#expect(archiveContents == "123456789012")
		}
	}

	@Test
	func `rotation ages archives and removes the oldest archive`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let firstArchiveURL = directoryURL.appending(path: "events.1.log")
			let secondArchiveURL = directoryURL.appending(path: "events.2.log")
			let thirdArchiveURL = directoryURL.appending(path: "events.3.log")
			try Data("active-record".utf8).write(to: fileURL)
			try Data("first archive".utf8).write(to: firstArchiveURL)
			try Data("second archive".utf8).write(to: secondArchiveURL)
			try Data("third archive".utf8).write(to: thirdArchiveURL)
			let writer = makeWriter(fileURL: fileURL, maximumFileSize: 16)

			writer.enqueue(logMessage: "next")
			try await writer.flush()

			let activeContents = try? String(contentsOf: fileURL, encoding: .utf8)
			let firstArchiveContents = try? String(contentsOf: firstArchiveURL, encoding: .utf8)
			let secondArchiveContents = try? String(contentsOf: secondArchiveURL, encoding: .utf8)
			let thirdArchiveContents = try? String(contentsOf: thirdArchiveURL, encoding: .utf8)
			#expect(activeContents == "next")
			#expect(firstArchiveContents == "active-record")
			#expect(secondArchiveContents == "first archive")
			#expect(thirdArchiveContents == "second archive")
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

private func makeWriter(
	fileURL: URL,
	maximumFileSize: Int = 4 * 1024 * 1024,
	maximumArchiveCount: Int = 3
) -> RotatingFileWriter {
	withDependencies {
		$0.fileSystemClient = FileSystemClient.liveValue
	} operation: {
		RotatingFileWriter(
			fileURL: fileURL,
			configuration: .init(
				maximumFileSize: maximumFileSize,
				maximumArchiveCount: maximumArchiveCount
			)
		)
	}
}
