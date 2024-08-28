// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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
