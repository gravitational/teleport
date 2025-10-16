//
//  DeviceTrust.swift
//  Foobar
//
//  Created by Rafał Cieślak on 2025-10-06.
//
import Connect
import ConnectNIO
import CryptoKit
import os
import SwiftProtobuf
import SwiftUI

private let logger = Logger(
  subsystem: Bundle.main.bundleIdentifier ?? "Foobar",
  category: "devicetrust"
)

protocol DeviceTrustP {
  func getSerialNumber() -> String
  func deleteDeviceKey() async -> Result<Bool, SecOSStatusError>
  func enrollDevice(hostname: String, port: Int?, user: String, userToken: String) async throws
}

final class DeviceTrust: DeviceTrustP {
  func getSerialNumber() -> String {
    UserDefaults.standard.string(forKey: "serialNumber") ?? "Unknown"
  }

  func deleteDeviceKey() async -> Result<Bool, SecOSStatusError> {
    DeviceKey.delete()
  }

  func enrollDevice(hostname: String, port: Int?, user _: String,
                    userToken _: String) async throws
  {
    let hostnameWithScheme = "https://\(hostname)"
    let hostnameWithPort = "\(hostnameWithScheme)\(port.map { ":\($0)" } ?? "")"
    let protocolClient = ProtocolClient(
      httpClient: NIOHTTPClient(host: hostnameWithScheme, port: port, timeout: nil),
      config: ProtocolClientConfig(
        host: "\(hostnameWithPort)/webapi/devicetrust/",
        networkProtocol: .connect,
        codec: ProtoCodec()
      )
    )
    let client = Teleport_Devicetrust_V1_DeviceTrustServiceClient(client: protocolClient)
    let cd = collectDeviceData()
    logger.debug("Collected device data: \(cd.debugDescription, privacy: .public)")

    let cred = try DeviceKey.getOrCreate()
    logger.debug("Got device key: \(cred.debugDescription, privacy: .public)")

    let createTokenResponse = await client
      .createDeviceEnrollToken(request: Teleport_Devicetrust_V1_CreateDeviceEnrollTokenRequest
        .with {
          $0.deviceData = cd
        })
    guard let deviceEnrollToken = createTokenResponse.message else {
      throw createTokenResponse.error!
    }

//    let request = Teleport_Devicetrust_V1_PingRequest()
//    print("Sending ping")
//    let response = await client.ping(request: request, headers: [:])
//    print("Got ping response: \(response)")
//    return

    let stream = client.enrollDevice()

    try stream.send(Teleport_Devicetrust_V1_EnrollDeviceRequest
      .with { $0.init_p = Teleport_Devicetrust_V1_EnrollDeviceInit.with {
        $0.deviceData = cd
        $0.credentialID = cred.id
        $0.token = deviceEnrollToken.token
        $0.macos = Teleport_Devicetrust_V1_MacOSEnrollPayload.with {
          $0.publicKeyDer = cred.publicKeyDer
        }
      } })
    logger.debug("Sent init request")

    let response1 = try await (getSingleMessage(stream)).get()
    guard case let .macosChallenge(challenge) = response1.payload else {
      throw UnexpectedPayload(expected: "macosChallenge", actual: response1.payload)
    }
    logger.debug("Got challenge")

    let signature = try DeviceKey.sign(challenge.challenge)

    try stream
      .send(Teleport_Devicetrust_V1_EnrollDeviceRequest
        .with {
          $0.macosChallengeResponse = Teleport_Devicetrust_V1_MacOSEnrollChallengeResponse.with {
            $0.signature = signature
          }
        })
    logger.debug("Sent signed challenge")

    let response2 = try await (getSingleMessage(stream)).get()
    guard case .success = response2.payload else {
      throw UnexpectedPayload(expected: "success", actual: response2.payload)
    }
    logger.debug("Successfully enrolled device")
  }
}

struct UnexpectedPayload<Actual>: Error & LocalizedError {
  let expected: String
  let actual: Actual?

  var errorDescription: String? {
    guard let actualValue = actual else {
      return "expected \(expected), got nil"
    }

    let actualMirror = Mirror(reflecting: actualValue)
    return "expected \(expected), got \(actualMirror.children.first?.label ?? "<unknown>")"
  }
}

enum StreamError: Error, LocalizedError {
  case endOfStream
  case error(ConnectError)
  case unknownError(Code, Error)
  case noMessage

