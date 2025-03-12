//
//  MenuBar.swift
//  TeleportMenuBar
//
//  Created by Rafał Cieślak on 2025-03-11.
//

import SwiftUI
import GRPCNIOTransportHTTP2

struct MenuBar: Scene {
  @State private var isVnetRunning: Bool = false
  var listRootClustersResponse: Teleport_Lib_Teleterm_V1_ListClustersResponse?
  var startVnet: () async -> Bool
  var stopVnet: () async -> Void
  var listRootClusters: () async -> Void
  var tshdClient: Teleport_Lib_Teleterm_V1_TerminalService.Client<HTTP2ClientTransport.Posix>

  var body: some Scene {
    MenuBarExtra("Teleport Menu Bar App", systemImage: "gearshape.fill") {
      if let response = listRootClustersResponse {
        let currentRootCluster = response.clusters.first(where: {rootCluster in
          rootCluster.uri == response.currentRootClusterUri})
        let otherClusters = response.clusters.filter({rootCluster in
          rootCluster.uri != currentRootCluster?.uri})
        let currentClusterLabel = if let cluster = currentRootCluster {
          getClusterLabel(cluster)
        } else {
          "No active cluster"
        }

        let rootClusterAdapter = Binding<String>(
          get: {
            currentRootCluster?.uri ?? ""
          }, set: { newClusterUri in
              Task {
                do {

                  try await tshdClient.updateCurrentProfile(.with {$0.rootClusterUri = newClusterUri})

                  await listRootClusters()
                } catch let error {
                  print("Could not update profile: \(error)")
                }
              }
          })

        if otherClusters.isEmpty {
          Text(currentClusterLabel)
        } else {
          Picker(currentClusterLabel, selection: rootClusterAdapter) {
            ForEach(response.clusters, id: \.uri) {
              rootCluster in Text(getClusterLabel(rootCluster))
            }
          }
        }
      }

      let isVnetRunningBinding = Binding<Bool>(
        get: { self.isVnetRunning },
        set: {newIsVnetRunning in
          if newIsVnetRunning {
            Task {
              let succces = await startVnet()
              if succces {
                self.isVnetRunning = true
              }
            }
          } else {
            Task {
              await stopVnet()
              self.isVnetRunning = false
            }
          }
        }
      )
      Toggle("VNet", isOn: isVnetRunningBinding).toggleStyle(.switch)
      Divider()
      Button("Quit") {
        NSApplication.shared.terminate(nil)
      }
    }
  }
}

func getClusterLabel(_ cluster: Teleport_Lib_Teleterm_V1_Cluster) -> String {
  if !cluster.loggedInUser.name.isEmpty {
    "\(cluster.loggedInUser.name)@\(cluster.name)"
  } else {
    cluster.name
  }
}
