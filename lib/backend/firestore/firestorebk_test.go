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
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/admin/adminpb"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// TestMarshal tests index operation metadata marshal and unmarshal
// to verify backwards compatibility. Gogoproto is incompatible with ApiV2 protoc-gen-go code.
//
// Track the issue here: https://github.com/gogo/protobuf/issues/678
func TestMarshal(t *testing.T) {
	meta := adminpb.IndexOperationMetadata{}
	data, err := proto.Marshal(&meta)
	require.NoError(t, err)
	out := adminpb.IndexOperationMetadata{}
	err = proto.Unmarshal(data, &out)
	require.NoError(t, err)
}

func firestoreParams() backend.Params {
	// Creating the indices on - even an empty - live Firestore collection
	// can take 5 minutes, so we re-use the same project and collection
	// names for each test.
	collection := "tp-cluster-data-test"
	projectID := "tp-testproj"
	endpoint := ""

	if c := os.Getenv("TELEPORT_FIRESTORE_TEST_COLLECTION"); c != "" {
		collection = c
	}

	if p := os.Getenv("TELEPORT_FIRESTORE_TEST_PROJECT"); p != "" {
		projectID = p
	}

	if e := os.Getenv("TELEPORT_FIRESTORE_TEST_ENDPOINT"); e != "" {
		endpoint = e
	}

	return map[string]any{
		"collection_name":                       collection,
		"project_id":                            projectID,
		"endpoint":                              endpoint,
		"purge_expired_documents_poll_interval": 300 * time.Millisecond,
	}
}

func ensureTestsEnabled(t *testing.T) {
	const varName = "TELEPORT_FIRESTORE_TEST"
	if os.Getenv(varName) == "" {
		t.Skipf("Firestore tests are disabled. Enable by defining the %v environment variable", varName)
	}
}

func ensureEmulatorRunning(t *testing.T, cfg map[string]any) {
	endpoint, _ := cfg["endpoint"].(string)
	if endpoint == "" {
		return
	}

	con, err := net.Dial("tcp", endpoint)
	if err != nil {
		t.Skip("Firestore emulator is not running, start it with: gcloud beta emulators firestore start --host-port=localhost:8618")
	}
	require.NoError(t, con.Close())
}

func TestFirestoreDB(t *testing.T) {
	cfg := firestoreParams()
	ensureTestsEnabled(t)
	ensureEmulatorRunning(t, cfg)

	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clocki.FakeClock, error) {
		testCfg, err := test.ApplyOptions(options)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		if testCfg.MirrorMode {
			return nil, nil, test.ErrMirrorNotSupported
		}

		// This would seem to be a bad thing for firestore to omit
		if testCfg.ConcurrentBackend != nil {
			return nil, nil, test.ErrConcurrentAccessNotSupported
		}

		clock := clockwork.NewRealClock()

		// we can't fiddle with clocks inside the firestore client, so instead of creating
		// and returning a fake clock, we wrap the real clock used by the client
		// in a FakeClock interface that sleeps instead of instantly advancing.
		sleepingClock := test.BlockingFakeClock{Clock: clock}

		uut, err := New(context.Background(), cfg, Options{Clock: sleepingClock})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		return uut, sleepingClock, nil
	}

	test.RunBackendComplianceSuite(t, newBackend)
}

// newBackend creates a self-closing firestore backend
func newBackend(t *testing.T, cfg map[string]any) *Backend {
	clock := clockwork.NewFakeClock()

	uut, err := New(context.Background(), cfg, Options{Clock: clock})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, uut.Close()) })

	return uut
}

func TestReadLegacyRecord(t *testing.T) {
	cfg := firestoreParams()
	ensureTestsEnabled(t)
	ensureEmulatorRunning(t, cfg)

	uut := newBackend(t, cfg)

	item := backend.Item{
		Key:     backend.NewKey("legacy-record"),
		Value:   []byte("foo"),
		Expires: uut.clock.Now().Add(time.Minute).Round(time.Second).UTC(),
	}

	// Write using legacy record format, emulating data written by an older
	// version of this backend.
	ctx := context.Background()
	rl := legacyRecord{
		Key:       item.Key.String(),
		Value:     string(item.Value),
		Expires:   item.Expires.UTC().Unix(),
		Timestamp: uut.clock.Now().UTC().Unix(),
	}
	_, err := uut.svc.Collection(uut.CollectionName).Doc(uut.keyToDocumentID(item.Key)).Set(ctx, rl)
	require.NoError(t, err)

	// Read the data back and make sure it matches the original item.
	got, err := uut.Get(ctx, item.Key)
	require.NoError(t, err)
	require.Equal(t, item.Key, got.Key)
	require.Equal(t, item.Value, got.Value)
	require.Equal(t, item.Expires, got.Expires)

	// Read the data back using a range query too.
	gotRange, err := uut.GetRange(ctx, item.Key, item.Key, 1)
	require.NoError(t, err)
	require.Len(t, gotRange.Items, 1)

	got = &gotRange.Items[0]
	require.Equal(t, item.Key, got.Key)
	require.Equal(t, item.Value, got.Value)
	require.Equal(t, item.Expires, got.Expires)
}

