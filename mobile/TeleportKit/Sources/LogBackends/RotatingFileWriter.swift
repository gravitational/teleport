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

public import Foundation
import Synchronization

public final class RotatingFileWriter: Sendable {
	private static let maximumFileSize: UInt64 = 4 * 1024 * 1024
	private static let archiveTimestampFormat = Date.ISO8601FormatStyle(
		dateSeparator: .omitted,
		dateTimeSeparator: .standard,
		timeSeparator: .omitted,
		timeZoneSeparator: .omitted,
		includingFractionalSeconds: true,
		timeZone: .gmt,
	)

	let queue = DispatchQueue(label: "rotating-file-writer")
	let state = Mutex(State())

	private let fileURL: URL

	public init(fileURL: URL) {
		self.fileURL = fileURL
	}

	func enqueue(record: Data) {
		state.withLock { state in
			state.queuedRecords.append(record)
		}
	}
}

// MARK: - RotatingFileWriter.State

extension RotatingFileWriter {
	struct State {
		var queuedRecords: [Data] = []
		var fileHandle: FileHandle? = nil
	}
}

// MARK: - Private Helpers

extension RotatingFileWriter {
	private func ensureFileAvailable(_ fileHandle: inout FileHandle?) throws {
		if fileHandle == nil {
			fileHandle = try openBaseFile()
		}

		guard let currentFileHandle = fileHandle else {
			return
		}

		let fileSize = try currentFileHandle.seekToEnd()
		guard fileSize > Self.maximumFileSize else {
			return
		}

		fileHandle = nil
		try currentFileHandle.close()
		try FileManager.default.moveItem(
			at: fileURL,
			to: archiveURL(timestamp: .now)
		)
		fileHandle = try openBaseFile()
	}

	private func openBaseFile() throws -> FileHandle {
		let fileManager = FileManager.default
		let directoryURL = fileURL.deletingLastPathComponent()

		try fileManager.createDirectory(
			at: directoryURL,
			withIntermediateDirectories: true
		)

		if !fileManager.fileExists(atPath: fileURL.path) {
			fileManager.createFile(atPath: fileURL.path, contents: nil)
		}

		let fileHandle = try FileHandle(forWritingTo: fileURL)
		try fileHandle.seekToEnd()
		return fileHandle
	}

	private func archiveURL(timestamp: Date) -> URL {
		let fileExtension = fileURL.pathExtension
		let baseName = fileURL.deletingPathExtension().lastPathComponent
		let timestamp = timestamp.formatted(Self.archiveTimestampFormat)
		var archiveURL = fileURL
			.deletingLastPathComponent()
			.appendingPathComponent("\(baseName)_\(timestamp)")

		if !fileExtension.isEmpty {
			archiveURL.appendPathExtension(fileExtension)
		}

		return archiveURL
	}
}

/*

 Idk, I can see the steady trot of progression and improvement. From X/Y giving us 3D pokemon models, to Sword/Shield trialing "open world" with the wild areas, to S/V bringing true open world. Unless I'm missing something, these games have all been done by the same developer, and lemme tell ya, it's hard to evolve a beloved franchise. I used to work for Apple as a software engineer and we received a lot of the same criticisms you're mentioning.
 We had "infinite resources" and yet the software still had major bugs. Something we learned early on in college when developing software is the concept of the "mythical man month" which basically says that 1 person working for 100 months is not the same as 100 people working for 1 month. Software, games, and any creative endeavor don't really work that way.

 I guess all I'm saying is that I see them trying and I have faith that the games will continue to get better. I don't buy the idea that they don't care.
 */
