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
	private let inbox: Mutex<Inbox>

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
		self.inbox = .init(Inbox(maximumQueuedBytes: configuration.maximumQueuedBytes))
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
		let maximumRetainedByteCount = min(configuration.maximumFileSize, configuration.maximumQueuedBytes)
		let logMessage = truncateIfNeeded(logMessage, toMaximumByteCount: maximumRetainedByteCount)
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
		let record = truncateIfNeeded(record, toMaximumByteCount: configuration.maximumFileSize)
		try writeToActiveFile(Data(record.utf8))
	}

	/// Appends a line to the log file indicating that some number of records were dropped, perhaps due to overflowing
	/// the queue.
	/// - Parameter droppedRecordCount: The number of log records that were dropped
	private func append(droppedRecordNoticeFor droppedRecordCount: Int) throws {
		let droppedRecordMessage = Data(Self.droppedRecordDescription(for: droppedRecordCount).utf8)
		try writeToActiveFile(droppedRecordMessage)
	}

	private static func droppedRecordDescription(for count: Int) -> String {
		"🗑️ Dropped \(count) record\(count == 1 ? "" : "s")\n"
	}

	/// Truncates a record when its UTF-8 byte count exceeds the supplied limit.
	///
	/// Truncation preserves Unicode scalar boundaries and reserves space for a visible truncation marker.
	///
	/// - Parameters:
	///   - record: The record to truncate.
	///   - maximumByteCount: The maximum number of UTF-8 bytes the returned record may contain.
	/// - Returns: The record, truncated only if necessary.
	private func truncateIfNeeded(_ record: String, toMaximumByteCount maximumByteCount: Int) -> String {
		let recordBytes = record.utf8

		// We take a best guess at where to end the prefix by slicing exactly at the maximum byte count, returning the
		// original record if it wasn't large enough to need truncation.
		//
		// This is imperfect, however, because we may be splitting a multi-byte unicode character. We repair this
		// scenario below.
		let cutoffIndex = recordBytes.index(
			recordBytes.startIndex,
			offsetBy: maximumByteCount,
			limitedBy: recordBytes.endIndex,
		)
		guard cutoffIndex != recordBytes.endIndex, var prefixEndIndex = cutoffIndex else { return record }

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
			recordBytes.formIndex(before: &prefixEndIndex)
		}

		var truncatedRecord = Data(recordBytes[record.startIndex ..< prefixEndIndex])
		let truncationMarker = "… [truncated]"
		truncatedRecord.append(contentsOf: Data(truncationMarker.utf8))
		return String(decoding: truncatedRecord, as: UTF8.self)
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
			// Once written to disk, we no longer need the file handle and can discard it.
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
				// We attempt to surface earlier filesystem errors at `flush` callsites, so we keep track of
				// `firstReportedError` during normal operation, and throw it here if it's non-nil. However, if file
				// synchronization fails, that supersedes the previously reported error. Regardless, we always clear
				// `firstReportedError` since either it or the synchronization error must have been thrown.
				defer { fileState.firstReportedError = nil }
				try fileState.fileClient?.synchronize()
				if let firstReportedError = fileState.firstReportedError {
					throw firstReportedError
				}
			}
			continuation.resume()
		} catch {
			continuation.resume(throwing: error)
		}
	}

	private func recordFailure(_ error: any Error) {
		fileState.withLock { fileState in
			guard fileState.firstReportedError == nil else { return }
			fileState.firstReportedError = error
		}
	}

	private func writeToActiveFile(_ record: Data) throws {
		try openActiveFileIfNeeded()
		try rotateActiveFileIfNeeded(forAppendingByteCount: record.count)

		try fileState.withLock { fileState in
			try write(data: record, to: &fileState)
		}
	}

	private func write(data: Data, to fileState: inout FileState) throws {
		guard let fileClient = fileState.fileClient else {
			assertionFailure("Active file should be open before appending")
			return
		}
		_ = try fileClient.seekToEnd()
		try fileClient.write(data: data)
	}
}

// MARK: - Supporting Types

extension RotatingFileWriter {
	struct Configuration {
		/// The minimum byte limit that leaves one byte of capacity beyond the truncation marker.
		static let minimumByteLimit = 16

		static let live = Configuration(
			maximumFileSize: 4 * 1024 * 1024,
			maximumArchiveCount: 3,
			maximumQueuedBytes: 4 * 1024 * 1024,
		)

		init(maximumFileSize: Int, maximumArchiveCount: Int, maximumQueuedBytes: Int) {
			precondition(
				maximumFileSize >= Self.minimumByteLimit,
				"Maximum file size must be at least \(Self.minimumByteLimit) bytes",
			)
			precondition(
				maximumArchiveCount >= 1,
				"Maximum archive count must be at least 1",
			)
			precondition(
				maximumQueuedBytes >= Self.minimumByteLimit,
				"Maximum queued bytes must be at least \(Self.minimumByteLimit) bytes",
			)

			self.maximumFileSize = maximumFileSize
			self.maximumArchiveCount = maximumArchiveCount
			self.maximumQueuedBytes = maximumQueuedBytes
		}

