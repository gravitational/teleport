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
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"slices"
	"strconv"
	"strings"
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

	for _, tc := range []struct {
		name string
		tags map[string]string
	}{
		{
			name: "delete keys with default tags",
		},
		{
			name: "delete keys with custom tags",
			tags: map[string]string{
				"test-key-1": "test-value-1",
			},
		},
		{
			name: "delete keys with override cluster tag",
			tags: map[string]string{
				"TeleportCluster": "test-cluster-2",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const pageSize int = 4
			fakeKMS := newFakeAWSKMSService(t, clock, "123456789012", "us-west-2", pageSize)
			cfg := servicecfg.KeystoreConfig{
				AWSKMS: &servicecfg.AWSKMSConfig{
					AWSAccount: "123456789012",
					AWSRegion:  "us-west-2",
					Tags:       tc.tags,
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

			var otherTags []kmstypes.Tag
			for k, v := range keyStore.backendForNewKeys.(*awsKMSKeystore).tags {
				if k != clusterTagKey {
					otherTags = append(otherTags, kmstypes.Tag{
						TagKey:   aws.String(k),
						TagValue: aws.String(v),
					})
				}
			}

			totalKeys := pageSize * 3
			for range totalKeys {
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
				Tags: append(otherTags, kmstypes.Tag{
					TagKey:   aws.String(clusterTagKey),
					TagValue: aws.String("other-cluster"),
				}),
			})
			require.NoError(t, err)
			otherClusterKeyARN := aws.ToString(output.KeyMetadata.Arn)

			clock.Advance(6 * time.Minute)
			err = keyStore.DeleteUnusedKeys(ctx, nil /*activeKeys*/)
			require.NoError(t, err)
			for _, key := range fakeKMS.keys {
				if key.arn.String() == otherClusterKeyARN {
					assert.Equal(t, kmstypes.KeyStateEnabled, key.state)
				} else {
					assert.Equal(t, kmstypes.KeyStatePendingDeletion, key.state)
				}
			}
		})
	}
}

