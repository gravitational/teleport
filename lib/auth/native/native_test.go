/*
Copyright 2017-2018 Gravitational, Inc.

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

package native

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// TestPrecomputeMode verifies that package enters precompute mode when
// PrecomputeKeys is called.
func TestPrecomputeMode(t *testing.T) {
	t.Parallel()

	PrecomputeKeys()

	select {
	case <-precomputedKeys:
	case <-time.After(time.Second * 10):
		t.Fatal("Key precompute routine failed to start.")
	}
}

// TestGenerateRSAPKSC1Keypair tests that GeneratePrivateKey generates
// a valid PKCS1 rsa key.
func TestGeneratePKSC1RSAKey(t *testing.T) {
	t.Parallel()

	priv, err := GeneratePrivateKey()
	require.NoError(t, err)

	block, rest := pem.Decode(priv.PrivateKeyPEM())
	require.NoError(t, err)
	require.Empty(t, rest)

	_, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)
}
