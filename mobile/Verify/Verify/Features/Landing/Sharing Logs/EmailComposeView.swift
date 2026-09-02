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

import MessageUI
import SwiftUI

/// A SwiftUI bridge to the system email composition interface.
struct EmailComposeView: UIViewControllerRepresentable {
	let draft: EmailDraft

	@Environment(\.dismiss)
	private var dismiss

	func makeCoordinator() -> Coordinator {
		Coordinator(dismiss: dismiss)
	}

	func makeUIViewController(context: Context) -> MFMailComposeViewController {
		let viewController = MFMailComposeViewController()
		viewController.mailComposeDelegate = context.coordinator
		viewController.setToRecipients(draft.recipients)
		viewController.setSubject(draft.subject)
		viewController.setMessageBody(draft.body, isHTML: false)

		for attachment in draft.attachments {
			viewController.addAttachmentData(
				attachment.data,
				mimeType: attachment.mimeType,
				fileName: attachment.fileName,
			)
		}

		return viewController
	}

	func updateUIViewController(_ viewController: MFMailComposeViewController, context: Context) {
		// MessageUI only supports configuring a draft before presenting it.
	}
}

// MARK: - EmailComposeView.Coordinator

extension EmailComposeView {
	@MainActor
	final class Coordinator: NSObject, MFMailComposeViewControllerDelegate {
		private let dismiss: DismissAction

		init(dismiss: DismissAction) {
			self.dismiss = dismiss
		}

		func mailComposeController(
			_ controller: MFMailComposeViewController,
			didFinishWith result: MFMailComposeResult,
			error: (any Error)?,
		) {
			dismiss()
		}
	}
}
