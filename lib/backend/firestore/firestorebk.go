/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package firestore

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	apiv1 "cloud.google.com/go/firestore/apiv1/admin"
	"cloud.google.com/go/firestore/apiv1/admin/adminpb"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils/interval"
)

func init() {
	backend.MustRegister(GetName(), func(ctx context.Context, p backend.Params) (backend.Backend, error) {
		return New(ctx, p, Options{})
	})
}

const (
	// maxTxnAttempts is the maximum amount of internal retries to be used by the
	// firestore package during RunTransaction. As of the time of writing the default
	// value used by the library is 5.
	maxTxnAttempts = 16
)

// Config structure represents Firestore configuration as appears in `storage` section of Teleport YAML
type Config struct {
	// Credentials path for the Firestore client
	CredentialsPath string `json:"credentials_path,omitempty"`
	// Google Project ID of Collection containing events
	ProjectID string `json:"project_id,omitempty"`
	// CollectName is the name of the collection containing events
	CollectionName string `json:"collection_name,omitempty"`
	// PurgeExpiredDocumentsPollInterval is the poll interval used to purge expired documents
	PurgeExpiredDocumentsPollInterval time.Duration `json:"purge_expired_documents_poll_interval,omitempty"`
	// RetryPeriod is a period between retry executions of long-lived document snapshot queries and purging expired records
	RetryPeriod time.Duration `json:"retry_period,omitempty"`
	// DisableExpiredDocumentPurge
	DisableExpiredDocumentPurge bool `json:"disable_expired_document_purge,omitempty"`
	// EndPoint is used to point the Firestore clients at emulated Firestore storage.
	EndPoint string `json:"endpoint,omitempty"`
	// DatabaseID is the identifier of a specific Firestore database to use. If not specified, the
	// default database for the ProjectID is used.
	DatabaseID string `json:"database_id,omitempty"`
}

type backendConfig struct {
	// FirestoreConfig base config composed into FirestoreBK-specific config
	Config
	// BufferSize is a default buffer size used to pull events
	BufferSize int `json:"buffer_size,omitempty"`
	// LimitWatchQuery is a parameter that will limit the document snapshot watcher on startup to the current time
	LimitWatchQuery bool `json:"limit_watch_query,omitempty"`
}

// CheckAndSetDefaults is a helper returns an error if the supplied configuration
// is not enough to connect to Firestore
func (cfg *backendConfig) CheckAndSetDefaults() error {
	// table is not configured?
	if cfg.CollectionName == "" {
		return trace.BadParameter("firestore: collection_name is not specified")
	}
	if cfg.ProjectID == "" {
		return trace.BadParameter("firestore: project_id is not specified")
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = backend.DefaultBufferCapacity
	}
	if cfg.PurgeExpiredDocumentsPollInterval == 0 {
		cfg.PurgeExpiredDocumentsPollInterval = defaultPurgeInterval
	}
	if cfg.RetryPeriod == 0 {
		cfg.RetryPeriod = defaults.HighResPollingPeriod
	}
	return nil
}

// Backend is a Firestore-backed key value backend implementation.
type Backend struct {
	logger *slog.Logger
	backendConfig
	// svc is the primary Firestore client
	svc *firestore.Client
	// clock is the
	clock clockwork.Clock
	// buf
	buf *backend.CircularBuffer
	// clientContext firestore client contexts
	clientContext context.Context
	// clientCancel firestore context cancel funcs
	clientCancel context.CancelFunc
}

type record struct {
	Key        []byte `firestore:"key,omitempty"`
	Timestamp  int64  `firestore:"timestamp,omitempty"`
	Expires    int64  `firestore:"expires,omitempty"`
	Value      []byte `firestore:"value,omitempty"`
	RevisionV2 string `firestore:"revision,omitempty"`
	RevisionV1 string `firestore:"-"`
}

func (r *record) updates() []firestore.Update {
	return []firestore.Update{
		{Path: keyDocProperty, Value: r.Key},
		{Path: timestampDocProperty, Value: r.Timestamp},
		{Path: expiresDocProperty, Value: r.Expires},
		{Path: valueDocProperty, Value: r.Value},
		{Path: revisionDocProperty, Value: r.RevisionV2},
	}
}

// legacyRecord is an older version of record used to marshal backend.Items.
// The only difference is the Value field: string (legacy) vs []byte (new).
//
// Firestore encoder enforces string fields to be valid UTF-8, which Go does
// not. Some data we store have binary values.
// Firestore decoder will not transparently unmarshal string records into
// []byte fields for us, so we have to do it manually.
// See newRecordFromDoc below.
type legacyRecord struct {
	Key       string `firestore:"key,omitempty"`
	Timestamp int64  `firestore:"timestamp,omitempty"`
	Expires   int64  `firestore:"expires,omitempty"`
	Value     string `firestore:"value,omitempty"`
}

type brokenKey []byte

