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
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

import Dependencies
import SQLiteData
import SwiftUI

/// This type is the entrypoint of our app.
///
/// Roughly speaking, the core of our app can be broken into two main subtrees: a view tree and a view model tree. They
/// mostly mirror each other (though it's possible and reasonable to have views that don't have view models). In
/// general, the view model tree attempts to be the stateful source of truth for what the views should do, while the
/// views attempt to be as dumb as possible and simply display the stuff the view models tell them too. This ensures
/// that:
///
/// 1. We always have programmatic control over what the views are displaying.
/// 2. Much more of our code is testable (since the behavior all lives in view models and SwiftUI views can't be
/// tested).
///
/// This type, by serving as the entrypoint into our app, also serves as the origin of both the view and view model
/// trees. However, for the sake of symmetry, we have ``VerifyAppModel`` type that will serve as the actual place we
/// initialize our root-most view models. We also have it because it's useful to have a reference type that shares your
/// app's lifecycle, which we accomplish by marking the `appModel` property here with `@State`.
///
/// With this understanding, we can construct the following rough sketch of how the iOS app is structured.
///
/// - Important: The below diagram is meant for illustrative purposes only and doesn't reflect the constantly evolving
/// nature of the app. Types you see here are not meant to reflect the live structure of the app, but merely its shape.
///
/// ```
///      ┌─────────────┐                    ┌──────────────────┐
///      │  VerifyApp  │───────Owns────────▶│  VerifyAppModel  │
///      └─────────────┘                    └──────────────────┘
///             │                            │                ▲
///             │                            │                │
///             │                            │
///             │                            │                │
///           Owns                         Owns           Delegate
///             │                            │
///             │                            │                │
///             │                            │
///             ▼                            ▼                │
///     ┌───────────────┐                  ┌────────────────────┐
///     │  LandingView  │───References────▶│  LandingViewModel  │
///     └───────────────┘                  └────────────────────┘
///             │                            │                ▲
///             │                            │
///             │                            │                │
///             │                            │
///             ▼                            ▼                │
///           etc.                                   etc.
/// ```
@main
struct VerifyApp: App {
	@State
	private var appModel: VerifyAppModel

	init() {
		prepareDependencies {
			$0.defaultDatabase = AppDatabase.makeLiveDatabase()
		}
		// Only initialize the model after our app's dependencies have been prepared
		appModel = VerifyAppModel()
	}

	var body: some Scene {
		WindowGroup {
			LandingView(viewModel: appModel.landingViewModel)
				.tint(.teleport)
				.onOpenURL { url in
					appModel.openDeepLink(url)
				}
		}
	}
}
