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
import DequeModule
public import Foundation
import Synchronization
import SystemClients

/// Opens a file for writing which rotates to a new file whenever the original is at capacity.
public final class RotatingFileWriter: Sendable {
	// MARK: Mutable State

	///	An inbox that maintains the pending items to be written.
	///
	/// This mutext protects pending-item bookkeeping only. Filesystem work must never occur while holding this mutex,
	/// keeping enqueue operations independent of slow I/O.
	private let inbox = Mutex(Inbox())

	/// A handle to the currently open/active file.
	///
	/// This mutex protects the active file state separately from the inbox mutext because it may be held during
	/// synchronous filesystem operations.
	private let fileState = Mutex(FileState())

	// MARK: Immutable State

	private let activeFileURL: URL
	private let configuration: Configuration
	private let queue = DispatchQueue(label: "com.goteleport.teleportkit.rotating-file-writer")
	private let fileSystemClient: FileSystemClient

	public convenience init(fileURL: URL) {
		self.init(fileURL: fileURL, configuration: .live)
	}

	init(fileURL: URL, configuration: Configuration) {
		self.activeFileURL = fileURL
		self.configuration = configuration

		// Retrieving the dependency once during init is safe because the file system implementation should not
		// change mid-process.
		@Dependency(\.fileSystemClient)
		var fileSystemClient

		self.fileSystemClient = fileSystemClient
	}

	/// Waits for all previously enqueued records to be processed and synchronizes the active file.
	public func flush() async throws {
		try await withCheckedThrowingContinuation { continuation in
			// Enqueuing the continuation allows us to ensure all earlier items are processed before the active file is
			// synchronized and the continuation resumes.
			enqueue(item: .flush(continuation))
		}
	}

	/// Enqueues a log message to be written to disk
	func enqueue(logMessage: String) {
		enqueue(item: .logMessage(logMessage))
	}
}

// MARK: - Inbox Processing

extension RotatingFileWriter {
	/// Adds an item to the inbox and schedules processing if it isn't already scheduled or active.
	private func enqueue(item: PendingItem) {
		let shouldScheduleProcessing = inbox.withLock { inbox in
			inbox.append(item)
		}

		guard shouldScheduleProcessing else { return }
		queue.async {
			self.processInbox()
		}
	}

	/// The top level inbox processing function that iterates through the pending items and performs their corresponding
	/// operation.
	private func processInbox() {
		inbox.withLock { $0.beginProcessing() }

		while let item = inbox.withLock({ $0.nextPendingItem() }) {
			switch item {
				case let .logMessage(record):
					do {
						try append(record: record)
					} catch {
						recordFailure(error)
					}
				case let .droppedRecordNotice(count):
					do {
						try append(droppedRecordNoticeFor: count)
					} catch {
						recordFailure(error)
					}
				case let .flush(continuation):
					processFlushRequest(continuation)
			}
		}
	}
}

// MARK: - File Operations

extension RotatingFileWriter {
	/// Appends the record to the currently active file.
	/// - Parameter record: The record to write to disk.
	private func append(record: String) throws {
		let record = encodeAndTruncateIfNeeded(record)

		try openActiveFileIfNeeded()
		try rotateActiveFileIfNeeded(forAppendingByteCount: record.count)

		try fileState.withLock { fileState in
			guard let fileClient = fileState.fileClient else {
				assertionFailure("Active file should be open before appending")
				return
			}
			_ = try fileClient.seekToEnd()
			try fileClient.write(data: record)
		}
	}

	/// Appends a line to the log file indicating that some number of records were dropped, perhaps due to overflowing
	/// the queue.
	/// - Parameter count: The number of log records
	private func append(droppedRecordNoticeFor count: Int) throws {
		// TODO: Implement dropped record logging
	}

