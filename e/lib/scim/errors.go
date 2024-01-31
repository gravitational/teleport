package scim

import (
	"encoding/json"
	"strconv"
)

const (
	errorSchema = "urn:ietf:params:scim:api:messages:2.0:Error"
)

// ErrorResponse encodes an error in the expected SCIM schema
type ErrorResponse struct {
	Schemas  []string `json:"schemas,omitempty"`
	Detail   string   `json:"detail,omitempty"`
	SCIMType string   `json:"scimType,omitempty"`
	Status   string   `json:"status"`
}

func FormatErrorResponse(statusCode int, detail string) ([]byte, error) {
	response := ErrorResponse{
		Schemas: []string{errorSchema},
		Status:  strconv.Itoa(statusCode),
		Detail:  detail,
	}
	return json.Marshal(&response)
}
