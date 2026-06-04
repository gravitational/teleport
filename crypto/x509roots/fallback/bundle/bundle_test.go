// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bundle

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"testing"
)

func TestBundle(t *testing.T) {
	for i, unparsed := range unparsedCertificates {
		cert, err := x509.ParseCertificate(rawCerts[unparsed.certStartOff : unparsed.certStartOff+unparsed.certLength])
		if err != nil {
			t.Errorf("ParseCertificate(unparsedCertificates[%v]) unexpected error: %v", i, err)
			continue
		}

		if unparsed.cn != cert.Subject.String() {
			t.Errorf("unparsedCertificates[%v].cn = %q; want = %q", i, unparsed.cn, cert.Subject.String())
		}

		sum := sha256.Sum256(cert.Raw)
		sumHex := hex.EncodeToString(sum[:])
		if sumHex != unparsed.sha256Hash {
			t.Errorf("unparsedCertificates[%v].sha256Hash = %q; want = %q", i, unparsed.sha256Hash, sumHex)
		}
	}
}
