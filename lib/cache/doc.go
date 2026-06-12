/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// Package cache implements event-driven cache layer
// that is used by auth servers, proxies and nodes.
//
// The cache fetches resources and then subscribes
// to the events watcher to receive updates.
//
// This approach allows cache to be up to date without
// time based expiration and avoid re-fetching all
// resources reducing bandwidth.
//
// There are two types of cache backends used:
//
// * SQLite-based in-memory used for auth nodes
// * SQLite-based on disk persistent cache for nodes and proxies
// providing resilliency in the face of auth servers failures.
package cache
