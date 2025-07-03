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
	"github.com/aws/smithy-go/tracing/smithyoteltracing"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"go.opentelemetry.io/otel"
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
	awsOAEPHash   = crypto.SHA256

	pendingKeyBaseRetryInterval = time.Second / 2
	pendingKeyMaxRetryInterval  = 4 * time.Second
	// TODO(dboslee): waiting on AWS support to answer question regarding
	// long time for GetPublicKey to succeed after updating key via UpdatePrimaryRegion.
	pendingKeyTimeout = 120 * time.Second
)

type awsKMSKeystore struct {
	kms                kmsClient
	mrk                mrkClient
	awsAccount         string
	awsRegion          string
	multiRegionEnabled bool
	primaryRegion      string
	replicaRegions     map[string]struct{}
	tags               map[string]string
	clock              clockwork.Clock
	logger             *slog.Logger
}

func newAWSKMSKeystore(ctx context.Context, cfg *servicecfg.AWSKMSConfig, opts *Options) (*awsKMSKeystore, error) {
	stsClient, kmsClient := opts.awsSTSClient, opts.awsKMSClient
	mrkClient := opts.mrkClient

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
			stsClient = stsutils.NewFromConfig(awsCfg, func(o *sts.Options) {
				o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
			})

		}
		if kmsClient == nil || mrkClient == nil {
			realKMS := kms.NewFromConfig(awsCfg, func(o *kms.Options) {
				o.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
				o.Region = cfg.AWSRegion
			})
			if kmsClient == nil {
				kmsClient = realKMS
			}
			if mrkClient == nil {
				mrkClient = realKMS
			}
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
	primary := cfg.MultiRegion.PrimaryRegion
	if primary == "" {
		primary = cfg.AWSRegion
	}
	replicas := make(map[string]struct{})
	for _, region := range append(cfg.MultiRegion.ReplicaRegions, primary, cfg.AWSRegion) {
		replicas[region] = struct{}{}
	}

	return &awsKMSKeystore{
		awsAccount:         cfg.AWSAccount,
		awsRegion:          cfg.AWSRegion,
		tags:               tags,
		multiRegionEnabled: cfg.MultiRegion.Enabled,
		primaryRegion:      primary,
		replicaRegions:     replicas,
		kms:                kmsClient,
		mrk:                mrkClient,
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

func (u keyUsage) toAWS() kmstypes.KeyUsageType {
	switch u {
	case keyUsageDecrypt:
		return kmstypes.KeyUsageTypeEncryptDecrypt
	default:
		return kmstypes.KeyUsageTypeSignVerify
	}
}

func (a *awsKMSKeystore) generateKey(ctx context.Context, algorithm cryptosuites.Algorithm, usage keyUsage) (awsKMSKeyID, error) {
	alg, err := awsAlgorithm(algorithm)
	if err != nil {
		return awsKMSKeyID{}, trace.Wrap(err)
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
		KeyUsage:    usage.toAWS(),
		Tags:        tags,
		MultiRegion: aws.Bool(a.multiRegionEnabled),
	})
	if err != nil {
		return awsKMSKeyID{}, trace.Wrap(err)
	}
	if output.KeyMetadata == nil {
		return awsKMSKeyID{}, trace.Errorf("KeyMetadata of generated key is nil")
	}
	keyARN := aws.ToString(output.KeyMetadata.Arn)
	key, err := keyIDFromArn(keyARN)
	if err != nil {
		return awsKMSKeyID{}, trace.Wrap(err)
	}

	return key, nil
}

// generateSigner creates a new private key and returns its identifier and a crypto.Signer. The returned
// identifier can be passed to getSigner later to get an equivalent crypto.Signer.
func (a *awsKMSKeystore) generateSigner(ctx context.Context, algorithm cryptosuites.Algorithm) ([]byte, crypto.Signer, error) {
	key, err := a.generateKey(ctx, algorithm, keyUsageSign)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	keyID, err := a.applyMRKConfig(ctx, key)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	signer, err := a.newKMSKey(ctx, key)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return keyID, signer, nil
}

// generateDecrypter creates a new private key and returns its identifier and a crypto.Decrypter. The returned
// identifier can be passed to getDecrypter later to get an equivalent crypto.Decrypter.
func (a *awsKMSKeystore) generateDecrypter(ctx context.Context, algorithm cryptosuites.Algorithm) ([]byte, crypto.Decrypter, crypto.Hash, error) {
	key, err := a.generateKey(ctx, algorithm, keyUsageDecrypt)
	if err != nil {
		return nil, nil, awsOAEPHash, trace.Wrap(err)
	}
	keyID, err := a.applyMRKConfig(ctx, key)
	if err != nil {
		return nil, nil, awsOAEPHash, trace.Wrap(err)
	}
	decrypter, err := a.newKMSKey(ctx, key)
	if err != nil {
		return nil, nil, awsOAEPHash, trace.Wrap(err)
	}
	return keyID, decrypter, awsOAEPHash, nil
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
	key, err := parseAWSKMSKeyID(rawKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return a.newKMSKeyWithPublicKey(ctx, key, publicKey)
}

// getDecrypter returns a crypto.Decrypter for the given key identifier, if it is found.
func (a *awsKMSKeystore) getDecrypter(ctx context.Context, rawKey []byte, publicKey crypto.PublicKey, hash crypto.Hash) (crypto.Decrypter, error) {
	key, err := parseAWSKMSKeyID(rawKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return a.newKMSKeyWithPublicKey(ctx, key, publicKey)
}

type awsKMSKey struct {
	key awsKMSKeyID
	pub crypto.PublicKey
	kms kmsClient
}

func (a *awsKMSKeystore) newKMSKey(ctx context.Context, key awsKMSKeyID) (*awsKMSKey, error) {
	var pubkeyDER []byte
	err := a.retryOnConsistencyError(ctx, func(ctx context.Context) error {
		a.logger.DebugContext(ctx, "Fetching public key", "key_arn", key.arn)
		output, err := a.kms.GetPublicKey(ctx, &kms.GetPublicKeyInput{
			KeyId: aws.String(key.id),
		})
		if err != nil {
			a.logger.DebugContext(ctx, "Failed to fetch public key", "key_arn", key.arn, "err", err)
			return trace.Wrap(err, "fetching public key")
		}
		pubkeyDER = output.PublicKey
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pub, err := x509.ParsePKIXPublicKey(pubkeyDER)
	if err != nil {
		return nil, trace.Wrap(err, "unexpected error parsing public key der")
	}
	return a.newKMSKeyWithPublicKey(ctx, key, pub)
}

// retryOnConsistencyError handles retrying KMS key operations that may fail
// temporarily due to eventual consistency.
// https://docs.aws.amazon.com/kms/latest/developerguide/programming-eventual-consistency.html
func (a *awsKMSKeystore) retryOnConsistencyError(ctx context.Context, fn func(ctx context.Context) error) error {
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:  pendingKeyBaseRetryInterval,
		Driver: retryutils.NewExponentialDriver(pendingKeyBaseRetryInterval),
		Max:    pendingKeyMaxRetryInterval,
		Jitter: retryutils.HalfJitter,
		Clock:  a.clock,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	ctx, cancel := context.WithTimeout(ctx, pendingKeyTimeout)
	defer cancel()
	timeout := a.clock.NewTimer(pendingKeyTimeout)
	defer timeout.Stop()
	for {
		err := fn(ctx)
		if err == nil {
			return nil
		}
		var (
			notFound     *kmstypes.NotFoundException
			invalidState *kmstypes.KMSInvalidStateException
		)
		if !errors.As(err, &notFound) && !errors.As(err, &invalidState) {
			return trace.Wrap(err, "unexpected error")
		}

		select {
		case <-retry.After():
			retry.Inc()
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-timeout.Chan():
			return trace.Wrap(err, "timeout retrying eventual consistency errors")
		}
	}
}

func (a *awsKMSKeystore) newKMSKeyWithPublicKey(_ context.Context, key awsKMSKeyID, publicKey crypto.PublicKey) (*awsKMSKey, error) {
	return &awsKMSKey{
		key: key,
		pub: publicKey,
		kms: a.kms,
	}, nil
}

// Public returns the public key for the signer.
func (a *awsKMSKey) Public() crypto.PublicKey {
	return a.pub
}

// Sign signs the message digest.
func (a *awsKMSKey) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
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
		KeyId:            aws.String(a.key.id),
		Message:          digest,
		MessageType:      kmstypes.MessageTypeDigest,
		SigningAlgorithm: signingAlg,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return output.Signature, nil
}

// Decrypt decrypts data encrypted with the public key
func (a *awsKMSKey) Decrypt(rand io.Reader, ciphertext []byte, opts crypto.DecrypterOpts) (plaintext []byte, err error) {
	var encAlg kmstypes.EncryptionAlgorithmSpec
	switch a.pub.(type) {
	case *rsa.PublicKey:
		encAlg = kmstypes.EncryptionAlgorithmSpecRsaesOaepSha256
	default:
		return nil, trace.BadParameter("unsupported key algorithm for AWS KMS decryption")
	}

	output, err := a.kms.Decrypt(context.TODO(), &kms.DecryptInput{
		KeyId:               aws.String(a.key.id),
		CiphertextBlob:      ciphertext,
		EncryptionAlgorithm: encAlg,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return output.Plaintext, nil
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

// canUseKey returns true if this KeyStore is able to sign with the given
// key.
func (a *awsKMSKeystore) canUseKey(ctx context.Context, raw []byte, keyType types.PrivateKeyType) (bool, error) {
	if keyType != types.PrivateKeyType_AWS_KMS {
		return false, nil
	}
	key, err := parseAWSKMSKeyID(raw)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return key.account == a.awsAccount && (key.region == a.awsRegion || key.isMRK()), nil
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
		keyIsRelevent, err := a.canUseKey(ctx, activeKey, keyType(activeKey))
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
		activeAWSKMSKeys[keyID.id] = 0
	}

	var keysToDelete []string
	var mu sync.RWMutex
	err := a.forEachKey(ctx, func(ctx context.Context, arn string) error {
		key, err := keyIDFromArn(arn)
		if err != nil {
			return trace.Wrap(err)
		}
		mu.RLock()
		_, active := activeAWSKMSKeys[key.id]
		mu.RUnlock()
		if active {
			// This is a known active key, record that it was found and return
			// (since it should never be deleted).
			mu.Lock()
			defer mu.Unlock()
			activeAWSKMSKeys[key.id] += 1
			return nil
		}

		// Check if this key was created by this Teleport cluster.
		output, err := a.kms.ListResourceTags(ctx, &kms.ListResourceTagsInput{
			KeyId: aws.String(key.id),
		})
		if err != nil {
			// It's entirely expected that we won't be allowed to fetch
			// tags for some keys, don't worry about deleting those.
			a.logger.DebugContext(ctx, "failed to fetch tags for AWS KMS key, skipping", "key_arn", arn, "error", err)
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
			KeyId: aws.String(key.id),
		})
		if err != nil {
			return trace.Wrap(err, "failed to describe AWS KMS key %q", arn)
		}
		if describeOutput.KeyMetadata == nil {
			return trace.Errorf("failed to describe AWS KMS key %q", arn)
		}
		if keyState := describeOutput.KeyMetadata.KeyState; keyState != kmstypes.KeyStateEnabled {
			a.logger.InfoContext(ctx, "deleteUnusedKeys skipping AWS KMS key which is not in enabled state.",
				"key_arn", arn, "key_state", keyState)
			return nil
		}
		creationDate := aws.ToTime(describeOutput.KeyMetadata.CreationDate)
		if a.clock.Now().Sub(creationDate).Abs() < 5*time.Minute {
			// Never delete keys created in the last 5 minutes in case they were
			// created by a different auth server and just haven't been added to
			// the backend CA yet (which is why they don't appear in activeKeys).
			a.logger.InfoContext(ctx, "deleteUnusedKeys skipping AWS KMS key which was created in the past 5 minutes.",
				"key_arn", arn)
			return nil
		}

		mu.Lock()
		defer mu.Unlock()
		keysToDelete = append(keysToDelete, *describeOutput.KeyMetadata.Arn)
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
			keyID := aws.ToString(keyEntry.KeyArn)
			errGroup.Go(func() error {
				return trace.Wrap(fn(ctx, keyID))
			})
		}
	}
	return trace.Wrap(errGroup.Wait())
}

func (a *awsKMSKeystore) applyMultiRegionConfig(ctx context.Context, keyID []byte) ([]byte, error) {
	if keyType(keyID) != types.PrivateKeyType_AWS_KMS {
		return keyID, nil
	}
	key, err := parseAWSKMSKeyID(keyID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	keyID, err = a.applyMRKConfig(ctx, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keyID, nil
}

func (a *awsKMSKeystore) applyMRKConfig(ctx context.Context, key awsKMSKeyID) ([]byte, error) {
	if !key.isMRK() {
		if a.multiRegionEnabled {
			a.logger.WarnContext(ctx, "Unable to replicate single-region key. A CA rotation is required to migrate to a multi-region key.", "key_arn", key.arn)
		}
		return key.marshal(), nil
	}

	tags := make([]kmstypes.Tag, 0, len(a.tags))
	for k, v := range a.tags {
		tags = append(tags, kmstypes.Tag{
			TagKey:   aws.String(k),
			TagValue: aws.String(v),
		})
	}

	client := a.mrk
	var describeKeyOut *kms.DescribeKeyOutput
	err := a.retryOnConsistencyError(ctx, func(ctx context.Context) error {
		var err error
		describeKeyOut, err = client.DescribeKey(ctx, &kms.DescribeKeyInput{
			KeyId: aws.String(key.id),
		})
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	currRegionKey, err := keyIDFromArn(*describeKeyOut.KeyMetadata.Arn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.waitForKeyEnabled(ctx, client, currRegionKey); err != nil {
		return nil, trace.Wrap(err)
	}
	if describeKeyOut.KeyMetadata.MultiRegionConfiguration == nil {
		// This error is not expected to be reached since we check that the key
		// is a multi-region key above.
		return nil, trace.Errorf("kms key %s missing multi-region configuration", currRegionKey.arn)
	}

	currPrimaryKey, err := keyIDFromArn(*describeKeyOut.KeyMetadata.MultiRegionConfiguration.PrimaryKey.Arn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var existingReplicas []awsKMSKeyID
	for _, replica := range append(
		describeKeyOut.KeyMetadata.MultiRegionConfiguration.ReplicaKeys,
		*describeKeyOut.KeyMetadata.MultiRegionConfiguration.PrimaryKey,
	) {
		key, err := keyIDFromArn(*replica.Arn)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		existingReplicas = append(existingReplicas, key)
	}

	// Only the primary region can replicate keys and update the primary region
	// so return early if we are operating outside of the primary region.
	if currRegionKey.region != currPrimaryKey.region {
		return key.marshal(), nil
	}

	for region := range a.replicaRegions {
		// Check if a replica already exists in this region.
		if slices.ContainsFunc(existingReplicas, func(key awsKMSKeyID) bool {
			return key.region == region
		}) {
			continue
		}
		a.logger.DebugContext(ctx, "Replicating key", "kms_arn", currPrimaryKey.arn, "replica_region", region)
		out, err := client.ReplicateKey(ctx, &kms.ReplicateKeyInput{
			KeyId:         &key.id,
			ReplicaRegion: &region,
			Tags:          tags,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		key, err := keyIDFromArn(*out.ReplicaKeyMetadata.Arn)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		existingReplicas = append(existingReplicas, key)
	}
	if currPrimaryKey.region == a.primaryRegion {
		return currPrimaryKey.marshal(), nil
	}

	err = a.retryOnConsistencyError(ctx, func(ctx context.Context) error {
		a.logger.DebugContext(ctx, "Updating primary region", "kms_arn", currPrimaryKey.arn, "primary", a.primaryRegion)
		_, err := client.UpdatePrimaryRegion(ctx, &kms.UpdatePrimaryRegionInput{
			KeyId:         aws.String(currPrimaryKey.id),
			PrimaryRegion: aws.String(a.primaryRegion),
		})
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, key := range existingReplicas {
		if key.region == a.primaryRegion {
			return key.marshal(), nil
		}
	}
	return nil, trace.Errorf("failed to find updated primary key region=%s key_id=%s", a.primaryRegion, key.id)
}

func (a *awsKMSKeystore) waitForKeyEnabled(ctx context.Context, client mrkClient, key awsKMSKeyID) error {
	err := a.retryOnConsistencyError(ctx, func(ctx context.Context) error {
		a.logger.DebugContext(ctx, "Waiting for key to be enabled", "key_arn", key.arn)
		out, err := client.DescribeKey(ctx, &kms.DescribeKeyInput{
			KeyId: aws.String(key.id),
		})
		if err != nil {
			a.logger.DebugContext(ctx, "Failed to get key state", "key_arn", key.arn, "err", err)
			return trace.Wrap(err, "failed to get key state")
		}
		// Return a KMSInvalidStateException so this can be retired by
		// retryOnConsistencyError.
		if out.KeyMetadata.KeyState != kmstypes.KeyStateEnabled {
			return &kmstypes.KMSInvalidStateException{
				Message: aws.String("key is not enabled state=" + string(out.KeyMetadata.KeyState)),
			}
		}
		return nil
	})
	return trace.Wrap(err)
}

type awsKMSKeyID struct {
	id, arn, account, region string
}

func (a awsKMSKeyID) marshal() []byte {
	return []byte(awskmsPrefix + a.arn)
}

// isMRK checks if a key is a multi-region key.
func (a awsKMSKeyID) isMRK() bool {
	return strings.HasPrefix(a.id, "mrk-")
}

func keyIDFromArn(keyARN string) (awsKMSKeyID, error) {
	parsedARN, err := arn.Parse(keyARN)
	if err != nil {
		return awsKMSKeyID{}, trace.Wrap(err, "unable parse ARN of AWS KMS key")
	}
	id := strings.TrimPrefix(parsedARN.Resource, "key/")
	return awsKMSKeyID{
		id:      id,
		arn:     keyARN,
		account: parsedARN.AccountID,
		region:  parsedARN.Region,
	}, nil
}

func parseAWSKMSKeyID(raw []byte) (awsKMSKeyID, error) {
	if keyType(raw) != types.PrivateKeyType_AWS_KMS {
		return awsKMSKeyID{}, trace.BadParameter("unable to parse invalid AWS KMS key")
	}
	keyARN := strings.TrimPrefix(string(raw), awskmsPrefix)
	key, err := keyIDFromArn(keyARN)
	return key, trace.Wrap(err)
}

type kmsClient interface {
	CreateKey(context.Context, *kms.CreateKeyInput, ...func(*kms.Options)) (*kms.CreateKeyOutput, error)
	GetPublicKey(context.Context, *kms.GetPublicKeyInput, ...func(*kms.Options)) (*kms.GetPublicKeyOutput, error)
	ListKeys(context.Context, *kms.ListKeysInput, ...func(*kms.Options)) (*kms.ListKeysOutput, error)
	ScheduleKeyDeletion(context.Context, *kms.ScheduleKeyDeletionInput, ...func(*kms.Options)) (*kms.ScheduleKeyDeletionOutput, error)
	DescribeKey(context.Context, *kms.DescribeKeyInput, ...func(*kms.Options)) (*kms.DescribeKeyOutput, error)
	ListResourceTags(context.Context, *kms.ListResourceTagsInput, ...func(*kms.Options)) (*kms.ListResourceTagsOutput, error)
	Sign(context.Context, *kms.SignInput, ...func(*kms.Options)) (*kms.SignOutput, error)
	Decrypt(context.Context, *kms.DecryptInput, ...func(*kms.Options)) (*kms.DecryptOutput, error)
}

// mrkClient is a client for managing multi-region keys.
type mrkClient interface {
	ReplicateKey(context.Context, *kms.ReplicateKeyInput, ...func(*kms.Options)) (*kms.ReplicateKeyOutput, error)
	UpdatePrimaryRegion(context.Context, *kms.UpdatePrimaryRegionInput, ...func(*kms.Options)) (*kms.UpdatePrimaryRegionOutput, error)
	DescribeKey(context.Context, *kms.DescribeKeyInput, ...func(*kms.Options)) (*kms.DescribeKeyOutput, error)
}

type stsClient interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}
