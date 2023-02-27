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
	"crypto"
	"crypto/x509"

	"github.com/google/go-attestation/attest"
	"github.com/google/uuid"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var keyConfig = &attest.KeyConfig{
	Algorithm: attest.RSA,
	Size:      2048,
}

// TODO(joel): implement
func getOrCreateAK(tpm *attest.TPM) (*attest.AK, error) {
	return nil, nil
}

// TODO(joel): implement
func getOrCreateAppKey(tpm *attest.TPM, ak *attest.AK) (uuid.UUID, *attest.Key, error) {
	return uuid.UUID{}, nil, nil
}

func getEKPkix(tpm *attest.TPM) ([]byte, error) {
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
	tpm, err := attest.OpenTPM(nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer tpm.Close()

	ekPublic, err := getEKPkix(tpm)
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

func collectDeviceData() (*devicepb.DeviceCollectedData, error) {
	return &devicepb.DeviceCollectedData{
		CollectTime: timestamppb.Now(),
		OsType:      devicepb.OSType_OS_TYPE_WINDOWS,
		// TODO(joel): collect proper serial here
		SerialNumber: "",
	}, nil
}

func signChallenge(chal []byte) ([]byte, error) {
	tpm, err := attest.OpenTPM(nil)
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

	signer, ok := priv.(crypto.Signer)
	if !ok {
		return nil, trace.BadParameter("private key is not a crypto.Signer")
	}

	sig, err := signer.Sign(nil, chal, crypto.SHA256)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sig, nil
}

func tpmEnrollChallenge(encrypted []byte, credential []byte) ([]byte, error) {
	tpm, err := attest.OpenTPM(nil)
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
	tpm, err := attest.OpenTPM(nil)
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

	publicDer, err := x509.MarshalPKIXPublicKey(appKey.Public)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &devicepb.DeviceCredential{
		Id:           appKeyID.String(),
		PublicKeyDer: publicDer,
	}, nil
}
