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
import Logging

extension Logger.Metadata {
	/// Formats the metadata in a way suitable for appending to the end of a log message.
	///
	/// All metadata is joined into key value pairs of the form `\(key)=\(value)` with each pair including a prefix that
	/// allows the metadata to be easily scanned/parsed by the human eye.
	var formatted: String {
		guard !isEmpty else { return "" }
		let metadataAsStrings = map { key, value in
			let prefix = if key.lowercased().localizedStandardContains("error") {
				"❌"
			} else {
				"🔸"
			}
			return "\(prefix) \(key)=\(value)"
		}
		return metadataAsStrings.joined(separator: " ")
	}
}
