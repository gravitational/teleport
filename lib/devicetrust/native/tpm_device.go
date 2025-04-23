//go:build linux || windows

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

package native

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"log/slog"
	"math/big"
	"os"

	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust"
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
	ctx := context.Background()
	logger := slog.With(teleport.ComponentKey, "TPM")
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
			logger.DebugContext(ctx, "Failed to close TPM", "error", err)
		}
	}()

	// Try to load an existing AK in the case of re-enrollment, but, if the
	// AK does not exist, create one and persist it.
	ak, err := loadAK(tpm, stateDir.attestationKeyPath)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err, "loading ak")
		}
		logger.DebugContext(ctx, "No existing AK was found on disk, an AK will be created")
		ak, err = createAndSaveAK(tpm, stateDir.attestationKeyPath)
		if err != nil {
			return nil, trace.Wrap(err, "creating ak")
		}
	} else {
		logger.DebugContext(ctx, "Existing AK was found on disk, it will be reused")
	}
	defer ak.Close(tpm)

	deviceData, err := CollectDeviceData(CollectedDataAlwaysEscalate)
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
			slog.DebugContext(context.Background(), "Failed to close TPM",
				teleport.ComponentKey, "TPM",
				"error", err,
			)
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
	ctx := context.Background()
	logger := slog.With(teleport.ComponentKey, "TPM")

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
			logger.DebugContext(ctx, "Failed to close TPM", "error", err)
		}
	}()

	// Attempt to load the AK from well-known location.
	ak, err := loadAK(tpm, stateDir.attestationKeyPath)
	if err != nil {
		return nil, trace.Wrap(err, "loading ak")
	}
	defer ak.Close(tpm)

	// Next perform a platform attestation using the AK.
	platformsParams, err := attestPlatform(tpm, ak, challenge.AttestationNonce)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// First perform the credential activation challenge provided by the
	// auth server.
	logger.DebugContext(ctx, "Activating credential")
	encryptedCredential := devicetrust.EncryptedCredentialFromProto(
		challenge.EncryptedCredential,
	)
	if encryptedCredential == nil {
		return nil, trace.BadParameter("missing encrypted credential in challenge from server")
	}

	// Note: elevated flow only happens on Windows.
	elevated, err := d.isElevatedProcess()
	if err != nil {
		return nil, trace.Wrap(err, "checking if process is elevated")
	}

	var activationSolution []byte
	if elevated {
		logger.DebugContext(ctx, "Detected current process is elevated. Will run credential activation in current process")
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

	logger.DebugContext(ctx, "Enrollment challenge completed.")
	return &devicepb.TPMEnrollChallengeResponse{
		Solution: activationSolution,
		PlatformParameters: devicetrust.PlatformParametersToProto(
			platformsParams,
		),
	}, nil
}

//nolint:unused // Used by Windows builds.
func (d *tpmDevice) handleTPMActivateCredential(encryptedCredential, encryptedCredentialSecret string) error {
	ctx := context.Background()
	logger := slog.With(teleport.ComponentKey, "TPM")

	logger.DebugContext(ctx, "Performing credential activation")
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
			logger.DebugContext(ctx, "Failed to close TPM", "error", err)
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

	logger.DebugContext(ctx, "Completed credential activation, returning result to original process")
	return trace.Wrap(
		os.WriteFile(stateDir.credentialActivationPath, solution, 0600),
	)
}

func (d *tpmDevice) solveTPMAuthnDeviceChallenge(
	challenge *devicepb.TPMAuthenticateDeviceChallenge,
) (*devicepb.TPMAuthenticateDeviceChallengeResponse, error) {
	ctx := context.Background()
	logger := slog.With(teleport.ComponentKey, "TPM")

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
			logger.DebugContext(ctx, "Failed to close TPM", "error", err)
		}
	}()

	// Attempt to load the AK from well-known location.
	ak, err := loadAK(tpm, stateDir.attestationKeyPath)
	if err != nil {
		return nil, trace.Wrap(err, "loading ak")
	}
	defer ak.Close(tpm)

	// Next perform a platform attestation using the AK.
	platformsParams, err := attestPlatform(tpm, ak, challenge.AttestationNonce)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logger.DebugContext(ctx, "Authenticate device challenge completed")
	return &devicepb.TPMAuthenticateDeviceChallengeResponse{
		PlatformParameters: devicetrust.PlatformParametersToProto(
			platformsParams,
		),
	}, nil
}

func attestPlatform(tpm *attest.TPM, ak *attest.AK, nonce []byte) (*attest.PlatformParameters, error) {
	ctx := context.Background()
	logger := slog.With(teleport.ComponentKey, "TPM")

	config := &attest.PlatformAttestConfig{}

	logger.DebugContext(ctx, "Performing platform attestation")
	platformsParams, err := tpm.AttestPlatform(ak, nonce, config)
	if err == nil {
		return platformsParams, nil
	}

	// Retry attest errors with an empty event log. Ideally we'd check for
	// errors.Is(err, fs.ErrPermission), but the go-attestation version at time of
	// writing (v0.5.0) doesn't wrap the underlying error.
	// This is a common occurrence for Linux devices.
	logger.DebugContext(ctx, "Platform attestation failed with permission error, attempting without event log", "error", err)
	config.EventLog = []byte{}
	platformsParams, err = tpm.AttestPlatform(ak, nonce, config)
	return platformsParams, trace.Wrap(err, "attesting platform")
}
