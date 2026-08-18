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

import DependenciesMacros
public import Foundation

/// An interface to the filesystem operations used by TeleportKit.
///
/// As TeleportKit needs more filesystem capabilities, it's perfectly natural for this client to grow over time.
@DependencyClient
public struct FileSystemClient: Sendable {
	public var createDirectory: @Sendable (_ url: URL) throws -> Void
	public var fileExists: @Sendable (_ url: URL) -> Bool = { _ in false }
	public var createFile: @Sendable (_ url: URL, _ contents: Data?) throws -> Void
	public var openFileForWriting: @Sendable (_ url: URL) throws -> WritableFileClient
	public var moveItem: @Sendable (_ sourceURL: URL, _ destinationURL: URL) throws -> Void
	public var removeItem: @Sendable (_ url: URL) throws -> Void
}

public enum FileSystemClientError: Error {
	/// File creation failure doesn't surface an error, but rather returns a `Bool`, so we have our own error
	case couldNotCreateFile(path: String)
}

extension FileSystemClient {
	public static let liveValue = FileSystemClient(
		createDirectory: { url in
			try FileManager.default.createDirectory(
				at: url,
				withIntermediateDirectories: true,
			)
		},
		fileExists: { url in
			FileManager.default.fileExists(atPath: url.path)
		},
		createFile: { url, contents in
			guard FileManager.default.createFile(atPath: url.path, contents: contents) else {
				throw FileSystemClientError.couldNotCreateFile(path: url.path(percentEncoded: false))
			}
		},
		openFileForWriting: { url in
			try WritableFileClient.liveValue(FileHandle(forWritingTo: url))
		},
		moveItem: { sourceURL, destinationURL in
			try FileManager.default.moveItem(at: sourceURL, to: destinationURL)
		},
		removeItem: { url in
			try FileManager.default.removeItem(at: url)
		},
	)
}
