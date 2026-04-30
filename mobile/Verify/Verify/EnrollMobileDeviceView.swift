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

struct EnrollMobileDeviceView: View {
    let deepURL: EnrollMobileDeviceDeepURL
    let onCancel: () -> Void
    @State private var viewModel: EnrollMobileDeviceViewModel

    init(deepURL: EnrollMobileDeviceDeepURL, onCancel: @escaping () -> Void) {
        self.deepURL = deepURL
        self.onCancel = onCancel
        _viewModel = State(
            initialValue: EnrollMobileDeviceViewModel(deepURL: deepURL)
        )
    }

    var body: some View {
        VStack(spacing: 16) {
            Spacer()
            Image(systemName: "ipad.and.iphone")
                .font(.system(size: 37))
                .foregroundStyle(.primary)
                .frame(width: 75, height: 75)
                .background(
                    RoundedRectangle(cornerRadius: 16)
                        .fill(Color(.systemBackground))
                )
                .overlay(
                    RoundedRectangle(cornerRadius: 16)
                        .strokeBorder(Color(.separator), lineWidth: 1)
                )
            VStack(spacing: 8) {
                Text("Enroll Your Device")
                    .font(.title2)
                    .fontWeight(.semibold)
                Text(
                    "To finish enrolling this device, approve the request from your account settings on another device."
                )
                .multilineTextAlignment(.center)
                .foregroundStyle(.secondary)
            }
            Spacer()
            VStack(spacing: 16) {
                Button {
                    Task { await viewModel.requestEnrollToken() }
                } label: {
                    Group {
                        if viewModel.attempt.isLoading {
                            Label(
                                "Request in progress",
                                systemImage: "progress.indicator"
                            )
                            .labelStyle(.iconOnly)
                            .symbolEffect(
                                .variableColor.iterative,
                                options: .repeat(.continuous),
                                isActive: true
                            )
                        } else {
                            Text("Request Now")
                        }
                    }
                    .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .controlSize(.large)
                .animation(.easeInOut, value: viewModel.attempt.isLoading)
                .disabled(viewModel.attempt.isLoading)

                Button(role: .cancel, action: onCancel) {
                    Text("Cancel").frame(maxWidth: .infinity)
                }
                .buttonStyle(.bordered)
                .controlSize(.large)
                .disabled(viewModel.attempt.isLoading)
            }
        }
        .padding()
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Color(.systemGroupedBackground))
        .alert(
            "Enrollment",
            isPresented: Binding(
                get: {
                    switch viewModel.attempt {
                    case .success, .failure: return true
                    default: return false
                    }
                },
                set: { if !$0 { viewModel.attempt = .idle } }
            )
        ) {
            Button("OK", role: .cancel) {}
        } message: {
            switch viewModel.attempt {
            case .success(let token):
                Text("Got enrollment token: \(token)")
            case .failure(let error):
                Text("Error: \(error.localizedDescription)")
            default:
                EmptyView()
            }
        }
    }
}

#Preview("In ContentView") {
    ContentView(
        initialScreen: .enroll(
            EnrollMobileDeviceDeepURL(
                url: DeepURL(hostname: "example.com", port: 3080),
                enrollPairingToken: "abc123"
            )
        )
    )
}
