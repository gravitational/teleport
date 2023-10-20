/*
Copyright 2023 Gravitational, Inc.

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

package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

// StaticCredential is a TokenCredential that uses a prefetched access token.
type StaticCredential struct {
	token azcore.AccessToken
}

// NewStaticCredential creates a new static credential from a token.
func NewStaticCredential(token azcore.AccessToken) *StaticCredential {
	return &StaticCredential{
		token: token,
	}
}

// GetToken gets the access token.
func (c *StaticCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return c.token, nil
}
