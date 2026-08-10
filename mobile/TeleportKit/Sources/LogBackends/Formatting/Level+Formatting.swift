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

import Logging

extension Logger.Level {
	/// Formats the log level in a way that allows it to be easily distinguished in a console or a log file, using both
	/// colors and text.
	var formatted: String {
		"\(indicator) [\(description.uppercased())]"
	}

	private static let maxLevelStringLength = Logger.Level.allCases
		.map(\.description.count)
		.max() ?? 10 // Some reasonable default

	private var indicator: String {
		switch self {
			case .trace:
				"⚪️"
			case .debug:
				"🔵"
			case .info, .notice:
				"🟢"
			case .warning:
				"🟡"
			case .error, .critical:
				"🔴"
		}
	}
}
