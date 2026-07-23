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

#if DEBUG
	import Foundation

	/// A small utility for generating a fake serial number, mostly to support
	struct FakeSerialNumberGenerator {
		private static let base: String = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

		static func generate() -> String {
			func chunk() -> String {
				(0 ..< 4)
					.compactMap { _ in Self.base.randomElement() }
					.reduce(into: "") { $0.append($1) }
			}
			return "\(chunk())-\(chunk())"
		}
	}
#endif
