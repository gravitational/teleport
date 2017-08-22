// Go FIDO U2F Library
// Copyright 2015 The Go FIDO U2F Library Authors. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package u2f

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"testing"
)

func TestSignExample(t *testing.T) {
	// Example 8.2 in FIDO U2F Raw Message Formats.

	signResp, _ := hex.DecodeString("0100000001304402204b5f0cd17534cedd8c34ee09570ef542a353df4436030ce43d406de870b847780220267bb998fac9b7266eb60e7cb0b5eabdfd5ba9614f53c7b22272ec10047a923f")

	ar, err := parseSignResponse(signResp)
	if err != nil {
		t.Error(err)
	}

	pubKeyBytes, _ := hex.DecodeString("04d368f1b665bade3c33a20f1e429c7750d5033660c019119d29aa4ba7abc04aa7c80a46bbe11ca8cb5674d74f31f8a903f6bad105fb6ab74aefef4db8b0025e1d")
	x, y := elliptic.Unmarshal(elliptic.P256(), pubKeyBytes)
	pubKey := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}

	const appID = "https://gstatic.com/securitykey/a/example.com"
	const clientData = "{\"typ\":\"navigator.id.getAssertion\",\"challenge\":\"opsXqUifDriAAmWclinfbS0e-USY0CgyJHe_Otd7z8o\",\"cid_pubkey\":{\"kty\":\"EC\",\"crv\":\"P-256\",\"x\":\"HzQwlfXX7Q4S5MtCCnZUNBw3RMzPO9tOyWjBqRl4tJ8\",\"y\":\"XVguGFLIZx1fXg3wNqfdbn75hi4-_7-BxhMljw42Ht4\"},\"origin\":\"http://example.com\"}"

	err = verifyAuthSignature(*ar, pubKey, appID, []byte(clientData))
	if err != nil {
		t.Error(err)
	}
}
