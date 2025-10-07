//
//  ContentView.swift
//  Foobar
//
//  Created by Rafał Cieślak on 2025-09-11.
//

import os
import SwiftUI

let logger = Logger(
  subsystem: Bundle.main.bundleIdentifier ?? "Foobar",
  category: String(describing: ContentView.self)
)

struct ContentView: View {
  @State private var openedURL: DeepLinkParseResult?
  @State private var isConfirmingEnrollment: Bool = false
  @State private var enrollAttempt: Attempt<String, EnrollError> = .idle
  @State private var serialNumber: String?
  private var deviceTrust: DeviceTrustP

  init(deviceTrust: DeviceTrust) {
    self.deviceTrust = deviceTrust
  }

  var body: some View {
    ScannedURLView(
      openedURL: $openedURL,
      enrollAttempt: $enrollAttempt,
      serialNumber: $serialNumber,
      deviceTrust: deviceTrust
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

enum EnrollError: Error & Equatable {
  case unknownError(Error)

  static func == (lhs: EnrollError, rhs: EnrollError) -> Bool {
    if case let .unknownError(lhsError) = lhs,
       case let .unknownError(rhsError) = rhs
    {
      return (lhsError as NSError) === (rhsError as NSError)
    }
    return false
  }
}

typealias DeepLinkParseResult = Result<ParsedDeepLink, DeepLinkParseError>

struct ScannedURLView: View {
  @Binding var openedURL: Result<ParsedDeepLink, DeepLinkParseError>?
  @Binding var enrollAttempt: Attempt<String, EnrollError>
  @Binding var serialNumber: String?
  @State var deleteDeviceKeyAttempt: Attempt<Bool, SecOSStatusError> = .idle
  @State var showDeleteDeviceKeyAlert: Bool = false
  let deviceTrust: DeviceTrustP

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
        serialNumber = deviceTrust.getSerialNumber()
      }.alert(isPresented: .constant(serialNumber != nil)) {
        if let serialNumber {
          Alert(title: Text("Serial Number"), message: Text(serialNumber), dismissButton: nil)
        } else {
          Alert(title: Text("Expected serial number to not be nil"))
        }
      }.glassProminentButton()

      Button("Delete Device Key", role: .destructive) {
        if !deleteDeviceKeyAttempt.isIdle {
          return
        }
        deleteDeviceKeyAttempt = .loading
        showDeleteDeviceKeyAlert = false
        Task {
          switch await deviceTrust.deleteDeviceKey() {
          case let .success(deleted):
            deleteDeviceKeyAttempt = .success(deleted)
          case let .failure(error):
            deleteDeviceKeyAttempt = .failure(error)
          }
          showDeleteDeviceKeyAlert = true
        }
      }.glassProminentButton().alert(
        "Device Key Deletion",
        isPresented: $showDeleteDeviceKeyAlert,
        presenting: deleteDeviceKeyAttempt
      ) { _ in
        Button("OK") {
          deleteDeviceKeyAttempt = .idle
        }
      } message: { attempt in
        switch attempt {
        case let .success(deleted):
          Text(deleted ? "Device key deleted." : "Device key was already deleted.")
        case let .failure(error):
          Text("Failed to delete device key: \(error.description)")
        default:
          EmptyView()
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
                do {
                  // TODO: Cancel this somehow?
                  try await deviceTrust.enrollDevice(
                    hostname: deepURL.url.hostname,
                    port: deepURL.url.port,
                    user: deepURL.url.user,
                    userToken: deepURL.userToken
                  )
                } catch {
                  enrollAttempt = .failure(.unknownError(error))
                  return
                }
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
          if case let .failure(.unknownError(error)) = enrollAttempt {
            Text("Could not enroll this device").font(.headline)
            Text("There was an error: \(error.localizedDescription)")
          } else {
            Text("Do you want to enroll this device?").font(.headline)
            Text("""
            This will enable \(maybeDeepURL!.url.user) to authorize Device Trust web sessions \
            in \(maybeDeepURL!.url.hostname) with this device.
            """)
          }
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
    deviceTrust: DeviceTrust()
  )
}

#Preview("Serial number") {
  ScannedURLView(
    openedURL: .constant(nil),
    enrollAttempt: .constant(.idle),
    serialNumber: .constant("CF123458"),
    deviceTrust: DeviceTrust()
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
    deviceTrust: DeviceTrust()
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
    deviceTrust: DeviceTrust()
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
    deviceTrust: DeviceTrust()
  )
}

#Preview("Deep link error") {
  ScannedURLView(
    openedURL: .constant(.failure(.unsupportedPath)),
    enrollAttempt: .constant(.idle),
    serialNumber: .constant(nil),
    deviceTrust: DeviceTrust()
  )
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
