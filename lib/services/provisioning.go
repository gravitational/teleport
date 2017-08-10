/*
Copyright 2015 Gravitational, Inc.

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

package services

import (
	"fmt"
	"time"

	"github.com/gravitational/teleport"
)

// Provisioner governs adding new nodes to the cluster
type Provisioner interface {
	// UpsertToken adds provisioning tokens for the auth server
	UpsertToken(token string, roles teleport.Roles, ttl time.Duration) error

	// GetToken finds and returns token by id
	GetToken(token string) (*ProvisionToken, error)

	// DeleteToken deletes provisioning token
	DeleteToken(token string) error

	// GetTokens returns all non-expired tokens
	GetTokens() ([]ProvisionToken, error)
}

// ProvisionToken stores metadata about some provisioning token
type ProvisionToken struct {
	Roles   teleport.Roles `json:"roles"`
	Expires time.Time      `json:"expires"`
	Token   string         `json:"token"`
}

// String returns the human readable representation of a provisioning token.
func (p ProvisionToken) String() string {
	expires := "never"
	if p.Expires.Unix() != 0 {
		expires = p.Expires.String()
	}
	return fmt.Sprintf("ProvisionToken(Token=%v,Roles=%v,Expires=%v)",
		p.Token, p.Roles, expires)
}
