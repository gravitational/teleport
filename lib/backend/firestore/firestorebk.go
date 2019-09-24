/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

package firestore

import (
	"bytes"
	"context"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	apiv1 "cloud.google.com/go/firestore/apiv1/admin"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"google.golang.org/api/option"
	adminpb "google.golang.org/genproto/googleapis/firestore/admin/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// FirestoreConfig structure represents Firestore configuration as appears in `storage` section of Teleport YAML
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
		cfg.BufferSize = backend.DefaultBufferSize
	}
	if cfg.PurgeExpiredDocumentsPollInterval == 0 {
		cfg.PurgeExpiredDocumentsPollInterval = defaultPurgeInterval
	}
	if cfg.RetryPeriod == 0 {
		cfg.RetryPeriod = defaults.HighResPollingPeriod
	}
	return nil
}

// FirestoreBackend is a Firestore-backed key value backend implementation.
type FirestoreBackend struct {
	*log.Entry
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
	// watchStarted context tracking once firestore watchers mechanisms are up
	watchStarted context.Context
	// signalWatchStart signal function which indicates watcher mechanisms are up
	signalWatchStart context.CancelFunc
}

type record struct {
	Key       string `firestore:"key,omitempty"`
	Timestamp int64  `firestore:"timestamp,omitempty"`
	Expires   int64  `firestore:"expires,omitempty"`
	ID        int64  `firestore:"id,omitempty"`
	Value     string `firestore:"value,omitempty"`
}

// isExpired returns 'true' if the given object (record) has a TTL and it's due
func (r *record) isExpired() bool {
	if r.Expires == 0 {
		return false
	}
	expiryDateUTC := time.Unix(r.Expires, 0).UTC()
	return time.Now().UTC().After(expiryDateUTC)
}

const (
	// BackendName is the name of this backend
	BackendName = "firestore"
	// defaultPurgeInterval is the interval for the ticker that executes the expired record query and cleanup
	defaultPurgeInterval = time.Minute
	// keyDocProperty is used internally to query for records and matches the key in the record struct tag
	keyDocProperty = "key"
	// expiresDocProperty is used internally to query for records and matches the key in the record struct tag
	expiresDocProperty = "expires"
	// timestampDocProperty is used internally to query for records and matches the key in the record struct tag
	timestampDocProperty = "timestamp"
	// timeInBetweenIndexCreationStatusChecks
	timeInBetweenIndexCreationStatusChecks = time.Second * 10
	// documentNameIllegalCharacter the character key search criteria for normal document replacement
	documentNameIllegalCharacter = "/"
	// documentNameReplacementCharacter the replacement path separator for firestore records
	documentNameReplacementCharacter = "\\"
	// documentNameLockIllegalCharacter the character key search criteria for lock replacement
	documentNameLockIllegalCharacter = "."
	// documentNameLockReplacementCharacter the replacement key prefix for lock values
	documentNameLockReplacementCharacter = ""
)

// GetName is a part of backend API and it returns Firestore backend type
// as it appears in `storage/type` section of Teleport YAML
func GetName() string {
	return BackendName
}

// keep this here to test interface conformance
var _ backend.Backend = &FirestoreBackend{}

// CreateFirestoreClients creates a firestore admin and normal client given the supplied parameters
func CreateFirestoreClients(ctx context.Context, projectID string, endPoint string, credentialsFile string) (*apiv1.FirestoreAdminClient, *firestore.Client, error) {

	var args []option.ClientOption

	if len(endPoint) != 0 {
		args = append(args, option.WithoutAuthentication(), option.WithEndpoint(endPoint), option.WithGRPCDialOption(grpc.WithInsecure()))
	} else if len(credentialsFile) != 0 {
		args = append(args, option.WithCredentialsFile(credentialsFile))
	}

	firestoreClient, err := firestore.NewClient(ctx, projectID, args...)
	if err != nil {
		return nil, nil, ConvertGRPCError(err)
	}
	firestoreAdminClient, err := apiv1.NewFirestoreAdminClient(ctx, args...)
	if err != nil {
		return nil, nil, ConvertGRPCError(err)
	}

	return firestoreAdminClient, firestoreClient, nil
}

