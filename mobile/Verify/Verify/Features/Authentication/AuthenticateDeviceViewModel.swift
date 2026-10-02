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

import Core
import Dependencies
import Foundation
import Logging
import Observation
import SQLiteData

@Observable @MainActor
final class AuthenticateDeviceViewModel {
	private let logger = Logger.forType(AuthenticateDeviceViewModel.self)

	var loadingState: LoadingState<URL> = .idle
	private var requiresNewLogin = false
	private var deviceAuthenticationTask: Task<Void, Never>? = nil
	private let deepLink: AuthenticateDeviceDeepLink

	@ObservationIgnored
	@Dependency(\.deviceTrustNetworkClient)
	private var deviceTrustNetworkClient

	@ObservationIgnored
	@Dependency(\.deviceTrustCredentialClient)
	private var deviceTrustCredentialClient

	@ObservationIgnored
	@Dependency(\.defaultDatabase)
	private var database

	weak var delegate: any Delegate? = nil

	init(deepLink: AuthenticateDeviceDeepLink, delegate: any Delegate? = nil) {
		self.deepLink = deepLink
		self.delegate = delegate
	}

	func performDeviceAuthentication() async {
		loadingState = .loading
		// If we send the token across the boundary to Go, it's possible that the token will be spent by the time the
		// call to the Go code returns. If that happens, an entire new login is required.
		requiresNewLogin = false

		do {
			try Task.checkCancellation()
			// We check first to see if the cluster with the specified domain is one with which we are registered. This
			// prevents a relay attack where an attacker can sit in between this client and a server and receive the
			// device
			// trust session token in our stead.
			let savedCluster = try await database.read { db in
				try Cluster
					.where { $0.host.eq(deepLink.hostname) && $0.port.eq(deepLink.port) }
					.fetchOne(db)
			}
			guard savedCluster != nil else {
				throw Error.clusterIsNotRegistered
			}

			try Task.checkCancellation()
			let credential = try deviceTrustCredentialClient.load()

			try Task.checkCancellation()
			requiresNewLogin = true
			let confirmationToken = try await deviceTrustNetworkClient.authenticateDevice(
				hostname: deepLink.hostname,
				port: deepLink.port,
				webTokenID: deepLink.webTokenID,
				webToken: deepLink.webToken,
				credentialID: credential.id,
				signChallenge: { [deviceTrustCredentialClient] challenge in
					try await deviceTrustCredentialClient.signChallenge(challenge: challenge, purpose: .authentication)
				},
			)

			try Task.checkCancellation()
			let confirmationURL = try makeConfirmationURL(token: confirmationToken)
			loadingState = .success(confirmationURL)
			logger.info("Successfully authenticated device", metadata: deepLink.logMetadata)
			await delegate?.authenticateDeviceViewModel(self, didSucceedWith: confirmationURL)
		} catch DeviceTrustCredentialError.signingAuthorizationCancelled {
			loadingState = .failure(DeviceTrustCredentialError.signingAuthorizationCancelled)
		} catch let error as CancellationError {
			loadingState = .failure(error)
		} catch {
			logger.error("Failed to authenticate device", error: error, metadata: deepLink.logMetadata)
			loadingState = .failure(error)
		}
	}
}

// MARK: - AuthenticateDeviceViewModel.Delegate

extension AuthenticateDeviceViewModel {
	@MainActor
	protocol Delegate: AnyObject {
		func authenticateDeviceViewModelDidCancelOperation(_ viewModel: AuthenticateDeviceViewModel)
		func authenticateDeviceViewModel(_ viewModel: AuthenticateDeviceViewModel, didRequestReturnToWebWith url: URL)
		func authenticateDeviceViewModel(
			_ viewModel: AuthenticateDeviceViewModel,
			didSucceedWith confirmationURL: URL,
		) async
	}
}

// MARK: - SheetPresentable

