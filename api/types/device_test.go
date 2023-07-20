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

package types

import (
	"crypto"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

func TestDeviceConversions_toAndFrom(t *testing.T) {
	t1 := time.UnixMilli(1680276526972000) // Fri Mar 31 2023 15:28:46 UTC
	t11 := t1.Add(100 * time.Millisecond)
	t2 := t1.Add(1 * time.Second)
	t22 := t1.Add(100 * time.Millisecond)

	const osType = devicepb.OSType_OS_TYPE_MACOS
	const assetTag = "llama14"
	dev := &devicepb.Device{
		ApiVersion:   "v1",
		Id:           "0af7c335-5f2c-4756-8266-9965a47ccbd3",
		OsType:       osType,
		AssetTag:     assetTag,
		CreateTime:   timestamppb.New(t1),
		UpdateTime:   timestamppb.New(t2),
		EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED,
		Credential: &devicepb.DeviceCredential{
			Id:                    "557762f0-4cd4-4b75-aaee-575c57237c0b",
			PublicKeyDer:          []byte("insert public key here"),
			DeviceAttestationType: devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_UNSPECIFIED,
			TpmEkcertSerial:       "00:00:00:00:00:00:00:00:00:00:00:DE:AD:BE:EF:CA:FE",
			TpmAkPublic:           []byte("a TPMT_PUBLIC encoded blob"),
		},
		CollectedData: []*devicepb.DeviceCollectedData{
			{
				CollectTime:  timestamppb.New(t1),
				RecordTime:   timestamppb.New(t11),
				OsType:       osType,
				SerialNumber: assetTag,
			},
			{
				CollectTime:             timestamppb.New(t2),
				RecordTime:              timestamppb.New(t22),
				OsType:                  osType,
				SerialNumber:            assetTag,
				ModelIdentifier:         "MacBookPro9,2",
				OsVersion:               "13.1.2",
				OsBuild:                 "22D68",
				OsUsername:              "llama",
				JamfBinaryVersion:       "9.27",
				MacosEnrollmentProfiles: "Enrolled via DEP: No\nMDM enrollment: Yes (User Approved)\nMDM server: ...",
				ReportedAssetTag:        assetTag + "-reported",
				SystemSerialNumber:      assetTag + "-system",
				BaseBoardSerialNumber:   assetTag + "-board",
				TpmPlatformAttestation: &devicepb.TPMPlatformAttestation{
					Nonce: []byte("foo-bar-bizz"),
					PlatformParameters: &devicepb.TPMPlatformParameters{
						EventLog: []byte("dummy-event-log"),
						Quotes: []*devicepb.TPMQuote{
							{
								Quote:     []byte("fake-quote-1"),
								Signature: []byte("fake-signature-1"),
							},
							{
								Quote:     []byte("fake-quote-2"),
								Signature: []byte("fake-signature-2"),
							},
						},
						Pcrs: []*devicepb.TPMPCR{
							{
								Index:     0,
								Digest:    []byte("fake-sha1-digest"),
								DigestAlg: uint64(crypto.SHA1),
							},
							{
								Index:     1,
								Digest:    []byte("fake-sha256-digest"),
								DigestAlg: uint64(crypto.SHA256),
							},
						},
					},
				},
			},
		},
		Source: &devicepb.DeviceSource{
			Name:   "myscript",
			Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_API,
		},
		Profile: &devicepb.DeviceProfile{
			UpdateTime:          timestamppb.New(t1),
			ModelIdentifier:     "MacBookPro9,2",
			OsVersion:           "13.1.2",
			OsBuild:             "22F82",
			OsBuildSupplemental: "22F770820d",
			OsUsernames:         []string{"admin", "llama"},
			JamfBinaryVersion:   "9.27",
			ExternalId:          "99",
		},
	}

	gotRes := DeviceToResource(dev)
	// Assert some of the more "unusual" or missing fields.
	// We know other information isn't lost because of the conversion below,
	// therefore it must be present in the resource.
	assert.Equal(t, dev.ApiVersion, gotRes.Version, "resource.Version is not the ApiVersion")
	assert.Equal(t, dev.Id, gotRes.Metadata.Name, "resource.Metadata.Name is not the Id")
	assert.NotEmpty(t, gotRes.Metadata.Namespace, "resource.Metadata.Namespace")

	gotDev, err := DeviceFromResource(gotRes)
	require.NoError(t, err, "DeviceFromResource failed")
	if diff := cmp.Diff(dev, gotDev, protocmp.Transform()); diff != "" {
		t.Errorf("DeviceFromResource mismatch (-want +got)\n%s", diff)
	}
}

func TestResourceAttestationType_toAndFrom(t *testing.T) {
	t.Parallel()
	tests := []struct {
		attestationType string
		wantEmpty       bool
		wantErr         string
	}{
		{
			attestationType: "unspecified",
			wantEmpty:       true,
		},
		{
			attestationType: "tpm_ekpub",
		},
		{
			attestationType: "tpm_ekcert",
		},
		{
			attestationType: "tpm_ekcert_trusted",
		},
		{
			attestationType: "quantum_entanglement",
			wantErr:         "unknown attestation type",
		},
	}
	for _, tt := range tests {
		t.Run(tt.attestationType, func(t *testing.T) {
			asEnum, err := ResourceDeviceAttestationTypeFromString(tt.attestationType)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr, "ResourceDeviceAttestationTypeFromString error mismatch")
				return
			}

			got := ResourceDeviceAttestationTypeToString(asEnum)
			want := tt.attestationType
			if tt.wantEmpty {
				want = ""
			}
			require.Equal(t, want, got, "ResourceDeviceAttestationTypeToString mismatch")
		})
	}
}

