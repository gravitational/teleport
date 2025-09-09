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

package scimsdk

import (
	"fmt"
	"net/url"
)

// WithUserNameFilter returns a QueryOption that filters the query by userName
// https://datatracker.ietf.org/doc/html/rfc7644#section-3.4.2.2
func WithUserNameFilter(userName string) QueryOption {
	return func(o *QueryOptions) {
		o.filter = stringPtr(fmt.Sprintf("userName eq %q", userName))
		o.startIndex = intPtr(1)
		o.count = intPtr(1)
	}
}

// WithDisplayNameFilter returns a QueryOption that filters the query by displayName
func WithDisplayNameFilter(displayName string) QueryOption {
	return func(o *QueryOptions) {
		o.filter = stringPtr(fmt.Sprintf("displayName eq %q", displayName))
		o.startIndex = intPtr(1)
		o.count = intPtr(1)
	}
}

// WithFilter returns a QueryOption with arbitrary filter expression
func WithFilter(filterExpression string) QueryOption {
	return func(o *QueryOptions) {
		o.filter = stringPtr(filterExpression)
	}
}

// WithStartIndex sets the query start index. Default value is defined by the SCIM server.
func WithStartIndex(n int) QueryOption {
	return func(o *QueryOptions) {
		o.startIndex = intPtr(n)
	}
}

// WithCount sets the page size for the returned results. Actual returned page size may
// be smaller, depending on the SCIM server. Default value is defined by the SCM server.
func WithCount(n int) QueryOption {
	return func(o *QueryOptions) {
		o.count = intPtr(n)
	}
}

// QueryOptions represents the options for a SCIM query.
type QueryOptions struct {
	filter     *string
	startIndex *int
	count      *int
}

func (o *QueryOptions) toQuery() url.Values {
	q := make(url.Values)
	if o.filter != nil {
		q.Set("filter", *o.filter)
	}
	if o.startIndex != nil {
		q.Set("startIndex", fmt.Sprintf("%v", *o.startIndex))
	}
	if o.count != nil {
		q.Set("count", fmt.Sprintf("%v", *o.count))
	}
	return q
}

// QueryOption is a functional option for a SCIM query.
type QueryOption func(*QueryOptions)

func stringPtr(s string) *string {
	return &s
}
func intPtr(i int) *int {
	return &i
}

// StartIndex fetches and validates the QueryOptions start index. Also returns a flag
// indicating that the value has been set, similar to a `map` read.
func (o *QueryOptions) StartIndex() (int, bool) {
	if o.startIndex == nil {
		return 0, false
	}
	return max(*o.startIndex, 1), true
}

// StartIndex fetches the QueryOptions page size. Also returns a flag indicating that the
// value has been set, similar to a `map` read.
func (o *QueryOptions) Count() (int, bool) {
	if o.count == nil {
		return 0, false
	}
	return *o.count, true
}

func (o *QueryOptions) Filter() (string, bool) {
	if o.filter == nil {
		return "", false
	}
	return *o.filter, true
}
