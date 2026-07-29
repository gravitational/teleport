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

/// For the purposes of presentation, we often need some kind of identifier which will distinguish between two view
/// models of the same type. This protocol formalizes the concept and gives us a place to put convenience accessors
/// and documentation.
protocol SheetPresentable {
	associatedtype PresentationID: Hashable
	/// The ID to use for presentation purposes in things like `.sheet` SwiftUI modifiers
	///
	/// If there is no real distinction between instances of a particular type, consider supplying a constant value.
	var presentationID: PresentationID { get }
}
