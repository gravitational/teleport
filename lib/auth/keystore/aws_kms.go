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
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

const (
	awskmsPrefix  = "awskms:"
	clusterTagKey = "TeleportCluster"

	pendingKeyBaseRetryInterval = time.Second / 2
	pendingKeyMaxRetryInterval  = 4 * time.Second
	pendingKeyTimeout           = 30 * time.Second
)

type awsKMSKeystore struct {
	kms                kmsClient
	awsAccount         string
	awsRegion          string
	multiRegionEnabled bool
	tags               map[string]string
	clock              clockwork.Clock
	logger             *slog.Logger
}

func newAWSKMSKeystore(ctx context.Context, cfg *servicecfg.AWSKMSConfig, opts *Options) (*awsKMSKeystore, error) {
	stsClient, kmsClient := opts.awsSTSClient, opts.awsKMSClient
	if stsClient == nil || kmsClient == nil {
		useFIPSEndpoint := aws.FIPSEndpointStateUnset
		if opts.FIPS {
			useFIPSEndpoint = aws.FIPSEndpointStateEnabled
		}
		awsCfg, err := config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.AWSRegion),
			config.WithUseFIPSEndpoint(useFIPSEndpoint),
		)
		if err != nil {
			return nil, trace.Wrap(err, "loading default AWS config")
		}
		if stsClient == nil {
			stsClient = stsutils.NewFromConfig(awsCfg)
		}
		if kmsClient == nil {
			kmsClient = kms.NewFromConfig(awsCfg)
		}
	}
	id, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, trace.Wrap(err, "checking AWS account of local credentials for AWS KMS")
	}
	if aws.ToString(id.Account) != cfg.AWSAccount {
		return nil, trace.BadParameter("configured AWS KMS account %q does not match AWS account of ambient credentials %q",
			cfg.AWSAccount, aws.ToString(id.Account))
	}

	tags := cfg.Tags
	if tags == nil {
		tags = make(map[string]string, 1)
	}
	if _, ok := tags[clusterTagKey]; !ok {
		tags[clusterTagKey] = opts.ClusterName.GetClusterName()
	}

	clock := opts.clockworkOverride
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	return &awsKMSKeystore{
		awsAccount:         cfg.AWSAccount,
		awsRegion:          cfg.AWSRegion,
		tags:               tags,
		multiRegionEnabled: cfg.MultiRegion.Enabled,
		kms:                kmsClient,
		clock:              clock,
		logger:             opts.Logger,
	}, nil
}

func (a *awsKMSKeystore) name() string {
	return storeAWS
}

// keyTypeDescription returns a human-readable description of the types of keys
// this backend uses.
func (a *awsKMSKeystore) keyTypeDescription() string {
	return fmt.Sprintf("AWS KMS keys in account %s and region %s", a.awsAccount, a.awsRegion)
}