// brokenRecord is an incorrect version of record used to marshal backend.Items.
// The Key type was inadvertently changed from a []byte to backend.Key which
// causes problems reading existing data prior to the conversion.
type brokenRecord struct {
	Key        brokenKey `firestore:"key,omitempty"`
	Timestamp  int64     `firestore:"timestamp,omitempty"`
	Expires    int64     `firestore:"expires,omitempty"`
	Value      []byte    `firestore:"value,omitempty"`
	RevisionV2 string    `firestore:"revision,omitempty"`
}

func newRecord(from backend.Item, clock clockwork.Clock) record {
	r := record{
		Key:       []byte(from.Key.String()),
		Value:     from.Value,
		Timestamp: clock.Now().UTC().Unix(),
	}

	if isRevisionV2(from.Revision) {
		r.RevisionV2 = from.Revision
	} else {
		r.RevisionV1 = from.Revision
	}

	if !from.Expires.IsZero() {
		r.Expires = from.Expires.UTC().Unix()
	}
	return r
}

// TODO(tigrato|rosstimothy): Simplify this function by removing the brokenRecord and legacyRecord struct in 19.0.0
func newRecordFromDoc(doc *firestore.DocumentSnapshot) (*record, error) {
	k, err := doc.DataAt(keyDocProperty)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var r record
	switch k.(type) {
	case []any:
		// If the key is a slice of any, then the key was mistakenly persisted
		// as a backend.Key directly.
		var br brokenRecord
		if doc.DataTo(&br) != nil {
			return nil, ConvertGRPCError(err)
		}

		r = record{
			Key:        br.Key,
			Value:      br.Value,
			Timestamp:  br.Timestamp,
			Expires:    br.Expires,
			RevisionV2: br.RevisionV2,
		}
	default:
		if err := doc.DataTo(&r); err != nil {
			// If unmarshal failed, try using the old format of records, where
			// Value was a string. This document could've been written by an older
			// version of our code.
			var rl legacyRecord
			if doc.DataTo(&rl) != nil {
				return nil, ConvertGRPCError(err)
			}
			r = record{
				Key:       []byte(rl.Key),
				Value:     []byte(rl.Value),
				Timestamp: rl.Timestamp,
				Expires:   rl.Expires,
			}
		}
	}

	if r.RevisionV2 == "" {
		r.RevisionV1 = toRevisionV1(doc.UpdateTime)
	}
	return &r, nil
}

// isExpired returns 'true' if the given object (record) has a TTL and it's due
func (r *record) isExpired(now time.Time) bool {
	if r.Expires == 0 {
		return false
	}

	return now.UTC().Unix() > r.Expires
}

func (r *record) backendItem() backend.Item {
	bi := backend.Item{
		Key:   backend.KeyFromString(string(r.Key)),
		Value: r.Value,
	}

	if r.RevisionV2 != "" {
		bi.Revision = r.RevisionV2
	} else {
		bi.Revision = r.RevisionV1
	}

	if r.Expires != 0 {
		bi.Expires = time.Unix(r.Expires, 0).UTC()
	}
	return bi
}

const (
	// BackendName is the name of this backend
	BackendName = "firestore"
	// defaultPurgeInterval is the interval for the ticker that executes the expired record query and cleanup
	defaultPurgeInterval = time.Minute
	// keyDocProperty is used internally to query for records and matches the key in the record struct tag
	keyDocProperty = "key"
	// expiresDocProperty is used internally to query for records and matches the expiration timestamp in the record struct tag
	expiresDocProperty = "expires"
	// timestampDocProperty is used internally to query for records and matches the timestamp in the record struct tag
	timestampDocProperty = "timestamp"
	// valueDocProperty references the value of the record
	valueDocProperty = "value"
	// revisionDocProperty references the record's revision
	revisionDocProperty = "revision"
	// timeInBetweenIndexCreationStatusChecks
	timeInBetweenIndexCreationStatusChecks = time.Second * 10
)

// GetName is a part of backend API and it returns Firestore backend type
// as it appears in `storage/type` section of Teleport YAML
func GetName() string {
	return BackendName
}

// keep this here to test interface conformance
var _ backend.Backend = (*Backend)(nil)

// ownerCredentials adds the needed authorization headers when
// interacting with the emulator to allow access to the
// batched write api. Without the header, the emulator returns
// the following error:
// rpc error: code = PermissionDenied desc = Batch writes require admin authentication
//
// See the following issues for more details:
// https://github.com/firebase/firebase-tools/issues/1363
// https://github.com/firebase/firebase-tools/issues/3833
type ownerCredentials struct{}

func (t ownerCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"Authorization": "Bearer owner"}, nil
}

func (t ownerCredentials) RequireTransportSecurity() bool { return false }

