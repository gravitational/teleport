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

import SwiftUI

/// A small view that requests log collection on appearance and displaying sharing options to the user when the logs are
/// ready.
struct ShareLogsView: View {
	let viewModel: ShareLogsViewModel

	@Environment(\.dismiss)
	var dismiss

	/// The draft to use when composing the email
	///
	/// We do our best to avoid putting state in our views because SwiftUI views are not testable. However this view,
	/// by virtue of the fact that there's no programmatic or state-based way to trigger a Share Sheet in SwiftUI,
	/// is already unable to have its destinations tested. Instead, we use the built-in `ShareLink` for Share Sheet
	/// behavior. Since this state primarily exists to drive the destination to the email composition sheet, I decided
	/// to make this the view's `@State` as a sign post to not bother trying to test this.
	///
	/// - Tag: share-logs-email-destination
	@State
	private var emailDraft: EmailDraft? = nil

	var body: some View {
		NavigationStack {
			ScrollView {
				VStack(alignment: .leading, spacing: .medium) {
					Text(
						"""
						Sensitive data such as private keys are _never_ stored in log files, but logs may contain \
						other information associated with you and your use of Teleport’s services.
						""",
					)
					.padding(.horizontal)
					.foregroundStyle(Color.Foreground.slightlyMuted)

					if viewModel.logCollectionLoadingState.isLoading {
						ProgressView()
							.frame(maxWidth: .infinity, alignment: .center)
					} else {
						if !viewModel.isEmailComposeAvailable {
							Text(
								"""
								Copy the email address below before sharing the logs to your email app.
								""",
							)
							.font(.footnote)
							.padding(.horizontal)
							.foregroundStyle(Color.Foreground.slightlyMuted)

							Button(action: viewModel.userTappedCopyEmailButton) {
								Label {
									ZStack {
										Text(viewModel.supportEmail)
											.hidden()
										Text(viewModel.copyEmailText)
									}
								} icon: {
									Image(systemName: "document.on.document")
								}
							}
							.frame(maxWidth: .infinity)
						}
					}
				}
			}
			.scrollBounceBehavior(.basedOnSize)
			.safeAreaInset(edge: .bottom) {
				bottomButtons
			}
			.navigationTitle("Share Logs")
		}
		.sensoryFeedback(.success, trigger: viewModel.hapticTrigger)
		.presentationDetents([.medium, .large])
		.sheet(item: $emailDraft) { draft in
			EmailComposeView(draft: draft)
		}
		.task {
			await viewModel.collectLogs()
		}
	}
}

// MARK: - Subviews

extension ShareLogsView {
	/// The sharing buttons that sit at the bottom of the view.
	@ViewBuilder
	private var bottomButtons: some View {
		if let zippedLogsURL = viewModel.zippedLogsURL {
			HStack {
				let shareLink = ShareLink(item: zippedLogsURL) {
					Label("Share", systemImage: "square.and.arrow.up")
						.shareButtonStyling()
				}

				if viewModel.isEmailComposeAvailable {
					shareLink.buttonStyle(.bordered)

					Button {
						Task {
							emailDraft = await viewModel.makeEmailDraft()
						}
					} label: {
						Label("Email", systemImage: "envelope")
							.shareButtonStyling()
					}
					.buttonStyle(.borderedProminent)
				} else {
					shareLink.buttonStyle(.borderedProminent)
				}
			}
			.buttonBorderShape(.roundedRectangle(radius: .small))
			.padding(.horizontal)
		}
	}
}

extension Label {
	/// A collection of modifiers to describe how we want to style the share button labels.
	fileprivate func shareButtonStyling() -> some View {
		font(.headline)
			.fontWeight(.semibold)
			.padding(.vertical, .small)
			.frame(maxWidth: .infinity)
	}
}
