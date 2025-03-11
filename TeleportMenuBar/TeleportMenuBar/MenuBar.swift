//
//  MenuBar.swift
//  TeleportMenuBar
//
//  Created by Rafał Cieślak on 2025-03-11.
//

import SwiftUI

struct MenuBar: Scene {
  @State var isVnetRunning: Bool = false

  var body: some Scene {
    MenuBarExtra("Teleport Menu Bar App", systemImage: "gearshape.fill") {
      if isVnetRunning {
        Button("Stop VNet") {
          isVnetRunning = false
        }
      } else {
        Button("Start VNet") {
          isVnetRunning = true
        }
      }
      Divider()
      Button("Quit") {
        NSApplication.shared.terminate(nil)
      }
    }
  }
}

