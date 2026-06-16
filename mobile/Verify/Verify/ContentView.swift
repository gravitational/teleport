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

import SwiftUI
import os

private let logger = Logger(
    subsystem: Bundle.main.bundleIdentifier ?? "com.gravitational.verify",
    category: String(describing: ContentView.self)
)

enum AppScreen {
    case landing
    case enroll(EnrollMobileDeviceDeepURL)
}

struct ContentView: View {
    @State private var screen: AppScreen
    @State private var parseError: DeepLinkParseError?

    init(initialScreen: AppScreen = .landing) {
        _screen = State(initialValue: initialScreen)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Image("logo")
                .resizable()
                .scaledToFit()
                .frame(height: 32)
                .padding()
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(Color(.systemBackground))
            Group {
                switch screen {
                case .landing:
                    LandingView()
                case .enroll(let deepURL):
                    EnrollMobileDeviceView(
                        deepURL: deepURL,
                        onCancel: { screen = .landing }
                    )
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
        .onOpenURL { url in
            let result = parseDeepLink(url)
            logger
                .debug(
                    "Got URL: \(url, privacy: .public), parsed: \(String(describing: result), privacy: .public)"
                )
            switch result {
            case .success(.enrollMobileDevice(let deepURL)):
                parseError = nil
                screen = .enroll(deepURL)
            case .failure(let error):
                parseError = error
            }
        }
        .alert(
            "Cannot open the link",
            isPresented: Binding(
                get: { parseError != nil },
                set: { if !$0 { parseError = nil } }
            ),
            presenting: parseError
        ) { _ in
            Button("Dismiss", role: .cancel) {}
        } message: { error in
            switch error {
            case .unsupportedPath:
                Text(
                    "This version of the app does not support the action represented by this link."
                )
            case .urlComponentsFailed:
                Text(
                    "The link appears to be malformed and could not be parsed."
                )
            case .missingPart(let part):
                Text("The \(part) part of the link is missing.")
            }
        }
    }
}

#Preview {
    ContentView()
}
