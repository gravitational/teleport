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

@Table("clusters")
struct Cluster: Identifiable {
	let id: UUID
	var host: String
	var port: Int
}

// MARK: - CustomDebugStringConvertible

extension Cluster: CustomDebugStringConvertible {
	var debugDescription: String {
		"\(id):\(host):\(port)"
	}
}

extension Cluster {
	var url: URL? {
		var components = URLComponents()
		components.host = host
		components.port = port
		components.scheme = "https"
		return components.url
	}
}

// MARK: - Preview Data

extension Cluster {
	static let previews: [Cluster] = [
		Cluster(id: UUID(0), host: "production.teleport.example.com", port: 443),
		Cluster(id: UUID(1), host: "staging.teleport.example.com", port: 3080),
		Cluster(id: UUID(2), host: "development.teleport.example.com", port: 443),
	]
}
