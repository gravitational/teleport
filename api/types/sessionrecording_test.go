package types_test

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestPrivateKeyType(t *testing.T) {
	awsKeyType := types.PrivateKeyType_AWS_KMS
	gcpKeyType := types.PrivateKeyType_GCP_KMS
	pkcs11KeyType := types.PrivateKeyType_PKCS11
	rawKeyType := types.PrivateKeyType_RAW

	awsKeyName := "aws_kms"
	gcpKeyName := "gcp_kms"
	pkcs11KeyName := "pkcs11"
	rawKeyName := "software"

	awsKeyEnc, err := json.Marshal(awsKeyType)
	require.NoError(t, err)

	gcpKeyEnc, err := json.Marshal(gcpKeyType)
	require.NoError(t, err)

	pkcs11KeyEnc, err := json.Marshal(pkcs11KeyType)
	require.NoError(t, err)

	rawKeyEnc, err := json.Marshal(rawKeyType)
	require.NoError(t, err)

	require.Equal(t, strconv.Quote(awsKeyName), string(awsKeyEnc))
	require.Equal(t, strconv.Quote(gcpKeyName), string(gcpKeyEnc))
	require.Equal(t, strconv.Quote(pkcs11KeyName), string(pkcs11KeyEnc))
	require.Equal(t, strconv.Quote(rawKeyName), string(rawKeyEnc))

	var awsKeyDec types.PrivateKeyType
	err = json.Unmarshal(awsKeyEnc, &awsKeyDec)
	require.NoError(t, err)

	var gcpKeyDec types.PrivateKeyType
	err = json.Unmarshal(gcpKeyEnc, &gcpKeyDec)
	require.NoError(t, err)

	var pkcs11KeyDec types.PrivateKeyType
	err = json.Unmarshal(pkcs11KeyEnc, &pkcs11KeyDec)
	require.NoError(t, err)

	var rawKeyDec types.PrivateKeyType
	err = json.Unmarshal(rawKeyEnc, &rawKeyDec)
	require.NoError(t, err)
}