// CreateFirestoreClients creates a firestore admin and normal client given the supplied parameters
func CreateFirestoreClients(ctx context.Context, projectID, database string, endpoint string, credentialsFile string) (*apiv1.FirestoreAdminClient, *firestore.Client, error) {
	var args []option.ClientOption

	if endpoint != "" {
		args = append(args,
			option.WithTelemetryDisabled(),
			option.WithoutAuthentication(),
			option.WithEndpoint(endpoint),
			option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
			option.WithGRPCDialOption(grpc.WithPerRPCCredentials(ownerCredentials{})),
		)
	} else if credentialsFile != "" {
		args = append(args, option.WithCredentialsFile(credentialsFile))
	}

	firestoreAdminClient, err := apiv1.NewFirestoreAdminClient(ctx, args...)
	if err != nil {
		return nil, nil, ConvertGRPCError(err)
	}

	if database == "" {
		firestoreClient, err := firestore.NewClient(ctx, projectID, args...)
		if err != nil {
			return nil, nil, ConvertGRPCError(err)
		}

		return firestoreAdminClient, firestoreClient, nil
	}

	firestoreClient, err := firestore.NewClientWithDatabase(ctx, projectID, database, args...)
	if err != nil {
		return nil, nil, ConvertGRPCError(err)
	}

	return firestoreAdminClient, firestoreClient, nil
}

// Options describes the set of parameters to the Firestore backend that are
// not exposed to configuration files.
type Options struct {
	// Clock is the clock used to measure time for the backend, including
	// record TTL, keep-alives, etc.
	Clock clockwork.Clock
}

func (opts *Options) checkAndSetDefaults() error {
	if opts.Clock == nil {
		opts.Clock = clockwork.NewRealClock()
	}

	return nil
}

// New returns new instance of Firestore backend.
// It's an implementation of backend API's NewFunc
func New(ctx context.Context, params backend.Params, options Options) (*Backend, error) {
	l := slog.With(teleport.ComponentKey, BackendName)
	var cfg *backendConfig
	err := apiutils.ObjectToStruct(params, &cfg)
	if err != nil {
		return nil, trace.BadParameter("firestore: configuration is invalid: %v", err)
	}
	l.InfoContext(ctx, "Initializing backend.")

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := options.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	closeCtx, cancel := context.WithCancel(ctx)
	firestoreAdminClient, firestoreClient, err := CreateFirestoreClients(closeCtx, cfg.ProjectID, cfg.DatabaseID, cfg.EndPoint, cfg.CredentialsPath)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}
	// Admin client is only used for building indexes at startup.
	// It won't be needed after New returns.
	defer firestoreAdminClient.Close()

	buf := backend.NewCircularBuffer(
		backend.BufferCapacity(cfg.BufferSize),
	)

	b := &Backend{
		svc:           firestoreClient,
		logger:        l,
		backendConfig: *cfg,
		clock:         options.Clock,
		buf:           buf,
		clientContext: closeCtx,
		clientCancel:  cancel,
	}

	if len(cfg.EndPoint) == 0 {
		err = b.ensureIndexes(firestoreAdminClient)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// kicking off async tasks
	linearConfig := retryutils.LinearConfig{
		Step: b.RetryPeriod / 10,
		Max:  b.RetryPeriod,
	}
	go RetryingAsyncFunctionRunner(b.clientContext, linearConfig, b.logger.With("task_name", "watch_collection"), b.watchCollection)
	if !cfg.DisableExpiredDocumentPurge {
		go RetryingAsyncFunctionRunner(b.clientContext, linearConfig, b.logger.With("task_name", "purged_expired_documents"), b.purgeExpiredDocuments)
	}

	// Migrate incorrect key types to the correct type.
	// TODO(tigrato|rosstimothy): DELETE in 19.0.0
	go func() {
		migrationInterval := interval.New(interval.Config{
			Duration:      time.Hour * 12,
			FirstDuration: retryutils.FullJitter(time.Minute * 5),
			Jitter:        retryutils.SeventhJitter,
			Clock:         b.clock,
		})
		defer migrationInterval.Stop()
		for {
			select {
			case <-migrationInterval.Next():
				b.migrateIncorrectKeyTypes()
			case <-b.clientContext.Done():
				return
			}
		}
	}()

	l.InfoContext(b.clientContext, "Backend created.")

	return b, nil
}

func (b *Backend) GetName() string {
	return GetName()
}

// Create creates item if it does not exist
func (b *Backend) Create(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	item.Revision = createRevisionV2()
	r := newRecord(item, b.clock)
	_, err := b.svc.Collection(b.CollectionName).
		Doc(b.keyToDocumentID(item.Key)).
		Create(ctx, r)
	if err != nil {
		return nil, ConvertGRPCError(err)
	}
	return backend.NewLease(item), nil
}

// Put puts value into backend (creates if it does not exist, updates it otherwise)
func (b *Backend) Put(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	item.Revision = createRevisionV2()
	r := newRecord(item, b.clock)
	_, err := b.svc.Collection(b.CollectionName).
		Doc(b.keyToDocumentID(item.Key)).
		Set(ctx, r)
	if err != nil {
		return nil, ConvertGRPCError(err)
	}
	return backend.NewLease(item), nil
}

