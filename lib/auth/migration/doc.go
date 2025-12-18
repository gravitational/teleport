// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

// Package migration used to hold a migration-like API based on Go interfaces.
// It was removed by https://github.com/gravitational/teleport/pull/62376.
//
// It leaves behind, in storage, a "/migrations/current" key with the following
// JSON object recorded:
//
//	{
//	  "version":   int,
//	  "phase":     int (pending=0, inProgress=1, complete=2, error=3),
//	  "started":   time.Time,
//	  "completed": time.Time,
//	}
package migration
