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
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/kms/kmsiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/cloud"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
)

const (
	awskmsPrefix  = "awskms:"
	clusterTagKey = "TeleportCluster"

	pendingKeyBaseRetryInterval = time.Second / 2
	pendingKeyMaxRetryInterval  = 4 * time.Second
	pendingKeyTimeout           = 30 * time.Second
)

type CloudClientProvider interface {
	// GetAWSSTSClient returns AWS STS client for the specified region.
	GetAWSSTSClient(ctx context.Context, region string, opts ...cloud.AWSOptionsFn) (stsiface.STSAPI, error)
	// GetAWSKMSClient returns AWS KMS client for the specified region.
	GetAWSKMSClient(ctx context.Context, region string, opts ...cloud.AWSOptionsFn) (kmsiface.KMSAPI, error)
}

// AWSKMSConfig holds configuration parameters specific to AWS KMS keystores.
type AWSKMSConfig struct {
	Cluster    string
	AWSAccount string
	AWSRegion  string

	CloudClients CloudClientProvider
	clock        clockwork.Clock
}

// CheckAndSetDefaults checks that required parameters of the config are
// properly set and sets defaults.
func (c *AWSKMSConfig) CheckAndSetDefaults() error {
	if c.Cluster == "" {
		return trace.BadParameter("cluster is required")
	}
	if c.AWSAccount == "" {
		return trace.BadParameter("AWS account is required")
	}
	if c.AWSRegion == "" {
		return trace.BadParameter("AWS region is required")
	}
	if c.CloudClients == nil {
		return trace.BadParameter("CloudClients is required")
	}
	if c.clock == nil {
		c.clock = clockwork.NewRealClock()
	}
	return nil
}

type awsKMSKeystore struct {
	kms        kmsiface.KMSAPI
	cluster    string
	awsAccount string
	awsRegion  string
	clock      clockwork.Clock
	logger     logrus.FieldLogger
}

func newAWSKMSKeystore(ctx context.Context, cfg *AWSKMSConfig, logger logrus.FieldLogger) (*awsKMSKeystore, error) {
	stsClient, err := cfg.CloudClients.GetAWSSTSClient(ctx, cfg.AWSRegion, cloud.WithAmbientCredentials())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	id, err := stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if aws.StringValue(id.Account) != cfg.AWSAccount {
		return nil, trace.BadParameter("configured AWS KMS account %q does not match AWS account of ambient credentials %q",
			cfg.AWSAccount, aws.StringValue(id.Account))
	}
	kmsClient, err := cfg.CloudClients.GetAWSKMSClient(ctx, cfg.AWSRegion, cloud.WithAmbientCredentials())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &awsKMSKeystore{
		cluster:    cfg.Cluster,
		awsAccount: cfg.AWSAccount,
		awsRegion:  cfg.AWSRegion,
		kms:        kmsClient,
		clock:      cfg.clock,
		logger:     logger,
	}, nil
}

// keyTypeDescription returns a human-readable description of the types of keys
// this backend uses.
func (a *awsKMSKeystore) keyTypeDescription() string {
	return fmt.Sprintf("AWS KMS keys in account %s and region %s", a.awsAccount, a.awsRegion)
}