// Update updates value in the backend
func (b *Backend) Update(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	item.Revision = createRevisionV2()
	r := newRecord(item, b.clock)
	_, err := b.svc.Collection(b.CollectionName).
		Doc(b.keyToDocumentID(item.Key)).
		Update(ctx, r.updates())
	if err != nil {
		return nil, ConvertGRPCError(err)
	}
	return backend.NewLease(item), nil
}

func (b *Backend) getRangeDocs(ctx context.Context, startKey, endKey backend.Key, limit int) ([]*firestore.DocumentSnapshot, error) {
	if startKey.IsZero() {
		return nil, trace.BadParameter("missing parameter startKey")
	}
	if endKey.IsZero() {
		return nil, trace.BadParameter("missing parameter endKey")
	}
	if limit <= 0 {
		limit = backend.DefaultRangeLimit
	}
	docs, err := b.svc.Collection(b.CollectionName).
		Where(keyDocProperty, ">=", []byte(startKey.String())).
		Where(keyDocProperty, "<=", []byte(endKey.String())).
		Limit(limit).
		Documents(ctx).GetAll()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	legacyDocs, err := b.svc.Collection(b.CollectionName).
		Where(keyDocProperty, ">=", startKey.String()).
		Where(keyDocProperty, "<=", endKey.String()).
		Limit(limit).
		Documents(ctx).GetAll()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	brokenDocs, err := b.svc.Collection(b.CollectionName).
		Where(keyDocProperty, ">=", brokenKey(startKey.String())).
		Where(keyDocProperty, "<=", brokenKey(endKey.String())).
		Limit(limit).
		Documents(ctx).GetAll()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allDocs := append(append(docs, legacyDocs...), brokenDocs...)
	if len(allDocs) >= backend.DefaultRangeLimit {
		b.logger.WarnContext(ctx, "Range query hit backend limit. (this is a bug!)", "start_key", startKey, "limit", backend.DefaultRangeLimit)
	}
	return allDocs, nil
}

// GetRange returns range of elements
func (b *Backend) GetRange(ctx context.Context, startKey, endKey backend.Key, limit int) (*backend.GetResult, error) {
	docSnaps, err := b.getRangeDocs(ctx, startKey, endKey, limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var values []backend.Item
	for _, docSnap := range docSnaps {
		r, err := newRecordFromDoc(docSnap)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if r.isExpired(b.clock.Now()) {
			if _, err := docSnap.Ref.Delete(ctx, firestore.LastUpdateTime(docSnap.UpdateTime)); err != nil && status.Code(err) == codes.FailedPrecondition {
				// If the document has been updated, then attempt one additional get to see if the
				// resource was updated and is no longer expired.
				docSnap, err := b.svc.Collection(b.CollectionName).
					Doc(docSnap.Ref.ID).
					Get(ctx)
				if err != nil {
					return nil, ConvertGRPCError(err)
				}
				r, err := newRecordFromDoc(docSnap)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				if !r.isExpired(b.clock.Now()) {
					values = append(values, r.backendItem())
				}
			}
			// Do not include this document in the results.
			continue
		}

		values = append(values, r.backendItem())
	}
	return &backend.GetResult{Items: values}, nil
}

// DeleteRange deletes range of items with keys between startKey and endKey
func (b *Backend) DeleteRange(ctx context.Context, startKey, endKey backend.Key) error {
	docs, err := b.getRangeDocs(ctx, startKey, endKey, backend.DefaultRangeLimit)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(b.deleteDocuments(docs))
}

// Get returns a single item or not found error
func (b *Backend) Get(ctx context.Context, key backend.Key) (*backend.Item, error) {
	if key.IsZero() {
		return nil, trace.BadParameter("missing parameter key")
	}

	documentID := b.keyToDocumentID(key)
	docSnap, err := b.svc.Collection(b.CollectionName).
		Doc(documentID).
		Get(ctx)
	if err != nil {
		return nil, ConvertGRPCError(err)
	}
	r, err := newRecordFromDoc(docSnap)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if r.isExpired(b.clock.Now()) {
		if _, err := docSnap.Ref.Delete(ctx, firestore.LastUpdateTime(docSnap.UpdateTime)); err != nil && status.Code(err) == codes.FailedPrecondition {
			// If the document has been updated, then attempt one additional get to see if the
			// resource was updated and is no longer expired.
			docSnap, err := b.svc.Collection(b.CollectionName).
				Doc(documentID).
				Get(ctx)
			if err != nil {
				return nil, ConvertGRPCError(err)
			}
			r, err := newRecordFromDoc(docSnap)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			if !r.isExpired(b.clock.Now()) {
				bi := r.backendItem()
				return &bi, nil
			}
		}
		return nil, trace.NotFound("the supplied key: %q does not exist", key.String())
	}

	bi := r.backendItem()
	return &bi, nil
}

// CompareAndSwap compares the expected item with the existing item and replaces is with replaceWith
// if the contents of the two items match.
func (b *Backend) CompareAndSwap(ctx context.Context, expected backend.Item, replaceWith backend.Item) (*backend.Lease, error) {
	if expected.Key.IsZero() {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if replaceWith.Key.IsZero() {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if expected.Key.Compare(replaceWith.Key) != 0 {
		return nil, trace.BadParameter("expected and replaceWith keys should match")
	}

	replaceWith.Revision = createRevisionV2()
	replaceRec := newRecord(replaceWith, b.clock)

	docRef := b.svc.Collection(b.CollectionName).Doc(b.keyToDocumentID(replaceWith.Key))

	err := b.svc.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		docSnap, err := tx.Get(docRef)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return trace.CompareFailed("key %q was concurrently deleted", replaceWith.Key.String())
			}
			return trace.Wrap(ConvertGRPCError(err))
		}

		currentRec, err := newRecordFromDoc(docSnap)
		if err != nil {
			return trace.Wrap(err)
		}

		if !bytes.Equal(currentRec.Value, expected.Value) {
			return trace.CompareFailed("expected item value %v does not match actual item value %v", string(expected.Value), currentRec.Value)
		}

		if err := tx.Set(docRef, replaceRec); err != nil {
			return trace.Wrap(ConvertGRPCError(err))
		}

		return nil
	}, firestore.MaxAttempts(maxTxnAttempts))
	if err != nil {
		if status.Code(err) == codes.Aborted {
			// RunTransaction does not officially document what error is returned if MaxAttempts is exceeded,
			// but as currently implemented it should simply bubble up the Aborted error from the most recent
			// failed commit attempt.
			return nil, trace.Errorf("too many attempts during CompareAndSwap for key %q", replaceWith.Key)
		}

		return nil, trace.Wrap(ConvertGRPCError(err))
	}

	return backend.NewLease(replaceWith), nil
}

