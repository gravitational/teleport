// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azsessions

import (
	"errors"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/gravitational/trace"
)

var eTagAny = azcore.ETagAny

var blobDoesNotExist = blob.AccessConditions{
	ModifiedAccessConditions: &blob.ModifiedAccessConditions{
		IfNoneMatch: &eTagAny,
	},
}

// cErr0 attempts to convert err to a meaningful trace error if it's a
// *azblob.StorageError; if it can't, it'll return the error, wrapped.
func cErr0(err error) error {
	if err == nil {
		return nil
	}

	var stErr *azcore.ResponseError
	if !errors.As(err, &stErr) || stErr == nil {
		return trace.Wrap(err)
	}

	return trace.WrapWithMessage(trace.ReadError(stErr.StatusCode, nil), stErr.ErrorCode)
}

// cErr converts the error as in cErr, leaving the first argument untouched.
func cErr[T any](v T, err error) (T, error) {
	return v, cErr0(err)
}
