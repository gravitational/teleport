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
@_implementationOnly import Enroll

public struct EnrollClient {
    public var enroll: (_ proxyServer: String, _ pairingToken: String) async throws(Error) -> String
}

public enum EnrollError: Error {
    case clientCreationFailed
    case unknownError(any Error)
}

public extension EnrollClient {
    static var liveValue: Self {
        EnrollClient(
            enroll: { proxyServer, pairingToken throws(EnrollError) in
                guard let client = Enroll.EnrollClient(proxyServer, insecure: false) else {
                    throw .clientCreationFailed
                }
                do {
                    let token = try client.createMobileEnrollToken(
                        pairingToken,
                        deviceData: EnrollDeviceCollectedData()
                    )
                    return token.token
                } catch {
                    throw .unknownError(error)
                }

            }
        )
    }
}
