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
import SwiftUI

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
        codec: ProtoCodec(),
      )
    )
    let client = Teleport_Devicetrust_V1_DeviceTrustServiceClient(client: protocolClient)
    let cd = collectDeviceData()
    print("\(cd)")

    let cred = try DeviceKey.getOrCreate()
    print("\(cred)")

    let request = Teleport_Devicetrust_V1_PingRequest()
    print("Sending ping")
    let response = await client.ping(request: request, headers: [:])
    print("Got ping response: \(response)")
    return

    let stream = client.enrollDevice()

    try stream
      .send(Teleport_Devicetrust_V1_EnrollDeviceRequest
        .with { $0.init_p = Teleport_Devicetrust_V1_EnrollDeviceInit.with {
          $0.deviceData = cd
          $0.credentialID = cred.id
          $0.macos = Teleport_Devicetrust_V1_MacOSEnrollPayload.with {
            $0.publicKeyDer = cred.publicKeyDer
          }
        } })
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

/// SecOSStatusError wraps OSStatus values returned by functions from the Security framework.
struct SecOSStatusError: Error & Equatable & CustomStringConvertible {
  let status: OSStatus

  var description: String {
    SecCopyErrorMessageString(status, nil) as String? ?? "Unknown status \(status)"
  }
}

class DeviceKey {
  static func getOrCreate() throws -> Teleport_Devicetrust_V1_DeviceCredential {
    if let existingKey = try get() {
      print("Got existing key")
      return existingKey
    }
    print("Creating new key")
    return try create()
  }

  static func get() throws -> Teleport_Devicetrust_V1_DeviceCredential? {
    let query: NSDictionary = [
      kSecClass: kSecClassKey,
      kSecAttrKeyType: kSecAttrKeyTypeECSECPrimeRandom,
      kSecMatchLimit: kSecMatchLimitOne,
      kSecReturnRef: true,
      kSecReturnAttributes: true,
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
