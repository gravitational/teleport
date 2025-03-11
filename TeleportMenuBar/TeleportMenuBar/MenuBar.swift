//
//  MenuBar.swift
//  TeleportMenuBar
//
//  Created by Rafał Cieślak on 2025-03-11.
//

import SwiftUI

struct MenuBar: Scene {
  @State var counter: Int = 0

  var body: some Scene {
    MenuBarExtra("Teleport Menu Bar App", systemImage: "gearshape.fill") {
      Text("The counter is \(counter)")
      Button("Increase") {
        counter += 1
      }
      Button("Decrease") {
        counter -= 1
      }
      Divider()
      Button("Quit") {
        NSApplication.shared.terminate(nil)
      }
    }
  }
}
