//
//  TeleportMenuBarApp.swift
//  TeleportMenuBar
//
//  Created by Grzegorz Zdunek on 11/03/2025.
//

import Foundation
import SwiftUI

let addr = "unix://\(FileManager.default.homeDirectoryForCurrentUser.path())/.tsh/tsh.socket";

@main
struct TeleportMenuBarApp: App {
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
  }
}
