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
	"bytes"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"time"

	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/windowsexec"
)

const (
	deviceStateFolderName        = ".teleport-device"
	attestationKeyFileName       = "attestation.key"
	credentialActivationFileName = "credential-activation"
)

type deviceState struct {
	attestationKeyPath       string
	credentialActivationPath string
}

// setupDeviceStateDir ensures that device state directory exists.
// It returns the absolute path to where the attestation key can be found:
// $CONFIG_DIR/teleport-device/attestation.key
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

	if _, err := os.Stat(deviceStateDirPath); err != nil {
		if os.IsNotExist(err) {
			// If it doesn't exist, we can create it and return as we know
			// the perms are correct as we created it.
			if err := os.Mkdir(deviceStateDirPath, 700); err != nil {
				return nil, trace.Wrap(err)
			}
			return ds, nil
		}
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
		return nil, trace.Wrap(err, "marshalling ak")
	}
	err = os.WriteFile(persistencePath, ref, 600)
	if err != nil {
		return nil, trace.Wrap(err, "writing ak to disk")
	}

	return ak, nil
}

func enrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	stateDir, err := setupDeviceStateDir(os.UserConfigDir)
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
		return nil, trace.Wrap(err, "marshalling ek")
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

// getDeviceSerial returns the serial number of the device using PowerShell to
// grab the correct WMI objects. Getting it without calling into PS is possible,
// but requires interfacing with the ancient Win32 COM APIs.
func getDeviceSerial() (string, error) {
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"Get-WmiObject Win32_BIOS | Select -ExpandProperty SerialNumber",
	)
	// ThinkPad P P14s:
	// PS > Get-WmiObject Win32_BIOS | Select -ExpandProperty SerialNumber
	// PF47WND6
	out, err := cmd.Output()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(bytes.TrimSpace(out)), nil
}

func getReportedAssetTag() (string, error) {
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"Get-WmiObject Win32_SystemEnclosure | Select -ExpandProperty SMBIOSAssetTag",
	)
	// ThinkPad P P14s:
	// PS > Get-WmiObject Win32_SystemEnclosure | Select -ExpandProperty SMBIOSAssetTag
	// winaia_1337
	out, err := cmd.Output()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(bytes.TrimSpace(out)), nil
}

func getDeviceModel() (string, error) {
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"Get-WmiObject Win32_ComputerSystem | Select -ExpandProperty Model",
	)
	// ThinkPad P P14s:
	// PS> Get-WmiObject Win32_ComputerSystem | Select -ExpandProperty Model
	// 21J50013US
	out, err := cmd.Output()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(bytes.TrimSpace(out)), nil
}

func getDeviceBaseBoardSerial() (string, error) {
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"Get-WmiObject Win32_BaseBoard | Select -ExpandProperty SerialNumber",
	)
	// ThinkPad P P14s:
	// PS> Get-WmiObject Win32_BaseBoard | Select -ExpandProperty SerialNumber
	// L1HF2CM03ZT
	out, err := cmd.Output()
	if err != nil {
		return "", trace.Wrap(err)
	}

	return string(bytes.TrimSpace(out)), nil
}

func getOSVersion() (string, error) {
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"Get-WmiObject Win32_OperatingSystem | Select -ExpandProperty Version",
	)
	// ThinkPad P P14s:
	// PS>  Get-WmiObject Win32_OperatingSystem | Select -ExpandProperty Version
	// 10.0.22621
	out, err := cmd.Output()
	if err != nil {
		return "", trace.Wrap(err)
	}

	return string(bytes.TrimSpace(out)), nil
}

func getOSBuildNumber() (string, error) {
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"Get-WmiObject Win32_OperatingSystem | Select -ExpandProperty BuildNumber",
	)
	// ThinkPad P P14s:
	// PS>  Get-WmiObject Win32_OperatingSystem | Select -ExpandProperty BuildNumber
	// 22621
	out, err := cmd.Output()
	if err != nil {
		return "", trace.Wrap(err)
	}

	return string(bytes.TrimSpace(out)), nil
}

func firstOf(strings ...string) string {
	for _, str := range strings {
		if str != "" {
			return str
		}
	}
	return ""
}

