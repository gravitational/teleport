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

struct EnrollMobileDeviceDeepLink: Equatable {
	var hostname: String
	var port: Int
	var enrollPairingToken: String
}

extension EnrollMobileDeviceDeepLink {
	static let enrollPairingTokenKey = "enroll_pairing_token"

	init(from url: URL) throws(DeepLinkParseError) {
		guard let components = URLComponents(url: url, resolvingAgainstBaseURL: false) else {
			throw DeepLinkParseError.urlComponentsFailed
		}
		guard let hostname = components.host, hostname != "" else {
			throw DeepLinkParseError.missingPart("hostname")
		}
		guard let enrollPairingToken = components.nonEmptyQueryValue(named: Self.enrollPairingTokenKey) else {
			throw DeepLinkParseError.missingPart("enroll pairing token")
		}

		let defaultHTTPSPort = 443
		self.init(
			hostname: hostname,
			port: components.port ?? defaultHTTPSPort,
			enrollPairingToken: enrollPairingToken,
		)
	}
}

// MARK: - CustomDebugStringConvertible

extension EnrollMobileDeviceDeepLink: CustomDebugStringConvertible {
	var debugDescription: String {
		var components = URLComponents()
		components.scheme = "https"
		components.host = hostname
		components.port = port
		components.queryItems = [URLQueryItem(name: Self.enrollPairingTokenKey, value: enrollPairingToken)]
		return components.debugDescription
	}
}
