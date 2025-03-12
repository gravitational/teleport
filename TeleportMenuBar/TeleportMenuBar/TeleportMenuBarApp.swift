//
//  TeleportMenuBarApp.swift
//  TeleportMenuBar
//
//  Created by Grzegorz Zdunek on 11/03/2025.
//

import Connect
import ConnectNIO
import SwiftUI
import Foundation

let addr = "unix://\(FileManager.default.homeDirectoryForCurrentUser)/.tsh/tsh.socket";

@main
struct TeleportMenuBarApp: App {
  @State private var client = ProtocolClient(
    httpClient: NIOHTTPClient(host: addr),
    config: ProtocolClientConfig(
      host: addr,
      networkProtocol: .grpc, // Or .grpcWeb
      codec: ProtoCodec() // Or JSONCodec()
    )
  )

  var body: some Scene {
    MenuBar()
  }

  func runTshd() {
    let process = try! Process.run(Bundle.main.url(forResource: "tsh", withExtension: "").unsafelyUnwrapped,
                     arguments: ["daemon", "start", "--addr=\(addr)",
                                 "--certs-dir=nothing",
                                 "--prehog-addr=127.0.0.1:9090",
                                 "--kubeconfigs-dir=${settings.",
                                 "--agents-dir=${agentsDir}",
                                 "--installation-id=${settings.installationId}",
                                 "--add-keys-to-agent=no"])
  }

  init () {
    runTshd()
    let request = Teleport_Lib_Teleterm_V1_ListClustersRequest()
    let service = Teleport_Lib_Teleterm_V1_TerminalServiceClient(client: self.client)


    Task {
      await list()
    }

    func list() async {
      let resp = await service.listRootClusters(request:  request)
      try! resp.result.get().clusters.forEach { print($0.name) }
    }
  }
}
