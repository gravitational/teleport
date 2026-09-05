package storage_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
)

// TestProperty_ValidateDeviceCredential_SecureEnclave asserts that for the
// Secure Enclave platforms (macOS, iOS, iPadOS) ValidateDeviceCredential
// accepts exactly the credentials with a well-formed ID and a parseable PKIX
// public key: an accepted key round-trips back to the caller, every defect is
// rejected as a BadParameter naming it, and a rejection never returns a key.
func TestProperty_ValidateDeviceCredential_SecureEnclave(t *testing.T) {
	t.Parallel()

	// Secure Enclave keys are ECDSA P-256. The key bytes never steer validation,
	// only the DER framing does, so one key serves every draw.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	pubDER, err := x509.MarshalPKIXPublicKey(key.Public())
	require.NoError(t, err)

	rapid.Check(t, func(t *rapid.T) {
		osType := rapid.SampledFrom([]devicepb.OSType{
			devicepb.OSType_OS_TYPE_MACOS,
			devicepb.OSType_OS_TYPE_IOS,
			devicepb.OSType_OS_TYPE_IPADOS,
		}).Draw(t, "os")

		// maxCredentialIDLength is 64 bytes. StringN bounds the minimum in runes
		// and the cap in bytes, so both draws stay on their side of the boundary.
		cred := devicepb.DeviceCredential_builder{
			Id:           rapid.StringN(1, -1, 64).Draw(t, "id"),
			PublicKeyDer: pubDER,
		}.Build()

		wantErr := ""
		switch defect := rapid.SampledFrom([]string{
			"none", "missing ID", "oversized ID", "missing key", "truncated key DER",
		}).Draw(t, "defect"); defect {
		case "none":
		case "missing ID":
			cred.SetId("")
			wantErr = "credential ID required"
		case "oversized ID":
			cred.SetId(rapid.StringN(65, 256, -1).Draw(t, "long_id"))
			wantErr = "credential ID exceeds"
		case "missing key":
			cred.SetPublicKeyDer(nil)
			wantErr = "credential public key required"
		case "truncated key DER":
			// A strict prefix cannot parse: the outer SEQUENCE length no
			// longer matches the data.
			cut := rapid.IntRange(1, len(pubDER)-1).Draw(t, "cut")
			cred.SetPublicKeyDer(pubDER[:cut])
			wantErr = "invalid credential public key DER"
		default:
			t.Fatalf("unhandled defect %q", defect)
		}

		gotKey, err := storage.ValidateDeviceCredential(cred, osType)
		if wantErr == "" {
			require.NoError(t, err)
			require.True(t, key.PublicKey.Equal(gotKey), "returned key does not match the marshaled key")
			return
		}
		require.ErrorAs(t, err, new(*trace.BadParameterError))
		require.ErrorContains(t, err, wantErr)
		require.Nil(t, gotKey)
	})
}
