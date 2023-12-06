/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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

// cErr converts the error as in [cErr0], leaving the first argument untouched.
func cErr[T any](v T, err error) (T, error) {
	return v, cErr0(err)
}
