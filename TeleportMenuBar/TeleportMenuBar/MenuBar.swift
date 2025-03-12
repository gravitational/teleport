//
//  MenuBar.swift
//  TeleportMenuBar
//
//  Created by Rafał Cieślak on 2025-03-11.
//

import SwiftUI

struct MenuBar: Scene {
  @State var isVnetRunning: Bool = false
  var rootClusters: [Teleport_Lib_Teleterm_V1_Cluster] = []

  var body: some Scene {
    MenuBarExtra("Teleport Menu Bar App", systemImage: "gearshape.fill") {
      ForEach(rootClusters, id: \.uri) { rootCluster in
        Text(rootCluster.name)
      }
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

