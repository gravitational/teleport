// Package pagination supplies common types and definitions to help make paging
// through record sets less error prone. The purpose of having different types
// for the "page request" and "next page" is to make them incompatible and
// require an explicit conversion, which makes accidentally using the wrong
// token value while paging a compile-time error rather than a runtime bug.
package pagination

// PageRequestToken is a request for a given page of data. The only requirement
// is that the underlying format of the page request is the same as its
// corresponding NextPageToken.
type PageRequestToken string

// NextPageToken is an indication returned by a function showing where the next
// page of data to be returned starts.
type NextPageToken string

const (
	EndOfList NextPageToken = ""
)