func collectDeviceData() (*devicepb.DeviceCollectedData, error) {
	log.Debug("TPM: Collecting device data.")
	systemSerial, err := getDeviceSerial()
	if err != nil {
		return nil, trace.Wrap(err, "fetching system serial")
	}
	model, err := getDeviceModel()
	if err != nil {
		return nil, trace.Wrap(err, "fetching device model")
	}
	baseBoardSerial, err := getDeviceBaseBoardSerial()
	if err != nil {
		return nil, trace.Wrap(err, "fetching base board serial")
	}
	reportedAssetTag, err := getReportedAssetTag()
	if err != nil {
		return nil, trace.Wrap(err, "fetching reported asset tag")
	}
	osVersion, err := getOSVersion()
	if err != nil {
		return nil, trace.Wrap(err, "fetching os version")
	}
	osBuildNumber, err := getOSBuildNumber()
	if err != nil {
		return nil, trace.Wrap(err, "fetching os build number")
	}
	u, err := user.Current()
	if err != nil {
		return nil, trace.Wrap(err, "fetching user")
	}

	serial := firstOf(reportedAssetTag, systemSerial, baseBoardSerial)
	if serial == "" {
		return nil, trace.BadParameter("unable to determine serial number")
	}

	dcd := &devicepb.DeviceCollectedData{
		CollectTime:           timestamppb.Now(),
		OsType:                devicepb.OSType_OS_TYPE_WINDOWS,
		SerialNumber:          serial,
		ModelIdentifier:       model,
		OsUsername:            u.Username,
		OsVersion:             osVersion,
		OsBuild:               osBuildNumber,
		SystemSerialNumber:    systemSerial,
		BaseBoardSerialNumber: baseBoardSerial,
		ReportedAssetTag:      reportedAssetTag,
	}
	log.WithField(
		"device_collected_data", dcd,
	).Debug("TPM: Device data collected.")
	return dcd, nil
}

// getDeviceCredential will only return the credential ID on windows. The
// other information is determined server-side.
func getDeviceCredential() (*devicepb.DeviceCredential, error) {
	stateDir, err := setupDeviceStateDir(os.UserConfigDir)
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

func solveTPMEnrollChallenge(
	challenge *devicepb.TPMEnrollChallenge,
	debug bool,
) (*devicepb.TPMEnrollChallengeResponse, error) {
	stateDir, err := setupDeviceStateDir(os.UserConfigDir)
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

	var activationSolution []byte
	if windows.GetCurrentProcessToken().IsElevated() {
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
		_, _ = fmt.Fprintln(os.Stderr, "Detected that tsh is not running with elevated privileges. Triggering a UAC prompt to complete the enrollment in an elevated process.")
		activationSolution, err = activateCredentialInElevatedChild(
			*encryptedCredential,
			stateDir.credentialActivationPath,
			debug,
		)
		if err != nil {
			return nil, trace.Wrap(err, "activating credential with challenge using elevated child")
		}
		_, _ = fmt.Fprintln(os.Stderr, "Successfully completed credential activation in elevated process.")
	}

	log.Debug("TPM: Enrollment challenge completed.")
	return &devicepb.TPMEnrollChallengeResponse{
		Solution: activationSolution,
		PlatformParameters: devicetrust.PlatformParametersToProto(
			platformsParams,
		),
	}, nil
}

// activateCredentialInElevated child uses `runas` to trigger a child process
// with elevated privileges. This is necessary because the process must have
// elevated privileges in order to invoke the TPM 2.0 ActivateCredential
// command.
func activateCredentialInElevatedChild(
	encryptedCredential attest.EncryptedCredential,
	credActivationPath string,
	debug bool,
) ([]byte, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err, "determining current executable path")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, trace.Wrap(err, "determining current working directory")
	}

	// Clear up the results of any previous credential activation
	if err := os.Remove(credActivationPath); err != nil {
		err := trace.ConvertSystemError(err)
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err, "clearing previous credential activation results")
		}
	}

	// Assemble the parameter list. We encoded any binary data in base64.
	// These parameters cause `tsh` to invoke HandleTPMActivateCredential.
	params := []string{
		"device",
		"tpm-activate-credential",
		"--encrypted-credential",
		base64.StdEncoding.EncodeToString(encryptedCredential.Credential),
		"--encrypted-credential-secret",
		base64.StdEncoding.EncodeToString(encryptedCredential.Secret),
	}
	if debug {
		params = append(params, "-d")
	}

	log.Debug("Starting elevated process.")
	// https://learn.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-shellexecutew
	err = windowsexec.RunAsAndWait(
		exe,
		cwd,
		time.Second*10,
		params,
	)
	if err != nil {
		return nil, trace.Wrap(err, "invoking ShellExecute")
	}

	// Ensure we clean up the results of the execution once we are done with
	// it.
	defer func() {
		if err := os.Remove(credActivationPath); err != nil {
			log.WithError(err).Debug("Failed to clean up credential activation result")
		}
	}()

	solutionBytes, err := os.ReadFile(credActivationPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return solutionBytes, nil
}

func handleTPMActivateCredential(encryptedCredential, encryptedCredentialSecret string) error {
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

	stateDir, err := setupDeviceStateDir(os.UserConfigDir)
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

func solveTPMAuthnDeviceChallenge(
	challenge *devicepb.TPMAuthenticateDeviceChallenge,
) (*devicepb.TPMAuthenticateDeviceChallengeResponse, error) {
	stateDir, err := setupDeviceStateDir(os.UserConfigDir)
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

// signChallenge is not implemented on windows as TPM platform attestation
// is used instead.
func signChallenge(_ []byte) (sig []byte, err error) {
	return nil, trace.BadParameter("called signChallenge on windows")
}
