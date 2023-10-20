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

package types

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/wrappers"
)

// GenerateAppTokenRequest are the parameters used to generate an application token.
type GenerateAppTokenRequest struct {
	// Username is the Teleport identity.
	Username string

	// Roles are the roles assigned to the user within Teleport.
	Roles []string

	// Traits are the traits assigned to the user within Teleport.
	Traits wrappers.Traits

	// Expiry is time to live for the token.
	Expires time.Time

	// URI is the URI of the recipient application.
	URI string
}

// Check validates the request.
func (p *GenerateAppTokenRequest) Check() error {
	if p.Username == "" {
		return trace.BadParameter("username missing")
	}
	if p.Expires.IsZero() {
		return trace.BadParameter("expires missing")
	}
	if p.URI == "" {
		return trace.BadParameter("uri missing")
	}
	return nil
}

// GenerateSnowflakeJWT are the parameters used to generate a Snowflake JWT.
type GenerateSnowflakeJWT struct {
	// Username is the Teleport identity.
	Username string
	// Account is the Snowflake account name.
	Account string
}

// Check validates the request.
func (p *GenerateSnowflakeJWT) Check() error {
	if p.Username == "" {
		return trace.BadParameter("username missing")
	}
	if p.Account == "" {
		return trace.BadParameter("missing account")
	}
	return nil
}
