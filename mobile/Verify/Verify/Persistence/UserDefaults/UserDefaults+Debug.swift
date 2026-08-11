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
	import Sharing

	extension UserDefaults {
		static let debugSuiteName = "com.gravitational.teleport.verify.debug"

		/// A UserDefaults suite that's only available in debug builds.
		///
		/// This is marked as `nonisolated(unsafe)` because UserDefaults isn't marked Sendable, but it _is_ thread safe.
		/// See <https://developer.apple.com/documentation/foundation/userdefaults>
		nonisolated(unsafe) static let debug: UserDefaults = {
			guard let store = UserDefaults(suiteName: debugSuiteName) else {
				preconditionFailure("Could not instantiate debug user defaults.")
			}
			return store
		}()
	}

	// MARK: - Keys

	/// Type-safe keys for use when querying the debug user defaults
	enum DebugUserDefaultsKey: String {
		case debugSerialNumber
	}

	/// A small helper function to access the debug store via the Sharing library
	extension SharedReaderKey where Self == AppStorageKey<String?> {
		static func debugStorage(
			_ key: DebugUserDefaultsKey,
		) -> Self {
			.appStorage(key.rawValue, store: .debug)
		}
	}

#endif
