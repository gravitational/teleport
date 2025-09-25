//
//  FoobarApp.swift
//  Foobar
//
//  Created by Rafał Cieślak on 2025-09-11.
//

import Connect
import ConnectNIO
import SwiftUI

@main
struct FoobarApp: App {
  @State private var client = ProtocolClient(
    httpClient: NIOHTTPClient(
      host: "https://teleport-mbp.ocelot-paradise.ts.net",
      port: 3030,
      timeout: nil
    ),
    config: ProtocolClientConfig(
      host: "https://teleport-mbp.ocelot-paradise.ts.net:3030/webapi/devicetrust/",
      networkProtocol: .connect,
      codec: ProtoCodec(),
    )
  )

  var body: some Scene {
    WindowGroup {
      ContentView(
        viewModel: DeviceTrustViewModel()
      )
    }
  }
}
