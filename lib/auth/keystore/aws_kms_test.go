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
	"fmt"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/kms/kmsiface"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

// TestAWSKMS_DeleteUnusedKeys tests the AWS KMS keystore's DeleteUnusedKeys
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
	cfg := Config{
		AWSKMS: AWSKMSConfig{
			Cluster:    "test-cluster",
			AWSAccount: "123456789012",
			AWSRegion:  "us-west-2",
			KMS:        fakeKMS,
			clock:      clock,
		},
	}
	keyStore, err := NewManager(ctx, cfg)
	require.NoError(t, err)

	totalKeys := pageSize * 3
	for i := 0; i < totalKeys; i++ {
		_, err := keyStore.NewSSHKeyPair(ctx)
		require.NoError(t, err)
	}

	// Newly created keys should not be deleted.
	err = keyStore.DeleteUnusedKeys(ctx, nil /*activeKeys*/)
	require.NoError(t, err)
	for _, key := range fakeKMS.keys {
		assert.Equal(t, "Enabled", key.state)
	}

	// Keys created more than 5 minutes ago should be deleted.
	clock.Advance(6 * time.Minute)
	err = keyStore.DeleteUnusedKeys(ctx, nil /*activeKeys*/)
	require.NoError(t, err)
	for _, key := range fakeKMS.keys {
		assert.Equal(t, "PendingDeletion", key.state)
	}

	// Insert a key created by a different Teleport cluster, it should not be
	// deleted by the keystore.
	output, err := fakeKMS.CreateKey(&kms.CreateKeyInput{
		Tags: []*kms.Tag{
			&kms.Tag{
				TagKey:   aws.String(clusterTagKey),
				TagValue: aws.String("other-cluster"),
			},
		},
	})
	require.NoError(t, err)
	otherClusterKeyARN := aws.StringValue(output.KeyMetadata.Arn)

	clock.Advance(6 * time.Minute)
	err = keyStore.DeleteUnusedKeys(ctx, nil /*activeKeys*/)
	require.NoError(t, err)
	for _, key := range fakeKMS.keys {
		if key.arn == otherClusterKeyARN {
			assert.Equal(t, "Enabled", key.state)
		} else {
			assert.Equal(t, "PendingDeletion", key.state)
		}
	}
}

type fakeAWSKMSService struct {
	kmsiface.KMSAPI

	keys      []*fakeAWSKMSKey
	clock     clockwork.Clock
	account   string
	region    string
	pageLimit int
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
	tags         []*kms.Tag
	creationDate time.Time
	state        string
}

func (f *fakeAWSKMSService) CreateKey(input *kms.CreateKeyInput) (*kms.CreateKeyOutput, error) {
	id := uuid.NewString()
	a := arn.ARN{
		Partition: "aws",
		Service:   "kms",
		Region:    f.region,
		AccountID: f.account,
		Resource:  id,
	}
	f.keys = append(f.keys, &fakeAWSKMSKey{
		arn:          a.String(),
		tags:         input.Tags,
		creationDate: f.clock.Now(),
		state:        "Enabled",
	})
	return &kms.CreateKeyOutput{
		KeyMetadata: &kms.KeyMetadata{
			Arn:   aws.String(a.String()),
			KeyId: aws.String(id),
		},
	}, nil
}

func (f *fakeAWSKMSService) GetPublicKey(input *kms.GetPublicKeyInput) (*kms.GetPublicKeyOutput, error) {
	key, ok := f.findKey(aws.StringValue(input.KeyId))
	if !ok {
		return nil, trace.NotFound("key %q not found", aws.StringValue(input.KeyId))
	}
	if key.state != "Enabled" {
		return nil, trace.NotFound("key %q is not enabled", aws.StringValue(input.KeyId))
	}
	return &kms.GetPublicKeyOutput{
		PublicKey: testRawPublicKeyDER,
	}, nil
}

