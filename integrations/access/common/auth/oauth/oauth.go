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

package oauth

import (
	"context"

	storage "github.com/gravitational/teleport/integrations/access/common/auth/storage"
)

// Authorizer is the composite interface of Exchanger and Refresher.
type Authorizer interface {
	Exchanger
	Refresher
}

// Exchanger implements the OAuth2 authorization code exchange operation.
type Exchanger interface {
	Exchange(ctx context.Context, authorizationCode string, redirectURI string) (*storage.Credentials, error)
}

// Refresher implements the OAuth2 bearer token refresh operation.
type Refresher interface {
	Refresh(ctx context.Context, refreshToken string) (*storage.Credentials, error)
}