  var errorDescription: String? {
    switch self {
    case .endOfStream:
      return "unexpected end of stream"
    case let .error(error):
      if let message = error.message {
        return message
      }
      return "stream ended with error (code \(error.code)): \(error.localizedDescription)"
    case let .unknownError(code, error):
      return "stream ended with unknown error (code \(code)): \(error.localizedDescription)"
    case .noMessage:
      return "stream did not return a message"
    }
  }
}

func getSingleMessage<Request, Response>(_ stream: any BidirectionalAsyncStreamInterface<
  Request,
  Response
>) async -> Result<Response, StreamError> {
  for await result in stream.results() {
    switch result {
    case let .message(message):
      return .success(message)
    case let .complete(code, error, _):
      if let error {
        if let connectError = error as? ConnectError {
          return .failure(.error(connectError))
        }
        return .failure(.unknownError(code, error))
      }
      // If there's no error, assume that code == .ok.
      return .failure(.endOfStream)
    case .headers:
      // Ignore headers.
      break
    }
  }

  return .failure(.noMessage)
}

func collectDeviceData() -> Teleport_Devicetrust_V1_DeviceCollectedData {
  let device = UIDevice.current
  var cd = Teleport_Devicetrust_V1_DeviceCollectedData()
  let serialNumber = "FOO1234"
  cd.collectTime = Google_Protobuf_Timestamp(date: Date())
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
    $0.withMemoryRebound(to: CChar.self, capacity: 1) { ptr in
      String(validatingUTF8: ptr)
    }
  }
  return modelCode
}

let deviceKeyLabel = "com.gravitational.teleport.devicekey"

enum DeviceKeyError: Error {
  /// noApplicationTag is thrown when an existing item returned by SecItemCopyMatching has no
  /// associated application tag.
  /// This is a programmer error which most likely means that kSecAttrApplicationTag was missing in
  /// private key attributes during creation.
  case noApplicationTag
  /// noValueRef represents a case where an existing item returned by SecItemCopyMatching did not
  /// include a ref to the SecKey.
  /// This is a programmer error caused by kSecReturnRef not being included in the query dictionary.
  case noValueRef
  /// copyFailed is used when getting a public key from a private key has failed.
  case copyFailed
}

enum DeviceKeySignError: Error {
  case noDeviceKey
}

/// SecOSStatusError wraps OSStatus values returned by functions from the Security framework.
struct SecOSStatusError: Error & Equatable & CustomStringConvertible {
  // TODO: Make SecOSStatusError be LocalizedError too.
  let status: OSStatus

  var description: String {
    SecCopyErrorMessageString(status, nil) as String? ?? "Unknown status \(status)"
  }
}

class DeviceKey {
  static func getOrCreate() throws -> Teleport_Devicetrust_V1_DeviceCredential {
    if let existingKey = try get() {
      logger.debug("Got existing key")
      return existingKey
    }
    logger.debug("Creating new key")
    return try create()
  }

  static func get() throws -> Teleport_Devicetrust_V1_DeviceCredential? {
    guard let item = try find(returnAttrs: true) else {
      return nil
    }
    guard let existingItem = item as? [String: Any],
          let appTagData = existingItem[kSecAttrApplicationTag as String] as? Data,
          let appTag = String(data: appTagData, encoding: .utf8)
    else {
      throw DeviceKeyError.noApplicationTag
    }
    // swiftlint:disable:next force_cast
    guard let privateKey = existingItem[kSecValueRef as String] as! SecKey? else {
      throw DeviceKeyError.noValueRef
    }
    guard let publicKey = SecKeyCopyPublicKey(privateKey) else {
      throw DeviceKeyError.copyFailed
    }
    var error: Unmanaged<CFError>?
    guard let publicKeyRep = SecKeyCopyExternalRepresentation(publicKey, &error) else {
      throw error!.takeRetainedValue() as Error
    }
    let p256 = try P256.KeyAgreement.PublicKey(x963Representation: publicKeyRep as Data)

    return Teleport_Devicetrust_V1_DeviceCredential.with {
      $0.id = appTag
      $0.publicKeyDer = Data(p256.derRepresentation)
    }
  }

  /// sign signs the challenge with the device key and returns the resulting signature.
  static func sign(_ challenge: Data) throws -> Data {
    guard let item = try find(returnAttrs: false) else {
      throw DeviceKeySignError.noDeviceKey
    }
    let digest = SHA256.hash(data: challenge)
    var error: Unmanaged<CFError>?
    guard let signature = SecKeyCreateSignature(
      // swiftlint:disable:next force_cast
      item as! SecKey,
      SecKeyAlgorithm.ecdsaSignatureDigestX962SHA256,
      Data(digest) as CFData,
      &error
    ) as Data? else {
      throw error!.takeRetainedValue() as Error
    }
    return signature
  }

