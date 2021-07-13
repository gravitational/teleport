// +build !roletester

/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package datalog

import (
	"github.com/gravitational/teleport/lib/auth"
)

// NodeAccessRequest defines a request for access for a specific user, login, and node
type NodeAccessRequest struct {
	Username  string
	Login     string
	Node      string
	Namespace string
}

// AccessResponse returns no response
type AccessResponse struct{}

// QueryAccess returns a list of accesses to Teleport. Note this function does nothing
func (c *NodeAccessRequest) QueryAccess(client auth.ClientI) (*AccessResponse, error) {
	return &AccessResponse{}, nil
}

// BuildStringOutput creates the UI for displaying access responses.
func (r *AccessResponse) BuildStringOutput() string {
	return "Role tester is not available. Please recompile Teleport with rust and cargo installed."
}
