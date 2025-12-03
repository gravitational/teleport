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
  @State private var viewModel = FindViewModel()
  @FocusState private var isFocused: Bool

  var body: some View {
    VStack(spacing: 16) {
      VStack {
        Image("logo").resizable().aspectRatio(contentMode: .fit)
          .frame(height: 60)
      }.padding(16)

      switch viewModel.findAttempt {
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
      case let .success(.grpc(response)):
        FindResponseView(response: response)
      case let .success(.webapi(response)):
        WebapiFindResponseView(response: response)
      case let .failure(failure):
        Spacer()
        VStack {
          Text("Could not inspect the cluster").font(.headline)
          Text("There was an error: \(failure.localizedDescription)")
        }.padding(hPadding)
      }

      Spacer()
      if viewModel.canCancel {
        Button("Cancel", role: .destructive) {
          viewModel.cancelFind()
        }.buttonStyle(.bordered)
      }
      TextField("Cluster Address", text: $viewModel.clusterAddress).autocorrectionDisabled(true)
        .keyboardType(.URL).textInputAutocapitalization(.never).focused($isFocused)
        .submitLabel(.send).textFieldStyle(.roundedBorder).padding(16)
        .modifier(ClearButton(text: $viewModel.clusterAddress)).onSubmit {
          viewModel.startFind()
        }
    }
  }
}

// It looks like @preconcurrency on the Ping framework poisons everything else.
// As long as we compare FindResponse values within the main actor (e.g. within SwiftUI views),
// everything should work fine. But the moment we compare it within a background task or a different
// actor,
// we'll get a runtime crash.
// https://forums.swift.org/t/default-main-actor-isolation-and-protocol-inheritance/80419
enum FindResponse: @preconcurrency Equatable, Sendable {
  case grpc(PingFindResponse)
  case webapi(WebapiFindResponse)
}

let GoErrorDomain = "go"

@Observable
class FindViewModel {
  var clusterAddress = ""
  var findAttempt: Attempt<FindResponse, FindError> = .idle
  private var findTask: Task<Void, Never>?

  func startFind() {
    // Retry an attempt on submit if previous one finished.
    if findAttempt.didFinish {
      findAttempt = .idle
    }
    if !findAttempt.isIdle {
      findTask?.cancel()
    }
    findAttempt = .loading
    let ctx = PingContext()!

    findTask = Task {
      defer { ctx.cancel() }

      await withTaskCancellationHandler {
        let ping = PingFindActor()
        do {
          let response = try await ping.find(ctx, proxyServer: clusterAddress)
          if Task.isCancelled { return }

          findAttempt = .success(.grpc(response))
          return
        } catch {
          if Task.isCancelled { return }
          let nsError = error as NSError
          let isPingServiceUnimplemented = nsError.domain == GoErrorDomain && nsError.description
            .contains("unknown service")
          if !isPingServiceUnimplemented {
            findAttempt = .failure(.unknownError(error))
            return
          }
        }

        // If find through gRPC failed because of PingService being unimplemented, fall back to
        // webapi.
        do {
          let response = try await webapiFind(proxyServer: clusterAddress)
          if Task.isCancelled { return }
          findAttempt = .success(.webapi(response))
        } catch {
          if Task.isCancelled { return }
          findAttempt = .failure(.unknownError(error))
        }
      } onCancel: {
        ctx.cancel()
      }
    }
  }

  func cancelFind() {
    findTask?.cancel()
    findAttempt = .idle
  }

  var canCancel: Bool {
    findAttempt.isLoading && findTask != nil && !(findTask?.isCancelled ?? true)
  }
}

