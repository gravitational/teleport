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
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gravitational/trace"
)

const (
	errorSchema = "urn:ietf:params:scim:api:messages:2.0:Error"
)

// ErrorResponse encodes an error in the expected SCIM schema
type ErrorResponse struct {
	// Schemas is a list of URNs that indicate the schema used
	// for the error response.
	Schemas []string `json:"schemas,omitempty"`
	// Detail is a human-readable message describing the error.
	Detail string `json:"detail,omitempty"`
	// SCIMType is a SCIM-specific error code.
	SCIMType string `json:"scimType,omitempty"`
	// Status is the HTTP status code.
	Status string `json:"status"`
}

// FormatErrorResponse formats an error response in the SCIM schema.
func FormatErrorResponse(statusCode int, detail string) ([]byte, error) {
	response := ErrorResponse{
		Schemas: []string{errorSchema},
		Status:  strconv.Itoa(statusCode),
		Detail:  detail,
	}
	return json.Marshal(&response)
}

func decodeError(resp *http.Response) error {
	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		return trace.BadParameter("unexpected status code: %v", resp.StatusCode)
	}
	return trace.BadParameter("unexpected status code: %v, detail: %v", resp.StatusCode, errResp.Detail)
}
