package storage_test

import (
	"crypto"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
)

// validAKPublic is a base64 encoded TPM AK public key to use in tests.
var validAKPublic = "AAEACwAFBHIAIJ3/y/NsODrmmfuYaNxty4nXFTiEvigDkiwSQVi/rSKuABAAFAAECAAAAAAAAQDCG4IIRdibXgZp5Jv6JMAv1uQD6ttEony9mtJpC/vp2bHvk2QPMlO8F87CS7BFjCxQVr+fpcMtCL/UGs956p5L27SCV5iAioM3Ny37XZkUK2QpPXJUxmK4CE2A3M0VHtvmxn20flgsYuhA04dJh5iszJVWL+7hMTFz5pNqM8xOcqdW5KPC4GyHGQDV8zvZ99/ar96jnErYPF+l2+3ipEkfIY2KyNcqL7/CBg0OwpeXXM/WEjs2oe++aADjCVW/IKEo52wd82ol4j7z0ZkEBjRFULyuUUgamN/MinP7+YODfLuQr2w+UtabkjvMAQpc9lLNdXoineEYnR/isdl6vsvr"

func TestValidateDeviceCredential(t *testing.T) {
	t.Parallel()

	validAKPublic, err := base64.StdEncoding.DecodeString(validAKPublic)
	require.NoError(t, err)
	tests := []struct {
		name string
		cred *devicepb.DeviceCredential
		os   devicepb.OSType

		wantErr string
	}{
		{
			name:    "nil credential",
			cred:    nil,
			os:      devicepb.OSType_OS_TYPE_MACOS,
			wantErr: "device credential required",
		},
		{
			name: "valid tpm ekpub credential",
			cred: &devicepb.DeviceCredential{
				Id:                    "KGT5SGj2utv7wNKQwwluEmMjKyzffzDKkpT+lX1zQ9E",
				DeviceAttestationType: devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKPUB,
				TpmAkPublic:           validAKPublic,
			},
			os: devicepb.OSType_OS_TYPE_WINDOWS,
		},
		{
			name: "tpm ekpub credential missing tpm ak public",
			cred: &devicepb.DeviceCredential{
				Id:                    "KGT5SGj2utv7wNKQwwluEmMjKyzffzDKkpT+lX1zQ9E",
				DeviceAttestationType: devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKPUB,
			},
			os:      devicepb.OSType_OS_TYPE_WINDOWS,
			wantErr: "credential TPM AK public required",
		},
		{
			name: "tpm ekpub credential invalid tpm ak public",
			cred: &devicepb.DeviceCredential{
				Id:                    "KGT5SGj2utv7wNKQwwluEmMjKyzffzDKkpT+lX1zQ9E",
				DeviceAttestationType: devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKPUB,
				TpmAkPublic:           []byte("garbage data"),
			},
			os:      devicepb.OSType_OS_TYPE_WINDOWS,
			wantErr: "invalid TPM credential public key DER",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publicKey, err := storage.ValidateDeviceCredential(tt.cred, tt.os)
			if tt.wantErr == "" {
				require.NoError(t, err)
				require.NotNil(t, publicKey)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, publicKey)
		})
	}
}

