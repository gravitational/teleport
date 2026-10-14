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

import CustomDump
import Foundation
import Testing
@testable import Verify

@MainActor
struct DeepLinkTests {
	@Test
	func `valid enrollment link should parse hostname, port, and token`() throws {
		let expected = DeepLink.enrollMobileDevice(
			EnrollMobileDeviceDeepLink(
				hostname: "my.cluster.com",
				port: 443,
				enrollPairingToken: "abc123",
			),
		)
		let got = try DeepLink(from: #require(
			URL(string: "teleport://my.cluster.com:443/enroll_mobile_device?enroll_pairing_token=abc123"),
		))

		expectNoDifference(expected, got)
	}

	@Test
	func `valid enrollment link should decode percent encoded token`() throws {
		let expected = DeepLink.enrollMobileDevice(
			EnrollMobileDeviceDeepLink(
				hostname: "my.cluster.com",
				port: 443,
				enrollPairingToken: "abc/def==",
			),
		)
		let got = try DeepLink(from: #require(
			URL(string: "teleport://my.cluster.com:443/enroll_mobile_device?enroll_pairing_token=abc%2Fdef%3D%3D"),
		))

		expectNoDifference(expected, got)
	}

	@Test
	func `valid authentication link should use the default HTTPS port`() throws {
		let expected = DeepLink.authenticateDevice(
			AuthenticateDeviceDeepLink(
				hostname: "my.cluster.com",
				port: Cluster.defaultPort,
				webTokenID: "token-id",
				webToken: "token-value",
				redirectURI: nil,
			),
		)
		let got = try DeepLink(from: #require(
			URL(string: "teleport://my.cluster.com/authenticate_web_device?id=token-id&token=token-value"),
		))

		expectNoDifference(expected, got)
	}

	@Test
	func `valid authentication link should parse an explicit port`() throws {
		let expected = DeepLink.authenticateDevice(
			AuthenticateDeviceDeepLink(
				hostname: "my.cluster.com",
				port: 3080,
				webTokenID: "token-id",
				webToken: "token-value",
				redirectURI: nil,
			),
		)
		let got = try DeepLink(from: #require(
			URL(string: "teleport://my.cluster.com:3080/authenticate_web_device?id=token-id&token=token-value"),
		))

		expectNoDifference(expected, got)
	}

	@Test
	func `authentication link username should not become an authentication input`() throws {
		let expected = DeepLink.authenticateDevice(
			AuthenticateDeviceDeepLink(
				hostname: "my.cluster.com",
				port: Cluster.defaultPort,
				webTokenID: "token-id",
				webToken: "token-value",
				redirectURI: nil,
			),
		)
		let got = try DeepLink(from: #require(
			URL(string: "teleport://alice@my.cluster.com/authenticate_web_device?id=token-id&token=token-value"),
		))

		expectNoDifference(expected, got)
	}

	@Test
	func `authentication link should percent decode query values`() throws {
		let expected = DeepLink.authenticateDevice(
			AuthenticateDeviceDeepLink(
				hostname: "my.cluster.com",
				port: 3030,
				webTokenID: "token/id=123",
				webToken: "secret+/=",
				redirectURI: "https://my.cluster.com:3030/web/resources?sort=name%3Adesc&pinnedOnly=false",
			),
		)
		let got = try DeepLink(from: #require(URL(
			string:
			"teleport://my.cluster.com:3030/authenticate_web_device?id=token%2Fid%3D123&token=secret%2B%2F%3D&redirect_uri=https%3A%2F%2Fmy.cluster.com%3A3030%2Fweb%2Fresources%3Fsort%3Dname%253Adesc%26pinnedOnly%3Dfalse",
		)))

		expectNoDifference(expected, got)
	}

	@Test
	func `authentication link without token ID should throw an error`() throws {
		#expect(throws: DeepLinkParseError.missingPart("id")) {
			try DeepLink(from: #require(URL(
				string:
				"teleport://my.cluster.com/authenticate_web_device?token=token-value",
			)))
		}
	}

	@Test
	func `authentication link with empty token ID should throw an error`() throws {
		#expect(throws: DeepLinkParseError.missingPart("id")) {
			try DeepLink(from: #require(URL(
				string:
				"teleport://my.cluster.com/authenticate_web_device?id=&token=token-value",
			)))
		}
	}

	@Test
	func `authentication link without token should throw an error`() throws {
		#expect(throws: DeepLinkParseError.missingPart("token")) {
			try DeepLink(from: #require(URL(
				string:
				"teleport://my.cluster.com/authenticate_web_device?id=token-id",
			)))
		}
	}

	@Test
	func `authentication link with empty token should throw an error`() throws {
		#expect(throws: DeepLinkParseError.missingPart("token")) {
			try DeepLink(from: #require(URL(
				string:
				"teleport://my.cluster.com/authenticate_web_device?id=token-id&token=",
			)))
		}
	}

	@Test
	func `enrollment link without hostname should throw an error`() throws {
		#expect(throws: DeepLinkParseError.missingPart("hostname")) {
			try DeepLink(from: #require(URL(string: "teleport:/enroll_mobile_device?enroll_pairing_token=abc123")))
		}
	}

	@Test
	func `enrollment link without token should throw an error`() throws {
		#expect(throws: DeepLinkParseError.missingPart("enroll_pairing_token")) {
			try DeepLink(from: #require(URL(string: "teleport://my.cluster.com/enroll_mobile_device")))
		}
	}

	@Test
	func `unsupported path should throw before checking path-specific parts`() throws {
		#expect(throws: DeepLinkParseError.unsupportedPath) {
			try DeepLink(from: #require(URL(string: "teleport://my.cluster.com/hello")))
		}
	}
}
