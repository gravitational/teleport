//
//  ContentView.swift
//  Foobar
//
//  Created by Rafał Cieślak on 2025-09-11.
//

import os
import SwiftUI
import Tagged

let logger = Logger(
  subsystem: Bundle.main.bundleIdentifier ?? "Foobar",
  category: String(describing: ContentView.self)
)

struct ContentView: View {
  @State private var openedURL: DeepLinkParseResult?
  @State private var isConfirmingEnrollment: Bool = false
  @State private var enrollAttempt: Attempt<String, Error> = .idle
  @State private var serialNumber: String?
  @ObservedObject private var viewModel: DeviceTrustViewModel

  init(viewModel: DeviceTrustViewModel) {
    self.viewModel = viewModel
  }

  var body: some View {
    ScannedURLView(
      openedURL: $openedURL,
      enrollAttempt: $enrollAttempt,
      serialNumber: $serialNumber,
      viewModel: viewModel
    )
    .onOpenURL { url in
      openedURL = parseDeepLink(url)
    }
  }
}

func parseDeepLink(_ url: URL) -> DeepLinkParseResult {
  let path = url.path(percentEncoded: false)
  switch path {
  case "/enroll_mobile_device":
    let maybeParts = URLComponents(string: url.absoluteString)
    guard let parts = maybeParts else {
      return .failure(.urlComponentsFailed)
    }
    guard let host = parts.host, host != "" else {
      return .failure(.missingPart("host"))
    }
    guard let user = parts.user, user != "" else {
      return .failure(.missingPart("user"))
    }
    guard let userToken = parts.queryItems?
      .first(where: { $0.name == "user_token" && $0.value != nil && $0.value != "" })?.value
    else {
      return .failure(.missingPart("user token"))
    }
    return .success(.enrollMobileDevice(EnrollMobileDeviceDeepURL(
      url: DeepURL(host: host, user: user),
      userToken: userToken
    )))
  default:
    logger.warning("Unsupported path: \(path, privacy: .public)")
    return .failure(.unsupportedPath)
  }
}

struct DeepURL {
  /// host is the hostname plus the port.
  var host: String
  /// user is percent-decoded username from the URL.
  var user: String
}

struct EnrollMobileDeviceDeepURL {
  var url: DeepURL
  /// user_token query params from the deep link.
  var userToken: String
}

enum ParsedDeepLink {
  case enrollMobileDevice(EnrollMobileDeviceDeepURL)
}

enum DeepLinkParseError: Error {
  case unsupportedPath
  case urlComponentsFailed
  case missingPart(String)
}

typealias DeepLinkParseResult = Result<ParsedDeepLink, DeepLinkParseError>

struct ScannedURLView: View {
  @Binding var openedURL: Result<ParsedDeepLink, DeepLinkParseError>?
  @Binding var enrollAttempt: Attempt<String, Error>
  @Binding var serialNumber: String?
  let viewModel: DeviceTrustViewModel

  private var maybeDeepURL: EnrollMobileDeviceDeepURL? {
    if case let .success(parsedDeepLink) = openedURL,
       case let .enrollMobileDevice(deepURL) = parsedDeepLink
    {
      return deepURL
    }
    return nil
  }

  private var isConfirmingEnrollment: Bool {
    maybeDeepURL != nil
  }

  private var hasDeepLinkParseError: Bool {
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
      Button("Show Serial Number") {
        serialNumber = viewModel.getSerialNumber()
      }.alert(isPresented: .constant(serialNumber != nil)) {
        if let serialNumber {
          Alert(title: Text("Serial Number"), message: Text(serialNumber), dismissButton: nil)
        } else {
          Alert(title: Text("Expected serial number to not be nil"))
        }
      }
      Spacer()
    }.sheet(
      isPresented: .constant(isConfirmingEnrollment),
      onDismiss: {
        openedURL = nil
        enrollAttempt = .idle
      },
      content: {
        VStack(spacing: 8) {
          HStack {
            Button("Cancel", role: .cancel) {
              openedURL = nil
              enrollAttempt = .idle
            }
            Spacer()
            Button(action: {
              enrollAttempt = .loading
              Task {
                // await viewModel.enrollDevice()
                enrollAttempt = .success("foo")
              }
            }, label: {
              if enrollAttempt.isLoading {
                Label("Enroll", systemImage: "progress.indicator")
              } else {
                Text("Enroll")
              }
            }).symbolEffect(
              .variableColor.iterative,
              options: .repeat(.continuous),
              isActive: enrollAttempt.isLoading
            ).buttonRepeatBehavior(.disabled)
          }.padding(8)
          Spacer()
          Text("Do you want to enroll this device?").font(.headline)
          Text("""
          This will enable \(maybeDeepURL!.url.user) to authorize Device Trust web sessions \
          in \(maybeDeepURL!.url.host) with this device.
          """)
          Spacer()
        }.presentationDetents([.medium]).padding(16).presentationCompactAdaptation(.sheet)
      }
    )
    .alert("Cannot open the link", isPresented: .constant(hasDeepLinkParseError)) {
      Button("Dismiss", role: .cancel) {}
    } message: {
      switch openedURL {
      case let .failure(failure):
        switch failure {
        case .unsupportedPath:
          Text("This version of the app does not support the action represented by this link.")
        case .urlComponentsFailed:
          Text("The link appears to be malformed and could not be parsed.")
        case let .missingPart(part):
          Text("The \(part) part of the link is missing.")
        }
      default: EmptyView()
      }
    }
  }
}

