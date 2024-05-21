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

package keystore

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/service/servicecfg"
)

func HSMTestConfig(t *testing.T) servicecfg.KeystoreConfig {
	if cfg, ok := yubiHSMTestConfig(t); ok {
		t.Log("Running test with YubiHSM")
		return cfg
	}
	if cfg, ok := cloudHSMTestConfig(t); ok {
		t.Log("Running test with AWS CloudHSM")
		return cfg
	}
	if cfg, ok := awsKMSTestConfig(t); ok {
		t.Log("Running test with AWS KMS")
		return cfg
	}
	if cfg, ok := gcpKMSTestConfig(t); ok {
		t.Log("Running test with GCP KMS")
		return cfg
	}
	if cfg, ok := softHSMTestConfig(t); ok {
		t.Log("Running test with SoftHSM")
		return cfg
	}
	t.Skip("No HSM available for test")
	return servicecfg.KeystoreConfig{}
}

func yubiHSMTestConfig(t *testing.T) (servicecfg.KeystoreConfig, bool) {
	yubiHSMPath := os.Getenv("TELEPORT_TEST_YUBIHSM_PKCS11_PATH")
	yubiHSMPin := os.Getenv("TELEPORT_TEST_YUBIHSM_PIN")
	if yubiHSMPath == "" || yubiHSMPin == "" {
		return servicecfg.KeystoreConfig{}, false
	}
	slotNumber := 0
	return servicecfg.KeystoreConfig{
		PKCS11: servicecfg.PKCS11Config{
			Path:       yubiHSMPath,
			SlotNumber: &slotNumber,
			Pin:        yubiHSMPin,
		},
	}, true
}

func cloudHSMTestConfig(t *testing.T) (servicecfg.KeystoreConfig, bool) {
	cloudHSMPin := os.Getenv("TELEPORT_TEST_CLOUDHSM_PIN")
	if cloudHSMPin == "" {
		return servicecfg.KeystoreConfig{}, false
	}
	return servicecfg.KeystoreConfig{
		PKCS11: servicecfg.PKCS11Config{
			Path:       "/opt/cloudhsm/lib/libcloudhsm_pkcs11.so",
			TokenLabel: "cavium",
			Pin:        cloudHSMPin,
		},
	}, true
}

func awsKMSTestConfig(t *testing.T) (servicecfg.KeystoreConfig, bool) {
	awsKMSAccount := os.Getenv("TELEPORT_TEST_AWS_KMS_ACCOUNT")
	awsKMSRegion := os.Getenv("TELEPORT_TEST_AWS_KMS_REGION")
	if awsKMSAccount == "" || awsKMSRegion == "" {
		return servicecfg.KeystoreConfig{}, false
	}
	return servicecfg.KeystoreConfig{
		AWSKMS: servicecfg.AWSKMSConfig{
			Cluster:    "test-cluster",
			AWSAccount: awsKMSAccount,
			AWSRegion:  awsKMSRegion,
		},
	}, true
}

func gcpKMSTestConfig(t *testing.T) (servicecfg.KeystoreConfig, bool) {
	gcpKeyring := os.Getenv("TELEPORT_TEST_GCP_KMS_KEYRING")
	if gcpKeyring == "" {
		return servicecfg.KeystoreConfig{}, false
	}
	return servicecfg.KeystoreConfig{
		GCPKMS: servicecfg.GCPKMSConfig{
			KeyRing:         gcpKeyring,
			ProtectionLevel: "SOFTWARE",
		},
	}, true
}

var (
	cachedSoftHSMConfig      *servicecfg.KeystoreConfig
	cachedSoftHSMConfigMutex sync.Mutex
)

// softHSMTestConfig is for use in tests only and creates a test SOFTHSM2 token.
// This should be used for all tests which need to use SoftHSM because the
// library can only be initialized once and SOFTHSM2_PATH and SOFTHSM2_CONF
// cannot be changed. New tokens added after the library has been initialized
// will not be found by the library.
//
// A new token will be used for each `go test` invocation, but it's difficult
// to create a separate token for each test because because new tokens
// added after the library has been initialized will not be found by the
// library. It's also difficult to clean up the token because tests for all
// packages are run in parallel there is not a good time to safely
// delete the token or the entire token directory. Each test should clean up
// all keys that it creates because SoftHSM2 gets really slow when there are
// many keys for a given token.
func softHSMTestConfig(t *testing.T) (servicecfg.KeystoreConfig, bool) {
	path := os.Getenv("SOFTHSM2_PATH")
	if path == "" {
		return servicecfg.KeystoreConfig{}, false
	}

	cachedSoftHSMConfigMutex.Lock()
	defer cachedSoftHSMConfigMutex.Unlock()

	if cachedSoftHSMConfig != nil {
		return *cachedSoftHSMConfig, true
	}

	if os.Getenv("SOFTHSM2_CONF") == "" {
		// create tokendir
		tokenDir, err := os.MkdirTemp("", "tokens")
		require.NoError(t, err)

		// create config file
		configFile, err := os.CreateTemp("", "softhsm2.conf")
		require.NoError(t, err)

		// write config file
		_, err = configFile.WriteString(fmt.Sprintf(
			"directories.tokendir = %s\nobjectstore.backend = file\nlog.level = DEBUG\n",
			tokenDir))
		require.NoError(t, err)
		require.NoError(t, configFile.Close())

		// set env
		os.Setenv("SOFTHSM2_CONF", configFile.Name())
	}

	// create test token (max length is 32 chars)
	tokenLabel := strings.Replace(uuid.NewString(), "-", "", -1)
	cmd := exec.Command("softhsm2-util", "--init-token", "--free", "--label", tokenLabel, "--so-pin", "password", "--pin", "password")
	t.Logf("Running command: %q", cmd)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			require.NoError(t, exitErr, "error creating test softhsm token: %s", string(exitErr.Stderr))
		}
		require.NoError(t, err, "error attempting to run softhsm2-util")
	}

	cachedSoftHSMConfig = &servicecfg.KeystoreConfig{
		PKCS11: servicecfg.PKCS11Config{
			Path:       path,
			TokenLabel: tokenLabel,
			Pin:        "password",
		},
	}
	return *cachedSoftHSMConfig, true
}
