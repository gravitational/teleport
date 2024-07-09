package msgraph

import "encoding/json"

type oDataPage struct {
	NextLink string `json:"@odata.nextLink"`
	Value    json.RawMessage
}
