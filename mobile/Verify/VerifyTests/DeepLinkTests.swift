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
	func `valid enrollment link should parse hostname and token`() throws {
		let expected = DeepLink.enrollMobileDevice(
			EnrollMobileDeviceDeepLink(
				hostname: "my.cluster.com",
				port: nil,
				enrollPairingToken: "abc123",
			),
		)
		let got = try DeepLink(from: #require(
			URL(string: "teleport://my.cluster.com/enroll_mobile_device?enroll_pairing_token=abc123"),
		))

		expectNoDifference(expected, got)
	}

	@Test
	func `valid enrollment link should parse explicit port`() throws {
		let expected = DeepLink.enrollMobileDevice(
			EnrollMobileDeviceDeepLink(
				hostname: "my.cluster.com",
				port: 3080,
				enrollPairingToken: "abc123",
			),
		)
		let got = try DeepLink(from: #require(
			URL(string: "teleport://my.cluster.com:3080/enroll_mobile_device?enroll_pairing_token=abc123"),
		))

		expectNoDifference(expected, got)
	}

	@Test
	func `valid enrollment link should decode percent encoded token`() throws {
		let expected = DeepLink.enrollMobileDevice(
			EnrollMobileDeviceDeepLink(
				hostname: "my.cluster.com",
				port: nil,
				enrollPairingToken: "abc/def==",
			),
		)
		let got = try DeepLink(from: #require(
			URL(string: "teleport://my.cluster.com/enroll_mobile_device?enroll_pairing_token=abc%2Fdef%3D%3D"),
		))

		expectNoDifference(expected, got)
	}

	@Test
	func `enrollment link without hostname should throw an error`() throws {
		#expect(throws: DeepLinkParseError.missingPart("hostname")) {
			try DeepLink(from: #require(URL(string: "teleport:/enroll_mobile_device?enroll_pairing_token=abc123")))
		}
	}

	@Test
	func `enrollment link without token should throw an error`() throws {
		#expect(throws: DeepLinkParseError.missingPart("enroll pairing token")) {
			try DeepLink(from: #require(URL(string: "teleport://my.cluster.com/enroll_mobile_device")))
		}
	}

	@Test
	func `unsupported path should throw before checking missing enrollment parts`() throws {
		#expect(throws: DeepLinkParseError.unsupportedPath) {
			try DeepLink(from: #require(URL(string: "teleport:/hello")))
		}
	}
}
