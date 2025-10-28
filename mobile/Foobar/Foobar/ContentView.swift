//
//  ContentView.swift
//  Foobar
//
//  Created by Rafał Cieślak on 2025-09-11.
//

import Authn
import os
import SwiftUI

private let logger = Logger(
  subsystem: Bundle.main.bundleIdentifier ?? "Foobar",
  category: String(describing: ContentView.self)
)

struct ContentView: View {
  @State private var openedURL: DeepLinkParseResult?
  @State private var isConfirmingEnrollment: Bool = false
  @State private var enrollAttempt: Attempt<Never?, EnrollError> = .idle
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
      // TODO: This must reset the state of ScannedURLView. If there's an auth attempt in progress,
      // opening a new URL through the app is not going to cancel the attempt in progress.
      openedURL = parseDeepLink(url)
      logger
        .debug(
          "Got URL: \(url, privacy: .public), parsed: \(openedURL.debugDescription, privacy: .public)"
        )
    }
    .onAppear {
      Task {
        let greeter = AuthnGreeter()!
        logger.debug("Attempting to send a ping")
        do {
          try greeter.ping()
          logger.debug("Ping successful")
        } catch {
          logger.error("Got an error: \(error)")
        }
      }
    }
  }
}

func parseDeepLink(_ url: URL) -> DeepLinkParseResult {
  let path = url.path(percentEncoded: false)
  switch path {
  case "/enroll_mobile_device":
    // Make sure to first switch on the path and only then attempt to parse out individual fields
    // out of
    // the URL. This way this function always returns the most important error first, which is the
    // error
    // about the unsupported path.
    return parseDeepURL(url) { deepURL, parts in
      guard let userToken = getQueryParam(parts, "user_token") else {
        return .failure(.missingPart("user token"))
      }
      return .success(.enrollMobileDevice(EnrollMobileDeviceDeepURL(
        url: deepURL,
        userToken: userToken
      )))
    }
  case "/authenticate_web_device":
    return parseDeepURL(url) { deepURL, parts in
      guard let id = getQueryParam(parts, "id") else {
        return .failure(.missingPart("id"))
      }
      guard let token = getQueryParam(parts, "token") else {
        return .failure(.missingPart("token"))
      }
      let redirectURI = getQueryParam(parts, "redirect_uri") ?? ""
      return .success(.authenticateWebDevice(AuthenticateWebDeviceDeepURL(
        url: deepURL,
        id: id,
        token: token,
        redirectURI: redirectURI
      )))
    }
  default:
    logger.warning("Unsupported path: \(path, privacy: .public)")
    return .failure(.unsupportedPath)
  }
}

// parseDeepURL could return Result<(DeepURL, URLComponents), DeepLinkParseError> instead, but
// there's no
// convienient way to unwrap it, that's why it accepts a trailing closure instead.
func parseDeepURL(
  _ url: URL,
  toParsedDeepLink: (_ deepURL: DeepURL, _ parts: URLComponents) -> DeepLinkParseResult
) -> DeepLinkParseResult {
  guard let parts = URLComponents(string: url.absoluteString) else {
    return .failure(.urlComponentsFailed)
  }
  guard let hostname = parts.host, hostname != "" else {
    return .failure(.missingPart("hostname"))
  }
  guard let user = parts.user, user != "" else {
    return .failure(.missingPart("user"))
  }
  let port = parts.port

  return toParsedDeepLink(DeepURL(hostname: hostname, port: port, user: user), parts)
}

