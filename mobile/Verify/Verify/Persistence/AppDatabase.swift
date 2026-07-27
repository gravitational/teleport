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
// along with this program.  If not, see http://www.gnu.org/licenses/

import Dependencies
import Foundation
import OSLog
import SQLiteData

/// A namespace that provides us a place to put our persistence code.
///
/// Database initialization with GRDB and SQLiteData can involve a lot of free functions. There's a good reason for
/// this; database initialization should only ever really happen once, and the clearest language construct that lets us
/// do that is a set of static variables and functions that tie the lifetime of the database to the runtime of the
/// application.
enum AppDatabase {
	static let logger = Logger.forType(AppDatabase.self)
}

// MARK: Initialization

extension AppDatabase {
	/// Initializes an on-disk database suitable for the app running live in production.
	static func makeLiveDatabase() -> any DatabaseWriter {
		var database: any DatabaseWriter
		do {
			let fileManager = FileManager.default
			// SQLite creates lots of auxiliary files as a normal part of its operation, so let's tuck it into its own
			// directory, mostly for organizational purposes.
			let databaseDirectoryName = "Databases"
			let databaseDirectoryURL = try fileManager.url(
				for: .applicationSupportDirectory,
				in: .userDomainMask,
				appropriateFor: nil,
				create: true,
			).appending(path: databaseDirectoryName)

			#if DEBUG
				// Allow for an easy reset of the database in case we want to manually test what a fresh database looks
				// like
				if CommandLine.arguments.contains("--reset-database") {
					try? fileManager.removeItem(at: databaseDirectoryURL)
				}
			#endif

			try fileManager.createDirectory(at: databaseDirectoryURL, withIntermediateDirectories: true)

			// Create the database file by initializing a GRDB DatabaseQueue.
			let databaseFileName = "AppDatabase.sqlite"
			let databasePath = databaseDirectoryURL.appending(path: databaseFileName).path(percentEncoded: false)
			logger.info("Initializing \(databaseFileName) database...")
			database = try DatabaseQueue(
				path: databasePath,
				configuration: defaultConfiguration,
			)
			logger.info("Successfully initialized \(databaseFileName)")

			// This log provides a convenient line we can copy/paste into our terminal so that we can open up our SQLite
			// client of choice.
			logger.info("open '\(databasePath)'")

			logger.info("Running database migrations...")
			try migrate(db: database)
			logger.info("Successfully ran all migrations")
		} catch {
			// If database initialization fails, it almost always means we did something wrong, like incorrectly
			// implementing a migration. In debug, we want to catch such issues very quickly, so we fatalError. In
			// production, crashing like this feels pretty bad when some important app behaviors don't rely on the
			// database, so instead we fall back to an in-memory database, and write to an error log.
			#if DEBUG
				fatalError("Database initialization failed: \(error)")
			#else
				logger.critical("Database initialization failed. Falling back to in-memory database: \(error)")
				database = makeInMemoryDatabase()
			#endif
		}

		return database
	}

	/// Initializes an in-memory database suitable for scenarios where persistence across runs is not required.
	static func makeInMemoryDatabase() -> any DatabaseWriter {
		do {
			logger.info("Initializing in-memory database...")
			let database = try DatabaseQueue(configuration: defaultConfiguration)
			logger.info("Successfully initialized in-memory database")
			logger.info("Running in-memory database migrations...")
			try migrate(db: database)
			logger.info("Successfully ran all migrations for in-memory database")
			return database
		} catch {
			fatalError("Error while initializing in-memory database: \(error)")
		}
	}

	/// Initializes an in-memory database with some dummy values pre-populated values for SwiftUI previews.
	static func makePreviewDatabase() -> any DatabaseWriter {
		let database = makeInMemoryDatabase()

		do {
			try database.write { db in
				try Cluster.insert { Cluster.previews }.execute(db)
			}
		} catch {
			// We `print(...)` to the console instead of calling `logger.error(...)` because the SwiftUI preview console
			// only sees stdout.
			let errorMessage = "Failed to seed preview database: \(error)"
			print(errorMessage)
			fatalError(errorMessage)
		}

		return database
	}
}

// MARK: - Database Configuration

extension AppDatabase {
	private static var defaultConfiguration: Configuration {
		@Dependency(\.context)
		var context

		var config = Configuration()

		// An opportunity to add required custom SQL functions or
		// collations, if needed:
		// config.prepareDatabase { db in
		//     db.add(function: ...)
		// }

		#if DEBUG
			// Log SQL statements if the --sql-trace argument is passed.
			// See <https://swiftpackageindex.com/groue/grdb.swift/documentation/grdb/database/trace(options:_:)>
			if CommandLine.arguments.contains("--sql-trace") {
				config.prepareDatabase { db in
					db.trace(options: .profile) {
						if context == .preview {
							print("\($0.expandedDescription)")
						} else {
							logger.debug("\($0.expandedDescription)")
						}
					}
				}
			}

			// Protect sensitive information by enabling verbose debugging in DEBUG builds only.
			// See <https://swiftpackageindex.com/groue/grdb.swift/documentation/grdb/configuration/publicstatementarguments>
			config.publicStatementArguments = true
		#endif

		return config
	}
}
