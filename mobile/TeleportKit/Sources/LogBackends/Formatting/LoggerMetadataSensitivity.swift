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

// MARK: - Logger.MetadataSensitivity

extension Logger {
	/// An attribute you can attach to logger metadata indicating its sensitivity.
	///
	/// You can attach this attribute to metadata as follows:
	///
	/// ```swift
	/// let metadata: Logger.Metadata = [
	/// 	"token": "\(token, sensitivity: .sensitive)",
	/// ]
	/// ```
	public enum MetadataSensitivity: Int64, Logger.MetadataValueAttributes.Attribute, Sendable {
		/// An attribute indicating this piece of metadata is sensitive and should be redacted.
		case sensitive = 1
	}
}

extension Logger.MetadataValue.StringInterpolation {
	public mutating func appendInterpolation(
		_ value: some CustomStringConvertible & Sendable,
		sensitivity: Logger.MetadataSensitivity,
	) {
		appendInterpolation(
			value,
			attributes: {
				$0[Logger.MetadataSensitivity.self] = sensitivity
			},
		)
	}
}
