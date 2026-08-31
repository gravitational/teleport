package scimsdk

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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
		errResp = ErrorResponse{} // ensure it's zero in case of error
	}

	switch resp.StatusCode {
	case http.StatusPreconditionFailed:
		return trace.CompareFailed("Resource version mismatch")
	case http.StatusTooManyRequests:
		return trace.LimitExceeded("Rate limit exceeded")
	case http.StatusUnauthorized:
		return trace.AccessDenied("SCIM API request unauthorized: the provided SCIM bearer token is invalid")
	case http.StatusForbidden:
		return trace.AccessDenied("SCIM API request forbidden: the provided SCIM bearer token does not have sufficient permissions")
	case http.StatusConflict:
		return trace.AlreadyExists("%s", cmp.Or(errResp.Detail, "Already exists"))
	case http.StatusNotFound:
		// Including the response detail is important for detecting when a 404
		// returned by the AWS Identity Center SCIM service refers to a group
		// member, rather than the group itself.
		return trace.NotFound("%s", cmp.Or(errResp.Detail, "Resource not found"))
	case http.StatusBadRequest:
		return trace.BadParameter("%s", cmp.Or(errResp.Detail, "Bad request"))
	}

	if errResp.Detail == "" {
		return trace.BadParameter("unexpected status code: %v", resp.StatusCode)
	}
	return trace.BadParameter("unexpected status code: %v, detail: %v", resp.StatusCode, errResp.Detail)
}

// AsInvalidMemberError attempts to extract an [*InvalidMemberError] from the
// supplied [error]'s error chain.
func AsInvalidMemberError(err error) (*InvalidMemberError, bool) {
	if err == nil {
		return nil, false
	}

	var ive *InvalidMemberError
	if errors.As(err, &ive) {
		return ive, true
	}
	return nil, false
}

// InvalidMemberError is an error that indicates a failed [Client.PatchGroupMembers]
// call caused by a member ID not recognized by the downstream SCIM service. The
// group patch will still have been applied
type InvalidMemberError struct {
	// Candidates lists the IDs of users that potentially caused the underlying
	// failure.
	Candidates []string
}

// Error implements the builtin [error] interface for [*InvalidMemberError].
func (err *InvalidMemberError) Error() string {
	var errorText strings.Builder
	errorText.WriteString("Attempted to add an invalid or obsolete user to a group.")

	if len(err.Candidates) > 0 {
		errorText.WriteString("\nCandidate invalid AWS User IDs:\n")
		for _, uid := range err.Candidates {
			fmt.Fprintf(&errorText, "\t[%s]\n", uid)
		}
	}

	return errorText.String()
}
