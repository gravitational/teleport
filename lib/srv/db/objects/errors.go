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

package objects

import "errors"

// ErrFetcherDisabled is a custom error that can be returned from fetcher constructor that will be reported with lower severity.
type ErrFetcherDisabled struct {
	reason string
}

func (e ErrFetcherDisabled) Error() string {
	return e.reason
}

// NewErrFetcherDisabled returns a new instance of ErrFetcherDisabled error.
func NewErrFetcherDisabled(reason string) *ErrFetcherDisabled {
	return &ErrFetcherDisabled{reason: reason}
}

// IsErrFetcherDisabled returns true if the error is ErrFetcherDisabled.
func IsErrFetcherDisabled(err error) (bool, string) {
	other := &ErrFetcherDisabled{}
	matched := errors.As(err, &other)
	return matched, other.reason
}
