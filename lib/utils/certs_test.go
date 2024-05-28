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

package utils

import (
	"runtime"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
)

func TestRejectsInvalidPEMData(t *testing.T) {
	t.Parallel()

	_, err := ReadCertificates([]byte("no data"))
	require.True(t,
		trace.IsNotFound(err),
		"want trace.NotFoundError, got %v (%T)", err, trace.Unwrap(err))
}

func TestRejectsSelfSignedCertificate(t *testing.T) {
	t.Parallel()

	certificateChain, err := ReadCertificatesFromPath("../../fixtures/certs/ca.pem")
	require.NoError(t, err)

	err = VerifyCertificateChain(certificateChain)
	switch runtime.GOOS {
	case constants.DarwinOS:
		require.ErrorContains(t, err, "certificate is not standards compliant")
	default:
		require.ErrorContains(t, err, "x509: certificate signed by unknown authority")
	}
}

func TestNewCertPoolFromPath(t *testing.T) {
	t.Parallel()

	pool, err := NewCertPoolFromPath("../../fixtures/certs/ca.pem")
	require.NoError(t, err)
	//nolint:staticcheck // Pool not returned by SystemCertPool
	require.Len(t, pool.Subjects(), 1)
}