func getQueryParam(_ parts: URLComponents, _ param: String) -> String? {
  parts.queryItems?
    .first(where: { $0.name == param && $0.value != nil && $0.value != "" })?.value
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

struct AuthenticateWebDeviceDeepURL {
  var url: DeepURL
  /// id is the ID of the device web token.
  var id: String
  /// token is the device web token.
  var token: String
  var redirectURI: String
}

enum ParsedDeepLink {
  case enrollMobileDevice(EnrollMobileDeviceDeepURL)
  case authenticateWebDevice(AuthenticateWebDeviceDeepURL)
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

enum AuthError: Error & Equatable {
  case unknownError(Error)

  static func == (lhs: AuthError, rhs: AuthError) -> Bool {
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
  // TODO: EnrollAttempt should be a @State not a @Binding, see authAttempt.
  @Binding var enrollAttempt: Attempt<Never?, EnrollError>
  @Binding var serialNumber: String?
  @State var deleteDeviceKeyAttempt: Attempt<Bool, SecOSStatusError> = .idle
  @State var showDeleteDeviceKeyAlert: Bool = false
  let deviceTrust: DeviceTrustP

  private var maybeEnrollMobileDeviceURL: EnrollMobileDeviceDeepURL? {
    if case let .success(.enrollMobileDevice(deepURL)) = openedURL {
      return deepURL
    }
    return nil
  }

  private var isConfirmingEnrollment: Bool {
    maybeEnrollMobileDeviceURL != nil
  }

  @State var authAttempt: Attempt<Never?, AuthError> = .idle
  private var maybeAuthenticateWebDeviceURL: AuthenticateWebDeviceDeepURL? {
    if case let .success(.authenticateWebDevice(deepURL)) = openedURL {
      return deepURL
    }
    return nil
  }

  private var isAuthenticatingWebDevice: Bool {
    maybeAuthenticateWebDeviceURL != nil
  }

  private var hasDeepLinkParseError: Bool {
    if case .failure = openedURL { return true }
    return false
  }

  @Environment(\.openURL) private var openURL

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

            // MARK: Enroll button

            Button(action: {
              guard let deepURL = maybeEnrollMobileDeviceURL else {
                return
              }
              if enrollAttempt.didFinish {
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
                enrollAttempt = .success(nil)
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
              .opacity(enrollAttempt.didFail ? 0 : 1)
          }.padding(8).controlSize(.large)
          Spacer()
          if case let .failure(.unknownError(error)) = enrollAttempt {
            Text("Could not enroll this device").font(.headline)
            Text("There was an error: \(error.localizedDescription)")
          } else {
            Text("Do you want to enroll this device?").font(.headline)
            Text("""
            This will enable \(maybeEnrollMobileDeviceURL!.url
              .user) to authorize Device Trust web sessions \
            in \(maybeEnrollMobileDeviceURL!.url.hostname) with this device.
            """)
          }
          Spacer()
        }.presentationDetents([.medium]).padding(16).presentationCompactAdaptation(.sheet)
      }
    ).sheet(
      isPresented: .constant(isAuthenticatingWebDevice),
      onDismiss: {
        openedURL = nil
        authAttempt = .idle
      },
      content: {
        VStack(spacing: 8) {
          HStack {
            Button("Cancel", systemImage: "xmark", role: .cancel) {
              openedURL = nil
              authAttempt = .idle
            }.glassButton().labelStyle(.iconOnly)
            Spacer()

            // MARK: Auth button

            Button(action: {
              guard let deepURL = maybeAuthenticateWebDeviceURL else {
                return
              }
              if authAttempt.didFinish {
                openedURL = nil
                authAttempt = .idle
                return
              }
              if !authAttempt.isIdle {
                return
              }
              authAttempt = .loading
              Task {
                let confirmToken: Teleport_Devicetrust_V1_DeviceConfirmationToken
                do {
                  confirmToken = try await deviceTrust.authenticateWebDevice(
                    hostname: deepURL.url.hostname,
                    port: deepURL.url.port,
                    user: deepURL.url.user,
                    deviceWebToken: Teleport_Devicetrust_V1_DeviceWebToken
                      .with { $0.id = deepURL.id
                        $0.token = deepURL.token
                      }
                  )
                } catch {
                  authAttempt = .failure(.unknownError(error))
                  return
                }
                authAttempt = .success(nil)

                var parts = URLComponents()
                parts.scheme = "https"
                parts.host = deepURL.url.hostname
                parts.port = deepURL.url.port
                parts.path = "/webapi/devices/webconfirm"
                parts.queryItems = [
                  URLQueryItem(name: "id", value: confirmToken.id),
                  URLQueryItem(name: "token", value: confirmToken.token),
                ]
                if deepURL.redirectURI != "" {
                  parts.queryItems?.append(URLQueryItem(
                    name: "redirect_uri",
                    value: deepURL.redirectURI
                  ))
                }
                let url = parts.url!
                // TODO: Add completion handler to openURL to detect if the URL was opened.
                openURL(url)
                openedURL = nil
                authAttempt = .idle
              }
            }, label: {
              switch authAttempt {
              case .idle:
                Text("Authenticate")
              case .loading:
                Label("Authentication in progress", systemImage: "progress.indicator")
                  .labelStyle(.iconOnly).symbolEffect(
                    .variableColor.iterative,
                    options: .repeat(.continuous),
                    isActive: true
                  )
              case .success:
                Label("Authenticated", systemImage: "checkmark").labelStyle(.iconOnly)
              case .failure:
                Label("Authentication error", systemImage: "xmark.octagon")
                  .labelStyle(.iconOnly)
              }
            }).glassProminentButton().animation(.easeInOut, value: authAttempt)
              .opacity(authAttempt.didFail ? 0 : 1)
          }.padding(8).controlSize(.large)
          Spacer()
          if case let .failure(.unknownError(error)) = authAttempt {
            Text("Could not authenticate this device").font(.headline)
            Text("There was an error: \(error.localizedDescription)")
          } else {
            Text("Do you want to authenticate this device?").font(.headline)
            Text("""
            Lorem ipsum dolor sit amet.
            """)
          }
          Spacer()
        }.presentationDetents([.medium]).padding(16).presentationCompactAdaptation(.sheet)
      }
    ).alert("Cannot open the link", isPresented: .constant(hasDeepLinkParseError)) {
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

#Preview("Auth prompt") {
  @Previewable @State var openedURL: DeepLinkParseResult? =
    .success(.authenticateWebDevice(AuthenticateWebDeviceDeepURL(
      url: DeepURL(hostname: "example.com", port: 3080, user: "alice"),
      id: "1234",
      token: "abcdf",
      redirectURI: "https://example.com:3080/web"
    )))
  ScannedURLView(
    openedURL: $openedURL,
    enrollAttempt: .constant(.idle),
    serialNumber: .constant(nil),
    deviceTrust: FakeDeviceTrust()
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
