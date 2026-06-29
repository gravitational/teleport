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
import Dependencies
import Observation
import SwiftNavigation

@Observable
@MainActor
class EnrollDeviceViewModel {
	// swiftformat:sort
	@CasePathable
	enum Destination {
		case loadingSheet
	}

	var destination: Destination? = nil
	var loadingState: LoadingState<String> = .idle
	private let deepLink: EnrollMobileDeviceDeepLink

	@ObservationIgnored
	@Dependency(\.enrollClient)
	private var enrollClient: EnrollClient

	weak var delegate: (any Delegate)? = nil

	init(
		deepLink: EnrollMobileDeviceDeepLink,
		delegate: (any Delegate)? = nil,
	) {
		self.deepLink = deepLink
		self.delegate = delegate
	}

	func requestEnrollToken() async {
		loadingState = .loading
		destination = .loadingSheet
		let defaultHTTPSPort = 443
		do {
			/*
			 TODO: Implement the call to requestEnrollmentToken
			 Right now, the backend doesn't have all the behavior we need to test enrollment token request end-to-end
			 so we just simulate the behavior for now with a small delay.

			 let token = try await enrollClient.requestEnrollmentToken(
			 	hostName: deepLink.hostname,
			 	port: deepLink.port ?? defaultHTTPSPort,
			 	pairingToken: deepLink.enrollPairingToken,
			 )
			  */

			try await Task.sleep(for: .seconds(1))
			loadingState = .success("fake-token-\(defaultHTTPSPort)")
			destination = nil
		} catch {
			loadingState = .failure(error)
		}
	}
}

// MARK: - EnrollDeviceViewModel.Delegate

extension EnrollDeviceViewModel {
	protocol Delegate: AnyObject {
		func enrollDeviceViewModelDidCancelOperation(_ viewModel: EnrollDeviceViewModel)
	}
}

// MARK: - User Actions

extension EnrollDeviceViewModel {
	func userTappedCancel() {
		delegate?.enrollDeviceViewModelDidCancelOperation(self)
	}
}
