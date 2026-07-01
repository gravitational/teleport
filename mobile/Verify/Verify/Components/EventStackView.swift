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

struct EventStackView<ID: Hashable>: View {
	struct Event: Identifiable {
		var id: ID
		var status: Status
		var message: String
	}

	// swiftformat:sort
	enum Status {
		case failure
		case loading
		case success
		case warning
	}

	let events: [Event]

	@ScaledMetric
	private var barHeight: CGFloat = 14 // hardcode the point size of SwiftUI's body font

	@ScaledMetric
	private var leadingPadding: CGFloat = .small

	var body: some View {
		VStack(alignment: .leading, spacing: .xxsmall) {
			ForEach(events) { event in
				Label {
					if event.status == .loading {
						Text(event.message)
							.foregroundStyle(Color.Foreground.slightlyMuted)
							.phaseAnimator([1.0, 0.5]) { content, opacity in
								content.opacity(opacity)
							} animation: { _ in
								.linear(duration: 0.8)
							}
					} else {
						Text(event.message)
							.foregroundStyle(Color.Foreground.slightlyMuted)
					}
				} icon: {
					icon(for: event.status)
				}
				if event.id != events.last?.id {
					Label {
						Text("")
					} icon: {
						Capsule()
							.fill(Color.Foreground.muted)
							.frame(maxWidth: .xxsmall, maxHeight: barHeight)
							.padding(.leading, leadingPadding)
					}
				}
			}
		}
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
}

#Preview {
	EventStackView(events: [
		.init(id: 1, status: .success, message: "You threw a Pokéball..."),
		.init(id: 2, status: .failure, message: "Oh no! The Pokémon broke free!"),
		.init(id: 3, status: .success, message: "You threw another Pokéball..."),
		.init(id: 4, status: .warning, message: "Shoot! It was so close, too!"),
		.init(id: 5, status: .success, message: "You threw yet another Pokéball..."),
		.init(id: 6, status: .loading, message: "*wobble*"),
	])
}
