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
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/trace"

	"github.com/google/go-attestation/attest"
	"github.com/google/go-tpm/tpm2"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	akFile    = "ak.ref"
	appSuffix = "-app.ref"
)

// keyConfig contains the parameters for the generated application keys.
// These have been picked for maximum compatibility.
var keyConfig = &attest.KeyConfig{
	Algorithm: attest.RSA,
	Size:      2048,
}

// TODO(joel): pass state from tsh profile
func tpmFilePath(elem ...string) string {
	fullElems := append([]string{profile.FullProfilePath("")}, elem...)
	return path.Join(fullElems...)
}

type windowsCmdChannel struct {
	io.ReadWriteCloser
}

func (cc *windowsCmdChannel) MeasurementLog() ([]byte, error) {
	return nil, nil
}

func openTPM() (*attest.TPM, error) {
	rawConn, err := tpm2.OpenTPM()
	if err != nil {
		return nil, err
	}

	cfg := &attest.OpenConfig{
		TPMVersion:     attest.TPMVersion20,
		CommandChannel: &windowsCmdChannel{rawConn},
	}

	tpm, err := attest.OpenTPM(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return tpm, nil
}

func getOrCreateAK(tpm *attest.TPM) (*attest.AK, error) {
	path := tpmFilePath(akFile)
	if ref, err := os.ReadFile(path); err == nil {
		ak, err := tpm.LoadAK(ref)
		if err == nil {
			return ak, nil
		}

		return ak, nil
	}

	ak, err := tpm.NewAK(nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ref, err := ak.Marshal()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = os.WriteFile(path, ref, 0644)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ak, nil
}

func getOrCreateAppKey(tpm *attest.TPM, ak *attest.AK) (uuid.UUID, *attest.Key, error) {
	basePath := tpmFilePath("")
	entries, err := ioutil.ReadDir(basePath)
	if err != nil {
		return uuid.UUID{}, nil, trace.Wrap(err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), appSuffix) {
			ref, err := os.ReadFile(filepath.Join(basePath, entry.Name()))
			if err != nil {
				return uuid.UUID{}, nil, trace.Wrap(err)
			}

			key, err := tpm.LoadKey(ref)
			if err != nil {
				return uuid.UUID{}, nil, trace.Wrap(err)
			}

			id, err := uuid.Parse(strings.TrimSuffix(entry.Name(), appSuffix))
			if err != nil {
				return uuid.UUID{}, nil, trace.Wrap(err)
			}

			return id, key, nil
		}
	}

	key, err := tpm.NewKey(ak, keyConfig)
	if err != nil {
		return uuid.UUID{}, nil, trace.Wrap(err)
	}

	id := uuid.New()
	ref, err := key.Marshal()
	if err != nil {
		return uuid.UUID{}, nil, trace.Wrap(err)
	}

	path := tpmFilePath(id.String() + appSuffix)
	err = os.WriteFile(path, ref, 0644)
	if err != nil {
		return uuid.UUID{}, nil, trace.Wrap(err)
	}

	return id, key, nil
}

func getEKPKIX(tpm *attest.TPM) ([]byte, error) {
	eks, err := tpm.EKs()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(eks) == 0 {
		return nil, trace.BadParameter("no endorsement keys found")
	}

	ekDer, err := x509.MarshalPKIXPublicKey(eks[0].Public)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ekDer, nil
}

func enrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	tpm, err := openTPM()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer tpm.Close()

	ekPublic, err := getEKPKIX(tpm)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ak, err := getOrCreateAK(tpm)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ap := ak.AttestationParameters()
	data, err := collectDeviceData()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appKeyID, appKey, err := getOrCreateAppKey(tpm, ak)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cp := appKey.CertificationParameters()
	return &devicepb.EnrollDeviceInit{
		CredentialId: appKeyID.String(),
		DeviceData:   data,
		Tpm: &devicepb.TPMEnrollPayload{
			EkPublic: ekPublic,
			AttestationData: &devicepb.TPMAttestationData{
				Public:            ap.Public,
				CreateData:        ap.CreateData,
				CreateAttestation: ap.CreateAttestation,
				CreateSignature:   ap.CreateSignature,
			},
			AppCertificationParams: &devicepb.TPMCertificationParameters{
				Public:            cp.Public,
				CreateData:        cp.CreateData,
				CreateAttestation: cp.CreateAttestation,
				CreateSignature:   cp.CreateSignature,
			},
		},
	}, nil
}

// getLikelyDeviceSerial returns the serial number of the device using
// PowerShell to grab the correct WMI objects. Getting it without
// calling into PS is possible, but requires interfacing with the ancient Win32 COM APIs.
func getDeviceSerial() (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "Get-WmiObject win32_bios | Select -ExpandProperty Serialnumber")
	out, err := cmd.Output()
	if err != nil {
		return "", trace.Wrap(err)
	}

	return strings.TrimSpace(string(bytes.ReplaceAll(out, []byte(" "), nil))), nil
}

func collectDeviceData() (*devicepb.DeviceCollectedData, error) {
	serial, err := getDeviceSerial()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &devicepb.DeviceCollectedData{
		CollectTime:  timestamppb.Now(),
		OsType:       devicepb.OSType_OS_TYPE_WINDOWS,
		SerialNumber: serial,
	}, nil
}

func signChallenge(chal []byte) ([]byte, error) {
	tpm, err := openTPM()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer tpm.Close()

	ak, err := getOrCreateAK(tpm)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, appKey, err := getOrCreateAppKey(tpm, ak)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	priv, err := appKey.Private(appKey.Public)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// This bit of code is rather deceptive.
	// The crypto.Signer interface actually calls into the TPM here transparently to sign the challenge using the key.
	signer, ok := priv.(crypto.Signer)
	if !ok {
		return nil, trace.BadParameter("private key is not a crypto.Signer. cannot complete signing challenge")
	}

	sig, err := signer.Sign(rand.Reader, chal, crypto.SHA256)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sig, nil
}

func tpmEnrollChallenge(encrypted []byte, credential []byte) ([]byte, error) {
	tpm, err := openTPM()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer tpm.Close()

	ak, err := getOrCreateAK(tpm)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secret, err := ak.ActivateCredential(tpm, attest.EncryptedCredential{
		Credential: credential,
		Secret:     encrypted,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return secret, nil
}

func getDeviceCredential() (*devicepb.DeviceCredential, error) {
	tpm, err := openTPM()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer tpm.Close()

	ak, err := getOrCreateAK(tpm)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appKeyID, appKey, err := getOrCreateAppKey(tpm, ak)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	publicDer, err := x509.MarshalPKIXPublicKey(appKey.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &devicepb.DeviceCredential{
		Id:           appKeyID.String(),
		PublicKeyDer: publicDer,
	}, nil
}
