// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package debug

const (
	// ServiceSocketName represents the Unix domain socket name of the debug
	// service.
	ServiceSocketName = "debug.sock"
	// LogLevelEndpoint is the HTTP endpoint used for retrieving and changing
	// log level.
	LogLevelEndpoint = "/log-level"
	// GetLogLevelMethod is the HTTP method used to retrieve the current log
	// level.
	GetLogLevelMethod = "GET"
	// SetLogLevelMethod is the HTTP method used to change the log level.
	SetLogLevelMethod = "PUT"
	// PProfEndpointsPrefix PProf endpoints path prefix.
	PProfEndpointsPrefix = "/debug/pprof/"
)
