// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// swift-tools-version: 6.3

import PackageDescription

let package = Package(
	name: "TeleportKit",
	platforms: [.iOS(.v26), .macOS(.v26)],
	products: [
		.library(name: "SystemClients", targets: ["SystemClients"]),
		.library(name: "LogBackends", targets: ["LogBackends"]),
	],
	dependencies: [
		.package(url: "https://github.com/pointfreeco/swift-dependencies", .upToNextMajor(from: "1.14.0")),
		.package(url: "https://github.com/pointfreeco/swift-sharing", .upToNextMajor(from: "2.9.1")),
		.package(url: "https://github.com/apple/swift-log", .upToNextMajor(from: "1.14.0")),
		.package(url: "https://github.com/apple/swift-collections", .upToNextMajor(from: "1.6.0")),
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
				.collections,
				.logging,
				.dependencies,
				"SystemClients",
			],
		),
		.testTarget(
			name: "LogBackendsTests",
			dependencies: [
				.dependencies,
				.logging,
				"LogBackends",
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
	fileprivate static let collections: Self = .product(
		name: "Collections",
		package: "swift-collections",
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
