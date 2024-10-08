// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package keystore

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
)

// TestAWSKMS_deleteUnusedKeys tests the AWS KMS keystore's deleteUnusedKeys
// method under conditions where the ListKeys response is paginated and/or
// includes keys created by other clusters.
//
// DeleteUnusedKeys is also generally tested under TestKeyStore, this test is
// for conditions specific to AWS KMS.
func TestAWSKMS_DeleteUnusedKeys(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	const pageSize int = 4
	fakeKMS := newFakeAWSKMSService(t, clock, "123456789012", "us-west-2", pageSize)
	cfg := servicecfg.KeystoreConfig{
		AWSKMS: servicecfg.AWSKMSConfig{
			AWSAccount: "123456789012",
			AWSRegion:  "us-west-2",
		},
	}
	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{ClusterName: "test-cluster"})
	require.NoError(t, err)
	opts := &Options{
		ClusterName:          clusterName,
		HostUUID:             "uuid",
		AuthPreferenceGetter: &fakeAuthPreferenceGetter{types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1},
		awsKMSClient:         fakeKMS,
		awsSTSClient: &fakeAWSSTSClient{
			account: "123456789012",
		},
		clockworkOverride: clock,
	}
	keyStore, err := NewManager(ctx, &cfg, opts)
	require.NoError(t, err)

	totalKeys := pageSize * 3
	for i := 0; i < totalKeys; i++ {
		_, err := keyStore.NewSSHKeyPair(ctx, cryptosuites.UserCASSH)
		require.NoError(t, err, trace.DebugReport(err))
	}

	// Newly created keys should not be deleted.
	err = keyStore.DeleteUnusedKeys(ctx, nil /*activeKeys*/)
	require.NoError(t, err)
	for _, key := range fakeKMS.keys {
		assert.Equal(t, kmstypes.KeyStateEnabled, key.state)
	}

	// Keys created more than 5 minutes ago should be deleted.
	clock.Advance(6 * time.Minute)
	err = keyStore.DeleteUnusedKeys(ctx, nil /*activeKeys*/)
	require.NoError(t, err)
	for _, key := range fakeKMS.keys {
		assert.Equal(t, kmstypes.KeyStatePendingDeletion, key.state)
	}

	// Insert a key created by a different Teleport cluster, it should not be
	// deleted by the keystore.
	output, err := fakeKMS.CreateKey(ctx, &kms.CreateKeyInput{
		KeySpec: kmstypes.KeySpecEccNistP256,
		Tags: []kmstypes.Tag{
			kmstypes.Tag{
				TagKey:   aws.String(clusterTagKey),
				TagValue: aws.String("other-cluster"),
			},
		},
	})
	require.NoError(t, err)
	otherClusterKeyARN := aws.ToString(output.KeyMetadata.Arn)

	clock.Advance(6 * time.Minute)
	err = keyStore.DeleteUnusedKeys(ctx, nil /*activeKeys*/)
	require.NoError(t, err)
	for _, key := range fakeKMS.keys {
		if key.arn == otherClusterKeyARN {
			assert.Equal(t, kmstypes.KeyStateEnabled, key.state)
		} else {
			assert.Equal(t, kmstypes.KeyStatePendingDeletion, key.state)
		}
	}
}

func TestAWSKMS_WrongAccount(t *testing.T) {
	clock := clockwork.NewFakeClock()
	cfg := &servicecfg.KeystoreConfig{
		AWSKMS: servicecfg.AWSKMSConfig{
			AWSAccount: "111111111111",
			AWSRegion:  "us-west-2",
		},
	}
	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{ClusterName: "test-cluster"})
	require.NoError(t, err)
	opts := &Options{
		ClusterName:          clusterName,
		HostUUID:             "uuid",
		AuthPreferenceGetter: &fakeAuthPreferenceGetter{types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1},
		awsKMSClient:         newFakeAWSKMSService(t, clock, "222222222222", "us-west-2", 1000),
		awsSTSClient: &fakeAWSSTSClient{
			account: "222222222222",
		},
	}
	_, err = NewManager(context.Background(), cfg, opts)
	require.ErrorIs(t, err, trace.BadParameter(`configured AWS KMS account "111111111111" does not match AWS account of ambient credentials "222222222222"`))
}

