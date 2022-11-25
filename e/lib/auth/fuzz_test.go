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

	f.Fuzz(func(t *testing.T, response string) {
		require.NotPanics(t, func() {
			ParseSAMLInResponseTo(base64.StdEncoding.EncodeToString([]byte(response)))
		})
	})
}
