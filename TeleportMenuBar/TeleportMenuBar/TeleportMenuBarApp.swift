//
//  TeleportMenuBarApp.swift
//  TeleportMenuBar
//
//  Created by Grzegorz Zdunek on 11/03/2025.
//

import GRPCCore
import GRPCNIOTransportHTTP2
import GRPCProtobuf
import Foundation
import SwiftUI

let addr = "\(FileManager.default.homeDirectoryForCurrentUser.path()).tsh/tsh.socket";

@MainActor
class AppModel: ObservableObject {
  @Published var listRootClustersResponse: Teleport_Lib_Teleterm_V1_ListClustersResponse?

  init() {
    runTshd()

    let client = GRPCClient(transport: try! .http2NIOPosix(
      target: .unixDomainSocket(path: addr),
      transportSecurity: .plaintext
    ))
    let serviceClient = Teleport_Lib_Teleterm_V1_TerminalService.Client(wrapping: client)

    Task {
      try await Task.sleep(nanoseconds: UInt64(250 * Double(NSEC_PER_MSEC)))
      do {
        try await client.runConnections()
      } catch let error {
        print("Error running connections: \(error)")
      }
    }

    NotificationCenter.default.addObserver(forName: NSApplication.willTerminateNotification, object: nil, queue: .main) {_ in
      client.beginGracefulShutdown()
    }

    Task {
      do {
        self.listRootClustersResponse = try await serviceClient.listRootClusters(.with {_ in })
        print("Fetched root clusters")
      } catch let error {
        print("Could not fetch root clusters: \(error)")
      }
    }
  }

  func runTshd() {
    let process = try! Process.run(Bundle.main.url(forResource: "tsh", withExtension: "").unsafelyUnwrapped,
                                      arguments: ["daemon", "start", "--addr=unix://\(addr)",
                                               "--certs-dir=nothing",
                                               "--prehog-addr=127.0.0.1:9090",
                                               "--kubeconfigs-dir=${settings.",
                                               "--agents-dir=${agentsDir}",
                                               "--installation-id=${settings.installationId}",
                                               "--add-keys-to-agent=no"])

    NotificationCenter.default.addObserver(forName: NSApplication.willTerminateNotification, object: nil, queue: .main) {_ in
      process.terminate()
    }
  }
}

@main
struct TeleportMenuBarApp: App {
  @StateObject private var model = AppModel()

  var body: some Scene {
    MenuBar(listRootClustersResponse: model.listRootClustersResponse)
  }
}
