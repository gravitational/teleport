//go:build windows

// Copyright 2023 Gravitational, Inc
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

package native

import (
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust"
)

const (
	deviceStateFolderName        = ".teleport-device"
	attestationKeyFileName       = "attestation.key"
	credentialActivationFileName = "credential-activation"
)

// tpmDevice implements the generic device trust client-side operations for
// TPM-based devices (Windows and Linux).
//
// Implementors must provide all function fields, as well as the top-level
// collectDeviceData method.
type tpmDevice struct {
	isElevatedProcess                 func() (bool, error)
	activateCredentialInElevatedChild func(
		encryptedCredential attest.EncryptedCredential,
		credActivationPath string,
		debug bool,
	) (solutionBytes []byte, err error)
}

type deviceState struct {
	attestationKeyPath       string
	credentialActivationPath string
}

// userDirFunc is used to determine where to save/lookup the device's
// attestation key.
// We use os.UserCacheDir instead of os.UserConfigDir because the latter is
// roaming (which we don't want for device-specific keys).
var userDirFunc = os.UserCacheDir

// setupDeviceStateDir ensures that device state directory exists.
// It returns a struct containing the path of each part of the device state,
// or nil and an error if it was not possible to set up the directory.
func setupDeviceStateDir(getBaseDir func() (string, error)) (*deviceState, error) {
	base, err := getBaseDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	deviceStateDirPath := filepath.Join(base, deviceStateFolderName)
	ds := &deviceState{
		attestationKeyPath:       filepath.Join(deviceStateDirPath, attestationKeyFileName),
		credentialActivationPath: filepath.Join(deviceStateDirPath, credentialActivationFileName),
	}

	switch _, err := os.Stat(deviceStateDirPath); {
	case os.IsNotExist(err):
		// If it doesn't exist, we can create it and return as we know
		// the perms are correct as we created it.
		if err := os.Mkdir(deviceStateDirPath, 0700); err != nil {
			return nil, trace.Wrap(err)
		}
	case err != nil:
		return nil, trace.Wrap(err)
	}

	return ds, nil
}

// getMarshaledEK returns the EK public key in PKIX, ASN.1 DER format.
func getMarshaledEK(tpm *attest.TPM) (ekKey []byte, ekCert []byte, err error) {
	eks, err := tpm.EKs()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if len(eks) == 0 {
		// This is a pretty unusual case, `go-attestation` will attempt to
		// create an EK if no EK Certs are present in the NVRAM of the TPM.
		// Either way, it lets us catch this early in case `go-attestation`
		// misbehaves.
		return nil, nil, trace.BadParameter("no endorsement keys found in tpm")
	}
	// The first EK returned by `go-attestation` will be an RSA based EK key or
	// EK cert. On Windows, ECC certs may also be returned following this. At
	// this time, we are only interested in RSA certs, so we just consider the
	// first thing returned.
	encodedEKKey, err := x509.MarshalPKIXPublicKey(eks[0].Public)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if eks[0].Certificate == nil {
		return encodedEKKey, nil, nil
	}
	return encodedEKKey, eks[0].Certificate.Raw, nil
}