// Delete deletes item by key
func (b *Backend) Delete(ctx context.Context, key backend.Key) error {
	if key.IsZero() {
		return trace.BadParameter("missing parameter key")
	}

	docRef := b.svc.Collection(b.CollectionName).Doc(b.keyToDocumentID(key))
	if _, err := docRef.Delete(ctx, firestore.Exists); err != nil {
		if status.Code(err) == codes.NotFound {
			return trace.NotFound("key %q does not exist", key.String())
		}

		return ConvertGRPCError(err)
	}

	return nil
}

// ConditionalDelete deletes item by key if the revision matches
func (b *Backend) ConditionalDelete(ctx context.Context, key backend.Key, rev string) error {
	if !isRevisionV2(rev) {
		return b.legacyConditionalDelete(ctx, key, rev)
	}

	docRef := b.svc.Collection(b.CollectionName).Doc(b.keyToDocumentID(key))

	err := b.svc.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		docSnap, err := tx.Get(docRef)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return trace.Wrap(backend.ErrIncorrectRevision)
			}
			return trace.Wrap(ConvertGRPCError(err))
		}

		rec, err := newRecordFromDoc(docSnap)
		if err != nil {
			return trace.Wrap(err)
		}

		if rec.RevisionV2 != rev {
			return trace.Wrap(backend.ErrIncorrectRevision)
		}

		if err := tx.Delete(docRef); err != nil {
			return trace.Wrap(ConvertGRPCError(err))
		}

		return nil
	}, firestore.MaxAttempts(maxTxnAttempts))
	if err != nil {
		if status.Code(err) == codes.Aborted {
			// RunTransaction does not officially document what error is returned if MaxAttempts is exceeded,
			// but as currently implemented it should simply bubble up the Aborted error from the most recent
			// failed commit attempt.
			return trace.Errorf("too many attempts during ConditionalDelete for key %q", key)
		}

		return trace.Wrap(ConvertGRPCError(err))
	}

	return nil
}

func (b *Backend) legacyConditionalDelete(ctx context.Context, key backend.Key, rev string) error {
	revision, err := fromRevisionV1(rev)
	if err != nil {
		return trace.Wrap(backend.ErrIncorrectRevision)
	}

	if _, err := b.svc.Collection(b.CollectionName).
		Doc(b.keyToDocumentID(key)).
		Delete(ctx, firestore.LastUpdateTime(revision)); err != nil {
		if status.Code(err) != codes.FailedPrecondition {
			return trace.Wrap(ConvertGRPCError(err))
		}
		return trace.Wrap(backend.ErrIncorrectRevision)
	}

	return nil
}

