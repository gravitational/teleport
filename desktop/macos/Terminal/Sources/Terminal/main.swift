import GRPC
import Logging
import NIO
import NIOSSL
import TickerModel

func tick(client ticker: Proto_TickServiceClient) {
    let logger = Logger(label: "Terminal")
    let req = Proto_TickRequest()
    let call = ticker.subscribe(req, handler: { result in
        logger.info("Tick \(result)")
    })
    let status = try! call.status.recover { _ in .processingError }.wait()
    if status.code != .ok {
        logger.error("RPC failed: \(status)")
    }
}

func run() throws {
    // Setup an `EventLoopGroup` for the connection to run on.
    //
    // See: https://github.com/apple/swift-nio#eventloops-and-eventloopgroups
    let group = MultiThreadedEventLoopGroup(numberOfThreads: 1)

    // Make sure the group is shutdown when we're done with it.
    defer {
        try! group.syncShutdownGracefully()
    }

    // Configure the channel, we're not using TLS so the connection is `insecure`.

    let channel = ClientConnection.secure(group: group)
        .withTLS(trustRoots: NIOSSLTrustRoots.file("../../../fixtures/cert.pem"))
        .connect(host: "localhost", port: 3000)

    // Close the connection when we're done with it.
    defer {
        try! channel.close().wait()
    }

    // Provide the connection to the generated client.
    let ticker = Proto_TickServiceClient(channel: channel)

    // Do the ticking.
    tick(client: ticker)
}

try! run()
