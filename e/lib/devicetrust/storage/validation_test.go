package storage

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

func TestValidateDeviceCredential(t *testing.T) {
	validAKPublic, err := base64.StdEncoding.DecodeString("AAEACwAFBHIAIJ3/y/NsODrmmfuYaNxty4nXFTiEvigDkiwSQVi/rSKuABAAFAAECAAAAAAAAQDCG4IIRdibXgZp5Jv6JMAv1uQD6ttEony9mtJpC/vp2bHvk2QPMlO8F87CS7BFjCxQVr+fpcMtCL/UGs956p5L27SCV5iAioM3Ny37XZkUK2QpPXJUxmK4CE2A3M0VHtvmxn20flgsYuhA04dJh5iszJVWL+7hMTFz5pNqM8xOcqdW5KPC4GyHGQDV8zvZ99/ar96jnErYPF+l2+3ipEkfIY2KyNcqL7/CBg0OwpeXXM/WEjs2oe++aADjCVW/IKEo52wd82ol4j7z0ZkEBjRFULyuUUgamN/MinP7+YODfLuQr2w+UtabkjvMAQpc9lLNdXoineEYnR/isdl6vsvr")
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
			publicKey, err := ValidateDeviceCredential(tt.cred, tt.os)
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