		/// The maximum size any individual file is allowed to be, in number of bytes
		let maximumFileSize: Int

		/// The maximum number of archived files. This does not include the file actively being written to.
		///
		/// For example, if the maximum archive count is 3, then at most the rotating file writer will govern 4 total
		/// files: the 3 archive files and the one active file.
		let maximumArchiveCount: Int

		/// The maximum combined UTF-8 byte count of log messages waiting in the inbox.
		///
		/// - Note: This limit does not apply to flush requests and dropped record notices. It only counts bytes
		/// actively enqueued by a caller.
		let maximumQueuedBytes: Int
	}

	/// Keeps track of the pending items in the queue.
	///
	/// This type is _not_ thread-safe and must be used with some synchronization mechanism like a Mutex.
	struct Inbox {
		/// The configured byte limit used to determine when appending a log message must evict older log messages.
		private let maximumQueuedBytes: Int

		/// A FIFO queue of items waiting to be processed.
		///
		/// At most one dropped-record notice is pending, and when present it is always first. Notices are prepended,
		/// log
		/// messages and flush requests are appended, and processing removes items from the front. This means repeated
		/// queue overflows can coalesce notices by inspecting only the first pending item.
		private var pendingItems: Deque<PendingItem> = []

		/// The number of bytes in the queue, manually mantained so as to preserve a near-constant-time enqueue/dequeue.
		private(set) var pendingLogMessageByteCount: Int = .zero

		/// The inbox worker's current lifecycle state, used to ensure a nonempty inbox is either scheduled or being
		/// processed.
		private var processingState = ProcessingState.idle

		init(maximumQueuedBytes: Int) {
			self.maximumQueuedBytes = maximumQueuedBytes
		}

		/// Appends an item to the pendingItems queue and marks the inbox as scheduled for processing if needed
		/// - Parameter item: The item to queue up
		/// - Returns: True if a new processing job should be started. False otherwise.
		mutating func append(_ item: PendingItem) -> Bool {
			if case .logMessage = item {
				ensureQueueIsUnderCapacity(forAppending: item)
			}
			pendingLogMessageByteCount += item.logMessageByteCount
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

			let nextItem = pendingItems.removeFirst()
			pendingLogMessageByteCount -= nextItem.logMessageByteCount
			return nextItem
		}

		/// Checks to see if appending this item to the queue would make it too big, and if it is, drops the oldest
		/// records (i.e. removes records from the front of the queue) until enough room is made.
		///
		/// Note that this implementation may drop records that are behind a pending flush request. Suppose, for
		/// example, that the `pendingItems` queue looks like this:
		///
		/// ```swift
		/// [ // front of the queue
		///		.droppedRecordNotice(2),
		///		.flush
		///		.logMessage("Hello, there.")
		///		.logMessage("So uncivilized.")
		/// ] // back of the queue
		/// ```
		///
		///	Suppose further that we need to drop the oldest message which reads "Hello, there.". When we drop it, it
		/// will be coalesced with the existing `.droppedRecordNotice`. The resulting queue would look like this:
		///
		/// ```swift
		/// [ // front of the queue
		///		.droppedRecordNotice(3),
		///		.flush
		///		.logMessage("So uncivilized.")
		/// ] // back of the queue
		/// ```
		///
		/// So even though the message would _not_ have been written to disk by the time the `.flush` item was
		/// processed, a record of it being dropped _is. This is the intended behavior.
		///
		/// - Parameters:
		///   - maxBytes: The maximum capacity of the queue as defined by some configuration
		///   - item: The item to add to the queue
		private mutating func ensureQueueIsUnderCapacity(forAppending item: PendingItem) {
			guard !pendingItems.isEmpty else { return }
			var droppedRecords = DroppedRecords()
			var cursor: Int = pendingItems.startIndex

			/// An inline loop condition became hard to read, so I'm using a local function instead
			func shouldDropRecords(_ droppedRecords: DroppedRecords, _ cursor: Int) -> Bool {
				guard cursor < pendingItems.count else { return false }
				let resultingByteCount = pendingLogMessageByteCount
					- droppedRecords.byteCount
					+ item.logMessageByteCount
				return resultingByteCount > maximumQueuedBytes
			}

			while shouldDropRecords(droppedRecords, cursor) {
				defer { pendingItems.formIndex(after: &cursor) }
				switch pendingItems[cursor] {
					case .logMessage:
						let pendingItemByteCount = pendingItems[cursor].logMessageByteCount
						if droppedRecords.subranges.isEmpty {
							droppedRecords.startNewSubrange(at: cursor, byteCount: pendingItemByteCount)
						} else {
							droppedRecords.incrementCurrentSubrange(byteCount: pendingItemByteCount)
						}
					case let .droppedRecordNotice(droppedRecordCount):
						droppedRecords.absorbPreviouslyDroppedRecords(count: droppedRecordCount)
					case .flush:
						while
							cursor < pendingItems.endIndex,
							case .flush = pendingItems[cursor]
						{
							pendingItems.formIndex(after: &cursor)
						}
						guard cursor < pendingItems.endIndex else { break }
						switch pendingItems[cursor] {
							case .logMessage:
								droppedRecords.startNewSubrange(
									at: cursor,
									byteCount: pendingItems[cursor].logMessageByteCount,
								)
							case let .droppedRecordNotice(droppedRecordCount):
								droppedRecords.absorbPreviouslyDroppedRecords(count: droppedRecordCount)
							case .flush:
								assertionFailure(
									"Reaching this line should only happen if we skipped past all consecutive flushes",
								)
						}
				}
			}

			// If we don't intend to drop any records, we can just bail because the queue is within maximum capacity.
			guard !droppedRecords.subranges.isEmpty else { return }

			// We iterate backwards over the subranges so that we don't clobber our indices as we replace the subranges
			// with dropped record notices, coalescing along the way.
			for subrange in droppedRecords.subranges.reversed() {
				pendingLogMessageByteCount -= subrange.byteCount
				pendingItems.replaceSubrange(
					subrange.startIndex ..< subrange.endIndex,
					with: [.droppedRecordNotice(subrange.droppedRecordCount)],
				)
				coalesceDroppedRecordNotice(at: subrange.startIndex)
			}
		}

