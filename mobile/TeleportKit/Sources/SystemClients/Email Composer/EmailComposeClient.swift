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

import DependenciesMacros
#if canImport(MessageUI)
	import MessageUI
#endif

/// Provides information about whether the system email composition interface is available.
@DependencyClient
public struct EmailComposeClient: Sendable {
	/// Returns whether the current device is configured to send email.
	public var isAvailable: @MainActor @Sendable () -> Bool = { false }
}

extension EmailComposeClient {
	public static let liveValue = EmailComposeClient(
		isAvailable: {
			#if canImport(MessageUI)
				MFMailComposeViewController.canSendMail()
			#else
				false
			#endif
		},
	)
}