func (b *Backend) ConditionalUpdate(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	if !isRevisionV2(item.Revision) {
		return b.legacyConditionalUpdate(ctx, item)
	}

	expectedRevision := item.Revision
	item.Revision = createRevisionV2()
	newRec := newRecord(item, b.clock)
	docRef := b.svc.Collection(b.CollectionName).Doc(b.keyToDocumentID(item.Key))

	err := b.svc.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		docSnap, err := tx.Get(docRef)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return trace.Wrap(backend.ErrIncorrectRevision)
			}
			return trace.Wrap(ConvertGRPCError(err))
		}

		existingRec, err := newRecordFromDoc(docSnap)
		if err != nil {
			return trace.Wrap(err)
		}

		if existingRec.RevisionV2 != expectedRevision {
			return trace.Wrap(backend.ErrIncorrectRevision)
		}

		if err := tx.Set(docRef, newRec); err != nil {
			return trace.Wrap(ConvertGRPCError(err))
		}

		return nil
	}, firestore.MaxAttempts(maxTxnAttempts))
	if err != nil {
		if status.Code(err) == codes.Aborted {
			// RunTransaction does not officially document what error is returned if MaxAttempts is exceeded,
			// but as currently implemented it should simply bubble up the Aborted error from the most recent
			// failed commit attempt.
			return nil, trace.Errorf("too many attempts during ConditionalUpdate for key %q", item.Key)
		}

		return nil, trace.Wrap(ConvertGRPCError(err))
	}

	return backend.NewLease(item), nil
}

func (b *Backend) legacyConditionalUpdate(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	revision, err := fromRevisionV1(item.Revision)
	if err != nil {
		return nil, trace.Wrap(backend.ErrIncorrectRevision)
	}

	r := newRecord(item, b.clock)
	res, err := b.svc.Collection(b.CollectionName).
		Doc(b.keyToDocumentID(item.Key)).
		Update(ctx, r.updates(), firestore.LastUpdateTime(revision))
	if err != nil {
		if status.Code(err) != codes.FailedPrecondition {
			return nil, trace.Wrap(ConvertGRPCError(err))
		}

		return nil, trace.Wrap(backend.ErrIncorrectRevision)
	}

	item.Revision = toRevisionV1(res.UpdateTime)
	return backend.NewLease(item), nil
}

// NewWatcher returns a new event watcher
func (b *Backend) NewWatcher(ctx context.Context, watch backend.Watch) (backend.Watcher, error) {
	return b.buf.NewWatcher(ctx, watch)
}

// KeepAlive keeps object from expiring, updates lease on the existing object,
// expires contains the new expiry to set on the lease,
// some backends may ignore expires based on the implementation
// in case if the lease managed server side
func (b *Backend) KeepAlive(ctx context.Context, lease backend.Lease, expires time.Time) error {
	if lease.Key.IsZero() {
		return trace.BadParameter("lease is missing key")
	}
	docSnap, err := b.svc.Collection(b.CollectionName).
		Doc(b.keyToDocumentID(lease.Key)).
		Get(ctx)
	if err != nil {
		return ConvertGRPCError(err)
	}

	if !docSnap.Exists() {
		return trace.NotFound("key %q does not exist, cannot extend lease", lease.Key.String())
	}

	r, err := newRecordFromDoc(docSnap)
	if err != nil {
		return trace.Wrap(err)
	}

	if r.isExpired(b.clock.Now()) {
		return trace.NotFound("key %q has already expired, cannot extend lease", lease.Key.String())
	}

	updates := []firestore.Update{
		{Path: expiresDocProperty, Value: expires.UTC().Unix()},
		{Path: timestampDocProperty, Value: b.clock.Now().UTC().Unix()},
		{Path: revisionDocProperty, Value: createRevisionV2()},
	}
	_, err = docSnap.Ref.Update(ctx, updates)
	if err != nil {
		return ConvertGRPCError(err)
	}
	return nil
}

// Close closes the Firestore client contexts and releases associated resources
func (b *Backend) Close() error {
	b.clientCancel()
	err := b.buf.Close()
	if err != nil {
		b.logger.ErrorContext(b.clientContext, "error closing buffer, continuing with closure of other resources...", "error", err)
	}
	return b.svc.Close()
}

// CloseWatchers closes all the watchers without closing the backend
func (b *Backend) CloseWatchers() {
	b.buf.Clear()
}

// Clock returns wall clock
func (b *Backend) Clock() clockwork.Clock {
	return b.clock
}

// keyToDocumentID converts key to a format supported by Firestore for document
// IDs. See
// https://firebase.google.com/docs/firestore/quotas#collections_documents_and_fields
// for Firestore limitations.
func (b *Backend) keyToDocumentID(key backend.Key) string {
	// URL-safe base64 will not have periods or forward slashes.
	// This should satisfy the Firestore requirements.
	return base64.URLEncoding.EncodeToString([]byte(key.String()))
}