// generateKey creates a new private key and returns its identifier and a crypto.Signer. The returned
// identifier can be passed to getSigner later to get an equivalent crypto.Signer.
func (a *awsKMSKeystore) generateKey(ctx context.Context, algorithm cryptosuites.Algorithm) ([]byte, crypto.Signer, error) {
	alg, err := awsAlgorithm(algorithm)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	a.logger.InfoContext(ctx, "Creating new AWS KMS keypair.",
		slog.Any("algorithm", algorithm),
		slog.Bool("multi_region", a.multiRegionEnabled))

	tags := make([]kmstypes.Tag, 0, len(a.tags))
	for k, v := range a.tags {
		tags = append(tags, kmstypes.Tag{
			TagKey:   aws.String(k),
			TagValue: aws.String(v),
		})
	}

	output, err := a.kms.CreateKey(ctx, &kms.CreateKeyInput{
		Description: aws.String("Teleport CA key"),
		KeySpec:     alg,
		KeyUsage:    kmstypes.KeyUsageTypeSignVerify,
		Tags:        tags,
		MultiRegion: aws.Bool(a.multiRegionEnabled),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if output.KeyMetadata == nil {
		return nil, nil, trace.Errorf("KeyMetadata of generated key is nil")
	}
	keyARN := aws.ToString(output.KeyMetadata.Arn)
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

func awsAlgorithm(alg cryptosuites.Algorithm) (kmstypes.KeySpec, error) {
	switch alg {
	case cryptosuites.RSA2048:
		return kmstypes.KeySpecRsa2048, nil
	case cryptosuites.ECDSAP256:
		return kmstypes.KeySpecEccNistP256, nil
	}
	return "", trace.BadParameter("unsupported algorithm for AWS KMS: %v", alg)
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
	kms    kmsClient
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
		Jitter: retryutils.HalfJitter,
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
		output, err := a.kms.GetPublicKey(ctx, &kms.GetPublicKeyInput{
			KeyId: aws.String(keyARN),
		})
		if err == nil {
			return output.PublicKey, nil
		}

		// Check if the error is one of the two expected eventual consistency
		// error types
		// https://docs.aws.amazon.com/kms/latest/developerguide/programming-eventual-consistency.html
		var (
			notFound     *kmstypes.NotFoundException
			invalidState *kmstypes.KMSInvalidStateException
		)
		if !errors.As(err, &notFound) && !errors.As(err, &invalidState) {
			return nil, trace.Wrap(err, "unexpected error fetching AWS KMS public key")
		}

		startedWaiting := a.clock.Now()
		select {
		case t := <-retry.After():
			a.logger.DebugContext(ctx, "Failed to fetch public key, retrying", "key_arn", keyARN, "retry_interval", t.Sub(startedWaiting))
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
	var signingAlg kmstypes.SigningAlgorithmSpec
	switch opts.HashFunc() {
	case crypto.SHA256:
		switch a.pub.(type) {
		case *rsa.PublicKey:
			signingAlg = kmstypes.SigningAlgorithmSpecRsassaPkcs1V15Sha256
		case *ecdsa.PublicKey:
			signingAlg = kmstypes.SigningAlgorithmSpecEcdsaSha256
		default:
			return nil, trace.BadParameter("unsupported hash func %q for AWS KMS key type %T", opts.HashFunc(), a.pub)
		}
	case crypto.SHA512:
		switch a.pub.(type) {
		case *rsa.PublicKey:
			signingAlg = kmstypes.SigningAlgorithmSpecRsassaPkcs1V15Sha512
		case *ecdsa.PublicKey:
			signingAlg = kmstypes.SigningAlgorithmSpecEcdsaSha512
		default:
			return nil, trace.BadParameter("unsupported hash func %q for AWS KMS key type %T", opts.HashFunc(), a.pub)
		}
	default:
		return nil, trace.BadParameter("unsupported hash func %q for AWS KMS key", opts.HashFunc())
	}
	output, err := a.kms.Sign(context.TODO(), &kms.SignInput{
		KeyId:            aws.String(a.keyARN),
		Message:          digest,
		MessageType:      kmstypes.MessageTypeDigest,
		SigningAlgorithm: signingAlg,
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
	_, err = a.kms.ScheduleKeyDeletion(ctx, &kms.ScheduleKeyDeletionInput{
		KeyId:               aws.String(keyID.arn),
		PendingWindowInDays: aws.Int32(7),
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
		output, err := a.kms.ListResourceTags(ctx, &kms.ListResourceTagsInput{
			KeyId: aws.String(keyARN),
		})
		if err != nil {
			// It's entirely expected that we won't be allowed to fetch
			// tags for some keys, don't worry about deleting those.
			a.logger.DebugContext(ctx, "failed to fetch tags for AWS KMS key, skipping", "key_arn", keyARN, "error", err)
			return nil
		}

		// All tags must match for this key to be considered for deletion.
		for k, v := range a.tags {
			if !slices.ContainsFunc(output.Tags, func(tag kmstypes.Tag) bool {
				return aws.ToString(tag.TagKey) == k && aws.ToString(tag.TagValue) == v
			}) {
				return nil
			}
		}

		// Check if this key is not enabled or was created in the past 5 minutes.
		describeOutput, err := a.kms.DescribeKey(ctx, &kms.DescribeKeyInput{
			KeyId: aws.String(keyARN),
		})
		if err != nil {
			return trace.Wrap(err, "failed to describe AWS KMS key %q", keyARN)
		}
		if describeOutput.KeyMetadata == nil {
			return trace.Errorf("failed to describe AWS KMS key %q", keyARN)
		}
		if keyState := describeOutput.KeyMetadata.KeyState; keyState != kmstypes.KeyStateEnabled {
			a.logger.InfoContext(ctx, "deleteUnusedKeys skipping AWS KMS key which is not in enabled state.",
				"key_arn", keyARN, "key_state", keyState)
			return nil
		}
		creationDate := aws.ToTime(describeOutput.KeyMetadata.CreationDate)
		if a.clock.Now().Sub(creationDate).Abs() < 5*time.Minute {
			// Never delete keys created in the last 5 minutes in case they were
			// created by a different auth server and just haven't been added to
			// the backend CA yet (which is why they don't appear in activeKeys).
			a.logger.InfoContext(ctx, "deleteUnusedKeys skipping AWS KMS key which was created in the past 5 minutes.",
				"key_arn", keyARN)
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
		a.logger.InfoContext(ctx, "Deleting unused AWS KMS key.", "key_arn", keyARN)
		if _, err := a.kms.ScheduleKeyDeletion(ctx, &kms.ScheduleKeyDeletionInput{
			KeyId:               aws.String(keyARN),
			PendingWindowInDays: aws.Int32(7),
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
		output, err := a.kms.ListKeys(ctx, &kms.ListKeysInput{
			Marker: markerInput,
			Limit:  aws.Int32(1000),
		})
		if err != nil {
			return trace.Wrap(err, "failed to list AWS KMS keys")
		}
		marker = aws.ToString(output.NextMarker)
		more = output.Truncated
		for _, keyEntry := range output.Keys {
			keyArn := aws.ToString(keyEntry.KeyArn)
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

type kmsClient interface {
	CreateKey(context.Context, *kms.CreateKeyInput, ...func(*kms.Options)) (*kms.CreateKeyOutput, error)
	GetPublicKey(context.Context, *kms.GetPublicKeyInput, ...func(*kms.Options)) (*kms.GetPublicKeyOutput, error)
	ListKeys(context.Context, *kms.ListKeysInput, ...func(*kms.Options)) (*kms.ListKeysOutput, error)
	ScheduleKeyDeletion(context.Context, *kms.ScheduleKeyDeletionInput, ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error)
	DescribeKey(context.Context, *kms.DescribeKeyInput, ...func(*kms.Options)) (*kms.DescribeKeyOutput, error)
	ListResourceTags(context.Context, *kms.ListResourceTagsInput, ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error)
	Sign(context.Context, *kms.SignInput, ...func(*kms.Options)) (*kms.SignOutput, error)
}

type stsClient interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}