	/// Encodes a record as UTF-8, truncating it when its encoded byte count exceeds the maximum file size.
	///
	/// Truncation preserves Unicode scalar boundaries and reserves space for a visible truncation marker.
	///
	/// - Parameter record: The record to encode and truncate
	/// - Returns: The record's UTF-8 data, truncated only if necessary
	private func encodeAndTruncateIfNeeded(_ record: String) -> Data {
		let recordBytes = Data(record.utf8)
		guard recordBytes.count > configuration.maximumFileSize else {
			return recordBytes
		}

		let truncationMarkerBytes = Data("… [truncated]".utf8)
		let maximumPrefixByteCount = configuration.maximumFileSize - truncationMarkerBytes.count

		// We take a best guess at where to end the prefix by slicing exactly at the maximum byte count. This is
		// imperfect, however, because we may be splitting a multi-byte unicode character. We repair this scenario
		// below.
		var prefixEndIndex = maximumPrefixByteCount

		// UTF-8 continuation bytes have the bit pattern 10xxxxxx. If the proposed boundary falls within the UTF-8
		// encoding of a Unicode scalar (i.e. it splits a multi-byte character down the middle) then we should back it
		// up to scalar's leading byte which will _not_ have that same 10xxxxxx bit pattern.
		//
		// See Unicode Standard section 3.9, Table 3-6.
		// <https://www.unicode.org/versions/Unicode17.0.0/core-spec/chapter-3/#G27288>

		func isUnicodeContinuationByte(_ byte: UInt8) -> Bool {
			byte & 0b1100_0000 == 0b1000_0000
		}
		while isUnicodeContinuationByte(recordBytes[prefixEndIndex]) {
			prefixEndIndex -= 1
		}

		var truncatedRecord = Data(recordBytes[0 ..< prefixEndIndex])
		truncatedRecord.append(contentsOf: truncationMarkerBytes)
		return truncatedRecord
	}

	/// Opens a file for appending if no active file is yet open.
	private func openActiveFileIfNeeded() throws {
		try fileState.withLock { fileState in
			guard fileState.fileClient == nil else { return }

			try fileSystemClient.createDirectory(url: activeFileURL.deletingLastPathComponent())
			if !fileSystemClient.fileExists(url: activeFileURL) {
				try fileSystemClient.createFile(url: activeFileURL, contents: nil)
			}
			fileState.fileClient = try fileSystemClient.openFileForWriting(url: activeFileURL)
		}
	}

	/// Rotates the active file if needed.
	///
	/// Rotation is "needed" when appending a record of the indicated count would cause the log file to exceed its
	/// maximum capacity.
	/// - Parameter byteCount: The number of bytes we intend to append to the log file.
	private func rotateActiveFileIfNeeded(forAppendingByteCount byteCount: Int) throws {
		try fileState.withLock { fileState in
			guard let fileClient = fileState.fileClient else {
				assertionFailure("Active file should be open before checking whether to rotate")
				return
			}

			let activeFileSize = try fileClient.seekToEnd()
			let maximumFileSize = UInt64(configuration.maximumFileSize)
			let unsignedByteCount = UInt64(byteCount)

			// Rotation isn't needed if appending the record would keep the file within limits
			let (resultingFileSize, additionOverflowed) = activeFileSize.addingReportingOverflow(unsignedByteCount)
			let appendingWouldExceedCapacity = additionOverflowed || resultingFileSize > maximumFileSize
			guard appendingWouldExceedCapacity else {
				return
			}

			// Rotation comes in N steps.

			// 1. Ensure the file is written to disk
			//
			// Once written to disk, the we no longer need the file handle and can discard it.
			try fileClient.synchronize()
			try fileClient.close()
			fileState.fileClient = nil

			// 2. Delete the oldest archive file, and rotate the remaining ones.
			//
			// We walk the files in reverse chronological order so that we don't accidentally overwrite anything. In
			// other words, if there are 3 total archive files, to rotate them we would do the following in order:
			// 		a. Remove archive 3
			// 		b. Rename archive 2 to archive 3
			// 		c. Rename archive 1 to archive 2
			let oldestArchiveFileURL = archiveFileURL(
				forAgeCounter: configuration.maximumArchiveCount,
			)
			if fileSystemClient.fileExists(oldestArchiveFileURL) {
				try fileSystemClient.removeItem(oldestArchiveFileURL)
			}
			for ageCounter in stride(
				from: configuration.maximumArchiveCount - 1,
				through: 1,
				by: -1,
			) {
				let sourceURL = archiveFileURL(forAgeCounter: ageCounter)
				guard fileSystemClient.fileExists(sourceURL) else { continue }

				let destinationURL = archiveFileURL(forAgeCounter: ageCounter + 1)
				try fileSystemClient.moveItem(sourceURL, destinationURL)
			}

			// 3. Rename the active file to archive it.
			//
			// Archives have the format: "\(originalFileName).\(ageCounter).\(originalExtension)" where `ageCounter`
			// is an incrementing counter beginning at 1 and going to N, where N is the max number of archive files.
			// The archive with ageCounter=1 is the newest archive, and where the active file gets rotated to.
			let newestArchiveFileURL = archiveFileURL(forAgeCounter: 1)
			try fileSystemClient.moveItem(activeFileURL, newestArchiveFileURL)

			// 4. Create a new active file, and retain the handle to it.
			try fileSystemClient.createFile(activeFileURL, nil)
			fileState.fileClient = try fileSystemClient.openFileForWriting(activeFileURL)
		}
	}

