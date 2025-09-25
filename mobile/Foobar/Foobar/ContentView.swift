//
//  ContentView.swift
//  Foobar
//
//  Created by Rafał Cieślak on 2025-09-11.
//

import SwiftUI

struct ContentView: View {
  @State private var openedURL: Result<DeepLink, DeepLinkError>?
  @State private var isConfirmingEnrollment: Bool = false
  @ObservedObject private var viewModel: DeviceTrustViewModel

  init(viewModel: DeviceTrustViewModel) {
    self.viewModel = viewModel
  }

  var body: some View {
    ScannedURLView(openedURL: $openedURL)
      .onOpenURL { url in
        switch url.path(percentEncoded: false) {
        case "/enroll_mobile_device":
          openedURL = .success(.enrollMobileDevice(url))
          isConfirmingEnrollment = true
        default:
          openedURL = .failure(.unsupportedURL)
        }
      }
      .onAppear {
        Task { await viewModel.enrollDevice() }
      }
  }
}

enum DeepLink {
  case enrollMobileDevice(URL)
}

enum DeepLinkError: Error {
  case unsupportedURL
}

struct ScannedURLView: View {
  @Binding var openedURL: Result<DeepLink, DeepLinkError>?
  private var isConfirmingEnrollment: Bool {
    if case .success = openedURL { return true }
    return false
  }

  private var maybeOpenedURL: URL? {
    switch openedURL {
    case let .success(deepLink):
      switch deepLink {
      case let .enrollMobileDevice(url):
        url
      }
    default:
      nil
    }
  }

  private var hasDeepLinkError: Bool {
    if case .failure = openedURL { return true }
    return false
  }

  var body: some View {
    VStack(spacing: 16) {
      VStack {
        Image("logo")
          .resizable()
          .aspectRatio(contentMode: .fit)
          .padding(.horizontal, 16)
          .frame(height: 60)
      }.padding(.top, 16)
      Spacer()
      VStack {
        VStack(alignment: .leading, spacing: 4) {
          Label(
            "Open Account Settings in the\u{00a0}Teleport Web\u{00a0}UI.",
            systemImage: "1.circle"
          )
          Label("Select Enroll Mobile Device.", systemImage: "2.circle")
          Label("Scan the QR code with the camera app.", systemImage: "3.circle")
        }
      }.padding(8)
      Spacer()
      Spacer()
    }.sheet(isPresented: .constant(isConfirmingEnrollment), onDismiss: { openedURL = nil }) {
      VStack(spacing: 8) {
        HStack {
          Button("Cancel", role: .cancel) { openedURL = nil }
          Spacer()
          Button("Enroll", systemImage: "progress.indicator") {}.symbolEffect(
            .variableColor.iterative,
            options: .repeat(.continuous),
            isActive: true
          ).buttonRepeatBehavior(.disabled)
        }.padding(8)
        Spacer()
        Text("Do you want to enroll this device?").font(.headline)
        Text("""
        This will enable \(maybeOpenedURL!
          .user(percentEncoded: false) ?? "") to authorize Device Trust web sessions \
        in \(maybeOpenedURL!.host(percentEncoded: false) ?? "") with this device.
        """)
        Spacer()
      }.presentationDetents([.medium]).padding(16).presentationCompactAdaptation(.sheet)
    }
    .alert("Cannot open the link", isPresented: .constant(hasDeepLinkError)) {
      Button("Dismiss", role: .cancel) {}
    } message: {
      switch openedURL {
      case let .failure(failure):
        switch failure {
        case .unsupportedURL:
          Text("""
          Either this version of the app does not support the action represented by this link \
          or the link is malformed.
          """)
        }
      default: EmptyView()
      }
    }
  }
}

#Preview("No URL") {
  ScannedURLView(openedURL: .constant(nil))
}

#Preview("URL") {
  ScannedURLView(
    openedURL: .constant(
      .success(.enrollMobileDevice(
        URL(string: "teleport://alice@example.com/enroll_mobile_device?user_token=1234")!
      ))
    )
  )
}

#Preview("URL with email username") {
  ScannedURLView(
    openedURL: .constant(
      .success(.enrollMobileDevice(
        URL(
          string: "teleport://alice%40example.com@example.com/enroll_mobile_device?user_token=1234"
        )!
      ))
    )
  )
}

#Preview("Deep link error") {
  ScannedURLView(
    openedURL: .constant(.failure(.unsupportedURL))
  )
}

final class DeviceTrustViewModel: ObservableObject {
  private let client: Teleport_Devicetrust_V1_DeviceTrustServiceClientInterface

  init(client: Teleport_Devicetrust_V1_DeviceTrustServiceClientInterface) {
    self.client = client
  }

  func enrollDevice() async {
    let request = Teleport_Devicetrust_V1_PingRequest()
    print("Sending ping")
    let response = await client.ping(request: request, headers: [:])
    print("Sent ping: \(response)")

    print("Starting a stream")
    let stream = client.enrollDevice(headers: [:])
    do {
      print("Sending a message over the stream")
      try stream
        .send(Teleport_Devicetrust_V1_EnrollDeviceRequest
          .with { $0.init_p = Teleport_Devicetrust_V1_EnrollDeviceInit() })
      print("Sent a message over the stream")
    } catch {
      print("Failed to send a message over the stream: \(error)")
      return
    }

    var streamMessage: Teleport_Devicetrust_V1_EnrollDeviceResponse?
    print("Waiting for message from the stream")
    resultsLoop: for await result in stream.results() {
      switch result {
      case let .message(message):
        streamMessage = message
        break resultsLoop
      case let .complete(code, error, _):
        print("Stream ended with code: \(code), error: \(error?.localizedDescription ?? "none")")
      default: ()
      }
    }
    print("Got message: \(streamMessage, default: "no message")")
  }
}
