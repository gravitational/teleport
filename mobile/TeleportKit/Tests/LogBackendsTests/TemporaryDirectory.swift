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

import Foundation
import Testing

func withTemporaryDirectory<Result>(
	_ operation: (URL) async throws -> Result
) async throws -> Result {
	let directoryURL = FileManager.default.temporaryDirectory
		.appending(path: "TeleportKitTests-\(UUID())", directoryHint: .isDirectory)
	try FileManager.default.createDirectory(at: directoryURL, withIntermediateDirectories: true)

	defer {
		do {
			try FileManager.default.removeItem(at: directoryURL)
		} catch {
			Issue.record("Failed to remove temporary test directory at \(directoryURL.path): \(error)")
		}
	}

	return try await operation(directoryURL)
}
