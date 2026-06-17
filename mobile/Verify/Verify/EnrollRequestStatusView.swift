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

struct EnrollRequestStatusView: View {
    let attempt: EnrollMobileDeviceViewModel.Attempt
    let onDismiss: () -> Void

    var body: some View {
        VStack(spacing: 24) {
            Spacer()
            Image(systemName: iconName)
                .font(.system(size: 60))
                .foregroundStyle(.primary)
                .contentTransition(.symbolEffect(.replace))
            VStack(spacing: 8) {
                Text(title)
                    .font(.title2)
                    .fontWeight(.semibold)
                    .contentTransition(.opacity)
                Text(message)
                    .multilineTextAlignment(.center)
                    .foregroundStyle(.secondary)
                    .contentTransition(.opacity)
            }
            Spacer()
            switch attempt {
            case .success:
                Button {
                    // TODO: Navigate to the cluster.
                } label: {
                    HStack {
                        Text("Go to Cluster")
                        Image(systemName: "arrow.right")
                    }
                    .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .controlSize(.large)
            case .failure:
                Button("Dismiss", action: onDismiss)
                    .buttonStyle(.bordered)
                    .controlSize(.large)
                    .frame(maxWidth: .infinity)
            case .idle, .loading:
                EmptyView()
            }
        }
        .padding()
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .animation(.default, value: iconName)
    }

    private var iconName: String {
        switch attempt {
        case .idle, .loading: "clock"
        case .success: "checkmark.circle"
        case .failure: "exclamationmark.triangle"
        }
    }

    private var title: String {
        switch attempt {
        case .idle, .loading: "Request Sent"
        case .success: "Request Approved"
        case .failure: "Request Failed"
        }
    }

    private var message: String {
        switch attempt {
        case .idle, .loading:
            "Your enrollment request has been sent. Approve it from the Web UI to continue."
        case .success:
            "This device is now trusted and can access protected resources."
        case .failure(let error):
            error.localizedDescription
        }
    }
}

#Preview("Request Sent") {
    EnrollRequestStatusView(attempt: .loading, onDismiss: {})
}

#Preview("Request Approved") {
    EnrollRequestStatusView(
        attempt: .success(token: "demo-placeholder-token"),
        onDismiss: {}
    )
}

#Preview("Request Failed") {
    struct DemoError: LocalizedError {
        var errorDescription: String? { "Network unreachable" }
    }
    return EnrollRequestStatusView(
        attempt: .failure(DemoError()),
        onDismiss: {}
    )
}
