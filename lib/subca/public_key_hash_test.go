// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package subca_test

import (
	"crypto/x509"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/subca"
)

func TestHashPublicKey(t *testing.T) {
	t.Parallel()

	// RawSubjectPublicKeyInfo extracted from a test certificate.
	const hexSubjPKInfo = `3059301306072a8648ce3d020106082a8648ce3d030107034200047a625150220aced3b42f7d18eec6c77bb4d66b97de02e5a265963d0a11035fa3a490f27ad35aab7701cded5c88836decfcf442936696e7fe6f0694dca5cdfda6`
	rawSubjPKInfo, err := hex.DecodeString(hexSubjPKInfo)
	require.NoError(t, err)

	const want = `99c968f51266531bde14cbb0cd1cc52d0cc5590bde0a760da27f9be8a7ea7ad7`

	t.Run("HashCertificatePublicKey", func(t *testing.T) {
		got := subca.HashCertificatePublicKey(&x509.Certificate{
			RawSubjectPublicKeyInfo: rawSubjPKInfo,
		})
		assert.Equal(t, want, got)
	})

	t.Run("HashPublicKey", func(t *testing.T) {
		got := subca.HashPublicKey(rawSubjPKInfo)
		assert.Equal(t, want, got)
	})
}
