package auth

import (
	"encoding/base64"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func FuzzParseSAMLInResponseTo(f *testing.F) {
	// Disable Go App Engine logging
	logrus.SetLevel(logrus.PanicLevel)

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
