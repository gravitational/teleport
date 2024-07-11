package msgraph

import "encoding/json"

type oDataPage struct {
	NextLink string          `json:"@odata.nextLink,omitempty"`
	Value    json.RawMessage `json:"value,omitempty"`
}

type oDataListResponse[T any] struct {
	Value []T `json:"value,omitempty"`
}
