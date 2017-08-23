// Go FIDO U2F Library
// Copyright 2015 The Go FIDO U2F Library Authors. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package u2f

import (
	"testing"
)

func TestVerifyClientDataWithoutChannelId(t *testing.T) {
	const clientData = "{\"typ\":\"navigator.id.finishEnrollment\",\"challenge\":\"KLWuflMwjv5UfJ9Ua1Kaaw\",\"origin\":\"http://localhost:3483\",\"cid_pubkey\":\"\"}"

	cbytes, _ := decodeBase64("KLWuflMwjv5UfJ9Ua1Kaaw")
	c := Challenge{
		Challenge:     cbytes,
		TrustedFacets: []string{"http://localhost:3483"},
	}

	err := verifyClientData([]byte(clientData), c)
	if err != nil {
		t.Error(err)
	}
}

func TestVerifyClientDataWithChannelId(t *testing.T) {
	const clientData = "{\"typ\":\"navigator.id.finishEnrollment\",\"challenge\":\"vqrS6WXDe1JUs5_c3i4-LkKIHRr-3XVb3azuA5TifHo\",\"cid_pubkey\":{\"kty\":\"EC\",\"crv\":\"P-256\",\"x\":\"HzQwlfXX7Q4S5MtCCnZUNBw3RMzPO9tOyWjBqRl4tJ8\",\"y\":\"XVguGFLIZx1fXg3wNqfdbn75hi4-_7-BxhMljw42Ht4\"},\"origin\":\"http://example.com\"}"

	cbytes, _ := decodeBase64("vqrS6WXDe1JUs5_c3i4-LkKIHRr-3XVb3azuA5TifHo")
	c := Challenge{
		Challenge:     cbytes,
		TrustedFacets: []string{"http://example.com"},
	}

	err := verifyClientData([]byte(clientData), c)
	if err != nil {
		t.Error(err)
	}
}
