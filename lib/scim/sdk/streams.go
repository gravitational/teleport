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
