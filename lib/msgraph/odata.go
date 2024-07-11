package msgraph

import "encoding/json"

// oDataPage defines the structure of a response to a paginated MS Graph endpoint.
// Value is an abstract `json.RawMessage` type to offer flexibility for the consumer,
// e.g. [client.IterateGroupMembers] will deserialize each of the array elements into potentially different concrete types.
type oDataPage struct {
	NextLink string          `json:"@odata.nextLink,omitempty"`
	Value    json.RawMessage `json:"value,omitempty"`
}

// oDataListResponse defines the structure of a simple "list" response from the MS Graph API.
type oDataListResponse[T any] struct {
	Value []T `json:"value,omitempty"`
}
