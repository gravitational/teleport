/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

// Package policy implements per-request HTTP authorization for Teleport
// App Access.
//
// The package provides wire-form request-path validation (see
// ValidateWireform) and path-pattern matching (see Compile and
// Matcher.Match). It has no dependency on HTTP, resources, or the
// backend, so it is exercised entirely by table-driven unit tests.
//
// Both pieces operate on the request path in the byte form Teleport
// forwards upstream: Go's r.URL.EscapedPath() (the percent-encoded
// path), with the query excluded. Matching is byte-literal and
// case-sensitive. The calling layer must source the path this way,
// rather than from the raw r.RequestURI, so the bytes the policy
// evaluates are the bytes the reverse proxy sends upstream, including
// for non-ASCII paths that Go re-encodes: a received /unié is
// forwarded, and matched, as /uni%C3%A9. The reverse proxy re-parses
// and re-encodes the path on the way out, so the agent request handler
// is responsible for sourcing the matcher input from the same value
// the forwarder emits; that equality is verified where the handler
// lands, not in this package.
package policy