// loadAK attempts to load an AK from disk. A NotFound error will be
// returned if no such file exists.
func loadAK(
	tpm *attest.TPM,
	persistencePath string,
) (*attest.AK, error) {
	ref, err := os.ReadFile(persistencePath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	ak, err := tpm.LoadAK(ref)
	if err != nil {
		return nil, trace.Wrap(err, "loading ak into tpm")
	}

	return ak, nil
}

func createAndSaveAK(
	tpm *attest.TPM,
	persistencePath string,
) (*attest.AK, error) {
	ak, err := tpm.NewAK(&attest.AKConfig{})
	if err != nil {
		return nil, trace.Wrap(err, "creating ak")
	}

	// Write it to the well-known location on disk
	ref, err := ak.Marshal()
	if err != nil {
		return nil, trace.Wrap(err, "marshaling ak")
	}
	err = os.WriteFile(persistencePath, ref, 0600)
	if err != nil {
		return nil, trace.Wrap(err, "writing ak to disk")
	}

	return ak, nil
}

func (d *tpmDevice) enrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	stateDir, err := setupDeviceStateDir(userDirFunc)
	if err != nil {
		return nil, trace.Wrap(err, "setting up device state directory")
	}

	tpm, err := attest.OpenTPM(&attest.OpenConfig{
		TPMVersion: attest.TPMVersion20,
	})
	if err != nil {
		return nil, trace.Wrap(err, "opening tpm")
	}
	defer func() {
		if err := tpm.Close(); err != nil {
			log.WithError(err).Debug("TPM: Failed to close TPM.")
		}
	}()

	// Try to load an existing AK in the case of re-enrollment, but, if the
	// AK does not exist, create one and persist it.
	ak, err := loadAK(tpm, stateDir.attestationKeyPath)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err, "loading ak")
		}
		log.Debug("TPM: No existing AK was found on disk, an AK will be created.")
		ak, err = createAndSaveAK(tpm, stateDir.attestationKeyPath)
		if err != nil {
			return nil, trace.Wrap(err, "creating ak")
		}
	} else {
		log.Debug("TPM: Existing AK was found on disk, it will be reused.")
	}
	defer ak.Close(tpm)

	deviceData, err := collectDeviceData()
	if err != nil {
		return nil, trace.Wrap(err, "collecting device data")
	}

	ekKey, ekCert, err := getMarshaledEK(tpm)
	if err != nil {
		return nil, trace.Wrap(err, "marshaling ek")
	}

	credentialID, err := credentialIDFromAK(ak)
	if err != nil {
		return nil, trace.Wrap(err, "determining credential id")
	}

	enrollPayload := &devicepb.TPMEnrollPayload{
		AttestationParameters: devicetrust.AttestationParametersToProto(
			ak.AttestationParameters(),
		),
	}
	switch {
	// Prefer ekCert over ekPub
	case ekCert != nil:
		enrollPayload.Ek = &devicepb.TPMEnrollPayload_EkCert{
			EkCert: ekCert,
		}
	case ekKey != nil:
		enrollPayload.Ek = &devicepb.TPMEnrollPayload_EkKey{
			EkKey: ekKey,
		}
	default:
		return nil, trace.BadParameter("tpm has neither ek_key or ek_cert")
	}

	return &devicepb.EnrollDeviceInit{
		CredentialId: credentialID,
		DeviceData:   deviceData,
		Tpm:          enrollPayload,
	}, nil
}

// credentialIDFromAK produces a deterministic, short-ish, unique-ish, printable
// string identifier for a given AK. This can then be used as a reference for
// this AK in the backend.
//
// To produce this, we perform a SHA256 hash over the constituent fields of
// the AKs public key and then base64 encode it to produce a human-readable
// string. This is similar to how SSH fingerprinting of public keys work.
func credentialIDFromAK(ak *attest.AK) (string, error) {
	akPub, err := attest.ParseAKPublic(
		attest.TPMVersion20,
		ak.AttestationParameters().Public,
	)
	if err != nil {
		return "", trace.Wrap(err, "parsing ak public")
	}
	publicKey := akPub.Public
	switch publicKey := publicKey.(type) {
	// at this time `go-attestation` only creates RSA 2048bit Attestation Keys.
	case *rsa.PublicKey:
		h := sha256.New()
		// This logic is roughly based off the openssh key fingerprinting,
		// but, the hash excludes "ssh-rsa" and the outputted id is not
		// prepended with "SHA256":
		//
		// It is imperative the order of the fields does not change in future
		// implementations.
		h.Write(big.NewInt(int64(publicKey.E)).Bytes())
		h.Write(publicKey.N.Bytes())
		return base64.RawStdEncoding.EncodeToString(h.Sum(nil)), nil
	default:
		return "", trace.BadParameter("unsupported public key type: %T", publicKey)
	}
}