func TestAllDeviceEnumsMapped(t *testing.T) {
	tests := []struct {
		name       string
		nameMap    map[int32]string // a proto enum "name" map, like MyEnum_name.
		toString   func(i int32) string
		fromString func(s string) (int32, error)
	}{
		{
			name:    "OSType",
			nameMap: devicepb.OSType_name,
			toString: func(i int32) string {
				return ResourceOSTypeToString(devicepb.OSType(i))
			},
			fromString: func(s string) (int32, error) {
				val, err := ResourceOSTypeFromString(s)
				return int32(val), err
			},
		},
		{
			name:    "DeviceEnrollStatus",
			nameMap: devicepb.DeviceEnrollStatus_name,
			toString: func(i int32) string {
				return ResourceDeviceEnrollStatusToString(devicepb.DeviceEnrollStatus(i))
			},
			fromString: func(s string) (int32, error) {
				val, err := ResourceDeviceEnrollStatusFromString(s)
				return int32(val), err
			},
		},
		{
			name:    "DeviceAttestationType",
			nameMap: devicepb.DeviceAttestationType_name,
			toString: func(i int32) string {
				return ResourceDeviceAttestationTypeToString(devicepb.DeviceAttestationType(i))
			},
			fromString: func(s string) (int32, error) {
				val, err := ResourceDeviceAttestationTypeFromString(s)
				return int32(val), err
			},
		},
		{
			name:    "DeviceOrigin",
			nameMap: devicepb.DeviceOrigin_name,
			toString: func(i int32) string {
				return ResourceDeviceOriginToString(devicepb.DeviceOrigin(i))
			},
			fromString: func(s string) (int32, error) {
				val, err := ResourceDeviceOriginFromString(s)
				return int32(val), err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for num, name := range test.nameMap {
				t.Run(name, func(t *testing.T) {
					s := test.toString(num)
					gotNum, err := test.fromString(s)
					require.NoError(t, err, "to/from enum conversion failed")
					require.Equal(t, num, gotNum, "to/from enum conversion changed the enum value")
				})
			}

			t.Run(`from "" (empty string)`, func(t *testing.T) {
				got, err := test.fromString("")
				require.NoError(t, err, `conversion from "" failed`)
				require.Equal(t, int32(0), got, `conversion from "" returned a non-zero value`)
			})
		})
	}
}
