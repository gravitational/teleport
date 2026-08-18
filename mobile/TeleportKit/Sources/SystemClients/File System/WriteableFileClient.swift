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

/// An interface to an open writable file.
///
/// This interface exists so that code which uses a writeable FileHandle can be tested. For documentation on what the
/// individual functions do, refer to the correspondingly named functions on `FileHandle`.
@DependencyClient
public struct WritableFileClient: Sendable {
	public var seekToEnd: @Sendable () throws -> UInt64
	public var write: @Sendable (_ data: Data) throws -> Void
	public var synchronize: @Sendable () throws -> Void
	public var close: @Sendable () throws -> Void
}

extension WritableFileClient {
	static func liveValue(_ fileHandle: FileHandle) -> WritableFileClient {
		WritableFileClient(
			seekToEnd: {
				try fileHandle.seekToEnd()
			},
			write: { data in
				try fileHandle.write(contentsOf: data)
			},
			synchronize: {
				try fileHandle.synchronize()
			},
			close: {
				try fileHandle.close()
			},
		)
	}
}
