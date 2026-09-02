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

@testable import LogBackends
import Logging
import Testing

struct LoggerMetadataSensitivityTests {
	@Test
	func `sensitivity interpolation preserves the value and attaches the attribute`() {
		let metadata: Logger.Metadata = [
			"token": "\("secret", sensitivity: .sensitive)",
		]

		#expect(metadata["token"]?.description == "secret")
		#expect(metadata["token"]?.attributes[Logger.MetadataSensitivity.self] == .sensitive)
	}

	@Test
	func `formatting redacts only sensitive metadata values`() {
		let metadata: Logger.Metadata = [
			"operation": "enroll",
			"token": "\("secret", sensitivity: .sensitive)",
		]

		let formattedMetadata = metadata.formatted

		#expect(formattedMetadata.contains("🔸 operation=enroll"))
		#expect(formattedMetadata.contains("🔸 token=<redacted>"))
		#expect(!formattedMetadata.contains("secret"))
	}
}
