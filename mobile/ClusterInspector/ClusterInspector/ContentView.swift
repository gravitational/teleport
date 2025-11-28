// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

//
//  ContentView.swift
//  ClusterInspector
//
//  Created by Rafał Cieślak on 2025-11-28.
//

@preconcurrency import Ping
import SwiftUI

struct ContentView: View {
  @State private var clusterAddress = ""
  @FocusState private var isFocused: Bool
  @State private var findAttempt: Attempt<PingFindResponse, FindError> = .idle

  var body: some View {
    VStack(spacing: 16) {
      VStack {
        Image("logo").resizable().aspectRatio(contentMode: .fit)
          .frame(height: 60)
      }.padding(16)

      switch findAttempt {
      case .idle:
        Spacer()
        Text("Enter the cluster address to inspect its properties.").padding(hPadding)
      case .loading:
        Spacer()
        Image(systemName: "progress.indicator").symbolEffect(
          .variableColor.iterative,
          options: .repeat(.continuous),
          isActive: true
        )
      case let .success(response):
        FindResponseView(response: response)
      case let .failure(failure):
        Spacer()
        VStack {
          Text("Could not inspect the cluster").font(.headline)
          Text("There was an error: \(failure.localizedDescription)")
        }.padding(hPadding)
      }

      Spacer()
      TextField("Cluster Address", text: $clusterAddress).autocorrectionDisabled(true)
        .keyboardType(.URL).textInputAutocapitalization(.never).focused($isFocused)
        .submitLabel(.send).textFieldStyle(.roundedBorder).padding(16)
        .modifier(ClearButton(text: $clusterAddress)).onSubmit {
          // Retry an attempt on submit if previous one finished.
          if findAttempt.didFinish {
            findAttempt = .idle
          }
          // Don't interrupt if an attempt is in progress.
          if !findAttempt.isIdle {
            return
          }
          findAttempt = .loading
          Task {
            let finder = PingFinder()!
            do {
              let response = try finder.find(clusterAddress)
              findAttempt = .success(response)
            } catch {
              findAttempt = .failure(.unknownError(error))
            }
          }
        }
    }
  }
}

let hPadding: CGFloat = 4

#Preview {
  ContentView()
}

struct FindResponseView: View {
  var response: PingFindResponse

  var body: some View {
    ScrollView {
      VStack(alignment: .leading, spacing: 10) {
        Text("Cluster name: \(response.clusterName)")
        Text("Server version: \(response.serverVersion)")
        Text("Min client version: \(response.minClientVersion)")
        Text(
          "TLS routing: \(response.proxy?.tlsRoutingEnabled ?? false ? "enabled" : "not enabled")"
        )
        OptionalField(label: "Public address", value: response.proxy?.sshProxySettings?.publicAddr)
        OptionalField(
          label: "SSH public address",
          value: response.proxy?.sshProxySettings?.sshPublicAddr
        )
        OptionalField(
          label: "Tunnel public address",
          value: response.proxy?.sshProxySettings?.tunnelPublicAddr
        )
        OptionalField(
          label: "Web listen address",
          value: response.proxy?.sshProxySettings?.webListenAddr
        )
        OptionalField(label: "Listen address", value: response.proxy?.sshProxySettings?.listenAddr)
        OptionalField(
          label: "Tunnel listen address",
          value: response.proxy?.sshProxySettings?.tunnelListenAddr
        )
        Text("Edition: \(response.edition)")
        Text("FIPS mode: \(response.fips ? "enabled" : "not enabled")")
      }
    }.padding(hPadding)
  }
}

struct OptionalField: View {
  var label: String
  var value: String?

  var body: some View {
    if value != nil, !value!.isEmpty {
      Text("\(label): \(value!)")
    }
  }
}

#Preview("FindResponseView") {
  let response = PingFindResponse()
  response.clusterName = "example.com"
  response.serverVersion = "18.2.4"
  return FindResponseView(response: response)
}

enum FindError: Error & Equatable, LocalizedError {
  case unknownError(Error)

  static func == (lhs: FindError, rhs: FindError) -> Bool {
    if case let .unknownError(lhsError) = lhs,
       case let .unknownError(rhsError) = rhs
    {
      return (lhsError as NSError) === (rhsError as NSError)
    }
    return false
  }

  var errorDescription: String? {
    switch self {
    case let .unknownError(error):
      (error as NSError).localizedDescription
    }
  }
}

struct ClearButton: ViewModifier {
  @Binding var text: String

  func body(content: Content) -> some View {
    HStack {
      content

      if !text.isEmpty {
        Button(action: {
          text = ""
        }) {
          Image(systemName: "xmark.circle.fill")
            .foregroundColor(Color(UIColor.opaqueSeparator))
            .padding(.trailing, 8)
            .padding(.vertical, 8)
            .contentShape(Rectangle())
        }
      }
    }
  }
}