// New returns new instance of Firestore backend.
// It's an implementation of backend API's NewFunc
func New(ctx context.Context, params backend.Params) (*FirestoreBackend, error) {
	l := log.WithFields(log.Fields{trace.Component: BackendName})
	var cfg *backendConfig
	err := utils.ObjectToStruct(params, &cfg)
	if err != nil {
		return nil, trace.BadParameter("firestore: configuration is invalid: %v", err)
	}
	l.Infof("Firestore: initializing backend.")
	defer l.Debug("Firestore: backend created.")
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	closeCtx, cancel := context.WithCancel(ctx)
	firestoreAdminClient, firestoreClient, err := CreateFirestoreClients(closeCtx, cfg.ProjectID, cfg.EndPoint, cfg.CredentialsPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	buf, err := backend.NewCircularBuffer(ctx, cfg.BufferSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	watchStarted, signalWatchStart := context.WithCancel(ctx)
	b := &FirestoreBackend{
		svc:              firestoreClient,
		Entry:            l,
		backendConfig:    *cfg,
		clock:            clockwork.NewRealClock(),
		buf:              buf,
		clientContext:    closeCtx,
		clientCancel:     cancel,
		watchStarted:     watchStarted,
		signalWatchStart: signalWatchStart,
	}
	if len(cfg.EndPoint) == 0 {
		err = b.ensureIndexes(firestoreAdminClient)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// kicking off async tasks
	linearConfig := utils.LinearConfig{
		Step: b.RetryPeriod / 10,
		Max:  b.RetryPeriod,
	}
	go RetryingAsyncFunctionRunner(b.clientContext, linearConfig, b.Logger, b.watchCollection, "watchCollection")
	if !cfg.DisableExpiredDocumentPurge {
		go RetryingAsyncFunctionRunner(b.clientContext, linearConfig, b.Logger, b.purgeExpiredDocuments, "purgeExpiredDocuments")
	}
	return b, nil
}

// Create creates item if it does not exist
func (b *FirestoreBackend) Create(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	var r record
	r.Key = string(item.Key)
	r.Value = string(item.Value)
	r.Timestamp = b.clock.Now().UTC().Unix()
	r.ID = b.clock.Now().UTC().UnixNano()
	if !item.Expires.IsZero() {
		r.Expires = item.Expires.UTC().Unix()
	}
	_, err := b.svc.Collection(b.CollectionName).Doc(b.convertKeyToSupportedDocumentID(item.Key)).Create(ctx, r)
	if err != nil {
		return nil, ConvertGRPCError(err)
	} else {
		return b.newLease(item), nil
	}
}

// Put puts value into backend (creates if it does not exists, updates it otherwise)
func (b *FirestoreBackend) Put(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	var r record
	r.Key = string(item.Key)
	r.Value = string(item.Value)
	r.Timestamp = b.clock.Now().UTC().Unix()
	r.ID = b.clock.Now().UTC().UnixNano()
	if !item.Expires.IsZero() {
		r.Expires = item.Expires.UTC().Unix()
	}
	_, err := b.svc.Collection(b.CollectionName).Doc(b.convertKeyToSupportedDocumentID(item.Key)).Set(ctx, r)
	if err != nil {
		return nil, ConvertGRPCError(err)
	} else {
		return b.newLease(item), nil
	}
}

// Update updates value in the backend
func (b *FirestoreBackend) Update(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	var r record
	r.Key = string(item.Key)
	r.Value = string(item.Value)
	r.Timestamp = b.clock.Now().UTC().Unix()
	r.ID = b.clock.Now().UTC().UnixNano()
	if !item.Expires.IsZero() {
		r.Expires = item.Expires.UTC().Unix()
	}
	_, err := b.svc.Collection(b.CollectionName).Doc(b.convertKeyToSupportedDocumentID(item.Key)).Get(ctx)
	if err != nil {
		return nil, ConvertGRPCError(err)
	}
	_, err = b.svc.Collection(b.CollectionName).Doc(b.convertKeyToSupportedDocumentID(item.Key)).Set(ctx, r)
	if err != nil {
		return nil, ConvertGRPCError(err)
	} else {
		return b.newLease(item), nil
	}
}

func (b *FirestoreBackend) getRangeDocs(ctx context.Context, startKey []byte, endKey []byte, limit int) ([]*firestore.DocumentSnapshot, error) {
	if len(startKey) == 0 {
		return nil, trace.BadParameter("missing parameter startKey")
	}
	if len(endKey) == 0 {
		return nil, trace.BadParameter("missing parameter endKey")
	}
	if limit <= 0 {
		limit = backend.DefaultLargeLimit
	}
	docSnaps, _ := b.svc.Collection(b.CollectionName).Where(keyDocProperty, ">=", string(startKey)).
		Where(keyDocProperty, "<=", string(endKey)).Limit(limit).Documents(ctx).GetAll()

	return docSnaps, nil
}

// GetRange returns range of elements
func (b *FirestoreBackend) GetRange(ctx context.Context, startKey []byte, endKey []byte, limit int) (*backend.GetResult, error) {
	docSnaps, err := b.getRangeDocs(ctx, startKey, endKey, limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	values := make([]backend.Item, 0)
	for _, docSnap := range docSnaps {
		var r record
		err = docSnap.DataTo(&r)
		if err != nil {
			return nil, ConvertGRPCError(err)
		}

		if r.isExpired() {
			err = b.Delete(ctx, []byte(r.Key))
			if err != nil {
				return nil, ConvertGRPCError(err)
			}
		}

		values = append(values, backend.Item{
			Key:   []byte(r.Key),
			Value: []byte(r.Value),
		})
	}
	return &backend.GetResult{Items: values}, nil
}

// DeleteRange deletes range of items with keys between startKey and endKey
func (b *FirestoreBackend) DeleteRange(ctx context.Context, startKey, endKey []byte) error {
	docSnaps, err := b.getRangeDocs(ctx, startKey, endKey, backend.DefaultLargeLimit)
	if err != nil {
		return trace.Wrap(err)
	}
	batch := b.svc.Batch()
	numDeleted := 0
	for _, docSnap := range docSnaps {
		batch.Delete(docSnap.Ref)
		numDeleted++
	}
	_, err = batch.Commit(ctx)
	if numDeleted > 0 {
		if err != nil {
			return ConvertGRPCError(err)
		}
	}
	return nil
}

// Get returns a single item or not found error
func (b *FirestoreBackend) Get(ctx context.Context, key []byte) (*backend.Item, error) {
	if len(key) == 0 {
		return nil, trace.BadParameter("missing parameter key")
	}
	docSnap, err := b.svc.Collection(b.CollectionName).Doc(b.convertKeyToSupportedDocumentID(key)).Get(ctx)
	if err != nil {
		return nil, ConvertGRPCError(err)
	}
	var r record
	err = docSnap.DataTo(&r)
	if err != nil {
		return nil, ConvertGRPCError(err)
	}

	if r.isExpired() {
		err = b.Delete(ctx, key)
		if err != nil {
			return nil, ConvertGRPCError(err)
		} else {
			return nil, trace.NotFound("the supplied key: `%v` does not exist", string(key))
		}
	}

	item := &backend.Item{
		Key:   []byte(r.Key),
		Value: []byte(r.Value),
		ID:    r.ID,
	}
	if r.Expires != 0 {
		item.Expires = time.Unix(r.Expires, 0)
	}
	return item, nil
}

// CompareAndSwap compares and swap values in atomic operation
// CompareAndSwap compares item with existing item
// and replaces is with replaceWith item
func (b *FirestoreBackend) CompareAndSwap(ctx context.Context, expected backend.Item, replaceWith backend.Item) (*backend.Lease, error) {
	if len(expected.Key) == 0 {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if len(replaceWith.Key) == 0 {
		return nil, trace.BadParameter("missing parameter Key")
	}
	if bytes.Compare(expected.Key, replaceWith.Key) != 0 {
		return nil, trace.BadParameter("expected and replaceWith keys should match")
	}

	expectedDocSnap, err := b.svc.Collection(b.CollectionName).Doc(b.convertKeyToSupportedDocumentID(expected.Key)).Get(ctx)
	if err != nil {
		return nil, trace.CompareFailed("error or object not found, error: %v", ConvertGRPCError(err))
	}

	existingRecord := record{}
	err = expectedDocSnap.DataTo(&existingRecord)
	if err != nil {
		return nil, ConvertGRPCError(err)
	}

	if existingRecord.Value != string(expected.Value) {
		return nil, trace.CompareFailed("expected item value %v does not match actual item value %v", string(expected.Value), existingRecord.Value)
	}

	r := record{
		Key:       string(replaceWith.Key),
		Value:     string(replaceWith.Value),
		Timestamp: b.clock.Now().UTC().Unix(),
		ID:        b.clock.Now().UTC().UnixNano(),
	}
	if !replaceWith.Expires.IsZero() {
		r.Expires = replaceWith.Expires.UTC().Unix()
	}

	_, err = expectedDocSnap.Ref.Set(ctx, r)
	if err != nil {
		return nil, ConvertGRPCError(err)
	} else {
		return b.newLease(replaceWith), nil
	}
}

// Delete deletes item by key
func (b *FirestoreBackend) Delete(ctx context.Context, key []byte) error {
	if len(key) == 0 {
		return trace.BadParameter("missing parameter key")
	}
	docRef := b.svc.Collection(b.CollectionName).Doc(b.convertKeyToSupportedDocumentID(key))
	doc, _ := docRef.Get(ctx)

	if !doc.Exists() {
		return trace.NotFound("key %s does not exist", string(key))
	}
	_, err := docRef.Delete(ctx)

	if err != nil {
		return ConvertGRPCError(err)
	}
	return nil
}

// NewWatcher returns a new event watcher
func (b *FirestoreBackend) NewWatcher(ctx context.Context, watch backend.Watch) (backend.Watcher, error) {
	select {
	case <-b.watchStarted.Done():
	case <-ctx.Done():
		return nil, trace.ConnectionProblem(ctx.Err(), "context is closing")
	}
	return b.buf.NewWatcher(ctx, watch)
}

// KeepAlive keeps object from expiring, updates lease on the existing object,
// expires contains the new expiry to set on the lease,
// some backends may ignore expires based on the implementation
// in case if the lease managed server side
func (b *FirestoreBackend) KeepAlive(ctx context.Context, lease backend.Lease, expires time.Time) error {
	if len(lease.Key) == 0 {
		return trace.BadParameter("lease is missing key")
	}
	docSnap, err := b.svc.Collection(b.CollectionName).Doc(b.convertKeyToSupportedDocumentID(lease.Key)).Get(ctx)
	if err != nil {
		return ConvertGRPCError(err)
	}

	if !docSnap.Exists() {
		return trace.NotFound("key %s does not exist, cannot extend lease", lease.Key)
	}

	var r record
	err = docSnap.DataTo(&r)
	if err != nil {
		return ConvertGRPCError(err)
	}

	if r.isExpired() {
		return trace.NotFound("key %s has already expired, cannot extend lease", lease.Key)
	}

	updates := []firestore.Update{{Path: expiresDocProperty, Value: expires.UTC().Unix()}, {Path: timestampDocProperty, Value: b.clock.Now().UTC().Unix()}}
	_, err = docSnap.Ref.Update(ctx, updates)
	if err != nil {
		return ConvertGRPCError(err)
	}
	return err
}

// Close closes the Firestore client contexts and releases associated resources
func (b *FirestoreBackend) Close() error {
	b.clientCancel()
	err := b.buf.Close()
	if err != nil {
		b.Logger.Error("error closing buffer, continuing with closure of other resources...", err)
	}
	return b.svc.Close()
}

// CloseWatchers closes all the watchers without closing the backend
func (b *FirestoreBackend) CloseWatchers() {
	b.buf.Reset()
}

// Clock returns wall clock
func (b *FirestoreBackend) Clock() clockwork.Clock {
	return b.clock
}

func (b *FirestoreBackend) newLease(item backend.Item) *backend.Lease {
	var lease backend.Lease
	if item.Expires.IsZero() {
		return &lease
	}
	lease.Key = item.Key
	return &lease
}

// convertKeyToSupportedDocumentID converts the key for the stored member to one supported by Firestore
func (b *FirestoreBackend) convertKeyToSupportedDocumentID(key []byte) string {
	return strings.Replace(strings.Replace(string(key), documentNameLockIllegalCharacter, documentNameLockReplacementCharacter, 1),
		documentNameIllegalCharacter, documentNameReplacementCharacter, -1)
}

// RetryingAsyncFunctionRunner wraps a task target in retry logic
func RetryingAsyncFunctionRunner(ctx context.Context, retryConfig utils.LinearConfig, logger *log.Logger, task func() error, taskName string) {
	retry, err := utils.NewLinear(retryConfig)
	if err != nil {
		logger.Errorf("bad retry parameters: %v, returning and not running", err)
		return
	}
	for {
		err := task()
		if err != nil && (err != context.Canceled || status.Convert(err).Code() != codes.Canceled) {
			logger.Errorf("%v returned with error: %v", taskName, err)
		}
		logger.Debugf("Reloading %v for %s.", retry, taskName)
		select {
		case <-retry.After():
			retry.Inc()
		case <-ctx.Done():
			logger.Debugf("Returning from %v loop.", taskName)
			return
		}
	}
}

// watchCollection watches a firestore collection for changes and pushes those changes, events into the buffer for watchers
func (b *FirestoreBackend) watchCollection() error {
	var snaps *firestore.QuerySnapshotIterator
	if b.LimitWatchQuery {
		snaps = b.svc.Collection(b.CollectionName).Query.Where(timestampDocProperty, ">=", b.clock.Now().UTC().Unix()).Snapshots(b.clientContext)
	} else {
		snaps = b.svc.Collection(b.CollectionName).Snapshots(b.clientContext)
	}
	b.signalWatchStart()
	defer snaps.Stop()
	for {
		querySnap, err := snaps.Next()
		if err == context.Canceled {
			return nil
		} else if err != nil {
			return ConvertGRPCError(err)
		}
		for _, change := range querySnap.Changes {
			var r record
			err = change.Doc.DataTo(&r)
			if err != nil {
				return ConvertGRPCError(err)
			}
			var expires time.Time
			if r.Expires != 0 {
				expires = time.Unix(r.Expires, 0)
			}
			var e backend.Event
			switch change.Kind {
			case firestore.DocumentAdded:
				fallthrough
			case firestore.DocumentModified:
				e = backend.Event{
					Type: backend.OpPut,
					Item: backend.Item{
						Key:     []byte(r.Key),
						Value:   []byte(r.Value),
						Expires: expires,
						ID:      r.ID,
					},
				}
			case firestore.DocumentRemoved:
				e = backend.Event{
					Type: backend.OpDelete,
					Item: backend.Item{
						Key: []byte(r.Key),
					},
				}
			}
			b.Logger.Debugf("pushing event %v for key '%v'.", e.Type.String(), r.Key)
			b.buf.Push(e)
		}
	}
}

// purgeExpiredDocuments ticks on configured interval and removes expired documents from firestore
func (b *FirestoreBackend) purgeExpiredDocuments() error {
	t := time.NewTicker(b.PurgeExpiredDocumentsPollInterval)
	defer t.Stop()
	for {
		select {
		case <-b.clientContext.Done():
			return nil
		case <-t.C:
			expiryTime := b.clock.Now().UTC().Unix()
			numDeleted := 0
			batch := b.svc.Batch()
			docs, _ := b.svc.Collection(b.CollectionName).Where(expiresDocProperty, "<=", expiryTime).Documents(b.clientContext).GetAll()
			for _, doc := range docs {
				batch.Delete(doc.Ref)
				numDeleted++
			}
			if numDeleted > 0 {
				_, err := batch.Commit(b.clientContext)
				if err != nil {
					return ConvertGRPCError(err)
				}
			}
		}
	}
}

// ConvertGRPCError converts GRPC errors
func ConvertGRPCError(err error, args ...interface{}) error {
	if err == nil {
		return nil
	}
	status, ok := status.FromError(err)
	if !ok {
		return trace.Errorf("Unable to convert error to GRPC status code, error: %s", err)
	}
	switch status.Code() {
	case codes.FailedPrecondition:
		return trace.BadParameter(err.Error(), args...)
	case codes.NotFound:
		return trace.NotFound(err.Error(), args...)
	case codes.AlreadyExists:
		return trace.AlreadyExists(err.Error(), args...)
	case codes.OK:
		return nil
	default:
		return trace.Wrap(err, args...)
	}
}

func (b *FirestoreBackend) getIndexParent() string {
	return "projects/" + b.ProjectID + "/databases/(default)/collectionGroups/" + b.CollectionName
}

// deleteAllItems deletes all items from the database, used in tests
func (b *FirestoreBackend) deleteAllItems() {
	DeleteAllDocuments(b.clientContext, b.svc, b.CollectionName)
}

// DeleteAllDocuments will delete all documents in a collection.
func DeleteAllDocuments(ctx context.Context, svc *firestore.Client, collectionName string) {
	docs, _ := svc.Collection(collectionName).Documents(ctx).GetAll()
	batch := svc.Batch()
	for _, snap := range docs {
		batch.Delete(snap.Ref)
	}
	_, _ = batch.Commit(ctx)
}

func (b *FirestoreBackend) ensureIndexes(adminSvc *apiv1.FirestoreAdminClient) error {
	defer adminSvc.Close()
	tuples := make([]*IndexTuple, 0)
	tuples = append(tuples, &IndexTuple{
		FirstField:  keyDocProperty,
		SecondField: expiresDocProperty,
	})
	err := EnsureIndexes(b.clientContext, adminSvc, tuples, b.getIndexParent())
	return err
}

type IndexTuple struct {
	FirstField  string
	SecondField string
}

// EnsureIndexes is a function used by Firestore events and backend to generate indexes and will block until
// indexes are reported as created
func EnsureIndexes(ctx context.Context, adminSvc *apiv1.FirestoreAdminClient, tuples []*IndexTuple, indexParent string) error {
	l := log.WithFields(log.Fields{trace.Component: BackendName})

	ascendingFieldOrder := adminpb.Index_IndexField_Order_{
		Order: adminpb.Index_IndexField_ASCENDING,
	}

	tuplesToIndexNames := make(map[*IndexTuple]string)
	// create the indexes
	for _, tuple := range tuples {
		fields := make([]*adminpb.Index_IndexField, 0)
		fields = append(fields, &adminpb.Index_IndexField{
			FieldPath: tuple.FirstField,
			ValueMode: &ascendingFieldOrder,
		})
		fields = append(fields, &adminpb.Index_IndexField{
			FieldPath: tuple.SecondField,
			ValueMode: &ascendingFieldOrder,
		})
		operation, err := adminSvc.CreateIndex(ctx, &adminpb.CreateIndexRequest{
			Parent: indexParent,
			Index: &adminpb.Index{
				QueryScope: adminpb.Index_COLLECTION,
				Fields:     fields,
			},
		})
		if err != nil && status.Convert(err).Code() != codes.AlreadyExists {
			l.Debug("non-already exists error, returning.")
			return ConvertGRPCError(err)
		}
		if operation != nil {
			meta := adminpb.IndexOperationMetadata{}
			_ = meta.XXX_Unmarshal(operation.Metadata.Value)
			tuplesToIndexNames[tuple] = meta.Index
		}
	}

	// check for statuses and block
	for {
		if len(tuplesToIndexNames) == 0 {
			break
		}
		time.Sleep(timeInBetweenIndexCreationStatusChecks)
		for tuple, name := range tuplesToIndexNames {
			index, _ := adminSvc.GetIndex(ctx, &adminpb.GetIndexRequest{Name: name})
			l.Infof("Index for tuple %s-%s, %s, state is %s.", tuple.FirstField, tuple.SecondField, index.Name, index.State.String())
			if index.State == adminpb.Index_READY {
				delete(tuplesToIndexNames, tuple)
			}
		}
	}

	return nil
}
