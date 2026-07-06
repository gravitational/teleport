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

import Dependencies
import DependenciesMacros
import Enroll
import Foundation

/// Handles requests around enrolling the current device in Device Trust
@DependencyClient
public struct EnrollClient: Sendable {
	/// Sends a request for an enrollment token
	public var requestEnrollmentToken: @Sendable (
		_ hostName: String,
		_ port: Int,
		_ pairingToken: String,
	) async throws -> String
}

public enum EnrollClientError: Error, Sendable {
	case clientCreationFailed
}


extension EnrollClient {
	public static let liveValue = EnrollClient(
		requestEnrollmentToken: { hostName, port, pairingToken in
			try await Task.detached(priority: .userInitiated) {
				let proxyServer = "\(hostName):\(port)"
				guard let client = Enroll.EnrollClient(proxyServer, insecure: false) else {
					throw EnrollClientError.clientCreationFailed
				}

				let token = try client.createMobileEnrollToken(
					pairingToken,
					deviceData: Enroll.EnrollDeviceCollectedData(),
				)
				return token.token
			}.value
		},
	)
}

