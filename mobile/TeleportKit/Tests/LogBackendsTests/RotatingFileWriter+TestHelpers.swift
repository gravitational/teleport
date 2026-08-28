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

@testable import LogBackends

enum PendingItemSnapshot: Equatable {
	case logMessage(String)
	case droppedRecordNotice(Int)
	case flush
}

extension RotatingFileWriter.Inbox {
	/// Drains the inbox into an array whose values can be compared in tests.
	mutating func collectPendingItems() -> [PendingItemSnapshot] {
		beginProcessing()
		return collectPendingItemsAfterProcessingBegan()
	}

	/// Drains the inbox after a test has already started processing and optionally removed one or more items.
	mutating func collectPendingItemsAfterProcessingBegan() -> [PendingItemSnapshot] {
		var collectedItems: [PendingItemSnapshot] = []
		while let item = nextPendingItem() {
			switch item {
				case let .logMessage(record):
					collectedItems.append(.logMessage(record))
				case let .droppedRecordNotice(count):
					collectedItems.append(.droppedRecordNotice(count))
				case .flush:
					collectedItems.append(.flush)
			}
		}

		return collectedItems
	}
}