// PingFindActor puts the blocking PingFinder.find on a separate queue,
// unblocking the queue which handles UI updates.
// https://stackoverflow.com/questions/72862670/integrate-a-blocking-function-into-swift-async
actor PingFindActor {
  private let queue = DispatchSerialQueue(
    label: Bundle.main.bundleIdentifier! + ".PingFind",
    qos: .userInitiated
  )

  nonisolated var unownedExecutor: UnownedSerialExecutor {
    queue.asUnownedSerialExecutor()
  }

  func find(_ ctx: PingContext, proxyServer: String) throws -> PingFindResponse {
    let finder = PingFinder()!
    return try finder.find(ctx, proxyServer: proxyServer)
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

private let previewResponse: PingFindResponse = {
  let r = PingFindResponse()
  r.clusterName = "example.com"
  r.serverVersion = "18.2.4"
  return r
}()

// TODO(ravicious): This preview just doesn't work. I assume it's due to the @preconcurrency stuff.
#Preview("FindResponseView") {
  FindResponseView(response: previewResponse)
}

struct WebapiFindResponseView: View {
  var response: WebapiFindResponse

  var body: some View {
    ScrollView {
      VStack(alignment: .leading, spacing: 10) {
        Text("Cluster name: \(response.clusterName)")
        Text("Server version: \(response.serverVersion)")
        Text("Min client version: \(response.minClientVersion)")
        Text(
          "TLS routing: \(response.proxy.tlsRoutingEnabled ? "enabled" : "not enabled")"
        )
        Text("Public address: \(response.proxy.ssh.publicAddr)")
        Text("Web listen address: \(response.proxy.ssh.webListenAddr)")
        Text("Listen address: \(response.proxy.ssh.listenAddr)")
        Text("Tunnel listen address: \(response.proxy.ssh.tunnelListenAddr)")
        Text("Edition: \(response.edition)")
        Text("FIPS mode: \(response.fips ? "enabled" : "not enabled")")
      }
    }.padding(hPadding)
  }
}

#Preview("WebapiFindResponseView") {
  @Previewable let response = WebapiFindResponse(
    proxy: ProxySettings(ssh: SSHProxySettings(
      listenAddr: "0.0.0.0:3080",
      tunnelListenAddr: "0.0.0.0:3080",
      webListenAddr: "0.0.0.0:3080",
      publicAddr: "teleport.example.com:443"
    ), tlsRoutingEnabled: true),
    serverVersion: "18.2.4",
    minClientVersion: "17.0.0-aa",
    clusterName: "teleport.example.com",
    edition: "community",
    fips: false
  )
  WebapiFindResponseView(response: response)
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

// MARK: - WebapiFindResponse structs

struct WebapiFindResponse: Codable, Equatable, Sendable {
  let proxy: ProxySettings
  let serverVersion, minClientVersion, clusterName, edition: String
  let fips: Bool

  enum CodingKeys: String, CodingKey {
    case proxy
    case serverVersion = "server_version"
    case minClientVersion = "min_client_version"
    case clusterName = "cluster_name"
    case edition, fips
  }
}

struct ProxySettings: Codable, Equatable, Sendable {
  let ssh: SSHProxySettings
  let tlsRoutingEnabled: Bool

  enum CodingKeys: String, CodingKey {
    case ssh
    case tlsRoutingEnabled = "tls_routing_enabled"
  }
}

struct SSHProxySettings: Codable, Equatable, Sendable {
  let listenAddr, tunnelListenAddr, webListenAddr, publicAddr: String

  enum CodingKeys: String, CodingKey {
    case listenAddr = "listen_addr"
    case tunnelListenAddr = "tunnel_listen_addr"
    case webListenAddr = "web_listen_addr"
    case publicAddr = "public_addr"
  }
}

func webapiFind(proxyServer: String) async throws(WebapiFindError) -> WebapiFindResponse {
  guard let url = URL(string: "https://\(proxyServer)/webapi/find") else {
    throw WebapiFindError.incorrectURL
  }
  let request = URLRequest(url: url)
  let data: Data, response: URLResponse

  do {
    (data, response) = try await URLSession.shared.data(for: request)
  } catch {
    throw .clientError(error)
  }

  guard let httpResponse = response as? HTTPURLResponse,
        (200 ... 299).contains(httpResponse.statusCode)
  else {
    throw .serverError(response)
  }

  do {
    return try JSONDecoder().decode(WebapiFindResponse.self, from: data)
  } catch {
    throw .decodingError(error)
  }
}

enum WebapiFindError: Error & Equatable, LocalizedError {
  case incorrectURL
  case clientError(Error)
  case serverError(URLResponse)
  case decodingError(Error)

  static func == (lhs: WebapiFindError, rhs: WebapiFindError) -> Bool {
    if case .incorrectURL = lhs,
       case .incorrectURL = rhs
    {
      return true
    }

    if case let .clientError(lhsError) = lhs,
       case let .clientError(rhsError) = rhs
    {
      return (lhsError as NSError) === (rhsError as NSError)
    }

    if case let .serverError(lhsResp) = lhs,
       case let .serverError(rhsResp) = rhs
    {
      return lhsResp == rhsResp
    }

    if case let .decodingError(lhsError) = lhs,
       case let .decodingError(rhsError) = rhs
    {
      return (lhsError as NSError) === (rhsError as NSError)
    }

    return false
  }

  var errorDescription: String? {
    switch self {
    case .incorrectURL:
      "The provided proxy URL is incorrect."
    case let .clientError(error):
      (error as NSError).localizedDescription
    case let .serverError(response):
      "The proxy returned an error response: \(response)"
    case let .decodingError(error):
      (error as NSError).localizedDescription
    }
  }
}