func TestReadBrokenRecord(t *testing.T) {
	cfg := firestoreParams()
	ensureTestsEnabled(t)
	ensureEmulatorRunning(t, cfg)

	uut := newBackend(t, cfg)

	ctx := context.Background()

	prefix := test.MakePrefix()

	// Create a valid record with the correct key type.
	item := backend.Item{
		Key:   prefix("valid-record"),
		Value: []byte("llamas"),
	}
	_, err := uut.Put(ctx, item)
	require.NoError(t, err)

	// Create a legacy record with a string key type.
	lr := legacyRecord{
		Key:   prefix("legacy-record").String(),
		Value: "sheep",
	}
	_, err = uut.svc.Collection(uut.CollectionName).Doc(uut.keyToDocumentID(backend.KeyFromString(lr.Key))).Set(ctx, lr)
	require.NoError(t, err)

	// Create a broken record with a backend.Key key type.
	brokenItem := backend.Item{
		Key:     prefix("broken-record"),
		Value:   []byte("foo"),
		Expires: uut.clock.Now().Add(time.Minute).Round(time.Second).UTC(),
	}

	// Write using broken record format, emulating data written by an older
	// version of this backend.
	br := brokenRecord{
		Key:       brokenKey(brokenItem.Key.String()),
		Value:     brokenItem.Value,
		Expires:   brokenItem.Expires.UTC().Unix(),
		Timestamp: uut.clock.Now().UTC().Unix(),
	}
	_, err = uut.svc.Collection(uut.CollectionName).Doc(uut.keyToDocumentID(brokenItem.Key)).Set(ctx, br)
	require.NoError(t, err)

	// Read the data back and make sure it matches the original item.
	got, err := uut.Get(ctx, brokenItem.Key)
	require.NoError(t, err)
	require.Equal(t, brokenItem.Key, got.Key)
	require.Equal(t, brokenItem.Value, got.Value)
	require.Equal(t, brokenItem.Expires, got.Expires)

	// Read the data back using a range query too.
	gotRange, err := uut.GetRange(ctx, brokenItem.Key, brokenItem.Key, 1)
	require.NoError(t, err)
	require.Len(t, gotRange.Items, 1)

	got = &gotRange.Items[0]
	require.Equal(t, brokenItem.Key, got.Key)
	require.Equal(t, brokenItem.Value, got.Value)
	require.Equal(t, brokenItem.Expires, got.Expires)

	// Retrieve the entire key range to validate that there are no duplicate records
	results, err := uut.GetRange(ctx, prefix(""), backend.RangeEnd(prefix("")), 5)
	require.NoError(t, err)

	require.Len(t, results.Items, 3)

	for _, result := range results.Items {
		switch r := result.Key.String(); r {
		case item.Key.String():
			assert.Equal(t, item.Value, result.Value)
		case string(br.Key):
			assert.Equal(t, br.Value, result.Value)
		case lr.Key:
			assert.Equal(t, lr.Value, string(result.Value))
		default:
			t.Errorf("GetRange returned unexpected item key %s", r)
		}
	}

	// Update the value and ensure that it's set to the correct key value
	item.Value = []byte("llama")
	_, err = uut.Update(ctx, item)
	require.NoError(t, err)

	doc, err := uut.svc.Collection(uut.CollectionName).Doc(uut.keyToDocumentID(item.Key)).Get(ctx)
	require.NoError(t, err)

	var r record
	require.NoError(t, doc.DataTo(&r))
	require.Equal(t, []byte(item.Key.String()), r.Key)
	require.Equal(t, item.Value, r.Value)
}

type mockFirestoreServer struct {
	// Embed for forward compatibility.
	// Tests will keep working if more methods are added
	// in the future.
	firestorepb.FirestoreServer

	mu   sync.RWMutex
	reqs []proto.Message

	// If set, Commit returns this error.
	commitErr error
}

func (s *mockFirestoreServer) BatchWrite(ctx context.Context, req *firestorepb.BatchWriteRequest) (*firestorepb.BatchWriteResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}

	s.mu.Lock()
	s.reqs = append(s.reqs, req)
	s.mu.Unlock()

	if s.commitErr != nil {
		return nil, s.commitErr
	}

	resp := &firestorepb.BatchWriteResponse{}
	for range req.Writes {
		resp.Status = append(resp.Status, &status.Status{
			Code: int32(code.Code_OK),
		})

		resp.WriteResults = append(resp.WriteResults, &firestorepb.WriteResult{
			UpdateTime: timestamppb.Now(),
		})
	}

	return resp, nil
}

