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
	"context"
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/itertools/stream"
)

// StreamUsers fetches a single-read stream of users from the SCIM server,
// handling pagination as necessary. The resulting [streams.Stream] is a
// single-read sequence of ([*User], [error]) pairs. The stream will short
// circuit on the first error.
func StreamUsers(ctx context.Context, client Client, queryOptions ...QueryOption) stream.Stream[*User] {
	options := QueryOptions{
		startIndex: intPtr(1),
		count:      intPtr(50),
	}
	for _, fn := range queryOptions {
		fn(&options)
	}

	commonOptions := []QueryOption{
		WithCount(*options.count),
	}
	if options.filter != nil {
		commonOptions = append(commonOptions, WithFilter(*options.filter))
	}

	startIndex := *options.startIndex
	totalListed := 0
	isEOF := false

	return stream.PageFunc(func() ([]*User, error) {
		if isEOF {
			return nil, io.EOF
		}

		pageOptions := append(commonOptions, WithStartIndex(startIndex))
		resp, err := client.ListUsers(ctx, pageOptions...)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		pageSize := len(resp.Users)
		totalListed += pageSize
		startIndex = startIndex + pageSize
		isEOF = pageSize == 0 || totalListed >= int(resp.TotalResults)

		return resp.Users, nil
	})
}

// StreamUsers fetches a single-read stream of group records from the SCIM
// server via the supplied client, handling pagination as necessary. The resulting
// [streams.Stream] is a single-read sequence of ([*Group], [error]) pairs. The
// stream will short circuit on the first error.
func StreamGroups(ctx context.Context, client Client, queryOptions ...QueryOption) stream.Stream[*Group] {
	options := QueryOptions{
		startIndex: intPtr(1),
		count:      intPtr(50),
	}
	for _, fn := range queryOptions {
		fn(&options)
	}

	commonOptions := []QueryOption{
		WithCount(*options.count),
	}
	if options.filter != nil {
		commonOptions = append(commonOptions, WithFilter(*options.filter))
	}

	startIndex := *options.startIndex
	totalListed := 0
	isEOF := false

	return stream.PageFunc(func() ([]*Group, error) {
		if isEOF {
			return nil, io.EOF
		}

		pageOptions := append(commonOptions, WithStartIndex(startIndex))
		resp, err := client.ListGroups(ctx, pageOptions...)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		pageSize := len(resp.Groups)
		totalListed += pageSize
		isEOF = pageSize == 0 || totalListed >= int(resp.TotalResults)

		return resp.Groups, nil
	})
}