	/// Calculates the URL for an archive file with a given age counter.
	///
	/// The names of archive files have a simple, predictable format. Given an active file named "logs.txt" and an age
	/// counter of "1", the archive file will be named "logs.1.txt". Age counters identify archived logs in reverse
	/// chronological order. That is, "logs.1.txt" contains the records that immediately precede the active "logs.txt".
	/// Similarly, "logs.2.txt" contains records that immediately precede "logs.1.txt" and so on.
	///
	/// - Parameter ageCounter: The age counter associated with the archive file
	/// - Returns: A URL for the archive file
	private func archiveFileURL(forAgeCounter ageCounter: Int) -> URL {
		let archiveFileName = [
			activeFileURL.deletingPathExtension().lastPathComponent,
			String(ageCounter),
			activeFileURL.pathExtension,
		]
		.filter { !$0.isEmpty }
		.joined(separator: ".")

		return activeFileURL
			.deletingLastPathComponent()
			.appending(path: archiveFileName)
	}

	/// Flushes all pending writes to disk via the file handle, ensuring all records are persisted.
	/// - Parameter continuation: The continuation to resume when the synchronize operation is done.
	private func processFlushRequest(_ continuation: CheckedContinuation<Void, any Error>) {
		do {
			try fileState.withLock { fileState in
				try fileState.fileClient?.synchronize()
			}
			continuation.resume()
		} catch {
			continuation.resume(throwing: error)
		}
	}

	private func recordFailure(_ error: any Error) {}
}

// MARK: - Supporting Types

extension RotatingFileWriter {
	struct Configuration {
		/// We need a minimum file size that's at least as large as the smallest token we record in the file, which is
		/// our truncation marker  "… [truncated]"
		static let minimumFileSize = 16

		static let live = Configuration(
			maximumFileSize: 4 * 1024 * 1024,
			maximumArchiveCount: 3,
		)

		init(maximumFileSize: Int, maximumArchiveCount: Int) {
			precondition(
				maximumFileSize >= Self.minimumFileSize,
				"Maximum file size must be at least \(Self.minimumFileSize) bytes",
			)
			precondition(
				maximumArchiveCount >= 1,
				"Maximum archive count must be at least 1",
			)

			self.maximumFileSize = maximumFileSize
			self.maximumArchiveCount = maximumArchiveCount
		}

		/// The maximum size any individual file is allowed to be, in number of bytes
		let maximumFileSize: Int

		/// The maximum number of archived files. This does not include the file actively being written to.
		///
		/// For example, if the maximum archive count is 3, then at most the rotating file writer will govern 4 total
		/// files: the 3 archive files and the one active file.
		let maximumArchiveCount: Int
	}

	private struct Inbox {
		var pendingItems: Deque<PendingItem> = []
		var processingState = ProcessingState.idle

		/// Appends an item to the pendingItems queue and marks the inbox as scheduled for processing
		/// - Parameter item: The item to queue up
		/// - Returns: True if a new processing job should be started. False otherwise.
		mutating func append(_ item: PendingItem) -> Bool {
			pendingItems.append(item)

			guard processingState == .idle else { return false }
			processingState = .scheduled
			return true
		}

		/// Marks the inbox as being actively processed.
		mutating func beginProcessing() {
			assert(processingState == .scheduled)
			processingState = .processing
		}

		/// Retrieves the next item from the queue. If the queue is empty, that means processing is done and we mark the
		/// inbox as idle.
		/// - Returns: The next item to process from the queue, if the queue is non-empty. Nil otherwise.
		mutating func nextPendingItem() -> PendingItem? {
			assert(processingState == .processing)

			guard !pendingItems.isEmpty else {
				processingState = .idle
				return nil
			}

			return pendingItems.removeFirst()
		}
	}

	/// Enumerates the possible states the inbox can be in with respect to processing.
	///
	/// Lifecycle: idle → scheduled → processing → idle.
	///
	/// Whenever the mutex is released, a nonempty inbox must have processing scheduled or active.
	private enum ProcessingState {
		case idle
		case scheduled
		case processing
	}

	/// Contains mutable state that the worker needs in order to write to disk.
	struct FileState {
		var fileClient: WritableFileClient? = nil
	}

	/// An enumeration of the various items that can be enqueued in the inbox
	private enum PendingItem {
		/// A log message to be written to disk
		case logMessage(String)
		/// An indicator saying a certain number of records were dropped.
		case droppedRecordNotice(Int)
		/// A request to flush the current file handle to disk
		case flush(CheckedContinuation<Void, any Error>)
	}
}
