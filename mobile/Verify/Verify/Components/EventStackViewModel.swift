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

import Dependencies
import Foundation
import IdentifiedCollections
import Observation

@Observable @MainActor
final class EventStackViewModel<ID: Hashable> {
	private(set) var events: IdentifiedArrayOf<Event> = []

	init(events: IdentifiedArrayOf<Event> = []) {
		self.events = events
	}
}

// MARK: - Event Model

extension EventStackViewModel {
	/// A single event to be displayed inside an EventStackView.
	struct Event: Identifiable {
		var id: ID
		var status: Status
		var message: String
	}

	// swiftformat:sort
	/// The status of a particular event, which will determine its icon and UI treatment.
	enum Status {
		case failure
		case loading
		case success
		case warning
	}
}

// MARK: - Event Manipulation

extension EventStackViewModel {
	func addEvent(id: ID, message: String, status: Status = .loading) {
		events.append(Event(id: id, status: status, message: message))
	}

	func updateEvent(id: ID, message: String? = nil, status: Status? = nil) {
		guard var event = events[id: id] else { return }
		if let status {
			event.status = status
		}
		if let message {
			event.message = message
		}
		events[id: id] = event
	}

	func clearAllEvents() {
		events = []
	}
}
