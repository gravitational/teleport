//go:build piv

// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package piv

import (
	"context"
	"crypto"
	"crypto/x509/pkix"
	"os"
	"sync"
	"testing"

	"github.com/go-piv/piv-go/piv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

func TestConcurrentOperations(t *testing.T) {
	// This test will overwrite any PIV data on the yubiKey.
	if os.Getenv("TELEPORT_TEST_YUBIKEY_PIV") == "" {
		t.Skipf("Skipping TestGenerateYubiKeyPrivateKey because TELEPORT_TEST_YUBIKEY_PIV is not set")
	}

	y, err := FindYubiKey(0)
	require.NoError(t, err)

	y.Reset()
	t.Cleanup(func() { y.Reset() })

	usedSlot := piv.SlotAuthentication
	ref, err := y.generatePrivateKey(usedSlot, hardwarekey.PromptPolicyNone, hardwarekey.SignatureAlgorithmEC256, 0)
	require.NoError(t, err)
	require.NotNil(t, ref)

	unusedSlot := piv.SlotCardAuthentication
	cert, err := SelfSignedMetadataCertificate(pkix.Name{})
	require.NoError(t, err)

	// Run each PIV command several times concurrently to ensure the concurrency
	// protections in place properly protect each operations, especially those
	// which do not support concurrency.
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := y.conn.getSerialNumber()
			assert.NoError(t, err, "getSerialNumber")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := y.conn.sign(context.Background(), ref, piv.KeyAuth{PINPolicy: piv.PINPolicyNever}, nil, nil, make([]byte, 100), crypto.Hash(0))
			assert.NoError(t, err, "sign")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := y.conn.getVersion()
			assert.NoError(t, err, "getVersion")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := y.conn.setCertificate(piv.DefaultManagementKey, unusedSlot, cert)
			assert.NoError(t, err, "setCertificate")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := y.conn.certificate(usedSlot)
			assert.NoError(t, err, "certificate")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := y.conn.generateKey(piv.DefaultManagementKey, unusedSlot, piv.Key{
				Algorithm:   piv.AlgorithmEC256,
				TouchPolicy: piv.TouchPolicyNever,
				PINPolicy:   piv.PINPolicyNever,
			})
			assert.NoError(t, err, "generateKey")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := y.conn.attest(usedSlot)
			assert.NoError(t, err, "attest")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := y.conn.attestationCertificate()
			assert.NoError(t, err, "attestationCertificate")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := y.conn.setPIN(piv.DefaultPIN, piv.DefaultPIN)
			assert.NoError(t, err, "setPIN")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := y.conn.setPUK(piv.DefaultPUK, piv.DefaultPUK)
			assert.NoError(t, err, "setPUK")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := y.conn.unblock(piv.DefaultPUK, piv.DefaultPIN)
			assert.NoError(t, err, "unblock")
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := y.conn.verifyPIN(piv.DefaultPIN)
			assert.NoError(t, err, "verifyPIN")
		}()
	}

	wg.Wait()
}
