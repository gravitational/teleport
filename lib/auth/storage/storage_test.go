// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package storage

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestRDPLicense(t *testing.T) {
	ctx := context.Background()
	mem, err := memory.New(memory.Config{})
	require.NoError(t, err)
	storage := ProcessStorage{
		BackendStorage: mem,
		stateStorage:   mem,
	}

	_, err = storage.ReadRDPLicense(ctx, &types.RDPLicenseKey{
		Version:   1,
		Issuer:    "issuer",
		Company:   "company",
		ProductID: "productID",
	})
	require.True(t, trace.IsNotFound(err))

	licenseData := []byte{1, 2, 3}
	err = storage.WriteRDPLicense(ctx, &types.RDPLicenseKey{
		Version:   1,
		Issuer:    "issuer",
		Company:   "company",
		ProductID: "productID",
	}, licenseData)
	require.NoError(t, err)

	_, err = storage.ReadRDPLicense(ctx, &types.RDPLicenseKey{
		Version:   2,
		Issuer:    "issuer",
		Company:   "company",
		ProductID: "productID",
	})
	require.True(t, trace.IsNotFound(err))

	license, err := storage.ReadRDPLicense(ctx, &types.RDPLicenseKey{
		Version:   1,
		Issuer:    "issuer",
		Company:   "company",
		ProductID: "productID",
	})
	require.NoError(t, err)
	require.Equal(t, licenseData, license)
}
