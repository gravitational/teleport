// swift-tools-version: 6.3

import PackageDescription

let package = Package(
    name: "TeleportKit",
    platforms: [.iOS(.v26)],
    products: [
        .library(name: "SystemClients", targets: ["SystemClients"]),
    ],
    dependencies: [
        .package(url: "https://github.com/pointfreeco/swift-dependencies", .upToNextMajor(from:  "1.14.0")),
        .package(url: "https://github.com/pointfreeco/swift-sharing", .upToNextMajor(from:  "2.9.1")),
    ],
    targets: [
        .target(
            name: "SystemClients",
            dependencies: [
                .dependencies,
                .dependenciesMacros,
                .sharing,
            ]
        ),
    ],
    swiftLanguageModes: [.v6]
)

// MARK: - Custom Target.Dependency Values

private extension Target.Dependency {
    static let dependencies: Self = .product(name: "Dependencies", package: "swift-dependencies")
    static let dependenciesMacros: Self = .product(name: "DependenciesMacros", package: "swift-dependencies")
    static let sharing: Self = .product(name: "Sharing", package: "swift-sharing")
}

// MARK: - Build Settings

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
