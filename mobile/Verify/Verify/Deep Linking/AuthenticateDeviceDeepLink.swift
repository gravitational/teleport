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
import LogBackends
import Logging

struct AuthenticateDeviceDeepLink: Equatable {
	var hostname: String
	var port: Int
	var webTokenID: String
	var webToken: String
	var redirectURI: String? = nil
}

extension AuthenticateDeviceDeepLink {
	// 			URL(string: "teleport://my.cluster.com:3080/authenticate_web_device?id=token-id&token=token-value"),
	static let idKey = "id"
	static let tokenKey = "token"
	static let redirectURIKey = "redirect_uri"

	init(from url: URL) throws(DeepLinkParseError) {
		guard let components = URLComponents(url: url, resolvingAgainstBaseURL: false) else {
			throw .urlComponentsFailed
		}
		guard let hostname = components.host, hostname != "" else {
			throw .missingPart("hostname")
		}
		guard let tokenID = components.nonEmptyQueryValue(named: Self.idKey) else {
			throw .missingPart(Self.idKey)
		}
		guard let token = components.nonEmptyQueryValue(named: Self.tokenKey) else {
			throw .missingPart(Self.tokenKey)
		}

		let defaultHTTPSPort = 443
		self.init(
			hostname: hostname,
			port: components.port ?? defaultHTTPSPort,
			webTokenID: tokenID,
			webToken: token,
			redirectURI: components.nonEmptyQueryValue(named: Self.redirectURIKey),
		)
	}
}

// MARK: - Logging

extension AuthenticateDeviceDeepLink {
	var logMetadata: Logger.Metadata {
		// Intentionally omit token from logger metadata for safety
		var metadata: Logger.Metadata = [
			"deepLink.type": "authenticateDevice",
			"deepLink.hostname": "\(hostname)",
			"deepLink.port": "\(port)",
			"deepLink.tokenID": "\(webTokenID, sensitivity: .sensitive)",
		]

		if let redirectURI {
			metadata["deepLink.redirectURI"] = "\(redirectURI, sensitivity: .sensitive)"
		}

		return metadata
	}
}
