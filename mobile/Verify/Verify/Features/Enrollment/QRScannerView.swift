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

import Foundation
import SwiftUI
import Vision
import VisionKit

struct QRScannerView: UIViewControllerRepresentable {
	var onScan: (String) -> QRScannerDecision
	var onError: (any Error) -> Void = { _ in }

	func makeUIViewController(context: Context) -> DataScannerViewController {
		let viewController = DataScannerViewController(
			recognizedDataTypes: [.barcode(symbologies: [.qr])],
			qualityLevel: .balanced,
			recognizesMultipleItems: false,
			isHighFrameRateTrackingEnabled: false,
			isPinchToZoomEnabled: true,
			isGuidanceEnabled: true,
			isHighlightingEnabled: true,
		)

		viewController.delegate = context.coordinator
		return viewController
	}

	func updateUIViewController(_ uiViewController: DataScannerViewController, context: Context) {
		guard !uiViewController.isScanning else {
			return
		}

		guard DataScannerViewController.isSupported else {
			context.coordinator.reportError(QRScannerError.unsupportedDevice)
			return
		}

		guard DataScannerViewController.isAvailable else {
			context.coordinator.reportError(QRScannerError.scannerUnavailable)
			return
		}

		do {
			try uiViewController.startScanning()
		} catch {
			context.coordinator.reportError(error)
		}
	}

	static func dismantleUIViewController(_ uiViewController: DataScannerViewController, coordinator: Coordinator) {
		uiViewController.stopScanning()
	}

	func makeCoordinator() -> Coordinator {
		Coordinator(onScan: onScan, onError: onError)
	}

	@MainActor
	final class Coordinator: NSObject, DataScannerViewControllerDelegate {
		private var didScan = false
		private var didReportError = false
		private let onScan: (String) -> QRScannerDecision
		private let onError: (any Error) -> Void

		init(
			onScan: @escaping (String) -> QRScannerDecision,
			onError: @escaping (any Error) -> Void,
		) {
			self.onScan = onScan
			self.onError = onError
		}

		func reportError(_ error: any Error) {
			guard !didReportError else {
				return
			}

			didReportError = true
			onError(error)
		}

		func dataScanner(
			_ dataScanner: DataScannerViewController,
			didAdd addedItems: [RecognizedItem],
			allItems: [RecognizedItem],
		) {
			handle(items: addedItems, dataScanner: dataScanner)
		}

		func dataScanner(
			_ dataScanner: DataScannerViewController,
			didUpdate updatedItems: [RecognizedItem],
			allItems: [RecognizedItem],
		) {
			handle(items: updatedItems, dataScanner: dataScanner)
		}

		func dataScanner(
			_ dataScanner: DataScannerViewController,
			becameUnavailableWithError error: DataScannerViewController.ScanningUnavailable,
		) {
			reportError(error)
		}

		private func handle(items: [RecognizedItem], dataScanner: DataScannerViewController) {
			guard !didScan else {
				return
			}

			for item in items {
				guard
					case let .barcode(barcode) = item,
					let payload = barcode.payloadStringValue
				else {
					continue
				}

				switch onScan(payload) {
					case .continueScanning:
						break
					case .stopScanning:
						didScan = true
						dataScanner.stopScanning()
						return
				}
			}
		}
	}
}

enum QRScannerError: LocalizedError {
	case unsupportedDevice
	case scannerUnavailable

	var errorDescription: String? {
		switch self {
			case .unsupportedDevice:
				"QR code scanning is not supported on this device."
			case .scannerUnavailable:
				"QR code scanning is currently unavailable."
		}
	}
}

enum QRScannerDecision {
	case continueScanning, stopScanning
}
