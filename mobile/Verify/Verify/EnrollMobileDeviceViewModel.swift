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

import Enroll
import Observation

@Observable
@MainActor
class EnrollMobileDeviceViewModel {
    enum Attempt {
        case idle
        case loading
        case success(token: String)
        case failure(Error)

        var isLoading: Bool {
            if case .loading = self { return true }
            return false
        }
    }

    var attempt: Attempt = .idle
    private let deepURL: EnrollMobileDeviceDeepURL

    init(deepURL: EnrollMobileDeviceDeepURL) {
        self.deepURL = deepURL
    }

    func requestEnrollToken() async {
        attempt = .loading
        let proxyServer = "\(deepURL.url.hostname):\(deepURL.url.port ?? 443)"
        let pairingToken = deepURL.enrollPairingToken

        let outcome: Attempt = await Task.detached(priority: .userInitiated) {
            guard let client = EnrollClient(proxyServer, insecure: false) else {
                return .failure(EnrollViewModelError.clientCreationFailed)
            }
            do {
                let token = try client.createMobileEnrollToken(
                    pairingToken,
                    deviceData: EnrollDeviceCollectedData()
                )
                return .success(token: token.token)
            } catch {
                return .failure(error)
            }
        }.value

        attempt = outcome
    }
}

enum EnrollViewModelError: Error {
    case clientCreationFailed
}
