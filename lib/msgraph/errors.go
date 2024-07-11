package msgraph

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/trace"
)

// unsupportedGroupMember is an internal error to indicate that
// the `groupmembers` endpoint has returned a member of type that we do not support (yet).
type unsupportedGroupMember struct {
	Type string
}

func (u *unsupportedGroupMember) Error() string {
	return fmt.Sprintf("Unsupported group member: %q", u.Type)
}

type graphErrorResponse struct {
	Error *GraphError `json:"error,omitempty"`
}

// GraphError defines the structure of errors returned from MS Graph API.
type GraphError struct {
	Code       string       `json:"code,omitempty"`
	Message    string       `json:"message,omitempty"`
	InnerError *GraphError  `json:"innerError,omitempty"`
	Details    []GraphError `json:"details,omitempty"`
}

func (g *GraphError) Error() string {
	var parts []string
	if g.Code != "" {
		parts = append(parts, strings.TrimPrefix(g.Code, "Request_"))
	}

	if g.Message != "" {
		parts = append(parts, g.Message)
	}

	return strings.Join(parts, ": ")
}

func readError(r io.Reader) (*GraphError, error) {
	var errResponse graphErrorResponse
	if err := json.NewDecoder(r).Decode(&errResponse); err != nil {
		return nil, trace.Wrap(err)
	}
	if errResponse.Error != nil {
		return errResponse.Error, nil
	}
	return nil, nil
}
