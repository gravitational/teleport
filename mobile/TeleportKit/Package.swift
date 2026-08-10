// swift-tools-version: 6.3

import PackageDescription

let package = Package(
	name: "TeleportKit",
	platforms: [.iOS(.v26)],
	products: [
		.library(name: "SystemClients", targets: ["SystemClients"]),
		.library(name: "LogBackends", targets: ["LogBackends"]),
	],
	dependencies: [
		.package(url: "https://github.com/pointfreeco/swift-dependencies", .upToNextMajor(from: "1.14.0")),
		.package(url: "https://github.com/pointfreeco/swift-sharing", .upToNextMajor(from: "2.9.1")),
		.package(url: "https://github.com/apple/swift-log", .upToNextMajor(from: "1.14.0")),
	],
	targets: [
		.target(
			name: "SystemClients",
			dependencies: [
				.dependencies,
				.dependenciesMacros,
				.sharing,
			],
		),
		.target(
			name: "LogBackends",
			dependencies: [
				.logging,
				.dependencies,
				"SystemClients",
			],
		),
	],
	swiftLanguageModes: [.v6],
)

// MARK: - Custom Target.Dependency Values

extension Target.Dependency {
	fileprivate static let dependencies: Self = .product(
		name: "Dependencies",
		package: "swift-dependencies",
	)
	fileprivate static let dependenciesMacros: Self = .product(
		name: "DependenciesMacros",
		package: "swift-dependencies",
	)
	fileprivate static let sharing: Self = .product(
		name: "Sharing",
		package: "swift-sharing",
	)
	fileprivate static let logging: Self = .product(
		name: "Logging",
		package: "swift-log",
	)
}

// MARK: - Build Settings

/// We enable these settings for all targets in this package so that the dialect of swift we're using is same across
/// all our code.
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
