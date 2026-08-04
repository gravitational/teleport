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

// Package httprecorder records proxied application-session HTTP exchanges.
//
// Each exchange is written as request metadata, request body chunks, response
// metadata, and response body chunks. The events share one request ID so they
// can be stitched back together.
//
// Body chunks follow the underlying Read and Write calls, except that very
// large chunks are split at maxChunkSize. Consumers can rebuild a body by
// sorting on chunk_index and stopping at the chunk where is_last is true.

package httprecorder
