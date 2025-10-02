//
//  ContentView.swift
//  Foobar
//
//  Created by Rafał Cieślak on 2025-09-11.
//

import Connect
import ConnectNIO
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
  @State private var enrollAttempt: Attempt<String, EnrollError> = .idle
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
    guard let hostname = parts.host, hostname != "" else {
      return .failure(.missingPart("hostname"))
    }
    guard let user = parts.user, user != "" else {
      return .failure(.missingPart("user"))
    }
    guard let userToken = parts.queryItems?
      .first(where: { $0.name == "user_token" && $0.value != nil && $0.value != "" })?.value
    else {
      return .failure(.missingPart("user token"))
    }
    let port = parts.port
    return .success(.enrollMobileDevice(EnrollMobileDeviceDeepURL(
      url: DeepURL(hostname: hostname, port: port, user: user),
      userToken: userToken
    )))
  default:
    logger.warning("Unsupported path: \(path, privacy: .public)")
    return .failure(.unsupportedPath)
  }
}

struct DeepURL {
  /// hostname is just the name of the host without the port.
  var hostname: String
  // port of the host.
  var port: Int?
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

enum EnrollError: Error {
  case unknown
}

typealias DeepLinkParseResult = Result<ParsedDeepLink, DeepLinkParseError>

struct ScannedURLView: View {
  @Binding var openedURL: Result<ParsedDeepLink, DeepLinkParseError>?
  @Binding var enrollAttempt: Attempt<String, EnrollError>
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
      }.glassProminentButton()
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
            Button("Cancel", systemImage: "xmark", role: .cancel) {
              openedURL = nil
              enrollAttempt = .idle
            }.glassButton().labelStyle(.iconOnly)
            Spacer()
            Button(action: {
              guard let deepURL = maybeDeepURL else {
                return
              }
              if enrollAttempt.didSucceed {
                openedURL = nil
                enrollAttempt = .idle
                return
              }
              if !enrollAttempt.isIdle {
                return
              }
              enrollAttempt = .loading
              Task {
                await viewModel.enrollDevice(
                  hostname: deepURL.url.hostname,
                  port: deepURL.url.port,
                  user: deepURL.url.user,
                  userToken: deepURL.userToken
                )
                try await Task.sleep(for: .seconds(3))
                enrollAttempt = .success("foo")
              }
            }, label: {
              switch enrollAttempt {
              case .idle:
                Text("Enroll")
              case .loading:
                Label("Enrollment in progress", systemImage: "progress.indicator")
                  .labelStyle(.iconOnly).symbolEffect(
                    .variableColor.iterative,
                    options: .repeat(.continuous),
                    isActive: true
                  )
              case .success:
                Label("Enrolled", systemImage: "checkmark").labelStyle(.iconOnly)
              case .failure:
                Label("Enroll error", systemImage: "xmark.octagon").labelStyle(.iconOnly)
              }
            }).glassProminentButton().animation(.easeInOut, value: enrollAttempt)
          }.padding(8).controlSize(.large)
          Spacer()
          Text("Do you want to enroll this device?").font(.headline)
          Text("""
          This will enable \(maybeDeepURL!.url.user) to authorize Device Trust web sessions \
          in \(maybeDeepURL!.url.hostname) with this device.
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
          url: DeepURL(hostname: "example.com", user: "alice"),
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
          url: DeepURL(hostname: "example.com", user: "alice@example.com"),
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
          url: DeepURL(hostname: "example.com", user: "alice@example.com"),
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

  func enrollDevice(hostname: String, port: Int?, user _: String, userToken _: String) async {
    let hostnameWithScheme = "https://\(hostname)"
    let hostnameWithPort = "\(hostnameWithScheme)\(port.map { ":\($0)" } ?? "")"
    let protocolClient = ProtocolClient(
      httpClient: NIOHTTPClient(host: hostnameWithScheme, port: port, timeout: nil),
      config: ProtocolClientConfig(
        host: "\(hostnameWithPort)/webapi/devicetrust/",
        networkProtocol: .connect,
        codec: ProtoCodec(),
      )
    )
    let client = Teleport_Devicetrust_V1_DeviceTrustServiceClient(client: protocolClient)
    let cd = collectDeviceData()
    print("\(cd)")
    let request = Teleport_Devicetrust_V1_PingRequest()
    print("Sending ping")
    let response = await client.ping(request: request, headers: [:])
    print("Got ping response: \(response)")
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

func collectDeviceData() -> Teleport_Devicetrust_V1_DeviceCollectedData {
  let device = UIDevice.current
  var cd = Teleport_Devicetrust_V1_DeviceCollectedData()
  let serialNumber = "FOO1234"
  cd.osType = .macos
  cd.serialNumber = serialNumber
  cd.modelIdentifier = getDeviceCode() ?? ""
  cd.osVersion = device.systemVersion
  cd.osBuild = device.systemVersion.split(separator: ".", maxSplits: 4).dropFirst(2).first
    .map(String.init) ?? ""
  // cd.osUsername
  cd.systemSerialNumber = serialNumber
  return cd
}

func getDeviceCode() -> String? {
  var systemInfo = utsname()
  uname(&systemInfo)
  let modelCode = withUnsafePointer(to: &systemInfo.machine) {
    $0.withMemoryRebound(to: CChar.self, capacity: 1) {
      ptr in String(validatingUTF8: ptr)
    }
  }
  return modelCode
}

struct GlassButtonModifier: ViewModifier {
  func body(content: Content) -> some View {
    if #available(iOS 26.0, *) {
      content.buttonStyle(.glass)
    } else {
      content.buttonStyle(.bordered)
    }
  }
}

struct GlassProminentButtonModifier: ViewModifier {
  func body(content: Content) -> some View {
    if #available(iOS 26.0, *) {
      content.buttonStyle(.glassProminent)
    } else {
      content.buttonStyle(.borderedProminent)
    }
  }
}

extension View {
  func glassButton() -> some View {
    modifier(GlassButtonModifier())
  }

  func glassProminentButton() -> some View {
    modifier(GlassProminentButtonModifier())
  }
}
