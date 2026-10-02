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

import Darwin
import Dependencies
import DependenciesMacros
import Enroll
public import Foundation
import Logging
import SystemClients

/// Handles network requests for the current device's Device Trust operations.
@DependencyClient
public struct DeviceTrustNetworkClient: Sendable {
	/// Sends a request for an enrollment token
	public var requestEnrollmentToken: @Sendable (
		_ hostname: String,
		_ port: Int,
		_ pairingToken: String,
	) async throws -> String

	public var enrollDevice: @Sendable (
		_ hostname: String,
		_ port: Int,
		_ enrollmentToken: String,
		_ credentialID: String,
		_ publicKeyDER: Data,
		_ signChallenge: ChallengeSigner,
	) async throws -> Void

	public var authenticateDevice: @Sendable (
		_ hostname: String,
		_ port: Int,
		_ webTokenID: String,
		_ webToken: String,
		_ credentialID: String,
		_ signChallenge: ChallengeSigner,
	) async throws -> DeviceConfirmationToken
}

public enum DeviceTrustNetworkClientError: Error, Sendable {
	case clientCreationFailed
	case deviceInformationUnavailable(String)
}

extension DeviceTrustNetworkClient {
	private static let logger = Logger(label: "DeviceTrustNetworkClient")

	public static let liveValue = DeviceTrustNetworkClient(
		requestEnrollmentToken: { hostname, port, pairingToken in
			try await Task.detached(priority: .userInitiated) {
				let proxyServer = proxyServerString(hostname: hostname, port: port)
				guard let client = Enroll.EnrollClient(proxyServer, insecure: false) else {
					throw DeviceTrustNetworkClientError.clientCreationFailed
				}
				let token = try client.createPairedDeviceEnrollToken(
					pairingToken,
					deviceData: collectDeviceData(),
				)
				return token.token
			}.value
		},
		enrollDevice: { hostname, port, enrollmentToken, credentialID, publicKeyDER, signChallenge in
			let proxyServer = proxyServerString(hostname: hostname, port: port)
			guard let client = Enroll.EnrollClient(proxyServer, insecure: false) else {
				throw DeviceTrustNetworkClientError.clientCreationFailed
			}
			let operation = try client.newEnrollDeviceOperation(
				enrollmentToken,
				deviceData: collectDeviceData(),
				credentialIdentifier: credentialID,
				publicKeyDER: publicKeyDER,
			)
			defer {
				do {
					try operation.close()
				} catch {
					logger.warning("Failed to close EnrollDeviceOperation", error: error)
				}
			}

			let challenge = try operation.initialize()
			let signature = try await signChallenge(challenge)
			try operation.sendChallengeResponse(signature)
		},
		authenticateDevice: { hostname, port, webTokenID, webToken, credentialID, signChallenge in
			let proxyServer = proxyServerString(hostname: hostname, port: port)
			guard let client = Enroll.EnrollClient(proxyServer, insecure: false) else {
				throw DeviceTrustNetworkClientError.clientCreationFailed
			}
			let operation = try client.newAuthenticateDeviceOperation(
				webTokenID,
				webToken: webToken,
				deviceData: collectDeviceData(),
				credentialIdentifier: credentialID,
			)
			defer {
				do {
					try operation.close()
				} catch {
					logger.warning("Failed to close AuthenticateDeviceOperation", error: error)
				}
			}

			let challenge = try operation.initialize()
			let signature = try await signChallenge(challenge)
			let goConfirmationToken = try operation.sendChallengeResponse(signature)
			return DeviceConfirmationToken(id: goConfirmationToken.identifier, token: goConfirmationToken.token)
		},
	)
}

// MARK: - Supporting Types

public typealias ChallengeSigner = @Sendable (Data) async throws -> Data

public struct DeviceConfirmationToken: Sendable {
	public let id: String
	public let token: String

	public init(id: String, token: String) {
		self.id = id
		self.token = token
	}
}

// MARK: - Private Helpers

extension DeviceTrustNetworkClient {
	private static func collectDeviceData() throws -> Enroll.EnrollDeviceCollectedData {
		let serialNumber = try getSerialNumber()
		let collectedData = Enroll.EnrollDeviceCollectedData()
		collectedData.serialNumber = serialNumber
		collectedData.systemSerialNumber = serialNumber
		collectedData.versionOS = getOSVersion()
		collectedData.modelIdentifier = try getModelIdentifier()
		collectedData.buildOS = try getOSBuild()

		logger.debug("Collected device information", metadata: [
			"serialNumber": "\(serialNumber)",
			"deviceModel": "\(collectedData.modelIdentifier)",
			"osVersion": "\(collectedData.versionOS)",
			"osBuild": "\(collectedData.buildOS)",
		])
		return collectedData
	}

	/// Returns the dotted numeric OS version reported by Foundation, such as `18.5.0`.
	private static func getOSVersion() -> String {
		let version = ProcessInfo.processInfo.operatingSystemVersion
		return "\(version.majorVersion).\(version.minorVersion).\(version.patchVersion)"
	}

	/// Reads Apple's OS build identifier, such as `22F76`, from `kern.osversion`.
	private static func getOSBuild() throws -> String {
		guard let build = sysctlString(named: "kern.osversion"), !build.isEmpty else {
			throw DeviceTrustNetworkClientError.deviceInformationUnavailable("OS build")
		}
		return build
	}

	/// Returns the product-style model identifier used by device inventories, such as `iPhone15,2`.
	///
	/// Simulators publish the simulated model in their environment. Physical devices expose it
	/// through `hw.product`; `hw.machine` remains as a fallback for older OS versions.
	private static func getModelIdentifier() throws -> String {
		#if targetEnvironment(simulator)
			let modelIdentifier = ProcessInfo.processInfo.environment["SIMULATOR_MODEL_IDENTIFIER"]
		#else
			let modelIdentifier = sysctlString(named: "hw.product") ?? sysctlString(named: "hw.machine")
		#endif

		guard let modelIdentifier, !modelIdentifier.isEmpty else {
			throw DeviceTrustNetworkClientError.deviceInformationUnavailable("model identifier")
		}

		return modelIdentifier
	}

	/// Returns the serial number supplied through the app's managed configuration.
	///
	/// The serial number dependency provides a generated value when running a debug build.
	private static func getSerialNumber() throws -> String {
		@Dependency(\.serialNumberClient)
		var serialNumberClient

		return try serialNumberClient.getDeviceSerialNumber()
	}

	/// Reads a null-terminated string from the Darwin sysctl namespace.
	///
	/// `sysctlbyname` requires one call to determine the buffer size and a second to copy the value.
	/// Returns `nil` when either call fails or the reported value is empty.
	private static func sysctlString(named name: String) -> String? {
		var size: size_t = 0

		guard sysctlbyname(name, nil, &size, nil, 0) == 0, size > 1 else {
			return nil
		}

		var buffer = [CChar](repeating: 0, count: size)
		let status = buffer.withUnsafeMutableBufferPointer { pointer in
			sysctlbyname(name, pointer.baseAddress, &size, nil, 0)
		}

		guard status == 0 else {
			return nil
		}

		return buffer.withUnsafeBufferPointer { pointer in
			guard let baseAddress = pointer.baseAddress else {
				return nil
			}
			return String(cString: baseAddress)
		}
	}

	/// Joins the hostname and port for the proxy server in the format expected by the Go client.
	private static func proxyServerString(hostname: String, port: Int) -> String {
		"\(hostname):\(port)"
	}
}
