//
//  MenuBar.swift
//  TeleportMenuBar
//
//  Created by Rafał Cieślak on 2025-03-11.
//

import SwiftUI
import GRPCNIOTransportHTTP2

struct MenuBar: Scene {
  @State var isVnetRunning: Bool = false
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

        if otherClusters.isEmpty {
          Text(currentClusterLabel)
        } else {
          Menu(currentClusterLabel) {
            ForEach(otherClusters, id: \.uri) { rootCluster in
              Button(getClusterLabel(rootCluster)) {
                Task {
                  do {
                    try await tshdClient.updateCurrentProfile(.with {$0.rootClusterUri = rootCluster.uri})

                    await listRootClusters()
                  } catch let error {
                    print("Could not update profile: \(error)")
                  }
                }
              }
            }
          }
        }
      }

      if isVnetRunning {
        Button("Stop VNet") {
          Task {
            await stopVnet()
            isVnetRunning = false
          }
        }
      } else {
        Button("Start VNet") {
          Task {
            let succces = await startVnet()
            if succces {
              isVnetRunning = true
            }
          }
        }
      }
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