func firstValidAssetTag(assetTags ...string) string {
	for _, assetTag := range assetTags {
		// Skip empty serials and known bad values.
		if assetTag == "" ||
			strings.EqualFold(assetTag, "Default string") ||
			strings.EqualFold(assetTag, "No Asset Information") {
			continue
		}
		return assetTag
	}
	return ""
}

// getDeviceCredential returns the credential ID for TPM devices.
// Remaining information is determined server-side.
func (d *tpmDevice) getDeviceCredential() (*devicepb.DeviceCredential, error) {
	stateDir, err := setupDeviceStateDir(userDirFunc)
	if err != nil {
		return nil, trace.Wrap(err, "setting up device state directory")
	}
	tpm, err := attest.OpenTPM(&attest.OpenConfig{
		TPMVersion: attest.TPMVersion20,
	})
	if err != nil {
		return nil, trace.Wrap(err, "opening tpm")
	}
	defer func() {
		if err := tpm.Close(); err != nil {
			log.WithError(err).Debug("TPM: Failed to close TPM.")
		}
	}()

	// Attempt to load the AK from well-known location.
	ak, err := loadAK(tpm, stateDir.attestationKeyPath)
	if err != nil {
		return nil, trace.Wrap(err, "loading ak")
	}
	defer ak.Close(tpm)

	credentialID, err := credentialIDFromAK(ak)
	if err != nil {
		return nil, trace.Wrap(err, "determining credential id")
	}

	return &devicepb.DeviceCredential{
		Id: credentialID,
	}, nil
}

func (d *tpmDevice) solveTPMEnrollChallenge(
	challenge *devicepb.TPMEnrollChallenge,
	debug bool,
) (*devicepb.TPMEnrollChallengeResponse, error) {
	stateDir, err := setupDeviceStateDir(userDirFunc)
	if err != nil {
		return nil, trace.Wrap(err, "setting up device state directory")
	}

	tpm, err := attest.OpenTPM(&attest.OpenConfig{
		TPMVersion: attest.TPMVersion20,
	})
	if err != nil {
		return nil, trace.Wrap(err, "opening tpm")
	}
	defer func() {
		if err := tpm.Close(); err != nil {
			log.WithError(err).Debug("TPM: Failed to close TPM.")
		}
	}()

	// Attempt to load the AK from well-known location.
	ak, err := loadAK(tpm, stateDir.attestationKeyPath)
	if err != nil {
		return nil, trace.Wrap(err, "loading ak")
	}
	defer ak.Close(tpm)

	// Next perform a platform attestation using the AK.
	log.Debug("TPM: Performing platform attestation.")
	platformsParams, err := tpm.AttestPlatform(
		ak,
		challenge.AttestationNonce,
		&attest.PlatformAttestConfig{},
	)
	if err != nil {
		return nil, trace.Wrap(err, "attesting platform")
	}

	// First perform the credential activation challenge provided by the
	// auth server.
	log.Debug("TPM: Activating credential.")
	encryptedCredential := devicetrust.EncryptedCredentialFromProto(
		challenge.EncryptedCredential,
	)
	if encryptedCredential == nil {
		return nil, trace.BadParameter("missing encrypted credential in challenge from server")
	}

	elevated, err := d.isElevatedProcess()
	if err != nil {
		return nil, trace.Wrap(err, "checking if process is elevated")
	}

	var activationSolution []byte
	if elevated {
		log.Debug("TPM: Detected current process is elevated. Will run credential activation in current process.")
		// If we are running with elevated privileges, we can just complete the
		// credential activation here.
		activationSolution, err = ak.ActivateCredential(
			tpm,
			*encryptedCredential,
		)
		if err != nil {
			return nil, trace.Wrap(err, "activating credential with challenge")
		}
	} else {
		fmt.Fprintln(os.Stderr, "Detected that tsh is not running with elevated privileges. Triggering a UAC prompt to complete the enrollment in an elevated process.")
		activationSolution, err = d.activateCredentialInElevatedChild(
			*encryptedCredential,
			stateDir.credentialActivationPath,
			debug,
		)
		if err != nil {
			return nil, trace.Wrap(err, "activating credential with challenge using elevated child")
		}
		fmt.Fprintln(os.Stderr, "Successfully completed credential activation in elevated process.")
	}

	log.Debug("TPM: Enrollment challenge completed.")
	return &devicepb.TPMEnrollChallengeResponse{
		Solution: activationSolution,
		PlatformParameters: devicetrust.PlatformParametersToProto(
			platformsParams,
		),
	}, nil
}

