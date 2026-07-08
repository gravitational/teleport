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

import Foundation
import SQLiteData

/*
 ============= .__   __.   ______   .___________. __    ______  _______  =============
 ============= |  \ |  |  /  __  \  |           ||  |  /      ||   ____| =============
 ============= |   \|  | |  |  |  | `---|  |----`|  | |  ,----'|  |__    =============
 ============= |  . `  | |  |  |  |     |  |     |  | |  |     |   __|   =============
 ============= |  |\   | |  `--'  |     |  |     |  | |  `----.|  |____  =============
 ============= |__| \__|  \______/      |__|     |__|  \______||_______| =============

 This file contains database migrations. Once a particular migration has shipped to customers, it may NEVER be altered.
 Doing so could cause irreparable corruption to the data on customer devices. As such, changes to this file should
 almost always be additive, unless we are confident the migration has not shipped to customers.

 If you need to make a change to the database schema, you'd register a new migration that considers the existing shape
 of the database and converts it to the new shape. This is what it means to correctly and effectively migrate.

 For documentation on how to write migrations using GRDB, see <https://swiftpackageindex.com/groue/GRDB.swift/master/documentation/grdb/migrations>

 Proceed with caution.
 */

extension AppDatabase {
	static func migrate(db dbWriter: any DatabaseWriter) throws {
		var migrator = DatabaseMigrator()

		migrator.registerMigration("Create initial cluster database table") { db in
			try db.create(table: "clusters") { table in
				table.primaryKey("id", .text)
					.notNull(onConflict: .replace)
					// Use the uuid() function that SQLiteData defines in order to support automatic generation of UUIDs
					// upon insertion.
					.defaults(sql: "(uuid())")

				table.column("host", .text).notNull()
			}
		}

		try migrator.migrate(dbWriter)
	}
}