#Preview("Default") {
  ScannedURLView(
    openedURL: .constant(nil),
    enrollAttempt: .constant(.idle),
    serialNumber: .constant(nil),
    viewModel: DeviceTrustViewModel()
  )
}

#Preview("Serial number") {
  ScannedURLView(
    openedURL: .constant(nil),
    enrollAttempt: .constant(.idle),
    serialNumber: .constant("CF123458"),
    viewModel: DeviceTrustViewModel()
  )
}

#Preview("Enroll prompt") {
  ScannedURLView(
    openedURL: .constant(
      .success(.enrollMobileDevice(
//        teleport://alice@example.com/enroll_mobile_device?user_token=1234
        EnrollMobileDeviceDeepURL(
          url: DeepURL(host: "example.com", user: "alice"),
          userToken: "1234"
        )
      ))
    ),
    enrollAttempt: .constant(.idle),
    serialNumber: .constant(nil),
    viewModel: DeviceTrustViewModel()
  )
}

#Preview("Enroll prompt email username") {
  ScannedURLView(
    openedURL: .constant(
      .success(.enrollMobileDevice(
//        teleport://alice%40example.com@example.com/enroll_mobile_device?user_token=1234
        EnrollMobileDeviceDeepURL(
          url: DeepURL(host: "example.com", user: "alice@example.com"),
          userToken: "1234"
        )
      ))
    ),
    enrollAttempt: .constant(.idle),
    serialNumber: .constant(nil),
    viewModel: DeviceTrustViewModel()
  )
}

#Preview("Enrollment in progress") {
  ScannedURLView(
    openedURL: .constant(
      .success(.enrollMobileDevice(
        EnrollMobileDeviceDeepURL(
          url: DeepURL(host: "example.com", user: "alice@example.com"),
          userToken: "1234"
        )
      ))
    ),
    enrollAttempt: .constant(.loading),
    serialNumber: .constant(nil),
    viewModel: DeviceTrustViewModel()
  )
}

#Preview("Deep link error") {
  ScannedURLView(
    openedURL: .constant(.failure(.unsupportedPath)),
    enrollAttempt: .constant(.idle),
    serialNumber: .constant(nil),
    viewModel: DeviceTrustViewModel()
  )
}

final class DeviceTrustViewModel: ObservableObject {
  func getSerialNumber() -> String {
    UserDefaults.standard.string(forKey: "serialNumber") ?? "Unknown"
  }

  func enrollDevice(host _: String, user _: String, userToken _: String) async {
//    let request = Teleport_Devicetrust_V1_PingRequest()
//    print("Sending ping")
//    let response = await client.ping(request: request, headers: [:])
//    print("Sent ping: \(response)")
//
//    print("Starting a stream")
//    let stream = client.enrollDevice(headers: [:])
//    do {
//      print("Sending a message over the stream")
//      try stream
//        .send(Teleport_Devicetrust_V1_EnrollDeviceRequest
//          .with { $0.init_p = Teleport_Devicetrust_V1_EnrollDeviceInit() })
//      print("Sent a message over the stream")
//    } catch {
//      print("Failed to send a message over the stream: \(error)")
//      return
//    }
//
//    var streamMessage: Teleport_Devicetrust_V1_EnrollDeviceResponse?
//    print("Waiting for message from the stream")
//    resultsLoop: for await result in stream.results() {
//      switch result {
//      case let .message(message):
//        streamMessage = message
//        break resultsLoop
//      case let .complete(code, error, _):
//        print("Stream ended with code: \(code), error: \(error?.localizedDescription ?? "none")")
//      default: ()
//      }
//    }
//    print("Got message: \(streamMessage, default: "no message")")
  }
}

// func collectDeviceData() -> Result<Teleport_Devicetrust_V1_DeviceCollectedData, Error> {
// var cd = Teleport_Devicetrust_V1_DeviceCollectedData()
// TODO: Add iOS.
// cd.osType = .macos
// cd.serialNumber =
// cd.modelIdentifier
//  cd.osVersion
//  cd.osBuild
//  cd.osUsername
//  cd.systemSerialNumber
// }