extension AuthenticateDeviceViewModel: SheetPresentable {
	var presentationID: some Hashable {
		"AuthenticateDeviceViewModel"
	}
}

// MARK: - AuthenticateDeviceViewModel.PrimaryButtonState

extension AuthenticateDeviceViewModel {
	enum PrimaryButtonState {
		case authorize
		case loading
		case retry
		case returnToWeb
		case done
	}
}

// MARK: - View Lifecycle

extension AuthenticateDeviceViewModel {
	func onDisappear() {
		deviceAuthenticationTask?.cancel()
		deviceAuthenticationTask = nil
	}
}

// MARK: - User Actions

extension AuthenticateDeviceViewModel {
	func userTappedPrimaryButton() {
		switch primaryButtonState {
			case .authorize, .retry:
				guard deviceAuthenticationTask == nil else { return }

				deviceAuthenticationTask = Task { [weak self] in
					guard let self else { return }

					await performDeviceAuthentication()
					deviceAuthenticationTask = nil
				}
			case .returnToWeb:
				guard let url = makeReturnToWebURL() else { return }
				delegate?.authenticateDeviceViewModel(self, didRequestReturnToWebWith: url)
			case .loading, .done:
				break
		}
	}

	func userTappedCancel() {
		deviceAuthenticationTask?.cancel()
		deviceAuthenticationTask = nil
		delegate?.authenticateDeviceViewModelDidCancelOperation(self)
	}
}

// MARK: - Private Helpers

extension AuthenticateDeviceViewModel {
	private func makeReturnToWebURL() -> URL? {
		var components = URLComponents()
		components.scheme = "https"
		components.host = deepLink.hostname
		components.port = deepLink.port
		components.path = "/web"
		return components.url
	}

	private func makeConfirmationURL(token: DeviceConfirmationToken) throws -> URL {
		var components = URLComponents()
		components.scheme = "https"
		components.host = deepLink.hostname
		components.port = deepLink.port
		components.path = "/webapi/devices/webconfirm"
		components.queryItems = [
			URLQueryItem(name: "id", value: token.id),
			URLQueryItem(name: "token", value: token.token),
		]

		if let redirectURI = deepLink.redirectURI {
			components.queryItems?.append(URLQueryItem(name: "redirect_uri", value: redirectURI))
		}

		guard let url = components.url else {
			throw Error.confirmationURLCreationFailed
		}

		return url
	}
}

// MARK: - AuthenticateDeviceViewModel.Error

extension AuthenticateDeviceViewModel {
	enum Error: Swift.Error {
		case confirmationURLCreationFailed
		case clusterIsNotRegistered
	}
}

// MARK: - UI Helpers

extension AuthenticateDeviceViewModel {
	var hostname: String {
		deepLink.hostname
	}

	var primaryButtonState: PrimaryButtonState {
		switch loadingState {
			case .idle: .authorize
			case .loading, .reloading: .loading
			case .failure: requiresNewLogin ? .returnToWeb : .retry
			case .success: .done
		}
	}

	var failureMessage: String? {
		guard case let .failure(error) = loadingState else { return nil }
		if let error = error as? AuthenticateDeviceViewModel.Error, error == .clusterIsNotRegistered {
			return NSLocalizedString(
				"The cluster “\(deepLink.hostname):\(deepLink.port)” is not registered with this device. Re-enroll this device in Device Trust and try again.",
				comment: "Message shown when a device runs into an unknown host trying to authenticate.",
			)
		}
		if requiresNewLogin {
			return NSLocalizedString(
				"Sign in again to get a new authorization request.",
				comment: "Message shown when a device authorization request can no longer be retried.",
			)
		}
		return NSLocalizedString(
			"Failed to Authorize Device",
			comment: "Message shown when device authorization fails before contacting the cluster.",
		)
	}

	var isCancelButtonEnabled: Bool {
		switch loadingState {
			case .loading, .reloading, .failure: true
			case .idle, .success: false
		}
	}
}
