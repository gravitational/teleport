//
//  ContentView.swift
//  Foobar
//
//  Created by Rafał Cieślak on 2025-09-11.
//

import SwiftUI

struct ContentView: View {
  @State private var openedURL: URL?
  @State private var isConfirmingEnrollment: Bool = false
  @ObservedObject private var viewModel: DeviceTrustViewModel

  init(viewModel: DeviceTrustViewModel) {
    self.viewModel = viewModel
  }

  var body: some View {
    ScannedURLView(openedURL: $openedURL, isConfirmingEnrollment: $isConfirmingEnrollment)
      .onOpenURL { url in
        openedURL = url
        isConfirmingEnrollment = true
      }
      .onAppear {
        Task { await viewModel.enrollDevice() }
      }
  }
}

struct ScannedURLView: View {
  @Binding var openedURL: URL?
  @Binding var isConfirmingEnrollment: Bool

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
    }.confirmationDialog(
      "Do you want to enroll this device?",
      isPresented: $isConfirmingEnrollment,
      titleVisibility: .visible,
      presenting: openedURL
    ) { _ in
      Button("Enroll") {}
      Button("Cancel", role: .cancel) {
        openedURL = nil
      }
    } message: { url in
      Text(
        """
        This will enable \(url
          .user(percentEncoded: false) ?? "") to authorize Device Trust web sessions \
        in \(url.host(percentEncoded: false) ?? "") with this device.
        """
      )
    }
  }
}

#Preview("No URL") {
  ScannedURLView(openedURL: .constant(nil), isConfirmingEnrollment: .constant(false))
}

#Preview("URL") {
  ScannedURLView(
    openedURL: .constant(
      URL(string: "teleport://alice@example.com/enroll_mobile_device?user_token=1234")
    ),
    isConfirmingEnrollment: .constant(true)
  )
}

#Preview("URL with email username") {
  ScannedURLView(
    openedURL: .constant(
      URL(string: "teleport://alice%40example.com@example.com/enroll_mobile_device?user_token=1234")
    ),
    isConfirmingEnrollment: .constant(true)
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
