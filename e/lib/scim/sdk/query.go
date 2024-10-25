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