// RetryingAsyncFunctionRunner wraps a task target in retry logic
func RetryingAsyncFunctionRunner(ctx context.Context, retryConfig retryutils.LinearConfig, logger *slog.Logger, task func() error) {
	retry, err := retryutils.NewLinear(retryConfig)
	if err != nil {
		logger.ErrorContext(ctx, "Bad retry parameters, returning and not running.", "error", err)
		return
	}

	defer logger.DebugContext(ctx, "Returning from task loop.")

	for {
		err := task()

		if isCanceled(err) {
			return
		} else if err != nil {
			logger.ErrorContext(ctx, "Task %v has returned with error", "error", err)
		}

		logger.DebugContext(ctx, "Reloading task", "retry", retry.Duration())
		select {
		case <-retry.After():
			retry.Inc()

		case <-ctx.Done():
			return
		}
	}
}

func isCanceled(err error) bool {
	switch {
	case err == nil:
		return false

	case errors.Is(err, context.Canceled):
		return true

	case status.Code(err) == codes.Canceled:
		return true

	default:
		return false
	}
}

// driftTolerance is the amount of clock drift between auth servers that we
// will be resilient to.  Clock drift greater than this amount may result
// in cache inconsistencies due to missing events which aught to have a "happens after"
// relationship to associated reads.  This is because the firestore event stream
// starts at a timestamp field that is defined by the auth server.  If a different
// auth server is lagging behind us, it may modify a document after we established our
// listener, but we will miss the event because it used an old timestamp.  We combat this
// issue by starting our query slightly in the past.  If an auth server writes a document
// and is lagging less than driftTolerance, subscribing caches will be correctly updated.
// This has the unfortunate side-effect of potentially emitting old events, but this is OK
// (if somewhat confusing).  All caching logic assumes that it may see some events which
// happened before it's reads completed.  Missing an event that happened after is what
// can lead to permanently bad cache state.
const driftTolerance = time.Millisecond * 2500

// watchCollection watches a firestore collection for changes and pushes those changes, events into the buffer for watchers
func (b *Backend) watchCollection() error {
	// Filter any documents that don't have a key. If the collection is shared between
	// the cluster state and audit events, this filters out the event documents since they
	// have a different schema, and it's a requirement for all resources to have a key.
	query := b.svc.Collection(b.CollectionName).Where(keyDocProperty, "!=", "")
	if b.LimitWatchQuery {
		query = query.Where(timestampDocProperty, ">=", b.clock.Now().UTC().Add(-driftTolerance).Unix())
	}

	snaps := query.Snapshots(b.clientContext)
	b.buf.SetInit()
	defer b.buf.Reset()
	defer snaps.Stop()

	for {
		querySnap, err := snaps.Next()
		if errors.Is(err, context.Canceled) {
			return nil
		} else if err != nil {
			return ConvertGRPCError(err)
		}
		for _, change := range querySnap.Changes {
			r, err := newRecordFromDoc(change.Doc)
			if err != nil {
				return trace.Wrap(err)
			}
			var e backend.Event
			switch change.Kind {
			case firestore.DocumentAdded, firestore.DocumentModified:
				e = backend.Event{
					Type: types.OpPut,
					Item: r.backendItem(),
				}
			case firestore.DocumentRemoved:
				e = backend.Event{
					Type: types.OpDelete,
					Item: backend.Item{
						Key: backend.KeyFromString(string(r.Key)),
					},
				}
			}
			b.buf.Emit(e)
		}
	}
}

