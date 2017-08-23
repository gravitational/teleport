// Go FIDO U2F Library
// Copyright 2015 The Go FIDO U2F Library Authors. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package u2f

import (
	"bytes"
	"crypto/elliptic"
	"encoding/hex"
	"testing"
)

const testRegRespHex = "0504b174bc49c7ca254b70d2e5c207cee9cf174820ebd77ea3c65508c26da51b657c1cc6b952f8621697936482da0a6d3d3826a59095daf6cd7c03e2e60385d2f6d9402a552dfdb7477ed65fd84133f86196010b2215b57da75d315b7b9e8fe2e3925a6019551bab61d16591659cbaf00b4950f7abfe6660e2e006f76868b772d70c253082013c3081e4a003020102020a47901280001155957352300a06082a8648ce3d0403023017311530130603550403130c476e756262792050696c6f74301e170d3132303831343138323933325a170d3133303831343138323933325a3031312f302d0603550403132650696c6f74476e756262792d302e342e312d34373930313238303030313135353935373335323059301306072a8648ce3d020106082a8648ce3d030107034200048d617e65c9508e64bcc5673ac82a6799da3c1446682c258c463fffdf58dfd2fa3e6c378b53d795c4a4dffb4199edd7862f23abaf0203b4b8911ba0569994e101300a06082a8648ce3d0403020347003044022060cdb6061e9c22262d1aac1d96d8c70829b2366531dda268832cb836bcd30dfa0220631b1459f09e6330055722c8d89b7f48883b9089b88d60d1d9795902b30410df304502201471899bcc3987e62e8202c9b39c33c19033f7340352dba80fcab017db9230e402210082677d673d891933ade6f617e5dbde2e247e70423fd5ad7804a6d3d3961ef871"

func TestRegistrationExample(t *testing.T) {
	// Example 8.1 in FIDO U2F Raw Message Formats.

	regResp, _ := hex.DecodeString(testRegRespHex)

	r, sig, err := parseRegistration(regResp)
	if err != nil {
		t.Errorf("ParseRegistration error: %v", err)
	}

	const expectedKeyHandle = "2a552dfdb7477ed65fd84133f86196010b2215b57da75d315b7b9e8fe2e3925a6019551bab61d16591659cbaf00b4950f7abfe6660e2e006f76868b772d70c25"
	actualKeyHandle := hex.EncodeToString(r.KeyHandle)
	if actualKeyHandle != expectedKeyHandle {
		t.Errorf("unexpected key handle: %s vs %s",
			actualKeyHandle, expectedKeyHandle)
	}

	const expectedAttestationCert = "3082013c3081e4a003020102020a47901280001155957352300a06082a8648ce3d0403023017311530130603550403130c476e756262792050696c6f74301e170d3132303831343138323933325a170d3133303831343138323933325a3031312f302d0603550403132650696c6f74476e756262792d302e342e312d34373930313238303030313135353935373335323059301306072a8648ce3d020106082a8648ce3d030107034200048d617e65c9508e64bcc5673ac82a6799da3c1446682c258c463fffdf58dfd2fa3e6c378b53d795c4a4dffb4199edd7862f23abaf0203b4b8911ba0569994e101300a06082a8648ce3d0403020347003044022060cdb6061e9c22262d1aac1d96d8c70829b2366531dda268832cb836bcd30dfa0220631b1459f09e6330055722c8d89b7f48883b9089b88d60d1d9795902b30410df"
	actualAttestationCert := hex.EncodeToString(r.AttestationCert.Raw)
	if actualAttestationCert != expectedAttestationCert {
		t.Errorf("unexpected attestation cert: %s vs %s",
			actualAttestationCert, expectedAttestationCert)
	}

	const expectedSig = "304502201471899bcc3987e62e8202c9b39c33c19033f7340352dba80fcab017db9230e402210082677d673d891933ade6f617e5dbde2e247e70423fd5ad7804a6d3d3961ef871"
	actualSig := hex.EncodeToString(sig)
	if actualSig != expectedSig {
		t.Errorf("unexpected signature: %s vs %s",
			actualSig, expectedSig)
	}

	const expectedPubKey = "04b174bc49c7ca254b70d2e5c207cee9cf174820ebd77ea3c65508c26da51b657c1cc6b952f8621697936482da0a6d3d3826a59095daf6cd7c03e2e60385d2f6d9"
	actualPubKey := hex.EncodeToString(
		elliptic.Marshal(r.PubKey.Curve, r.PubKey.X, r.PubKey.Y))
	if actualPubKey != expectedPubKey {
		t.Errorf("unexpected pubkey: %s vs %s",
			actualPubKey, expectedPubKey)
	}

	const appID = "http://example.com"
	const clientData = "{\"typ\":\"navigator.id.finishEnrollment\",\"challenge\":\"vqrS6WXDe1JUs5_c3i4-LkKIHRr-3XVb3azuA5TifHo\",\"cid_pubkey\":{\"kty\":\"EC\",\"crv\":\"P-256\",\"x\":\"HzQwlfXX7Q4S5MtCCnZUNBw3RMzPO9tOyWjBqRl4tJ8\",\"y\":\"XVguGFLIZx1fXg3wNqfdbn75hi4-_7-BxhMljw42Ht4\"},\"origin\":\"http://example.com\"}"
	err = verifyRegistrationSignature(*r, sig, appID, []byte(clientData))
	if err != nil {
		t.Errorf("VerifySignature error: %v", err)
	}
}

func TestSerialize(t *testing.T) {
	regResp, _ := hex.DecodeString(testRegRespHex)

	reg, _, err := parseRegistration(regResp)
	if err != nil {
		t.Errorf("ParseRegistration error: %v", err)
	}

	buf, err := reg.MarshalBinary()
	if err != nil {
		t.Errorf("MarshalBinary error: %v", err)
	}

	var reg2 Registration
	if err := reg2.UnmarshalBinary(buf); err != nil {
		t.Errorf("UnmarshalBinary error: %v", err)
	}

	if bytes.Compare(reg.Raw, reg2.Raw) != 0 {
		t.Errorf("reg.Raw differs")
	}
	if bytes.Compare(reg.KeyHandle, reg2.KeyHandle) != 0 {
		t.Errorf("reg.KeyHandle differs")
	}
	if reg.PubKey.Curve != reg2.PubKey.Curve {
		t.Errorf("reg.PubKey.Curve differs")
	}
	if reg.PubKey.X.Cmp(reg2.PubKey.X) != 0 {
		t.Errorf("reg.PubKey.X differs")
	}
	if reg.PubKey.Y.Cmp(reg2.PubKey.Y) != 0 {
		t.Errorf("reg.PubKey.Y differs")
	}
	if bytes.Compare(reg.AttestationCert.Raw, reg2.AttestationCert.Raw) != 0 {
		t.Errorf("reg.AttestationCert differs")
	}
}
