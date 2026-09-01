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
import Synchronization
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
	func `a failed write is reported by the next flush exactly once`() async throws {
		struct WriteError: Error, Equatable {}

		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let expectedError = WriteError()
			let fileSystemClient = FileSystemClient.liveValue.failingFirstWrite(with: expectedError)
			let writer = makeWriter(fileURL: fileURL, fileSystemClient: fileSystemClient)

			writer.enqueue(logMessage: "record")
			await #expect(throws: expectedError) {
				try await writer.flush()
			}
			try await writer.flush()
		}
	}

	@Test
	func `multiple failed writes report the first error`() async throws {
		struct FirstWriteError: Error, Equatable {}
		struct SecondWriteError: Error {}

		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let expectedError = FirstWriteError()
			let fileSystemClient = FileSystemClient.liveValue.failingWrites(with: [
				expectedError,
				SecondWriteError(),
			])
			let writer = makeWriter(fileURL: fileURL, fileSystemClient: fileSystemClient)

			writer.enqueue(logMessage: "first")
			writer.enqueue(logMessage: "second")
			await #expect(throws: expectedError) {
				try await writer.flush()
			}
			try await writer.flush()
		}
	}

	@Test
	func `a failed record is not retried and later records continue`() async throws {
		struct WriteError: Error, Equatable {}

		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let expectedError = WriteError()
			let fileSystemClient = FileSystemClient.liveValue.failingFirstWrite(with: expectedError)
			let writer = makeWriter(fileURL: fileURL, fileSystemClient: fileSystemClient)

			writer.enqueue(logMessage: "failed")
			writer.enqueue(logMessage: "succeeded")
			await #expect(throws: expectedError) {
				try await writer.flush()
			}
			try await writer.flush()

			let expectedContents = "succeeded"
			let gotContents = try String(contentsOf: fileURL, encoding: .utf8)
			#expect(expectedContents == gotContents)
		}
	}

	@Test
	func `a synchronization error takes precedence over a previous write failure`() async throws {
		struct WriteError: Error {}
		struct SynchronizeError: Error, Equatable {}

		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let expectedError = SynchronizeError()
			let fileSystemClient = FileSystemClient.liveValue
				.failingFirstWrite(with: WriteError())
				.failingFirstSynchronize(with: expectedError)
			let writer = makeWriter(fileURL: fileURL, fileSystemClient: fileSystemClient)

			writer.enqueue(logMessage: "record")
			await #expect(throws: expectedError) {
				try await writer.flush()
			}
			try await writer.flush()
		}
	}

	@Test
	func `queue overflow evicts the oldest pending record and preserves the newest`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let (fileSystemClient, firstOpenGate) = FileSystemClient.liveValue.blockingFirstOpen()
			let writer = makeWriter(
				fileURL: fileURL,
				maximumQueuedBytes: 16,
				fileSystemClient: fileSystemClient,
			)

			writer.enqueue(logMessage: "first\n")
			try await firstOpenGate.withNextCallBlocked {
				writer.enqueue(logMessage: "old-record\n")
				writer.enqueue(logMessage: "newest-record\n")
			}
			try await writer.flush()

			let expectedContents = """
				first
				🗑️ Dropped 1 record
				newest-record

				"""
			let gotContents = try String(contentsOf: fileURL, encoding: .utf8)
			#expect(expectedContents == gotContents)
		}
	}

	@Test
	func `queue overflow keeps a dropped record notice behind an earlier flush request`() async throws {
		try await withCheckedThrowingContinuation { flushContinuation in
			var inbox = RotatingFileWriter.Inbox(maximumQueuedBytes: 16)
			_ = inbox.append(.flush(flushContinuation))
			_ = inbox.append(.logMessage("old-record"))
			_ = inbox.append(.logMessage("newest-record"))

			let expectedPendingItems: [PendingItemSnapshot] = [
				.flush,
				.droppedRecordNotice(1),
				.logMessage("newest-record"),
			]
			let gotPendingItems = inbox.collectPendingItems()
			#expect(expectedPendingItems == gotPendingItems)
			flushContinuation.resume()
		}
	}

	@Test
	func `repeated overflow cannot move a dropped record notice ahead of a pending flush`() async throws {
		try await withCheckedThrowingContinuation { flushContinuation in
			var inbox = RotatingFileWriter.Inbox(maximumQueuedBytes: 8)
			_ = inbox.append(.logMessage("before!!"))
			_ = inbox.append(.flush(flushContinuation))
			_ = inbox.append(.logMessage("after!!!"))

			inbox.beginProcessing()
			guard case .droppedRecordNotice(1) = inbox.nextPendingItem() else {
				Issue.record("Expected the first overflow notice to precede the flush")
				flushContinuation.resume()
				return
			}

			// Model the worker writing that notice while producers continue overflowing the queue. The replacement
			// notice describes a record enqueued after the flush, so it must remain behind the flush barrier.
			_ = inbox.append(.logMessage("newest!!"))

			let expectedPendingItems: [PendingItemSnapshot] = [
				.flush,
				.droppedRecordNotice(1),
				.logMessage("newest!!"),
			]
			let gotPendingItems = inbox.collectPendingItemsAfterProcessingBegan()
			#expect(expectedPendingItems == gotPendingItems)
			#expect(inbox.pendingLogMessageByteCount == 0)
			flushContinuation.resume()
		}
	}

	@Test
	func `queue overflow preserves consecutive flush boundaries`() async throws {
		try await withCheckedThrowingContinuation { flushContinuation in
			var inbox = RotatingFileWriter.Inbox(maximumQueuedBytes: 16)
			_ = inbox.append(.logMessage("before!!"))
			_ = inbox.append(.flush(flushContinuation))
			_ = inbox.append(.flush(flushContinuation))
			_ = inbox.append(.logMessage("after!!!"))
			_ = inbox.append(.logMessage("newest-record!!!"))

			let expectedPendingItems: [PendingItemSnapshot] = [
				.droppedRecordNotice(1),
				.flush,
				.flush,
				.droppedRecordNotice(1),
				.logMessage("newest-record!!!"),
			]
			let gotPendingItems = inbox.collectPendingItems()
			#expect(expectedPendingItems == gotPendingItems)
			#expect(inbox.pendingLogMessageByteCount == 0)
			flushContinuation.resume()
		}
	}

	@Test
	func `queue overflow drops records across multiple nonempty flush delimited segments`() async throws {
		try await withCheckedThrowingContinuation { flushContinuation in
			var inbox = RotatingFileWriter.Inbox(maximumQueuedBytes: 24)
			_ = inbox.append(.logMessage("before!!"))
			_ = inbox.append(.flush(flushContinuation))
			_ = inbox.append(.logMessage("middle!!"))
			_ = inbox.append(.flush(flushContinuation))
			_ = inbox.append(.logMessage("after!!!"))
			_ = inbox.append(.logMessage("newest-record!!!"))

			let expectedPendingItems: [PendingItemSnapshot] = [
				.droppedRecordNotice(1),
				.flush,
				.droppedRecordNotice(1),
				.flush,
				.logMessage("after!!!"),
				.logMessage("newest-record!!!"),
			]
			let gotPendingItems = inbox.collectPendingItems()
			#expect(expectedPendingItems == gotPendingItems)
			#expect(inbox.pendingLogMessageByteCount == 0)
			flushContinuation.resume()
		}
	}

	@Test
	func `repeated overflow behind a flush coalesces notices without crossing the boundary`() async throws {
		try await withCheckedThrowingContinuation { flushContinuation in
			var inbox = RotatingFileWriter.Inbox(maximumQueuedBytes: 8)
			_ = inbox.append(.flush(flushContinuation))
			_ = inbox.append(.logMessage("oldest!"))
			_ = inbox.append(.logMessage("middle!!"))
			_ = inbox.append(.logMessage("newest!!"))

			#expect(inbox.pendingLogMessageByteCount == 8)
			let expectedPendingItems: [PendingItemSnapshot] = [
				.flush,
				.droppedRecordNotice(2),
				.logMessage("newest!!"),
			]
			let gotPendingItems = inbox.collectPendingItems()
			#expect(expectedPendingItems == gotPendingItems)
			#expect(inbox.pendingLogMessageByteCount == 0)
			flushContinuation.resume()
		}
	}

	@Test
	func `queue overflow drops only enough variable sized records and maintains its byte count`() {
		var inbox = RotatingFileWriter.Inbox(maximumQueuedBytes: 20)
		_ = inbox.append(.logMessage("123"))
		_ = inbox.append(.logMessage("12345678"))
		_ = inbox.append(.logMessage("123456789"))
		_ = inbox.append(.logMessage("abcdefghij"))

		#expect(inbox.pendingLogMessageByteCount == 19)
		let expectedPendingItems: [PendingItemSnapshot] = [
			.droppedRecordNotice(2),
			.logMessage("123456789"),
			.logMessage("abcdefghij"),
		]
		let gotPendingItems = inbox.collectPendingItems()
		#expect(expectedPendingItems == gotPendingItems)
		#expect(inbox.pendingLogMessageByteCount == 0)
	}

	@Test
	func `repeated queue overflow coalesces adjacent dropped record notices`() {
		var inbox = RotatingFileWriter.Inbox(maximumQueuedBytes: 16)
		_ = inbox.append(.logMessage("oldest-record"))
		_ = inbox.append(.logMessage("middle-record"))
		_ = inbox.append(.logMessage("newest-record"))

		let expectedPendingItems: [PendingItemSnapshot] = [
			.droppedRecordNotice(2),
			.logMessage("newest-record"),
		]
		let gotPendingItems = inbox.collectPendingItems()
		#expect(expectedPendingItems == gotPendingItems)
	}

	@Test
	func `dropped record notices do not count toward the queue limit`() {
		var inbox = RotatingFileWriter.Inbox(maximumQueuedBytes: 16)
		_ = inbox.append(.logMessage("oldest-record"))
		_ = inbox.append(.logMessage("one!"))
		_ = inbox.append(.logMessage("two!"))

		let expectedPendingItems: [PendingItemSnapshot] = [
			.droppedRecordNotice(1),
			.logMessage("one!"),
			.logMessage("two!"),
		]
		let gotPendingItems = inbox.collectPendingItems()
		#expect(expectedPendingItems == gotPendingItems)
	}

	@Test
	func `flush resets the pending log message byte count`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let (fileSystemClient, firstSynchronizeGate) = FileSystemClient.liveValue.blockingFirstSynchronize()
			let writer = makeWriter(
				fileURL: fileURL,
				maximumQueuedBytes: 16,
				fileSystemClient: fileSystemClient,
			)

			writer.enqueue(logMessage: "first!!\n")
			async let blockedFlush: Void = writer.flush()
			try await firstSynchronizeGate.withNextCallBlocked {
				writer.enqueue(logMessage: "second!\n")
				writer.enqueue(logMessage: "third!!\n")
				#expect(writer.pendingLogMessageByteCountForTesting == 16)
			}
			try await blockedFlush

			// Ensure the records that were enqueued _after_ the `blockedFlush` was kicked off are also written to disk.
			try await writer.flush()
			#expect(writer.pendingLogMessageByteCountForTesting == 0)

			let expectedContents = """
				first!!
				second!
				third!!

				"""
			let gotContents = try String(contentsOf: fileURL, encoding: .utf8)
			#expect(expectedContents == gotContents)
		}
	}

	@Test
	func `queue limit counts UTF-8 bytes rather than characters`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let (fileSystemClient, firstOpenGate) = FileSystemClient.liveValue.blockingFirstOpen()
			let writer = makeWriter(
				fileURL: fileURL,
				maximumQueuedBytes: 16,
				fileSystemClient: fileSystemClient,
			)

			writer.enqueue(logMessage: "first\n")
			try await firstOpenGate.withNextCallBlocked {
				writer.enqueue(logMessage: "ééééé\n")
				writer.enqueue(logMessage: "newest!\n")
			}
			try await writer.flush()

			let expectedContents = """
				first
				🗑️ Dropped 1 record
				newest!

				"""
			let gotContents = try String(contentsOf: fileURL, encoding: .utf8)
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
	func `a record exceeding the queue limit is truncated before being enqueued`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let writer = makeWriter(
				fileURL: fileURL,
				maximumFileSize: 32,
				maximumQueuedBytes: 16,
			)
			let record = "12345678901234567"
			let expectedContents = "1… [truncated]"

			writer.enqueue(logMessage: record)
			try await writer.flush()

			let gotContents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(expectedContents == gotContents)
		}
	}

	@Test
	func `an oversized record rotates a nonempty active file before writing the truncated record`() async throws {
		try await withTemporaryDirectory { directoryURL in
			let fileURL = directoryURL.appending(path: "events.log")
			let archiveURL = directoryURL.appending(path: "events.1.log")
			try Data("existing".utf8).write(to: fileURL)
			let writer = makeWriter(fileURL: fileURL, maximumFileSize: 16)
			let record = "12345678901234567"

			writer.enqueue(logMessage: record)
			try await writer.flush()

			let expectedActiveContents = "1… [truncated]"
			let gotActiveContents = try? String(contentsOf: fileURL, encoding: .utf8)
			#expect(expectedActiveContents == gotActiveContents)

			let expectedArchiveContents = "existing"
			let gotArchiveContents = try? String(contentsOf: archiveURL, encoding: .utf8)
			#expect(expectedArchiveContents == gotArchiveContents)
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

// MARK: - Private Test Helpers

extension FileSystemClient {
	/// Returns this client with only its first file-write operation replaced by the provided error.
	fileprivate func failingFirstWrite(with error: any Error & Sendable) -> Self {
		failingWrites(with: [error])
	}

	/// Returns this client with its initial file-write operations replaced by the provided errors, in order.
	fileprivate func failingWrites(with errors: [any Error & Sendable]) -> Self {
		let nextErrorIndex = Mutex(0)
		var client = self
		let openFileForWriting = client.openFileForWriting
		client.openFileForWriting = { url in
			var fileClient = try openFileForWriting(url)
			let write = fileClient.write
			fileClient.write = { data in
				let error = nextErrorIndex.withLock { nextErrorIndex -> (any Error & Sendable)? in
					guard nextErrorIndex < errors.count else { return nil }
					defer { nextErrorIndex += 1 }
					return errors[nextErrorIndex]
				}
				if let error { throw error }

				try write(data)
			}
			return fileClient
		}
		return client
	}

	/// Returns this client with only its first file-synchronization operation replaced by the provided error.
	fileprivate func failingFirstSynchronize(with error: any Error & Sendable) -> Self {
		let shouldFailNextSynchronize = Mutex(true)
		var client = self
		let openFileForWriting = client.openFileForWriting
		client.openFileForWriting = { url in
			var fileClient = try openFileForWriting(url)
			let synchronize = fileClient.synchronize
			fileClient.synchronize = {
				let shouldFail = shouldFailNextSynchronize.withLock { shouldFailNextSynchronize in
					defer { shouldFailNextSynchronize = false }
					return shouldFailNextSynchronize
				}
				guard !shouldFail else { throw error }

				try synchronize()
			}
			return fileClient
		}
		return client
	}

	/// Returns this client with only its first file-synchronization operation paused, along with the gate controlling
	/// it.
	fileprivate func blockingFirstSynchronize() -> (client: Self, gate: BlockingCallGate) {
		let gate = BlockingCallGate()
		var client = self
		let openFileForWriting = client.openFileForWriting
		client.openFileForWriting = { url in
			var fileClient = try openFileForWriting(url)
			let synchronize = fileClient.synchronize
			fileClient.synchronize = {
				gate.blockNextCall()
				try synchronize()
			}
			return fileClient
		}
		return (client, gate)
	}

	/// Returns this client with only its first file-open operation paused, along with the gate controlling it.
	///
	/// The writer removes a record from its inbox before opening the file, so pausing here leaves later records
	/// pending.
	fileprivate func blockingFirstOpen() -> (client: Self, gate: BlockingCallGate) {
		let gate = BlockingCallGate()
		var client = self
		let openFileForWriting = client.openFileForWriting
		client.openFileForWriting = { url in
			gate.blockNextCall()
			return try openFileForWriting(url)
		}
		return (client, gate)
	}
}

private func makeWriter(
	fileURL: URL,
	maximumFileSize: Int = 4 * 1024 * 1024,
	maximumArchiveCount: Int = 3,
	maximumQueuedBytes: Int = 4 * 1024 * 1024,
	fileSystemClient: FileSystemClient = .liveValue,
) -> RotatingFileWriter {
	withDependencies {
		$0.fileSystemClient = fileSystemClient
	} operation: {
		RotatingFileWriter(
			fileURL: fileURL,
			configuration: .init(
				maximumFileSize: maximumFileSize,
				maximumArchiveCount: maximumArchiveCount,
				maximumQueuedBytes: maximumQueuedBytes,
			),
		)
	}
}