func TestAWSKMS_RetryWhilePending(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	kms := &fakeAWSKMSService{
		clock:     clock,
		account:   "111111111111",
		region:    "us-west-2",
		pageLimit: 1000,
	}
	cfg := &servicecfg.KeystoreConfig{
		AWSKMS: servicecfg.AWSKMSConfig{
			AWSAccount: "111111111111",
			AWSRegion:  "us-west-2",
		},
	}
	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{ClusterName: "test-cluster"})
	require.NoError(t, err)
	opts := &Options{
		ClusterName:          clusterName,
		HostUUID:             "uuid",
		AuthPreferenceGetter: &fakeAuthPreferenceGetter{types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1},
		awsKMSClient:         kms,
		awsSTSClient: &fakeAWSSTSClient{
			account: "111111111111",
		},
		clockworkOverride: clock,
	}
	manager, err := NewManager(ctx, cfg, opts)
	require.NoError(t, err)

	// Test with one retry required.
	kms.keyPendingDuration = pendingKeyBaseRetryInterval
	go func() {
		clock.BlockUntil(2)
		clock.Advance(kms.keyPendingDuration)
	}()
	_, err = manager.NewSSHKeyPair(ctx, cryptosuites.UserCASSH)
	require.NoError(t, err)

	// Test with two retries required.
	kms.keyPendingDuration = 4 * pendingKeyBaseRetryInterval
	go func() {
		clock.BlockUntil(2)
		clock.Advance(kms.keyPendingDuration / 2)
		clock.BlockUntil(2)
		clock.Advance(kms.keyPendingDuration / 2)
	}()
	_, err = manager.NewSSHKeyPair(ctx, cryptosuites.UserCASSH)
	require.NoError(t, err)

	// Test a timeout.
	kms.keyPendingDuration = 2 * pendingKeyTimeout
	go func() {
		clock.BlockUntil(2)
		clock.Advance(pendingKeyBaseRetryInterval)
		clock.BlockUntil(2)
		clock.Advance(pendingKeyTimeout)
	}()
	_, err = manager.NewSSHKeyPair(ctx, cryptosuites.UserCASSH)
	require.Error(t, err)
}

type fakeAWSKMSService struct {
	keys               []*fakeAWSKMSKey
	clock              clockwork.Clock
	account            string
	region             string
	pageLimit          int
	keyPendingDuration time.Duration
}

func newFakeAWSKMSService(t *testing.T, clock clockwork.Clock, account string, region string, pageLimit int) *fakeAWSKMSService {
	return &fakeAWSKMSService{
		clock:     clock,
		account:   account,
		region:    region,
		pageLimit: pageLimit,
	}
}

type fakeAWSKMSKey struct {
	arn          string
	privKeyPEM   []byte
	tags         []kmstypes.Tag
	creationDate time.Time
	state        kmstypes.KeyState
}

func (f *fakeAWSKMSService) CreateKey(_ context.Context, input *kms.CreateKeyInput, _ ...func(*kms.Options)) (*kms.CreateKeyOutput, error) {
	id := uuid.NewString()
	a := arn.ARN{
		Partition: "aws",
		Service:   "kms",
		Region:    f.region,
		AccountID: f.account,
		Resource:  id,
	}
	state := kmstypes.KeyStateEnabled
	if f.keyPendingDuration > 0 {
		state = kmstypes.KeyStateCreating
	}
	var privKeyPEM []byte
	switch input.KeySpec {
	case kmstypes.KeySpecRsa2048:
		privKeyPEM = testRSA2048PrivateKeyPEM
	case kmstypes.KeySpecEccNistP256:
		signer, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		privKeyPEM, err = keys.MarshalPrivateKey(signer)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("unsupported KeySpec %v", input.KeySpec)
	}
	f.keys = append(f.keys, &fakeAWSKMSKey{
		arn:          a.String(),
		privKeyPEM:   privKeyPEM,
		tags:         input.Tags,
		creationDate: f.clock.Now(),
		state:        state,
	})
	return &kms.CreateKeyOutput{
		KeyMetadata: &kmstypes.KeyMetadata{
			Arn:   aws.String(a.String()),
			KeyId: aws.String(id),
		},
	}, nil
}