		private mutating func coalesceDroppedRecordNotice(at cursor: Int) {
			guard case let .droppedRecordNotice(droppedRecordCount) = pendingItems[cursor] else {
				assertionFailure("This function should only be called when we know the item is a dropped record")
				return
			}

			// We'll check the left and right side of the cursor, coalescing any other dropped record notices we find.
			// Note that this implementation of coalescing is right associative, so we always drop the element on the
			// right and merge its contents to the left.

			var totalCount = droppedRecordCount
			var resultIndex = cursor

			// It's important that we check the right side first, because doing so preserves the indices of the left
			// side saving us from having to do some math and bounds checking.
			let rightIndex = pendingItems.index(after: cursor)
			if
				rightIndex < pendingItems.endIndex,
				case let .droppedRecordNotice(rightCount) = pendingItems[rightIndex]
			{
				totalCount += rightCount
				pendingItems.remove(at: rightIndex)
			}

			let leftIndex = pendingItems.index(before: cursor)
			if
				leftIndex >= pendingItems.startIndex,
				case let .droppedRecordNotice(leftCount) = pendingItems[leftIndex]
			{
				resultIndex = leftIndex
				totalCount += leftCount
				pendingItems.remove(at: cursor)
			}

			pendingItems[resultIndex] = .droppedRecordNotice(totalCount)
		}

		/// Keeps track of records we wish to drop, collecting subranges in the queue that are delimited by flush
		/// requests.
		///
		/// This type is not thread safe and is only designed to be used locally within the context of a synchronously
		/// executing function.
		private struct DroppedRecords {
			private(set) var subranges: [DroppedRecordSubrange] = []
			var byteCount: Int {
				subranges.reduce(0) { $0 + $1.byteCount }
			}

			mutating func startNewSubrange(at index: Int, byteCount: Int) {
				subranges.append(DroppedRecordSubrange(startIndex: index, endIndex: index + 1, byteCount: byteCount))
			}

			/// Updates the current subrange. No-ops if there no subranges have been started with
			/// `startNewSubrange(at:byteCount)`
			mutating func incrementCurrentSubrange(byteCount: Int) {
				guard let lastIndex = subranges.indices.last else { return }
				subranges[lastIndex].endIndex += 1
				subranges[lastIndex].byteCount += byteCount
			}

			mutating func absorbPreviouslyDroppedRecords(count: Int) {
				guard let lastIndex = subranges.indices.last else { return }
				subranges[lastIndex].previouslyDroppedRecordCount += count
			}
		}

		private struct DroppedRecordSubrange {
			var startIndex: Int
			var endIndex: Int
			var byteCount: Int
			var previouslyDroppedRecordCount: Int = 0

			var droppedRecordCount: Int {
				(endIndex - startIndex) + previouslyDroppedRecordCount
			}
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
		var firstReportedError: any Error? = nil
	}

	/// An enumeration of the various items that can be enqueued in the inbox
	enum PendingItem {
		/// A log message to be written to disk
		case logMessage(String)
		/// An indicator saying a certain number of records were dropped.
		case droppedRecordNotice(Int)
		/// A request to flush the current file handle to disk
		case flush(CheckedContinuation<Void, any Error>)

		/// If this pending item is a log message, returns the number of bytes in that message. Returns 0 otherwise.
		var logMessageByteCount: Int {
			if case let .logMessage(message) = self {
				message.utf8.count
			} else {
				.zero
			}
		}
	}
}

// MARK: - Testing Utilities

extension RotatingFileWriter {
	var pendingLogMessageByteCountForTesting: Int {
		inbox.withLock { $0.pendingLogMessageByteCount }
	}
}
