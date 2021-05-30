// swift-tools-version:5.3
// The swift-tools-version declares the minimum version of Swift required to build this package.

import PackageDescription

let package = Package(
    name: "Terminal",
    dependencies: [
    // GRPC dependencies:
    // Main SwiftNIO package
    .package(url: "https://github.com/apple/swift-nio.git", from: "2.28.0"),
    // HTTP2 via SwiftNIO
    .package(url: "https://github.com/apple/swift-nio-http2.git", from: "1.16.1"),
    // TLS via SwiftNIO
    .package(url: "https://github.com/apple/swift-nio-ssl.git", from: "2.8.0"),
    // Support for Network.framework where possible.
    .package(url: "https://github.com/apple/swift-nio-transport-services.git", from: "1.6.0"),
    // Extra NIO stuff; quiescing helpers.
    .package(url: "https://github.com/apple/swift-nio-extras.git", from: "1.4.0"),

    // Official SwiftProtobuf library, for [de]serializing data to send on the wire.
    .package(
      name: "SwiftProtobuf",
      url: "https://github.com/apple/swift-protobuf.git",
      from: "1.9.0"
    ),

    // GRPC
    .package(url: "https://github.com/grpc/grpc-swift.git", from: "1.0.0"),

    // Logging API.
    .package(url: "https://github.com/apple/swift-log.git", from: "1.4.0"),
    ],
    targets: [
      // Targets are the basic building blocks of a package. A target can define a module or a test suite.
        // Model for the Ticker
        .target(
          name: "TickerModel",
          dependencies: [
            .product(name: "GRPC", package: "grpc-swift"),
            .product(name: "NIO", package: "swift-nio"),
            .product(name: "SwiftProtobuf", package: "SwiftProtobuf"),
          ],
          path: "Sources/Model"
        ),              
        // Targets can depend on other targets in this package, and on products in packages this package depends on.
        .target(
            name: "Terminal",
            dependencies: [
              .target(name: "TickerModel"),
              .product(name: "GRPC", package: "grpc-swift"),
            ]),
        .testTarget(
            name: "TerminalTests",
            dependencies: ["Terminal"]),
    ]
)
