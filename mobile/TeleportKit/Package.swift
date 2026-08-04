// swift-tools-version: 6.3

import PackageDescription

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
        ),
        .testTarget(
            name: "TeleportKitTests",
            dependencies: ["TeleportKit"],
        ),
    ],
    swiftLanguageModes: [.v6]
)

// We enable these settings for all targets in this package so that the dialect of swift we're using is same across
// all our code.
let swiftSettings: [SwiftSetting] = [
    .enableUpcomingFeature("InferIsolatedConformances"),
    .enableUpcomingFeature("NonisolatedNonsendingByDefault"),
    .enableUpcomingFeature("ExistentialAny"),
    .enableUpcomingFeature("InternalImportsByDefault"),
    .enableUpcomingFeature("MemberImportVisibility"),
]

for target in package.targets {
	var settings = target.swiftSettings ?? []
	settings.append(contentsOf: swiftSettings)
	target.swiftSettings = settings
}
