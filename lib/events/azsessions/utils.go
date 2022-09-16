// Copyright 2022 Gravitational, Inc
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

package azsessions

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/gravitational/trace"
)

var eTagAny = azblob.ETagAny

var blobDoesNotExist = azblob.BlobAccessConditions{
	ModifiedAccessConditions: &azblob.ModifiedAccessConditions{
		IfNoneMatch: &eTagAny,
	},
}

// cErr attempts to convert err to a meaningful trace error if it's a
// *azblob.StorageError; if it can't, it'll return the error, wrapped.
func cErr(err error) error {
	if err == nil {
		return nil
	}

	var stErr *azblob.StorageError
	if !errors.As(err, &stErr) || stErr == nil {
		return trace.Wrap(err)
	}

	return trace.WrapWithMessage(trace.ReadError(stErr.StatusCode(), nil), stErr.ErrorCode)
}

// cErr2 converts the error as in cErr, leaving the first argument untouched.
func cErr2[T any](v T, err error) (T, error) {
	return v, cErr(err)
}

// cachedTokenCredential is a TokenCredential that will cache the last requested
// AccessToken, and will reuse it without fetching one again as long as the
// TokenRequestOptions match and the token has at least 10 more minutes before
// expiration.
type cachedTokenCredential struct {
	azcore.TokenCredential

	mu      sync.Mutex
	options policy.TokenRequestOptions
	token   azcore.AccessToken
}

var _ azcore.TokenCredential = (*cachedTokenCredential)(nil)

// GetToken implements azcore.TokenCredential
func (c *cachedTokenCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	c.mu.Lock()
	if reflect.DeepEqual(options, c.options) && c.token.ExpiresOn.After(time.Now().Add(10*time.Minute)) {
		defer c.mu.Unlock()
		return c.token, nil
	}
	c.mu.Unlock()

	token, err := c.TokenCredential.GetToken(ctx, options)
	if err != nil {
		return azcore.AccessToken{}, trace.Wrap(err)
	}

	c.mu.Lock()
	c.options, c.token = options, token
	c.mu.Unlock()

	return token, nil
}
