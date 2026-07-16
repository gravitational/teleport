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
import IdentifiedCollections
import SwiftUI

/// A general-purpose view for showing a sequence of events on a timeline with associated icons.
struct EventStackView<ID: Hashable>: View {
	typealias Event = EventStackViewModel<ID>.Event
	typealias Status = EventStackViewModel<ID>.Status

	let viewModel: EventStackViewModel<ID>

	@ScaledMetric
	private var minimumBarHeight: CGFloat = .medium

	@ScaledMetric(relativeTo: .body)
	private var barWidth: CGFloat = .xxsmall

	var body: some View {
		VStack(alignment: .leading, spacing: .xxsmall) {
			ForEach(viewModel.events) { event in
				row(for: event)
			}
		}
		.frame(maxWidth: .infinity, alignment: .leading)
	}

	private func row(for event: Event) -> some View {
		HStack(alignment: .firstTextBaseline) {
			VStack(alignment: .center) {
				icon(for: event.status)
				if event.id != viewModel.events.last?.id {
					Capsule()
						.fill(Color.Foreground.muted)
						.frame(maxWidth: barWidth, minHeight: minimumBarHeight)
				}
			}
			label(for: event)
				.padding(.bottom, .small)
		}
		/*
		 Distilling the accessibilty representation down to just a message does lose _some_ information by not
		 including the semantics of the icon, but this UI is general purpose enough that including semantic
		 infomation about the icon may not always be appropriate.
		 */
		.accessibilityLabel(event.message)
	}

	@ViewBuilder
	private func icon(for status: Status) -> some View {
		let image = switch status {
			case .failure:
				Image(systemName: "xmark.circle")
			case .loading:
				Image(systemName: "circle.dotted")
			case .success:
				Image(systemName: "checkmark.circle")
			case .warning:
				Image(systemName: "exclamationmark.circle")
		}
		let color: Color = switch status {
			case .failure: .danger
			case .loading: Color.Foreground.slightlyMuted
			case .success: .success
			case .warning: .alert
		}
		if status == .loading {
			image.foregroundStyle(color)
				.symbolEffect(.rotate)
		} else {
			image.foregroundStyle(color)
		}
	}

	private func label(for event: Event) -> some View {
		Group {
			if event.status == .loading {
				Text(event.message)
					.phaseAnimator([1.0, 0.5]) { content, opacity in
						content.opacity(opacity)
					} animation: { _ in
						.linear(duration: 0.8)
					}
			} else {
				Text(event.message)
			}
		}
		.foregroundStyle(Color.Foreground.slightlyMuted)
	}
}

#Preview {
	ScrollView {
		EventStackView(
			viewModel: EventStackViewModel(events: [
				.init(id: 1, status: .success, message: "You threw a Pokéball..."),
				.init(id: 2, status: .failure, message: "Oh no! The Pokémon broke free!"),
				.init(id: 3, status: .success, message: "You threw another Pokéball..."),
				.init(
					id: 4,
					status: .warning,
					message: "Shoot! It was so close, too! Some really really long text that will wrap even in small font sizes.",
				),
				.init(id: 5, status: .success, message: "You threw yet another Pokéball..."),
				.init(id: 6, status: .loading, message: "*wobble*"),
			]),
		)
		.padding(.horizontal)
	}
}
