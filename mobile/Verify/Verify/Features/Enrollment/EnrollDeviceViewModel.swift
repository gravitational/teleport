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
import Foundation
import Logging
import Observation
import SQLiteData
import SwiftNavigation

@Observable
@MainActor
class EnrollDeviceViewModel {
	private let logger = Logger.forType(EnrollDeviceViewModel.self)

	var loadingState: LoadingState<String> = .idle
	private let deepLink: EnrollMobileDeviceDeepLink
	let eventStackViewModel = EventStackViewModel<EnrollmentEventID>()

	@ObservationIgnored
	@Dependency(\.enrollClient)
	private var enrollClient: EnrollClient

	@ObservationIgnored
	@Dependency(\.defaultDatabase)
	private var database

	weak var delegate: (any Delegate)? = nil

	init(
		deepLink: EnrollMobileDeviceDeepLink,
		delegate: (any Delegate)? = nil,
	) {
		self.deepLink = deepLink
		self.delegate = delegate
	}

	func performDeviceEnrollment() async {
		loadingState = .loading
		eventStackViewModel.clearAllEvents()
		do {
			let token = try await requestEnrollmentToken()
			try await enrollDevice(using: token)
			let cluster = try await saveClusterToDatabase()
			logger.info("Successfully enrolled cluster", metadata: cluster?.logMetadata)
			delegate?.enrollDeviceViewModelDidEnrollCluster(self)
			loadingState = .success(token)
		} catch {
			logger.error("Failed to enroll in Device Trust", error: error)
			loadingState = .failure(error)
		}
	}
}

// MARK: - EnrollDeviceViewModel.Delegate

extension EnrollDeviceViewModel {
	protocol Delegate: AnyObject {
		func enrollDeviceViewModelDidCancelOperation(_ viewModel: EnrollDeviceViewModel)
		func enrollDeviceViewModelDidEnrollCluster(_ viewModel: EnrollDeviceViewModel)
	}
}

// MARK: - EnrollDeviceViewModel.EnrollmentEventID

extension EnrollDeviceViewModel {
	enum EnrollmentEventID: String, Identifiable {
		case enrollmentTokenRequest
		case enrollDeviceRequest
		case saveClusterToDatabaseTask

		var id: String {
			rawValue
		}
	}
}

// MARK: - User Actions

extension EnrollDeviceViewModel {
	func userTappedCancel() {
		delegate?.enrollDeviceViewModelDidCancelOperation(self)
	}
}

// MARK: - Private Helpers

extension EnrollDeviceViewModel {
	private func requestEnrollmentToken() async throws -> String {
		do {
			eventStackViewModel.addEvent(
				id: .enrollmentTokenRequest,
				message: "Requesting enrollment token…",
			)
			let token = try await enrollClient.requestEnrollmentToken(
				hostName: deepLink.hostname,
				port: deepLink.port,
				pairingToken: deepLink.enrollPairingToken,
			)
			eventStackViewModel.updateEvent(
				id: .enrollmentTokenRequest,
				message: "Enrollment token received",
				status: .success,
			)
			return token
		} catch {
			eventStackViewModel.updateEvent(
				id: .enrollmentTokenRequest,
				message: "Failed to retrieve enrollment token",
				status: .failure,
			)
			throw error
		}
	}

	private func enrollDevice(using enrollmentToken: String) async throws {
		struct DeviceEnrollmentNotImplemented: Error {}

		do {
			eventStackViewModel.addEvent(
				id: .enrollDeviceRequest,
				message: "Enrolling in Device Trust…",
			)

			// TODO: With enrollment token in hand, fire off the EnrollDevice RPC
			throw DeviceEnrollmentNotImplemented()

			// Leaving this code commented until this function is implemented to silence an unreachable code warning.
			// eventStackViewModel.updateEvent(
			// 	id: .enrollDeviceRequest,
			// 	message: "Device enrolled in Device Trust",
			// 	status: .success
			// )
		} catch {
			eventStackViewModel.updateEvent(
				id: .enrollDeviceRequest,
				message: "Failed to enroll in device trust; please try again",
				status: .failure,
			)
			throw error
		}
	}

	private func saveClusterToDatabase() async throws -> Cluster? {
		do {
			eventStackViewModel.addEvent(
				id: .saveClusterToDatabaseTask,
				message: "Saving cluster…",
			)
			let cluster = try await database.write { db in
				try Cluster.insert {
					Cluster.Draft(
						host: deepLink.hostname,
						port: deepLink.port,
					)
				}
				.returning(\.self)
				.fetchOne(db)
			}
			eventStackViewModel.updateEvent(
				id: .saveClusterToDatabaseTask,
				message: "Cluster saved successfully",
				status: .success,
			)
			return cluster
		} catch {
			eventStackViewModel.updateEvent(
				id: .saveClusterToDatabaseTask,
				message: "Failed to save cluster locally; please try again",
				status: .failure,
			)
			throw error
		}
	}
}
