// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"context"
	"time"
)

// Credentials represents the short-lived OAuth2 credentials.
type Credentials struct {
	// AccessToken is the Bearer token used to access the provider's API
	AccessToken string
	// RefreshToken is used to acquire a new access token.
	RefreshToken string
	// ExpiresAt marks the end of validity period for the access token.
	// The application must use the refresh token to acquire a new access token
	// before this time.
	ExpiresAt time.Time
}

// Store defines the interface for persisting the short-lived OAuth2 credentials.
type Store interface {
	GetCredentials(context.Context) (*Credentials, error)
	PutCredentials(context.Context, *Credentials) error
}
