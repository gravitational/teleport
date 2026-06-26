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

import OSLog

extension Logger {
	/// The subsystem that almost all loggers in this target should use.
	static let defaultSubsystem = "com.gravitational.Verify"

	/// Creates a logger for a specific type.
	static func forType<T>(_ type: T.Type) -> Logger {
		Logger(subsystem: defaultSubsystem, category: String(describing: type))
	}

	/// Creates a logger for the file in which it is declared.
	static func forFile(_ fileID: StaticString = #fileID) -> Logger {
		Logger(subsystem: defaultSubsystem, category: "\(fileID)")
	}
}
