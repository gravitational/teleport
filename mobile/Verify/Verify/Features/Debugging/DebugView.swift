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

#if DEBUG

	import SwiftUI

	/// DebugView serves as the entrypoint for in-development ideas, UI component catalogs, and any other bits that we
	/// want to be able to see when testing on a device, but which we don't want to ship to customers.
	///
	/// - Note: DebugView and all its associated components should always be wrapped in `#if DEBUG` checks.
	struct DebugView: View {
		var viewModel: DebugViewModel

		var body: some View {
			Text("Placeholder for DebugView")
		}
	}

#endif