func TestValidateCollectedData(t *testing.T) {
	t.Parallel()

	modifyValid := func(modify func(*devicepb.DeviceCollectedData)) *devicepb.DeviceCollectedData {
		cd := &devicepb.DeviceCollectedData{
			CollectTime:  timestamppb.Now(),
			OsType:       devicepb.OSType_OS_TYPE_WINDOWS,
			SerialNumber: "llama",
			TpmPlatformAttestation: &devicepb.TPMPlatformAttestation{
				Nonce: []byte("fake-nonce"),
				PlatformParameters: &devicepb.TPMPlatformParameters{
					Quotes: []*devicepb.TPMQuote{
						{
							Quote:     []byte("fake-quote"),
							Signature: []byte("fake-quote-signature"),
						},
					},
					Pcrs: []*devicepb.TPMPCR{
						{
							Index:     0,
							Digest:    []byte("fake-digest-0-sha1"),
							DigestAlg: uint64(crypto.SHA1),
						},
					},
					EventLog: []byte("fake-event-log"),
				},
			},
		}
		if modify != nil {
			modify(cd)
		}
		return cd
	}
	tests := []struct {
		name    string
		cd      *devicepb.DeviceCollectedData
		wantErr string
	}{
		{
			name: "valid with TPM platform attestation",
			cd:   modifyValid(nil),
		},
		{
			name: "valid without TPM platform attestation",
			cd: modifyValid(func(cd *devicepb.DeviceCollectedData) {
				cd.TpmPlatformAttestation = nil
			}),
		},
		{
			name: "missing nonce",
			cd: modifyValid(func(cd *devicepb.DeviceCollectedData) {
				cd.TpmPlatformAttestation.Nonce = nil
			}),
			wantErr: "nonce required",
		},
		{
			name: "missing platform parameters",
			cd: modifyValid(func(cd *devicepb.DeviceCollectedData) {
				cd.TpmPlatformAttestation.PlatformParameters = nil
			}),
			wantErr: "platform_parameters required",
		},
		{
			name: "missing platform parameters event log",
			cd: modifyValid(func(cd *devicepb.DeviceCollectedData) {
				cd.TpmPlatformAttestation.PlatformParameters.EventLog = nil
			}),
			wantErr: "platform_parameters.event_log required",
		},
		{
			name: "missing platform parameters quotes",
			cd: modifyValid(func(cd *devicepb.DeviceCollectedData) {
				cd.TpmPlatformAttestation.PlatformParameters.Quotes = nil
			}),
			wantErr: "platform_parameters.quotes required",
		},
		{
			name: "missing platform parameters quote quote",
			cd: modifyValid(func(cd *devicepb.DeviceCollectedData) {
				cd.TpmPlatformAttestation.PlatformParameters.Quotes[0].Quote = nil
			}),
			wantErr: "platform_parameters.quotes[0].quote required",
		},
		{
			name: "missing platform parameters quote signature",
			cd: modifyValid(func(cd *devicepb.DeviceCollectedData) {
				cd.TpmPlatformAttestation.PlatformParameters.Quotes[0].Signature = nil
			}),
			wantErr: "platform_parameters.quotes[0].signature required",
		},
		{
			name: "missing platform parameters pcrs",
			cd: modifyValid(func(cd *devicepb.DeviceCollectedData) {
				cd.TpmPlatformAttestation.PlatformParameters.Pcrs = nil
			}),
			wantErr: "platform_parameters.pcrs required",
		},
		{
			name: "missing platform parameters pcrs digest",
			cd: modifyValid(func(cd *devicepb.DeviceCollectedData) {
				cd.TpmPlatformAttestation.PlatformParameters.Pcrs[0].Digest = nil
			}),
			wantErr: "platform_parameters.pcrs[0].digest required",
		},
		{
			name: "missing platform parameters pcrs digest alg",
			cd: modifyValid(func(cd *devicepb.DeviceCollectedData) {
				cd.TpmPlatformAttestation.PlatformParameters.Pcrs[0].DigestAlg = 0
			}),
			wantErr: "platform_parameters.pcrs[0].digest_alg must be non-zero",
		},
		{
			name: "os_username max length",
			cd: modifyValid(func(cd *devicepb.DeviceCollectedData) {
				cd.OsUsername = strings.Repeat("A", 42)
			}),
			wantErr: "OS username",
		},
		{
			name: "os_login_user max length",
			cd: modifyValid(func(cd *devicepb.DeviceCollectedData) {
				cd.OsLoginUser = strings.Repeat("A", 42)
			}),
			wantErr: "OS login user",
		},
		{
			name: "ok: only OS username set",
			cd: modifyValid(func(cd *devicepb.DeviceCollectedData) {
				cd.OsUsername = "llama"
				cd.OsLoginUser = ""
			}),
		},
		{
			name: "ok: only OS login user set",
			cd: modifyValid(func(cd *devicepb.DeviceCollectedData) {
				cd.OsUsername = ""
				cd.OsLoginUser = "llama"
			}),
		},
		{
			name: "usernames must match if set",
			cd: modifyValid(func(cd *devicepb.DeviceCollectedData) {
				cd.OsUsername = "llama1"
				cd.OsLoginUser = "llama2"
			}),
			wantErr: "login user mismatch",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := storage.ValidateCollectedData(tt.cd)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// TestValidateCollectedDataAgainstDeviceStrict tests a few corner cases not
// covered by TestS_CreateDeviceEnrollTokenUsingData or
// TestS_CreateDeviceEnrollTokenUsingData_errors.
func TestValidateCollectedDataAgainstDeviceStrict(t *testing.T) {
	t.Parallel()

	nowPB := timestamppb.Now()
	dev := &devicepb.Device{
		ApiVersion:   "v1",
		Id:           uuid.NewString(),
		OsType:       devicepb.OSType_OS_TYPE_MACOS,
		AssetTag:     "llama",
		CreateTime:   nowPB,
		UpdateTime:   nowPB,
		EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
		Profile: &devicepb.DeviceProfile{
			UpdateTime:        nowPB,
			ModelIdentifier:   "MacBookPro9,2",
			OsVersion:         "13.3",
			OsBuild:           "22D68",
			OsUsernames:       []string{"llama"},
			JamfBinaryVersion: "9.27",
		},
	}

	modifyCD := func(fn func(*devicepb.DeviceCollectedData)) *devicepb.DeviceCollectedData {
		cd := collectedDataForDevice(dev)
		if fn != nil {
			fn(cd)
		}
		return cd
	}

	modifyDev := func(fn func(*devicepb.Device)) *devicepb.Device {
		cp := proto.Clone(dev).(*devicepb.Device)
		fn(cp)
		return cp
	}

	devSupplemental := proto.Clone(dev).(*devicepb.Device)
	devSupplemental.Profile.OsVersion = "13.4.1"
	devSupplemental.Profile.OsBuild = "22F82"
	devSupplemental.Profile.OsBuildSupplemental = "22F770820d"

	tests := []struct {
		name    string
		cd      *devicepb.DeviceCollectedData
		dev     *devicepb.Device
		wantErr string
	}{
		{
			name: "ok",
			cd:   modifyCD(nil),
			dev:  dev,
		},
		{
			name: "ok - nil device Profile",
			cd:   modifyCD(nil), // Collected data has profile-like info. This is fine.
			dev: modifyDev(func(d *devicepb.Device) {
				d.Profile = nil
			}),
		},
		{
			name: "Profile.OSVersion equivalent",
			cd: modifyCD(func(cd *devicepb.DeviceCollectedData) {
				cd.OsVersion = "13.3.0"
			}),
			dev: dev,
		},
		{
			name: "Profile.OSVersion equivalent (device-side)",
			cd:   modifyCD(nil),
			dev: modifyDev(func(d *devicepb.Device) {
				d.Profile.OsVersion = "13.3.0"
			}),
		},
		{
			name: "Profile.JamfBinaryVersion equivalent",
			cd: modifyCD(func(cd *devicepb.DeviceCollectedData) {
				cd.JamfBinaryVersion = "9.27.0"
			}),
			dev: dev,
		},
		{
			name: "complex Profile.JamfBinaryVersion",
			cd: modifyCD(func(cd *devicepb.DeviceCollectedData) {
				cd.JamfBinaryVersion = "10.46.1-t1683911857"
			}),
			dev: modifyDev(func(d *devicepb.Device) {
				d.Profile.JamfBinaryVersion = "10.46.1-t1683911857"
			}),
		},
		{
			name: "cd.OsBuild matches Profile.OsBuildSupplemental",
			cd: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devSupplemental)
				cd.OsBuild = devSupplemental.Profile.OsBuildSupplemental
				return cd
			}(),
			dev: devSupplemental,
		},
		{
			name: "ok - Profile.OsBuildSupplemental without Profile.OsBuild",
			cd: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devSupplemental)
				cd.OsBuild = devSupplemental.Profile.OsBuildSupplemental
				return cd
			}(),
			dev: func() *devicepb.Device {
				d := proto.Clone(devSupplemental).(*devicepb.Device)
				d.Profile.OsBuild = "" // Unusual, but allowed.
				return d
			}(),
		},
		{
			name: "nok - Profile.OsBuildSupplemental without Profile.OsBuild",
			cd: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devSupplemental)
				cd.OsBuild = "bad" // Doesn't match any build.
				return cd
			}(),
			dev: func() *devicepb.Device {
				d := proto.Clone(devSupplemental).(*devicepb.Device)
				d.Profile.OsBuild = "" // Unusual, but allowed.
				return d
			}(),
			wantErr: "OS build drift",
		},
		{
			name: "cd.OsBuild matches neither OsBuild",
			cd: func() *devicepb.DeviceCollectedData {
				cd := collectedDataForDevice(devSupplemental)
				cd.OsBuild = "bad" // Doesn't match any build.
				return cd
			}(),
			dev:     devSupplemental,
			wantErr: "OS build drift",
		},
		// Similar to TestS_CreateDeviceEnrollTokenUsingData_errors
		{
			name: "Profile.OSVersion drift",
			cd: modifyCD(func(cd *devicepb.DeviceCollectedData) {
				cd.OsVersion = "13.3.1" // upwards drift not OK in strict mode!
			}),
			dev:     dev,
			wantErr: "OS version drift",
		},
		// Similar to TestS_CreateDeviceEnrollTokenUsingData_errors
		{
			name: "Profile.JamfBinaryVersion drift",
			cd: modifyCD(func(cd *devicepb.DeviceCollectedData) {
				cd.JamfBinaryVersion = "9.27.1" // upwards drift not OK in strict mode!
			}),
			dev:     dev,
			wantErr: "jamf binary version drift",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := storage.ValidateCollectedDataAgainstDeviceStrict(test.cd, test.dev)
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr, "ValidateCollectedDataAgainstDeviceStrict error mismatch")
			} else if err != nil {
				t.Errorf("ValidateCollectedDataAgainstDeviceStrict returned err=%v, want nil", err)
			}
		})
	}
}
