package msgraph

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/gravitational/trace"
)

type graphErrorResponse struct {
	Error *GraphError `json:"error,omitempty"`
}

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
