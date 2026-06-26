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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

import Core
import Observation

@Observable
@MainActor
class EnrollMobileDeviceViewModel {
	enum Attempt {
		case idle
		case loading
		case success(token: String)
		case failure(any Error)

		var isLoading: Bool {
			if case .loading = self { return true }
			return false
		}
	}

	var attempt: Attempt = .idle
	private let deepURL: EnrollMobileDeviceDeepURL
	private let enrollClient: EnrollClient

	init(deepURL: EnrollMobileDeviceDeepURL, enrollClient: EnrollClient = .liveValue) {
		self.deepURL = deepURL
		self.enrollClient = enrollClient
	}

	func requestEnrollToken() async {
		attempt = .loading
		let defaultHTTPSPort = 443
		do {
			let token = try await enrollClient.requestEnrollmentToken(
				hostName: deepURL.url.hostname,
				port: deepURL.url.port ?? defaultHTTPSPort,
				pairingToken: deepURL.enrollPairingToken,
			)
			attempt = .success(token: token)
		} catch {
			attempt = .failure(error)
		}
	}
}