func (f *fakeAWSKMSService) GetPublicKey(_ context.Context, input *kms.GetPublicKeyInput, _ ...func(*kms.Options)) (*kms.GetPublicKeyOutput, error) {
	key, err := f.findKey(aws.ToString(input.KeyId))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if key.state != kmstypes.KeyStateEnabled {
		return nil, trace.NotFound("key %q is not enabled", aws.ToString(input.KeyId))
	}
	privateKey, err := keys.ParsePrivateKey(key.privKeyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	der, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &kms.GetPublicKeyOutput{
		PublicKey: der,
	}, nil
}

func (f *fakeAWSKMSService) Sign(_ context.Context, input *kms.SignInput, _ ...func(*kms.Options)) (*kms.SignOutput, error) {
	key, err := f.findKey(aws.ToString(input.KeyId))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if key.state != kmstypes.KeyStateEnabled {
		return nil, trace.NotFound("key %q is not enabled", aws.ToString(input.KeyId))
	}
	signer, err := keys.ParsePrivateKey(key.privKeyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var opts crypto.SignerOpts
	switch input.SigningAlgorithm {
	case kmstypes.SigningAlgorithmSpecRsassaPkcs1V15Sha256, kmstypes.SigningAlgorithmSpecEcdsaSha256:
		opts = crypto.SHA256
	case kmstypes.SigningAlgorithmSpecRsassaPkcs1V15Sha512:
		opts = crypto.SHA512
	default:
		return nil, trace.BadParameter("unsupported SigningAlgorithm %q", input.SigningAlgorithm)
	}
	signature, err := signer.Sign(rand.Reader, input.Message, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &kms.SignOutput{
		Signature: signature,
	}, nil
}

func (f *fakeAWSKMSService) ScheduleKeyDeletion(_ context.Context, input *kms.ScheduleKeyDeletionInput, _ ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error) {
	key, err := f.findKey(aws.ToString(input.KeyId))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key.state = kmstypes.KeyStatePendingDeletion
	return &kms.ScheduleKeyDeletionOutput{}, nil
}

func (f *fakeAWSKMSService) ListKeys(_ context.Context, input *kms.ListKeysInput, _ ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	pageLimit := min(int(aws.ToInt32(input.Limit)), f.pageLimit)
	output := &kms.ListKeysOutput{}
	i := 0
	if input.Marker != nil {
		var err error
		i, err = strconv.Atoi(aws.ToString(input.Marker))
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	for ; i < len(f.keys) && len(output.Keys) < pageLimit; i++ {
		output.Keys = append(output.Keys, kmstypes.KeyListEntry{
			KeyArn: aws.String(f.keys[i].arn),
		})
	}
	if i < len(f.keys) {
		output.NextMarker = aws.String(strconv.Itoa(i))
		output.Truncated = true
	}
	return output, nil
}

func (f *fakeAWSKMSService) ListResourceTags(_ context.Context, input *kms.ListResourceTagsInput, _ ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error) {
	key, err := f.findKey(aws.ToString(input.KeyId))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &kms.ListResourceTagsOutput{
		Tags: key.tags,
	}, nil
}

func (f *fakeAWSKMSService) DescribeKey(_ context.Context, input *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	key, err := f.findKey(aws.ToString(input.KeyId))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &kms.DescribeKeyOutput{
		KeyMetadata: &kmstypes.KeyMetadata{
			CreationDate: aws.Time(key.creationDate),
			KeyState:     key.state,
		},
	}, nil
}

func (f *fakeAWSKMSService) findKey(arn string) (*fakeAWSKMSKey, error) {
	i := slices.IndexFunc(f.keys, func(k *fakeAWSKMSKey) bool { return k.arn == arn })
	if i < 0 {
		return nil, &kmstypes.NotFoundException{
			Message: aws.String(fmt.Sprintf("key %q not found", arn)),
		}
	}
	key := f.keys[i]
	if key.state != kmstypes.KeyStateCreating {
		return key, nil
	}
	if f.clock.Now().Before(key.creationDate.Add(f.keyPendingDuration)) {
		return nil, &kmstypes.NotFoundException{
			Message: aws.String(fmt.Sprintf("key %q not found", arn)),
		}
	}
	key.state = kmstypes.KeyStateEnabled
	return key, nil
}

type fakeAWSSTSClient struct {
	account, arn, userID string
}

func (f *fakeAWSSTSClient) GetCallerIdentity(_ context.Context, _ *sts.GetCallerIdentityInput, _ ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Account: aws.String(f.account),
		Arn:     aws.String(f.arn),
		UserId:  aws.String(f.userID),
	}, nil
}