// generateRSA creates a new RSA private key and returns its identifier and
// a crypto.Signer. The returned identifier can be passed to getSigner
// later to get the same crypto.Signer.
func (a *awsKMSKeystore) generateRSA(ctx context.Context, opts ...RSAKeyOption) ([]byte, crypto.Signer, error) {
	output, err := a.kms.CreateKey(&kms.CreateKeyInput{
		Description: aws.String("Teleport CA key"),
		KeySpec:     aws.String("RSA_2048"),
		KeyUsage:    aws.String("SIGN_VERIFY"),
		Tags: []*kms.Tag{
			{
				TagKey:   aws.String(clusterTagKey),
				TagValue: aws.String(a.cluster),
			},
		},
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if output.KeyMetadata == nil {
		return nil, nil, trace.Errorf("KeyMetadata of generated key is nil")
	}
	keyARN := aws.StringValue(output.KeyMetadata.Arn)
	signer, err := a.newSigner(ctx, keyARN)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	keyID := awsKMSKeyID{
		arn:     keyARN,
		account: a.awsAccount,
		region:  a.awsRegion,
	}.marshal()
	return keyID, signer, nil
}

// getSigner returns a crypto.Signer for the given key identifier, if it is found.
func (a *awsKMSKeystore) getSigner(ctx context.Context, rawKey []byte, publicKey crypto.PublicKey) (crypto.Signer, error) {
	keyID, err := parseAWSKMSKeyID(rawKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return a.newSignerWithPublicKey(ctx, keyID.arn, publicKey)
}

type awsKMSSigner struct {
	keyARN string
	pub    crypto.PublicKey
	kms    kmsiface.KMSAPI
}

func (a *awsKMSKeystore) newSigner(ctx context.Context, keyARN string) (*awsKMSSigner, error) {
	pubkeyDER, err := a.getPublicKeyDER(ctx, keyARN)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pub, err := x509.ParsePKIXPublicKey(pubkeyDER)
	if err != nil {
		return nil, trace.Wrap(err, "unexpected error parsing public key der")
	}
	return a.newSignerWithPublicKey(ctx, keyARN, pub)
}

func (a *awsKMSKeystore) getPublicKeyDER(ctx context.Context, keyARN string) ([]byte, error) {
	// KMS is eventually-consistent, and this is called immediately after the
	// key has been recreated, so a few retries may be necessary.
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:  pendingKeyBaseRetryInterval,
		Driver: retryutils.NewExponentialDriver(pendingKeyBaseRetryInterval),
		Max:    pendingKeyMaxRetryInterval,
		Jitter: retryutils.NewHalfJitter(),
		Clock:  a.clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithTimeout(ctx, pendingKeyTimeout)
	defer cancel()
	timeout := a.clock.NewTimer(pendingKeyTimeout)
	defer timeout.Stop()
	for {
		output, err := a.kms.GetPublicKeyWithContext(ctx, &kms.GetPublicKeyInput{
			KeyId: aws.String(keyARN),
		})
		if err == nil {
			return output.PublicKey, nil
		}

		// Check if the error is one of the two expected eventual consistency
		// error types
		// https://docs.aws.amazon.com/kms/latest/developerguide/programming-eventual-consistency.html
		var (
			notFound     *kms.NotFoundException
			invalidState *kms.InvalidStateException
		)
		if !errors.As(err, &notFound) && !errors.As(err, &invalidState) {
			return nil, trace.Wrap(err, "unexpected error fetching AWS KMS public key")
		}

		startedWaiting := a.clock.Now()
		select {
		case t := <-retry.After():
			a.logger.Debugf("Failed to fetch public key for %q, retrying after waiting %v", keyARN, t.Sub(startedWaiting))
			retry.Inc()
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		case <-timeout.Chan():
			return nil, trace.Errorf("timed out waiting for AWS KMS public key")
		}
	}
}

func (a *awsKMSKeystore) newSignerWithPublicKey(ctx context.Context, keyARN string, publicKey crypto.PublicKey) (*awsKMSSigner, error) {
	return &awsKMSSigner{
		keyARN: keyARN,
		pub:    publicKey,
		kms:    a.kms,
	}, nil
}

// Public returns the public key for the signer.
func (a *awsKMSSigner) Public() crypto.PublicKey {
	return a.pub
}

// Sign signs the message digest.
func (a *awsKMSSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	var signingAlg string
	switch opts.HashFunc() {
	case crypto.SHA256:
		signingAlg = "RSASSA_PKCS1_V1_5_SHA_256"
	case crypto.SHA512:
		signingAlg = "RSASSA_PKCS1_V1_5_SHA_512"
	default:
		return nil, trace.BadParameter("unsupported hash func %q for AWS KMS key", opts.HashFunc())
	}
	output, err := a.kms.Sign(&kms.SignInput{
		KeyId:            aws.String(a.keyARN),
		Message:          digest,
		MessageType:      aws.String("DIGEST"),
		SigningAlgorithm: aws.String(signingAlg),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return output.Signature, nil
}

// deleteKey deletes the given key from the KeyStore.
func (a *awsKMSKeystore) deleteKey(ctx context.Context, rawKey []byte) error {
	keyID, err := parseAWSKMSKeyID(rawKey)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = a.kms.ScheduleKeyDeletion(&kms.ScheduleKeyDeletionInput{
		KeyId:               aws.String(keyID.arn),
		PendingWindowInDays: aws.Int64(7),
	})
	return trace.Wrap(err, "error deleting AWS KMS key")
}

// canSignWithKey returns true if this KeyStore is able to sign with the given
// key.
func (a *awsKMSKeystore) canSignWithKey(ctx context.Context, raw []byte, keyType types.PrivateKeyType) (bool, error) {
	if keyType != types.PrivateKeyType_AWS_KMS {
		return false, nil
	}
	keyID, err := parseAWSKMSKeyID(raw)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return keyID.account == a.awsAccount && keyID.region == a.awsRegion, nil
}

// DeleteUnusedKeys deletes all keys readable from the AWS KMS account and
// region if they:
// 1. Are not included in the argument activeKeys
// 2. Are labeled in AWS KMS as being created by this Teleport cluster
// 3. Were not created in the past 5 minutes.
//
// The activeKeys argument is meant to contain to complete set of raw key IDs as
// stored in the current CA specs in the backend.
//
// The reason this does not delete any keys created in the past 5 minutes is to
// avoid a race where:
// 1. A different auth server (auth2) creates a new key in GCP KMS
// 2. This function (running on auth1) deletes that new key
// 3. auth2 saves the id of this deleted key to the backend CA
func (a *awsKMSKeystore) deleteUnusedKeys(ctx context.Context, activeKeys [][]byte) error {
	activeAWSKMSKeys := make(map[string]int)
	for _, activeKey := range activeKeys {
		keyIsRelevent, err := a.canSignWithKey(ctx, activeKey, keyType(activeKey))
		if err != nil {
			// Don't expect this error to ever hit, safer to return if it does.
			return trace.Wrap(err)
		}
		if !keyIsRelevent {
			// Ignore active keys that are not AWS KMS keys or are not in the
			// account and region that this Auth is configured to use.
			continue
		}
		keyID, err := parseAWSKMSKeyID(activeKey)
		if err != nil {
			// Realistically we should not hit this since canSignWithKey already
			// calls parseAWSKMSKeyID.
			return trace.Wrap(err)
		}
		activeAWSKMSKeys[keyID.arn] = 0
	}

	var keysToDelete []string
	var mu sync.RWMutex
	err := a.forEachKey(ctx, func(ctx context.Context, keyARN string) error {
		mu.RLock()
		_, active := activeAWSKMSKeys[keyARN]
		mu.RUnlock()
		if active {
			// This is a known active key, record that it was found and return
			// (since it should never be deleted).
			mu.Lock()
			defer mu.Unlock()
			activeAWSKMSKeys[keyARN] += 1
			return nil
		}

		// Check if this key was created by this Teleport cluster.
		output, err := a.kms.ListResourceTagsWithContext(ctx, &kms.ListResourceTagsInput{
			KeyId: aws.String(keyARN),
		})
		if err != nil {
			err = awslib.ConvertRequestFailureError(err)
			if trace.IsAccessDenied(err) {
				// It's entirely expected that we'll not be allowed to fetch
				// tags for some keys, don't worry about deleting those.
				return nil
			}
			return trace.Wrap(err, "failed to fetch tags for AWS KMS key %q", keyARN)
		}
		if !slices.ContainsFunc(output.Tags, func(tag *kms.Tag) bool {
			return aws.StringValue(tag.TagKey) == clusterTagKey && aws.StringValue(tag.TagValue) == a.cluster
		}) {
			// This key was not created by this Teleport cluster, never delete it.
			return nil
		}

		// Check if this key is not enabled or was created in the past 5 minutes.
		describeOutput, err := a.kms.DescribeKeyWithContext(ctx, &kms.DescribeKeyInput{
			KeyId: aws.String(keyARN),
		})
		if err != nil {
			return trace.Wrap(err, "failed to describe AWS KMS key %q", keyARN)
		}
		if describeOutput.KeyMetadata == nil {
			return trace.Errorf("failed to describe AWS KMS key %q", keyARN)
		}
		if keyState := aws.StringValue(describeOutput.KeyMetadata.KeyState); keyState != "Enabled" {
			a.logger.WithFields(logrus.Fields{
				"key_arn":   keyARN,
				"key_state": keyState,
			}).Info("deleteUnusedKeys skipping AWS KMS key which is not in enabled state.")
			return nil
		}
		creationDate := aws.TimeValue(describeOutput.KeyMetadata.CreationDate)
		if a.clock.Now().Sub(creationDate).Abs() < 5*time.Minute {
			// Never delete keys created in the last 5 minutes in case they were
			// created by a different auth server and just haven't been added to
			// the backend CA yet (which is why they don't appear in activeKeys).
			a.logger.WithFields(logrus.Fields{
				"key_arn": keyARN,
			}).Info("deleteUnusedKeys skipping AWS KMS key which was created in the past 5 minutes.")
			return nil
		}

		mu.Lock()
		defer mu.Unlock()
		keysToDelete = append(keysToDelete, keyARN)
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// If any member of activeKeys which is in the same AWS account+region
	// queried here was not found in the ListKeys response, something has
	// gone wrong and there's a chance we have a bug or AWS has made a breaking
	// API change. In this case we should abort to avoid the chance of deleting
	// any currently active keys.
	for keyARN, found := range activeAWSKMSKeys {
		if found == 0 {
			return trace.NotFound("cannot find currently active CA key %q in AWS KMS, aborting attempt to delete unused keys", keyARN)
		}
	}

	for _, keyARN := range keysToDelete {
		a.logger.WithField("key_arn", keyARN).Info("Deleting unused AWS KMS key.")
		if _, err := a.kms.ScheduleKeyDeletion(&kms.ScheduleKeyDeletionInput{
			KeyId:               aws.String(keyARN),
			PendingWindowInDays: aws.Int64(7),
		}); err != nil {
			return trace.Wrap(err, "failed to schedule AWS KMS key %q for deletion", keyARN)
		}
	}
	return nil
}

// forEachKey calls fn with the AWS key ID of all keys in the AWS account and
// region that would be returned by ListKeys. It may call fn concurrently.
func (a *awsKMSKeystore) forEachKey(ctx context.Context, fn func(ctx context.Context, keyARN string) error) error {
	errGroup, ctx := errgroup.WithContext(ctx)
	marker := ""
	more := true
	for more {
		var markerInput *string
		if marker != "" {
			markerInput = aws.String(marker)
		}
		output, err := a.kms.ListKeysWithContext(ctx, &kms.ListKeysInput{
			Marker: markerInput,
			Limit:  aws.Int64(1000),
		})
		if err != nil {
			return trace.Wrap(err, "failed to list AWS KMS keys")
		}
		marker = aws.StringValue(output.NextMarker)
		more = aws.BoolValue(output.Truncated)
		for _, keyEntry := range output.Keys {
			keyArn := aws.StringValue(keyEntry.KeyArn)
			errGroup.Go(func() error {
				return trace.Wrap(fn(ctx, keyArn))
			})
		}
	}
	return trace.Wrap(errGroup.Wait())
}

type awsKMSKeyID struct {
	arn, account, region string
}

func (a awsKMSKeyID) marshal() []byte {
	return []byte(awskmsPrefix + a.arn)
}

func parseAWSKMSKeyID(raw []byte) (awsKMSKeyID, error) {
	if keyType(raw) != types.PrivateKeyType_AWS_KMS {
		return awsKMSKeyID{}, trace.BadParameter("unable to parse invalid AWS KMS key")
	}
	keyARN := strings.TrimPrefix(string(raw), awskmsPrefix)
	parsedARN, err := arn.Parse(keyARN)
	if err != nil {
		return awsKMSKeyID{}, trace.Wrap(err, "unable parse ARN of AWS KMS key")
	}
	return awsKMSKeyID{
		arn:     keyARN,
		account: parsedARN.AccountID,
		region:  parsedARN.Region,
	}, nil
}
