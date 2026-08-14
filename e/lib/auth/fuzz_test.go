package auth

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzParseSAMLInResponseTo(f *testing.F) {
	f.Add([]byte(respOkta))
	largeCompressedBytes, err := base64.StdEncoding.DecodeString(largeCompressedBody)
	require.NoError(f, err)
	f.Add(largeCompressedBytes)

	f.Fuzz(func(t *testing.T, response []byte) {
		require.NotPanics(t, func() {
			_, _ = ParseSAMLInResponseTo(base64.StdEncoding.EncodeToString(response))
		})
	})
}
