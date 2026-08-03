// swift-tools-version: 6.3

import PackageDescription

// We enable these settings for all targets in this package so that the dialect of swift we're using is same across
// all our code.
let swiftSettings: [SwiftSetting] = [
    .enableUpcomingFeature("InferIsolatedConformances"),
    .enableUpcomingFeature("NonisolatedNonsendingByDefault"),
    .enableUpcomingFeature("ExistentialAny"),
    .enableUpcomingFeature("InternalImportsByDefault"),
    .enableUpcomingFeature("MemberImportVisibility"),
]

let package = Package(
    name: "TeleportKit",
    platforms: [.iOS(.v26)],
    products: [
        .library(
            name: "TeleportKit",
            targets: ["TeleportKit"]
        ),
    ],
    targets: [
        .target(
            name: "TeleportKit",
            swiftSettings: swiftSettings,
        ),
        .testTarget(
            name: "TeleportKitTests",
            dependencies: ["TeleportKit"],
            swiftSettings: swiftSettings,
        ),
    ],
    swiftLanguageModes: [.v6]
)
