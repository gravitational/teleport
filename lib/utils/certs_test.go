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