  /// find returns the result of querying for the device key with SecItemCopyMatching.
  /// If returnAttrs is true, the returned value is a dictionary where the SecKey is available under
  /// the kSecValueRef field.
  /// If returnAttrs is false, the returned value can itself be cast to SecKey.
  private static func find(returnAttrs: Bool) throws(SecOSStatusError) -> CFTypeRef? {
    let query: NSDictionary = [
      kSecClass: kSecClassKey,
      kSecAttrKeyType: kSecAttrKeyTypeECSECPrimeRandom,
      kSecMatchLimit: kSecMatchLimitOne,
      kSecReturnRef: true,
      kSecReturnAttributes: returnAttrs,
      kSecAttrApplicationLabel: Data(deviceKeyLabel.utf8),
    ]
    var item: CFTypeRef?
    let status = SecItemCopyMatching(query as CFDictionary, &item)
    guard status == errSecSuccess else {
      if status == errSecItemNotFound {
        return nil
      }
      throw SecOSStatusError(status: status)
    }
    return item
  }

  static func delete() -> Result<Bool, SecOSStatusError> {
    let query: NSDictionary = [
      kSecClass: kSecClassKey,
      kSecAttrKeyType: kSecAttrKeyTypeECSECPrimeRandom,
      kSecAttrApplicationLabel: Data(deviceKeyLabel.utf8),
    ]
    let status = SecItemDelete(query as CFDictionary)
    guard status == errSecSuccess else {
      if status == errSecItemNotFound {
        return .success(false)
      }
      return .failure(SecOSStatusError(status: status))
    }
    return .success(true)
  }

  static func create() throws -> Teleport_Devicetrust_V1_DeviceCredential {
    let uuid = NSUUID().uuidString.lowercased()
    var error: Unmanaged<CFError>?
    guard let access = SecAccessControlCreateWithFlags(
      kCFAllocatorDefault,
      kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly,
      // TODO: Figure out why there's no prompt for Face ID when retrieving the key.
      [.privateKeyUsage, .userPresence],
      &error
    ) else {
      throw error!.takeRetainedValue() as Error
    }
    let attributes: NSDictionary = [
      kSecAttrKeyType: kSecAttrKeyTypeECSECPrimeRandom,
      kSecAttrKeySizeInBits: 256,
      kSecAttrTokenID: kSecAttrTokenIDSecureEnclave,
      kSecPrivateKeyAttrs: [
        kSecAttrIsPermanent: true,
        kSecAttrAccessControl: access,
        // kSecAttrLabel is a human-readable label.
        kSecAttrLabel: deviceKeyLabel,
        // kSecAttrApplicationLabel is used to lookup keys programmatically.
        kSecAttrApplicationLabel: Data(deviceKeyLabel.utf8),
        // kSecAttrApplicationTag is a private application tag.
        kSecAttrApplicationTag: Data(uuid.utf8),
      ],
    ]
    guard let privateKey = SecKeyCreateRandomKey(attributes as CFDictionary, &error) else {
      throw error!.takeRetainedValue() as Error
    }
    guard let publicKey = SecKeyCopyPublicKey(privateKey) else {
      throw DeviceKeyError.copyFailed
    }
    guard let publicKeyRep = SecKeyCopyExternalRepresentation(publicKey, &error) else {
      throw error!.takeRetainedValue() as Error
    }
    let p256 = try P256.KeyAgreement.PublicKey(x963Representation: publicKeyRep as Data)
    return Teleport_Devicetrust_V1_DeviceCredential.with {
      $0.id = uuid
      $0.publicKeyDer = Data(p256.derRepresentation)
    }
  }
}

// TODO: Use this in ContentView instead of bindings for attempts.
class FakeDeviceTrust: DeviceTrustP {
  let serialNumber: String = "123456"
  let deleteDeviceKeyResult: Result<Bool, SecOSStatusError> = .success(true)
  let enrollDeviceError: Error? = NSError(domain: "test", code: 0, userInfo: nil)

  func getSerialNumber() -> String {
    serialNumber
  }

  func deleteDeviceKey() async -> Result<Bool, SecOSStatusError> {
    deleteDeviceKeyResult
  }

  func enrollDevice(
    hostname _: String,
    port _: Int?,
    user _: String,
    userToken _: String
  ) async throws {
    if let enrollDeviceError {
      throw enrollDeviceError
    }
  }
}
