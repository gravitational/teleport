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

import AVFoundation
import Observation
import OSLog

@Observable @MainActor
final class EnrollCameraScannerViewModel {
	private static let logger = Logger.forType(EnrollCameraScannerViewModel.self)

	var cameraAuthorizationStatus: AVAuthorizationStatus = .notDetermined

	weak var delegate: (any Delegate)? = nil

	init(delegate: (any Delegate)? = nil) {
		self.delegate = delegate
	}
}

// MARK: - EnrollCameraScannerViewModel.Delegate

extension EnrollCameraScannerViewModel {
	protocol Delegate: AnyObject {
		func enrollCameraScannerViewModel(
			_ viewModel: EnrollCameraScannerViewModel,
			didReceiveEnrollMobileDeviceDeepLink deepLink: EnrollMobileDeviceDeepLink,
		)
	}
}

// MARK: - Scanner Actions

extension EnrollCameraScannerViewModel {
	func didScan(_ payload: String) -> QRScannerDecision {
		guard let enrollMobileDeviceDeepLink = validateScannedCode(payload) else {
			return .continueScanning
		}
		Self.logger.info("Scanned deep link: \(enrollMobileDeviceDeepLink.debugDescription)")
		delegate?.enrollCameraScannerViewModel(self, didReceiveEnrollMobileDeviceDeepLink: enrollMobileDeviceDeepLink)
		return .stopScanning
	}

	private func validateScannedCode(_ payload: String) -> EnrollMobileDeviceDeepLink? {
		Self.logger.debug("Validating scanned QR code: \(payload)")
		do {
			guard let url = URL(string: payload) else {
				return nil
			}
			let deepLink = try DeepLink(from: url)
			guard case let .enrollMobileDevice(enrollMobileDeviceDeepLink) = deepLink else {
				return nil
			}
			return enrollMobileDeviceDeepLink
		} catch {
			Self.logger.debug("\(payload) did not pass validation")
			return nil
		}
	}
}

// MARK: - Navigation Helpers

extension EnrollCameraScannerViewModel {
	/// For the purposes of presentation, there is no distinction between instances of EnrollCameraScannerViewModel,
	/// so we vend this constant presentation ID to express that to SwiftUI.
	var presentationID: String {
		"EnrollCameraScannerViewModel"
	}
}

// MARK: - Camera Authorization

extension EnrollCameraScannerViewModel {
	func requestCameraAccess() async {
		cameraAuthorizationStatus = AVCaptureDevice.authorizationStatus(for: .video)
		switch cameraAuthorizationStatus {
			case .notDetermined:
				await AVCaptureDevice.requestAccess(for: .video)
				cameraAuthorizationStatus = AVCaptureDevice.authorizationStatus(for: .video)
			case .restricted, .denied, .authorized:
				break
			@unknown default:
				Self.logger.warning(
					"Encountered unknown camera authorization status: \(self.cameraAuthorizationStatus.rawValue)",
				)
		}
	}
}