func TestDeleteDocuments(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		assertion   require.ErrorAssertionFunc
		responseErr error
		commitErr   error
		documents   int
	}{
		{
			name:      "failed to commit",
			assertion: require.Error,
			commitErr: errors.New("failed to commit documents"),
			documents: 1,
		},
		{
			name:      "commit success",
			assertion: require.NoError,
			documents: 1796,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			docs := make([]*firestore.DocumentSnapshot, 0, tt.documents)
			for i := range tt.documents {
				docs = append(docs, &firestore.DocumentSnapshot{
					Ref: &firestore.DocumentRef{
						Path: fmt.Sprintf("projects/test-project/databases/test-db/documents/test/%d", i+1),
					},
					CreateTime: time.Now(),
					UpdateTime: time.Now(),
				})

				// We really shouldn't need this, but the Firestore SDK made some unfortunate design
				// decisions that make it impossible to set the field of a DocumentRef used for the seemingly
				// useless deduplication in the BulkWriter API.
				rs := reflect.ValueOf(docs[i].Ref).Elem()
				rf := rs.FieldByName("shortPath")
				rf = reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem()
				rf.SetString(docs[i].Ref.Path)
			}

			mockFirestore := &mockFirestoreServer{
				commitErr: tt.commitErr,
			}
			srv := grpc.NewServer()
			firestorepb.RegisterFirestoreServer(srv, mockFirestore)

			lis, err := net.Listen("tcp", "localhost:0")
			require.NoError(t, err)

			errCh := make(chan error, 1)
			go func() { errCh <- srv.Serve(lis) }()
			t.Cleanup(func() {
				srv.Stop()
				require.NoError(t, <-errCh)
			})

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			conn, err := grpc.Dial(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
			require.NoError(t, err)

			client, err := firestore.NewClient(ctx, "test-project", option.WithGRPCConn(conn))
			require.NoError(t, err)

			b := &Backend{
				svc:           client,
				logger:        slog.With(teleport.ComponentKey, BackendName),
				clock:         clockwork.NewFakeClock(),
				clientContext: ctx,
				clientCancel:  cancel,
				backendConfig: backendConfig{
					Config: Config{
						CollectionName: "test-collection",
					},
				},
			}

			tt.assertion(t, b.deleteDocuments(docs))

			if tt.documents == 0 {
				return
			}

			var committed int
			mockFirestore.mu.RLock()
			for _, req := range mockFirestore.reqs {
				switch r := req.(type) {
				case *firestorepb.BatchWriteRequest:
					committed += len(r.Writes)
				}
			}
			mockFirestore.mu.RUnlock()

			require.Equal(t, tt.documents, committed)

		})
	}

}

// TestFirestoreMigration tests the migration of incorrect key types in Firestore.
// TODO(tigrato|rosstimothy): DELETE In 19.0.0: Remove this migration in 19.0.0.
func TestFirestoreMigration(t *testing.T) {
	cfg := firestoreParams()
	ensureTestsEnabled(t)
	ensureEmulatorRunning(t, cfg)

	clock := clockwork.NewRealClock()

	uut, err := New(context.Background(), cfg, Options{Clock: clock})
	require.NoError(t, err)

	// Empty the collection to make sure previous tests don't interfere
	snapshot, err := uut.svc.Collection(uut.CollectionName).Documents(context.Background()).GetAll()
	require.NoError(t, err)
	require.NoError(t, uut.deleteDocuments(snapshot))

	type byteAlias []byte
	type badRecord struct {
		Key        byteAlias `firestore:"key,omitempty"`
		Timestamp  int64     `firestore:"timestamp,omitempty"`
		Expires    int64     `firestore:"expires,omitempty"`
		Value      []byte    `firestore:"value,omitempty"`
		RevisionV2 string    `firestore:"revision,omitempty"`
		RevisionV1 string    `firestore:"-"`
	}

	for i := range 301 {
		key := fmt.Appendf(nil, "test-%d", i)
		_, err = uut.svc.Collection(uut.CollectionName).
			Doc(base64.URLEncoding.EncodeToString(key)).
			Set(context.Background(), &badRecord{
				Key:        key,
				Timestamp:  clock.Now().UTC().Unix(),
				Expires:    clock.Now().Add(time.Minute).UTC().Unix(),
				Value:      key,
				RevisionV2: "v2",
			})
		require.NoError(t, err)
	}

	// Migrate the collection
	uut.migrateIncorrectKeyTypes()

	// Ensure that all incorrect key types have been migrated
	docs, err := uut.svc.Collection(uut.CollectionName).
		Where(keyDocProperty, ">", byteAlias("/")).
		Limit(100).
		Documents(context.Background()).GetAll()
	require.NoError(t, err)

	require.Empty(t, docs, "expected all incorrect key types to be migrated")

	// Ensure that all incorrect key types have been migrated to the correct key type []byte
	docs, err = uut.svc.Collection(uut.CollectionName).
		Where(keyDocProperty, ">", []byte("/")).
		Documents(context.Background()).GetAll()
	require.NoError(t, err)
	require.Len(t, docs, 301)
}