// purgeExpiredDocuments ticks on configured interval and removes expired documents from firestore
func (b *Backend) purgeExpiredDocuments() error {
	t := time.NewTicker(b.PurgeExpiredDocumentsPollInterval)
	defer t.Stop()
	for {
		select {
		case <-b.clientContext.Done():
			return b.clientContext.Err()
		case <-t.C:
			expiryTime := b.clock.Now().UTC().Unix()
			// Find all documents that have expired, but EXCLUDE
			// any documents that do not have an expiry as indicated
			// by a value of 0.
			docs, err := b.svc.Collection(b.CollectionName).
				Where(expiresDocProperty, "<=", expiryTime).
				Where(expiresDocProperty, ">", 0).
				Documents(b.clientContext).
				GetAll()
			if err != nil {
				b.logger.WarnContext(b.clientContext, "Failed to get expired documents", "error", trail.FromGRPC(err))
				continue
			}

			if len(docs) == 0 {
				continue
			}

			if err := b.deleteDocuments(docs); err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

// deleteDocuments removes documents from firestore in batches to stay within the
// firestore write limits
func (b *Backend) deleteDocuments(docs []*firestore.DocumentSnapshot) error {
	seen := make(map[string]struct{}, len(docs))
	batch := b.svc.BulkWriter(b.clientContext)
	jobs := make([]*firestore.BulkWriterJob, 0, len(docs))

	for _, doc := range docs {
		// Deduplicate documents. The Firestore SDK will error if duplicates are found,
		// but existing callers of this function assume this is valid.
		if _, ok := seen[doc.Ref.Path]; ok {
			continue
		}
		seen[doc.Ref.Path] = struct{}{}

		job, err := batch.Delete(doc.Ref)
		if err != nil {
			return ConvertGRPCError(err)
		}

		jobs = append(jobs, job)
	}

	batch.End()
	var errs []error
	for _, job := range jobs {
		if _, err := job.Results(); err != nil {
			errs = append(errs, ConvertGRPCError(err))
		}
	}

	return trace.NewAggregate(errs...)
}

// ConvertGRPCError converts gRPC errors
func ConvertGRPCError(err error) error {
	if err == nil {
		return nil
	}
	switch status.Code(err) {
	case codes.Canceled:
		return context.Canceled
	case codes.DeadlineExceeded:
		return context.DeadlineExceeded
	case codes.FailedPrecondition:
		return trace.BadParameter("%s", err)
	case codes.NotFound:
		return trace.NotFound("%s", err)
	case codes.AlreadyExists:
		return trace.AlreadyExists("%s", err)
	case codes.OK:
		return nil
	default:
		return trace.Wrap(err)
	}
}

func (b *Backend) getIndexParent() string {
	database := cmp.Or(b.backendConfig.Config.DatabaseID, "(default)")
	return "projects/" + b.ProjectID + "/databases/" + database + "/collectionGroups/" + b.CollectionName
}

func (b *Backend) ensureIndexes(adminSvc *apiv1.FirestoreAdminClient) error {
	tuples := IndexList{}
	tuples.Index(Field(keyDocProperty, adminpb.Index_IndexField_ASCENDING), Field(expiresDocProperty, adminpb.Index_IndexField_ASCENDING))
	return EnsureIndexes(b.clientContext, adminSvc, b.logger, tuples, b.getIndexParent())
}

type IndexList [][]*adminpb.Index_IndexField

func (l *IndexList) Index(fields ...*adminpb.Index_IndexField) {
	list := []*adminpb.Index_IndexField{}
	list = append(list, fields...)
	*l = append(*l, list)
}

func Field(name string, order adminpb.Index_IndexField_Order) *adminpb.Index_IndexField {
	return &adminpb.Index_IndexField{
		FieldPath: name,
		ValueMode: &adminpb.Index_IndexField_Order_{
			Order: order,
		},
	}
}

type indexTask struct {
	operation *apiv1.CreateIndexOperation
	tuple     []*adminpb.Index_IndexField
}

// EnsureIndexes is a function used by Firestore events and backend to generate indexes and will block until
// indexes are reported as created
func EnsureIndexes(ctx context.Context, adminSvc *apiv1.FirestoreAdminClient, logger *slog.Logger, tuples IndexList, indexParent string) error {
	var tasks []indexTask

	// create the indexes
	for _, tuple := range tuples {
		operation, err := adminSvc.CreateIndex(ctx, &adminpb.CreateIndexRequest{
			Parent: indexParent,
			Index: &adminpb.Index{
				QueryScope: adminpb.Index_COLLECTION,
				Fields:     tuple,
			},
		})
		if err != nil && status.Code(err) != codes.AlreadyExists {
			return ConvertGRPCError(err)
		}
		// operation can be nil if error code is codes.AlreadyExists.
		if operation != nil {
			tasks = append(tasks, indexTask{operation, tuple})
		}
	}

	stop := periodIndexUpdate(logger)
	for _, task := range tasks {
		err := waitOnIndexCreation(ctx, logger, task)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	stop <- struct{}{}

	return nil
}

func periodIndexUpdate(l *slog.Logger) chan struct{} {
	ticker := time.NewTicker(timeInBetweenIndexCreationStatusChecks)
	quit := make(chan struct{})
	start := time.Now()
	go func() {
		for {
			select {
			case <-ticker.C:
				elapsed := time.Since(start)
				l.InfoContext(context.Background(), "Still creating indexes", "time_elapsed", elapsed)
			case <-quit:
				l.InfoContext(context.Background(), "Finished creating indexes")
				ticker.Stop()
				return
			}
		}
	}()
	return quit
}

func waitOnIndexCreation(ctx context.Context, l *slog.Logger, task indexTask) error {
	meta, err := task.operation.Metadata()
	if err != nil {
		return trace.Wrap(err)
	}
	l.InfoContext(ctx, "Creating index for tuple.", "tuple", task.tuple, "name", meta.Index)

	_, err = task.operation.Wait(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// revisionV2Prefix uniquely identifies version 2 firestore revision values. Older firestore documents
// use the update time as their revision. Newer documents use a random string value as their revision.
const revisionV2Prefix = "f2:"

func createRevisionV2() string {
	return revisionV2Prefix + backend.CreateRevision()
}

func isRevisionV2(r string) bool {
	return strings.HasPrefix(r, revisionV2Prefix)
}

func toRevisionV1(rev time.Time) string {
	return strconv.FormatInt(rev.UnixNano(), 10)
}

func fromRevisionV1(rev string) (time.Time, error) {
	n, err := strconv.ParseInt(rev, 10, 64)
	if err != nil {
		return time.Time{}, trace.BadParameter("invalid revision: %s", err)
	}

	return time.Unix(0, n), nil
}