func (d *tpmDevice) handleTPMActivateCredential(encryptedCredential, encryptedCredentialSecret string) error {
	log.Debug("Performing credential activation.")
	// The two input parameters are base64 encoded, so decode them.
	credentialBytes, err := base64.StdEncoding.DecodeString(encryptedCredential)
	if err != nil {
		return trace.Wrap(err, "decoding encrypted credential")
	}
	secretBytes, err := base64.StdEncoding.DecodeString(encryptedCredentialSecret)
	if err != nil {
		return trace.Wrap(err, "decoding encrypted credential secret")
	}

	stateDir, err := setupDeviceStateDir(userDirFunc)
	if err != nil {
		return trace.Wrap(err, "setting up device state directory")
	}

	tpm, err := attest.OpenTPM(&attest.OpenConfig{
		TPMVersion: attest.TPMVersion20,
	})
	if err != nil {
		return trace.Wrap(err, "opening tpm")
	}
	defer func() {
		if err := tpm.Close(); err != nil {
			log.WithError(err).Debug("TPM: Failed to close TPM.")
		}
	}()

	// Attempt to load the AK from well-known location.
	ak, err := loadAK(tpm, stateDir.attestationKeyPath)
	if err != nil {
		return trace.Wrap(err, "loading ak")
	}
	defer ak.Close(tpm)

	solution, err := ak.ActivateCredential(
		tpm,
		attest.EncryptedCredential{
			Credential: credentialBytes,
			Secret:     secretBytes,
		},
	)
	if err != nil {
		return trace.Wrap(err, "activating credential with challenge")
	}

	log.Debug("Completed credential activation. Returning result to original process.")
	return trace.Wrap(
		os.WriteFile(stateDir.credentialActivationPath, solution, 0600),
	)
}

func (d *tpmDevice) solveTPMAuthnDeviceChallenge(
	challenge *devicepb.TPMAuthenticateDeviceChallenge,
) (*devicepb.TPMAuthenticateDeviceChallengeResponse, error) {
	stateDir, err := setupDeviceStateDir(userDirFunc)
	if err != nil {
		return nil, trace.Wrap(err, "setting up device state directory")
	}

	tpm, err := attest.OpenTPM(&attest.OpenConfig{
		TPMVersion: attest.TPMVersion20,
	})
	if err != nil {
		return nil, trace.Wrap(err, "opening tpm")
	}
	defer func() {
		if err := tpm.Close(); err != nil {
			log.WithError(err).Debug("TPM: Failed to close TPM")
		}
	}()

	// Attempt to load the AK from well-known location.
	ak, err := loadAK(tpm, stateDir.attestationKeyPath)
	if err != nil {
		return nil, trace.Wrap(err, "loading ak")
	}
	defer ak.Close(tpm)

	// Next perform a platform attestation using the AK.
	log.Debug("TPM: Performing platform attestation.")
	platformsParams, err := tpm.AttestPlatform(
		ak,
		challenge.AttestationNonce,
		&attest.PlatformAttestConfig{},
	)
	if err != nil {
		return nil, trace.Wrap(err, "attesting platform")
	}

	log.Debug("TPM: Authenticate device challenge completed.")
	return &devicepb.TPMAuthenticateDeviceChallengeResponse{
		PlatformParameters: devicetrust.PlatformParametersToProto(
			platformsParams,
		),
	}, nil
}

// signChallenge is not implemented for TPM devices, as platform attestation
// is used instead.
func (d *tpmDevice) signChallenge(_ []byte) (sig []byte, err error) {
	// NotImplemented may be interpreted as lack of server-side support, so
	// BadParameter is used instead.
	return nil, trace.BadParameter("signChallenge not implemented for TPM devices")
}
