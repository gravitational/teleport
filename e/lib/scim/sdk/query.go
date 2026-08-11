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

func parseQueryOptions(options ...QueryOption) QueryOptions {
	var opts QueryOptions
	for _, option := range options {
		option(&opts)
	}
	return opts
}

// QueryOptions represents the options for a SCIM query.
type QueryOptions struct {
	filter     *string
	startIndex *int
	count      *int
}

// extractRange extracts the zero-based start index and count from the
// QueryOptions struct, supplying default values if not specified.
func (o *QueryOptions) extractRange(rangeLength int) (int, int) {
	// Pull out the optional count parameter, defaulting to the entire range
	count := rangeLength
	if o.count != nil {
		count = *o.count
	}

	// Pull out the optional start index parameter, defaulting to 1 if unspecified
	// or otherwise invalid.
	startIndex := 1
	if o.startIndex != nil {
		// As per RFC7644 § 3.4.2.4: "A value less than 1 SHALL be interpreted as 1"
		startIndex = max(1, *o.startIndex)
	}
	// translate the 1-based index it to a more useful 0-based index
	startIndex = startIndex - 1

	// ensure that the start index is within the valid range so it can be used to
	// create an empty slice when out of bounds, rather than crashing
	startIndex = min(startIndex, rangeLength)

	return startIndex, count
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
