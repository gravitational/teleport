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
@testable import LogBackends
import Logging
import SystemClients
import Testing

struct RotatingFileWriterTests {
	@Test
	func `flush appends all previously enqueued records in order`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let writer = makeWriter(fileURL: fileURL)

			writer.enqueue(logMessage: "first")
			writer.enqueue(logMessage: "second")
			try await writer.flush()

			let expectedContents = "firstsecond"
			let gotContents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(expectedContents == gotContents)
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

			let expectedContents = "firstsecond"
			let gotContents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(expectedContents == gotContents)
		}
	}

	@Test
	func `writing creates missing parent directories`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "nested/logs/events.log")
			let writer = makeWriter(fileURL: fileURL)

			writer.enqueue(logMessage: "record")
			try await writer.flush()

			let expectedContents = "record"
			let gotContents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(expectedContents == gotContents)
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

			let expectedContents = "existingnew"
			let gotContents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(expectedContents == gotContents)
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

			let expectedContents = "1234567890123456"
			let gotContents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(expectedContents == gotContents)
			#expect(!FileManager.default.fileExists(atPath: archiveURL.path))
		}
	}

	@Test
	func `a record matching the exact file size limit remains unchanged`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let archiveURL = directoryURL.appending(path: "events.1.log")
			let writer = makeWriter(fileURL: fileURL, maximumFileSize: 16)
			let expectedContents = "1234567890123456"

			writer.enqueue(logMessage: expectedContents)
			try await writer.flush()

			let gotContents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(expectedContents == gotContents)
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
			let expectedActiveContents = "1… [truncated]"

			writer.enqueue(logMessage: record)
			try await writer.flush()

			let gotActiveContents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(expectedActiveContents == gotActiveContents)
			#expect(!FileManager.default.fileExists(atPath: archiveURL.path))
		}
	}

	@Test
	func `truncation preserves valid UTF-8`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let writer = makeWriter(fileURL: fileURL, maximumFileSize: 18)

			// The letter `é` (U+00E9) encodes as two UTF-8 bytes (0xC3 0xA9). Only three bytes fit before the
			// truncation marker, so if we implemented truncation as a raw byte prefix, we would keep only `12` and only
			// the first byte of `é`. While 0xC3 is a valid leading byte, it's invalid UTF-8 on its own. So this test
			// ensures that our truncation code is UTF-8 aware.
			let record = "12é345678901234567"
			let expectedContents = "12… [truncated]"

			// These expectations are self-evident but I kept them here for clarity
			#expect(record.utf8.count == 19)
			#expect(expectedContents.utf8.count == 17)

			writer.enqueue(logMessage: record)
			try await writer.flush()

			let gotContents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(expectedContents == gotContents)
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

			let expectedActiveContents = "34567"
			let gotActiveContents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(expectedActiveContents == gotActiveContents)

			let expectedArchiveContents = "123456789012"
			let gotArchiveContents = try? String(contentsOf: archiveURL, encoding: .utf8)
			#expect(expectedArchiveContents == gotArchiveContents)
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

			let expectedActiveContents = "next"
			let gotActiveContents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(expectedActiveContents == gotActiveContents)

			let expectedFirstArchiveContents = "active-record"
			let gotFirstArchiveContents = try? String(contentsOf: firstArchiveURL, encoding: .utf8)
			#expect(expectedFirstArchiveContents == gotFirstArchiveContents)

			let expectedSecondArchiveContents = "first archive"
			let gotSecondArchiveContents = try? String(contentsOf: secondArchiveURL, encoding: .utf8)
			#expect(expectedSecondArchiveContents == gotSecondArchiveContents)

			let expectedThirdArchiveContents = "second archive"
			let gotThirdArchiveContents = try? String(contentsOf: thirdArchiveURL, encoding: .utf8)
			#expect(expectedThirdArchiveContents == gotThirdArchiveContents)
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

			let expectedContentsToContainMessage = true
			let gotContents = try? String(contentsOf: fileURL, encoding: .utf8)
			let gotContentsContainsMessage = gotContents?.contains("hello from the handler")
			#expect(expectedContentsToContainMessage == gotContentsContainsMessage)
		}
	}
}

private func makeWriter(
	fileURL: URL,
	maximumFileSize: Int = 4 * 1024 * 1024,
	maximumArchiveCount: Int = 3,
) -> RotatingFileWriter {
	withDependencies {
		$0.fileSystemClient = FileSystemClient.liveValue
	} operation: {
		RotatingFileWriter(
			fileURL: fileURL,
			configuration: .init(
				maximumFileSize: maximumFileSize,
				maximumArchiveCount: maximumArchiveCount,
			),
		)
	}
}
