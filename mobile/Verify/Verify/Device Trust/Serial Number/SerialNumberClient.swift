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
import Foundation
import Sharing

@DependencyClient
struct SerialNumberClient {
	var getDeviceSerialNumber: @Sendable () throws(SerialNumberError) -> String = { "" }
}

extension SerialNumberClient {
	static let liveValue = SerialNumberClient(
		getDeviceSerialNumber: { () throws(SerialNumberError) -> String in
			#if DEBUG
				@Shared(.debugStorage(.debugSerialNumber))
				var debugSerialNumber: String? = nil

				guard let debugSerialNumber else {
					let newDebugSerialNumber = FakeSerialNumberGenerator.generate()
					$debugSerialNumber.withLock { $0 = newDebugSerialNumber }
					return newDebugSerialNumber
				}
				return debugSerialNumber
			#else
				guard let serialNumber = ManagedAppConfiguration.serialNumber else {
					throw .missingSerialNumber
				}
				return serialNumber
			#endif
		},
	)
}

// MARK: - Supporting Types

enum SerialNumberError: Error {
	case missingSerialNumber
}
