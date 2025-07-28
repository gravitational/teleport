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

package hostid

import "github.com/gravitational/teleport/api/utils/retryutils"

type options struct {
	retryConfig    retryutils.RetryV2Config
	iterationLimit int
	hostID         string
}

// WithID sets the given host UUID instead of generating a new one.
// [ReadOrCreateFile].
func WithID(id string) func(*options) {
	return func(o *options) {
		o.hostID = id
	}
}

// WithBackoff overrides the default backoff configuration of
// [ReadOrCreateFile].
func WithBackoff(cfg retryutils.RetryV2Config) func(*options) {
	return func(o *options) {
		o.retryConfig = cfg
	}
}

// WithIterationLimit overrides the default number of time
// [ReadOrCreateFile] will attempt to produce a hostid.
func WithIterationLimit(limit int) func(*options) {
	return func(o *options) {
		o.iterationLimit = limit
	}
}