func TestAWSKMS_WrongAccount(t *testing.T) {
	clock := clockwork.NewFakeClock()
	cfg := &servicecfg.KeystoreConfig{
		AWSKMS: &servicecfg.AWSKMSConfig{
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
		AWSKMS: &servicecfg.AWSKMSConfig{
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

// TestKeyAWSKeyCreationParameters asserts that an AWS keystore created with a
// variety of parameters correctly passes these parameters to the AWS client.
// This gives very little real coverage since the AWS KMS service here is faked,
// but at least we know the keystore passed the parameters to the client correctly.
// TestBackends and TestManager are both able to run with a real AWS KMS client
// and you can confirm the keys are configured correctly there.
func TestAWSKeyCreationParameters(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	const pageSize int = 4
	fakeKMS := newFakeAWSKMSService(t, clock, "123456789012", "us-west-2", pageSize)
	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{ClusterName: "test-cluster"})
	require.NoError(t, err)
	opts := &Options{
		ClusterName:          clusterName,
		HostUUID:             "uuid",
		AuthPreferenceGetter: &fakeAuthPreferenceGetter{types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1},
		awsKMSClient:         fakeKMS,
		mrkClient:            fakeKMS,
		awsSTSClient: &fakeAWSSTSClient{
			account: "123456789012",
		},
		clockworkOverride: clock,
	}

	for _, tc := range []struct {
		name        string
		multiRegion bool
		tags        map[string]string
	}{
		{
			name:        "multi-region enabled with default tags",
			multiRegion: true,
		},
		{
			name:        "multi-region disabled with default tags",
			multiRegion: false,
		},
		{
			name:        "multi region disabled with custom tags",
			multiRegion: false,
			tags: map[string]string{
				"key": "value",
			},
		},
	} {

		t.Run(tc.name, func(t *testing.T) {
			cfg := servicecfg.KeystoreConfig{
				AWSKMS: &servicecfg.AWSKMSConfig{
					AWSAccount: "123456789012",
					AWSRegion:  "us-west-2",
					MultiRegion: servicecfg.MultiRegionKeyStore{
						Enabled: tc.multiRegion,
					},
					Tags: tc.tags,
				},
			}
			keyStore, err := NewManager(ctx, &cfg, opts)
			require.NoError(t, err)

			sshKeyPair, err := keyStore.NewSSHKeyPair(ctx, cryptosuites.UserCASSH)
			require.NoError(t, err)

			keyID, err := parseAWSKMSKeyID(sshKeyPair.PrivateKey)
			require.NoError(t, err)

			if tc.multiRegion {
				assert.Contains(t, keyID.arn, "mrk-")
			} else {
				assert.NotContains(t, keyID.arn, "mrk-")
			}

			tagsOut, err := fakeKMS.ListResourceTags(ctx, &kms.ListResourceTagsInput{KeyId: &keyID.arn})
			require.NoError(t, err)
			if len(tc.tags) == 0 {
				tc.tags = map[string]string{
					"TeleportCluster": clusterName.GetClusterName(),
				}
			}
			require.Len(t, tc.tags, len(tagsOut.Tags))
			for _, tag := range tagsOut.Tags {
				v := tc.tags[aws.ToString(tag.TagKey)]
				require.Equal(t, v, aws.ToString(tag.TagValue))
			}
		})
	}
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
	arn          arn.ARN
	privKeyPEM   []byte
	keyUsage     kmstypes.KeyUsageType
	tags         []kmstypes.Tag
	creationDate time.Time
	state        kmstypes.KeyState
	region       string
	replicas     []string
}

func (f fakeAWSKMSKey) replicaArn(region string) string {
	arn := f.arn
	arn.Region = region
	return arn.String()
}

func (f fakeAWSKMSKey) hasReplica(region string) bool {
	return region == f.region || slices.Contains(f.replicas, region)
}

func (f *fakeAWSKMSService) CreateKey(_ context.Context, input *kms.CreateKeyInput, _ ...func(*kms.Options)) (*kms.CreateKeyOutput, error) {
	id := uuid.NewString()
	if aws.ToBool(input.MultiRegion) {
		// AWS does this https://docs.aws.amazon.com/kms/latest/developerguide/concepts.html#key-id-key-ARN
		id = "mrk-" + id
	}
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
		arn:          a,
		privKeyPEM:   privKeyPEM,
		keyUsage:     input.KeyUsage,
		tags:         input.Tags,
		creationDate: f.clock.Now(),
		region:       f.region,
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
	if key.keyUsage != kmstypes.KeyUsageTypeSignVerify {
		return nil, trace.BadParameter("key %q is not a signing key", aws.ToString(input.KeyId))
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

func (f *fakeAWSKMSService) Decrypt(_ context.Context, input *kms.DecryptInput, _ ...func(*kms.Options)) (*kms.DecryptOutput, error) {
	key, err := f.findKey(aws.ToString(input.KeyId))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if key.state != kmstypes.KeyStateEnabled {
		return nil, trace.NotFound("key %q is not enabled", aws.ToString(input.KeyId))
	}
	if key.keyUsage != kmstypes.KeyUsageTypeEncryptDecrypt {
		return nil, trace.BadParameter("key %q is not a decryption key", aws.ToString(input.KeyId))
	}
	signer, err := keys.ParsePrivateKey(key.privKeyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	decrypter, ok := signer.Signer.(crypto.Decrypter)
	if !ok {
		return nil, trace.Errorf("private key is not a decrypter")
	}
	switch input.EncryptionAlgorithm {
	case kmstypes.EncryptionAlgorithmSpecRsaesOaepSha256:
	default:
		return nil, trace.BadParameter("unsupported EncryptionAlgorithm %q", input.EncryptionAlgorithm)
	}
	plaintext, err := decrypter.Decrypt(rand.Reader, input.CiphertextBlob, &rsa.OAEPOptions{Hash: crypto.SHA256})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &kms.DecryptOutput{
		Plaintext: plaintext,
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
		if !f.keys[i].hasReplica(f.region) {
			continue
		}
		output.Keys = append(output.Keys, kmstypes.KeyListEntry{
			KeyArn: aws.String(f.keys[i].arn.String()),
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
	out := &kms.DescribeKeyOutput{
		KeyMetadata: &kmstypes.KeyMetadata{
			KeyId:        aws.String(key.arn.Resource),
			Arn:          aws.String(key.replicaArn(f.region)),
			CreationDate: aws.Time(key.creationDate),
			KeyState:     key.state,
		},
	}
	if strings.HasPrefix(key.arn.Resource, "mrk-") {
		out.KeyMetadata.MultiRegionConfiguration = &kmstypes.MultiRegionConfiguration{
			PrimaryKey: &kmstypes.MultiRegionKey{
				Arn:    aws.String(key.arn.String()),
				Region: &key.arn.Region,
			},
		}
		var replicas []kmstypes.MultiRegionKey
		for _, replica := range key.replicas {
			replicas = append(replicas, kmstypes.MultiRegionKey{
				Arn:    aws.String(key.replicaArn(replica)),
				Region: aws.String(replica),
			})
		}
		out.KeyMetadata.MultiRegionConfiguration.ReplicaKeys = replicas
	}
	return out, nil
}

func (f *fakeAWSKMSService) findKey(arn string) (*fakeAWSKMSKey, error) {
	i := slices.IndexFunc(f.keys, func(k *fakeAWSKMSKey) bool { return k.arn.String() == arn || k.arn.Resource == arn })
	if i < 0 || !f.keys[i].hasReplica(f.region) {
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

func (f *fakeAWSKMSService) ReplicateKey(ctx context.Context, in *kms.ReplicateKeyInput, _ ...func(*kms.Options)) (*kms.ReplicateKeyOutput, error) {
	key, err := f.findKey(*in.KeyId)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if key.region != f.region {
		return nil, &kmstypes.InvalidKeyUsageException{
			Message: aws.String("must use primary key for key replication"),
		}
	}
	if key.hasReplica(*in.ReplicaRegion) {
		return nil, &kmstypes.AlreadyExistsException{
			Message: aws.String(fmt.Sprintf("replicas %s already exists", *in.ReplicaRegion)),
		}
	}
	key.replicas = append(key.replicas, *in.ReplicaRegion)
	return &kms.ReplicateKeyOutput{
		ReplicaKeyMetadata: &kmstypes.KeyMetadata{
			Arn: aws.String(key.replicaArn(*in.ReplicaRegion)),
		},
	}, nil
}

func (f *fakeAWSKMSService) UpdatePrimaryRegion(ctx context.Context, in *kms.UpdatePrimaryRegionInput, _ ...func(*kms.Options)) (*kms.UpdatePrimaryRegionOutput, error) {
	key, err := f.findKey(*in.KeyId)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if key.region != f.region {
		return nil, &kmstypes.InvalidKeyUsageException{
			Message: aws.String("must use primary key for updating primary region"),
		}
	}
	i := slices.Index(key.replicas, *in.PrimaryRegion)
	if i == -1 {
		return nil, &kmstypes.InvalidKeyUsageException{
			Message: aws.String("replica does not exist"),
		}
	}
	key.replicas[i] = key.region
	key.region = *in.PrimaryRegion
	key.arn.Region = *in.PrimaryRegion
	return &kms.UpdatePrimaryRegionOutput{}, nil
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

func TestMultiRegionKeyReplication(t *testing.T) {
	testAccount := "123456789"
	testPrimary := "us-west-2"
	testSecondary := "us-east-1"
	testReplicas := []string{testSecondary, "us-east-2"}

	tests := []struct {
		name             string
		config           servicecfg.AWSKMSConfig
		existingPrimary  string
		existingReplicas []string
		expectedReplicas []string
		expectedPrimary  string
	}{
		{
			name: "backwards compatibility when no primary/replicas are configured",
			config: servicecfg.AWSKMSConfig{
				AWSAccount: testAccount,
				AWSRegion:  testPrimary,
				MultiRegion: servicecfg.MultiRegionKeyStore{
					Enabled: true,
				},
			},
			existingReplicas: nil,
			expectedReplicas: []string{},
			expectedPrimary:  testPrimary,
		},
		{
			name: "replicas are created when specified from the primary region",
			config: servicecfg.AWSKMSConfig{
				AWSAccount: testAccount,
				AWSRegion:  testPrimary,
				MultiRegion: servicecfg.MultiRegionKeyStore{
					Enabled:        true,
					PrimaryRegion:  testPrimary,
					ReplicaRegions: testReplicas,
				},
			},
			existingReplicas: nil,
			expectedReplicas: testReplicas,
			expectedPrimary:  testPrimary,
		},
		{
			name: "replicas are not created from outside primary region",
			config: servicecfg.AWSKMSConfig{
				AWSAccount: testAccount,
				AWSRegion:  testSecondary,
				MultiRegion: servicecfg.MultiRegionKeyStore{
					Enabled:        true,
					PrimaryRegion:  testPrimary,
					ReplicaRegions: testReplicas,
				},
			},
			existingReplicas: []string{testSecondary},
			expectedReplicas: []string{testSecondary},
			expectedPrimary:  testPrimary,
		},
		{
			name: "primary region is updated from the existing primary region",
			config: servicecfg.AWSKMSConfig{
				AWSAccount: testAccount,
				AWSRegion:  testPrimary,
				MultiRegion: servicecfg.MultiRegionKeyStore{
					Enabled:        true,
					PrimaryRegion:  testSecondary,
					ReplicaRegions: []string{testPrimary},
				},
			},
			existingPrimary:  testPrimary,
			existingReplicas: []string{testSecondary},
			expectedReplicas: []string{testPrimary},
			expectedPrimary:  testSecondary,
		},
		{
			name: "primary region is not updated from a non-primary region",
			config: servicecfg.AWSKMSConfig{
				AWSAccount: testAccount,
				AWSRegion:  testSecondary,
				MultiRegion: servicecfg.MultiRegionKeyStore{
					Enabled:        true,
					PrimaryRegion:  testSecondary,
					ReplicaRegions: testReplicas,
				},
			},
			existingPrimary:  testPrimary,
			existingReplicas: testReplicas,
			expectedReplicas: testReplicas,
			expectedPrimary:  testPrimary,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			clock := clockwork.NewFakeClock()
			fakeKMS := newFakeAWSKMSService(t, clock, testAccount, tc.config.AWSRegion, 1)
			cluster, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{ClusterName: "test-cluster"})
			require.NoError(t, err)
			opts := &Options{
				ClusterName:          cluster,
				HostUUID:             "uuid",
				AuthPreferenceGetter: &fakeAuthPreferenceGetter{types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_HSM_V1},
				awsKMSClient:         fakeKMS,
				mrkClient:            fakeKMS,
				awsSTSClient: &fakeAWSSTSClient{
					account: testAccount,
				},
				clockworkOverride: clock,
			}

			existingPrimary := tc.existingPrimary
			if existingPrimary == "" {
				existingPrimary = testPrimary
			}
			fakeKMS.region = existingPrimary
			primary, err := NewManager(ctx, &servicecfg.KeystoreConfig{
				AWSKMS: &servicecfg.AWSKMSConfig{
					AWSAccount: tc.config.AWSAccount,
					AWSRegion:  testPrimary,
					MultiRegion: servicecfg.MultiRegionKeyStore{
						Enabled:        true,
						PrimaryRegion:  existingPrimary,
						ReplicaRegions: tc.existingReplicas,
					},
				},
			}, opts)
			require.NoError(t, err)

			kp, err := primary.NewTLSKeyPair(ctx, cluster.GetName(), cryptosuites.HostCATLS)
			require.NoError(t, err, trace.DebugReport(err))
			key, err := parseAWSKMSKeyID(kp.Key)
			require.NoError(t, err)
			require.Equal(t, key.region, existingPrimary)
			require.Contains(t, key.arn, "mrk-")
			require.ElementsMatch(t, tc.existingReplicas, fakeKMS.keys[0].replicas)

			fakeKMS.region = tc.config.AWSRegion
			mgr, err := NewManager(ctx, &servicecfg.KeystoreConfig{
				AWSKMS: &tc.config,
			}, opts)
			require.NoError(t, err)

			id, err := mgr.ApplyMultiRegionConfig(ctx, kp.Key)
			require.NoError(t, err)

			key, err = parseAWSKMSKeyID(id)
			require.NoError(t, err)
			require.Equal(t, tc.expectedPrimary, key.region)

			out, err := fakeKMS.DescribeKey(ctx, &kms.DescribeKeyInput{
				KeyId: &key.id,
			})
			require.NoError(t, err)

			mrc := out.KeyMetadata.MultiRegionConfiguration
			if tc.expectedPrimary != "" {
				require.Equal(t,
					tc.expectedPrimary,
					*mrc.PrimaryKey.Region,
				)
			}
			for _, replica := range tc.expectedReplicas {
				require.True(t, slices.ContainsFunc(mrc.ReplicaKeys, func(key kmstypes.MultiRegionKey) bool {
					return *key.Region == replica
				}), "expected %s found in replicas %v", replica, mrc.ReplicaKeys)
			}
			for _, replica := range mrc.ReplicaKeys {
				require.Contains(t, tc.expectedReplicas, *replica.Region)
			}
		})
	}

}