func (f *fakeAWSKMSService) Sign(input *kms.SignInput) (*kms.SignOutput, error) {
	key, ok := f.findKey(aws.StringValue(input.KeyId))
	if !ok {
		return nil, trace.NotFound("key %q not found", aws.StringValue(input.KeyId))
	}
	if key.state != "Enabled" {
		return nil, trace.NotFound("key %q is not enabled", aws.StringValue(input.KeyId))
	}
	var opts crypto.SignerOpts
	switch aws.StringValue(input.SigningAlgorithm) {
	case "RSASSA_PKCS1_V1_5_SHA_256":
		opts = crypto.SHA256
	case "RSASSA_PKCS1_V1_5_SHA_512":
		opts = crypto.SHA512
	default:
		return nil, trace.BadParameter("unsupported SigningAlgorithm %q", aws.StringValue(input.SigningAlgorithm))
	}
	signer, err := utils.ParsePrivateKeyPEM(testRawPrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signature, err := signer.Sign(rand.Reader, input.Message, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &kms.SignOutput{
		Signature: signature,
	}, nil
}

func (f *fakeAWSKMSService) ScheduleKeyDeletion(input *kms.ScheduleKeyDeletionInput) (*kms.ScheduleKeyDeletionOutput, error) {
	key, ok := f.findKey(aws.StringValue(input.KeyId))
	if !ok {
		return nil, trace.NotFound("key %q not found", aws.StringValue(input.KeyId))
	}
	key.state = "PendingDeletion"
	return &kms.ScheduleKeyDeletionOutput{}, nil
}

func (f *fakeAWSKMSService) ListKeysWithContext(ctx aws.Context, input *kms.ListKeysInput, opts ...request.Option) (*kms.ListKeysOutput, error) {
	pageLimit := min(int(aws.Int64Value(input.Limit)), f.pageLimit)
	output := &kms.ListKeysOutput{}
	i := 0
	if input.Marker != nil {
		var err error
		i, err = strconv.Atoi(aws.StringValue(input.Marker))
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	for ; i < len(f.keys) && len(output.Keys) < pageLimit; i++ {
		output.Keys = append(output.Keys, &kms.KeyListEntry{
			KeyArn: aws.String(f.keys[i].arn),
		})
	}
	if i < len(f.keys) {
		output.NextMarker = aws.String(strconv.Itoa(i))
		output.Truncated = aws.Bool(true)
	}
	fmt.Println("NIC ListKeys", aws.StringValue(input.Marker), len(output.Keys), output.NextMarker)
	return output, nil
}

func (f *fakeAWSKMSService) ListResourceTagsWithContext(ctx aws.Context, input *kms.ListResourceTagsInput, opts ...request.Option) (*kms.ListResourceTagsOutput, error) {
	key, ok := f.findKey(aws.StringValue(input.KeyId))
	if !ok {
		return nil, trace.NotFound("key %q not found", aws.StringValue(input.KeyId))
	}
	return &kms.ListResourceTagsOutput{
		Tags: key.tags,
	}, nil
}

func (f *fakeAWSKMSService) DescribeKeyWithContext(ctx aws.Context, input *kms.DescribeKeyInput, opts ...request.Option) (*kms.DescribeKeyOutput, error) {
	key, ok := f.findKey(aws.StringValue(input.KeyId))
	if !ok {
		return nil, trace.NotFound("key %q not found", aws.StringValue(input.KeyId))
	}
	return &kms.DescribeKeyOutput{
		KeyMetadata: &kms.KeyMetadata{
			CreationDate: aws.Time(key.creationDate),
			KeyState:     aws.String(key.state),
		},
	}, nil
}

func (f *fakeAWSKMSService) findKey(arn string) (*fakeAWSKMSKey, bool) {
	i := slices.IndexFunc(f.keys, func(k *fakeAWSKMSKey) bool { return k.arn == arn })
	if i < 0 {
		return nil, false
	}
	return f.keys[i], true
}
