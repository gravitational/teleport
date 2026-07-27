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
import Foundation
import SwiftUI

struct EnrollCameraScannerView: View {
	var viewModel: EnrollCameraScannerViewModel

	@Environment(\.openURL)
	var openURL

	var body: some View {
		Group {
			switch viewModel.cameraAuthorizationStatus {
				case .notDetermined:
					ProgressView()
				case .restricted, .denied:
					// TODO: Add a button below this view that opens the iOS settings
					ContentUnavailableView(
						"QR Scanner Unavailable",
						systemImage: "video.slash",
						description: unavailableCameraDescriptionText,
					)
				case .authorized:
					QRScannerView(onScan: viewModel.didScan(_:))
						.ignoresSafeArea()
				@unknown default:
					EmptyView()
			}
		}
		.task(viewModel.requestCameraAccess)
	}
}

// MARK: - Subviews

extension EnrollCameraScannerView {
	var unavailableCameraDescriptionText: Text {
		if viewModel.cameraAuthorizationStatus == .restricted {
			// If the authorization status is restricted, it usually means by some external mechanism like MDM or
			// parental controls. It's often something that the user doesn't have control over.
			Text("Your device has prevented Teleport Verify from accessing your camera.")
		} else {
			Text("Teleport Verify doesn't have permission to show the QR scanner. Grant permission in iOS settings.")
		}
	}
}
