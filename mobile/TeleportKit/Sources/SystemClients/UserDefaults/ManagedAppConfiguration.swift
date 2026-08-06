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

/// These are values managed by MDM (e.g. Jamf, Intune, etc.)
///
/// These values are non-nil if and only if both of the following are true:
///
/// 1. The device is managed by MDM
/// 2. The MDM configuration supplies the value with the indicated key
enum ManagedAppConfiguration {
	static var serialNumber: String? {
		value(for: "serialNumber")
	}
}

// MARK: - User Defaults

extension ManagedAppConfiguration {
	private static let managedConfigurationDictionaryKey = "com.apple.configuration.managed"
	private static func value<T>(for key: String) -> T? {
		UserDefaults.standard.dictionary(forKey: managedConfigurationDictionaryKey)?[key] as? T
	}
}
