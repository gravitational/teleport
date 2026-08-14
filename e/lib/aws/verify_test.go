package aws

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/fixtures"
)

func TestVerifyOK(t *testing.T) {
	meta, err := GetInstanceMetadata(func(uri string) ([]byte, error) {
		if strings.HasSuffix(uri, "document") {
			return []byte(fixtures.AWSDoc), nil
		}
		return []byte(fixtures.AWSSig), nil
	})
	require.NoError(t, err)
	assert.Equal(t, "126027368216", meta.AccountID)
	assert.Contains(t, meta.MarketplaceProductCodes, "9x4pv56fe6h1gj8hejc5r6a3z")
}

func TestVerifyTampered(t *testing.T) {
	_, err := GetInstanceMetadata(func(uri string) ([]byte, error) {
		if strings.HasSuffix(uri, "document") {
			return []byte(fixtures.AWSDoc + "tampered"), nil
		}
		return []byte(fixtures.AWSSig), nil
	})
	require.Error(t, err)
}
