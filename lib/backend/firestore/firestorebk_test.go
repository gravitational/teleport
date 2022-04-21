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
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	adminpb "google.golang.org/genproto/googleapis/firestore/admin/v1"
	firestorepb "google.golang.org/genproto/googleapis/firestore/v1"
	"google.golang.org/protobuf/proto"
	"gopkg.in/check.v1"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// TestMarshal tests index operation metadata marshal and unmarshal
// to verify backwards compatibility. Gogoproto is incompatible with ApiV2 protoc-gen-go code.
//
// Track the issue here: https://github.com/gogo/protobuf/issues/678
//
func TestMarshal(t *testing.T) {
	meta := adminpb.IndexOperationMetadata{}
	data, err := proto.Marshal(&meta)
	assert.NoError(t, err)
	out := adminpb.IndexOperationMetadata{}
	err = proto.Unmarshal(data, &out)
	assert.NoError(t, err)
}

func TestFirestoreDB(t *testing.T) { check.TestingT(t) }

type FirestoreSuite struct {
	bk    *Backend
	suite test.BackendSuite
}

var _ = check.Suite(&FirestoreSuite{})

func (s *FirestoreSuite) SetUpSuite(c *check.C) {
	if !emulatorRunning() {
		c.Skip("Firestore emulator is not running, start it with: gcloud beta emulators firestore start --host-port=localhost:8618")
	}

	newBackend := func() (backend.Backend, error) {
		return New(context.Background(), map[string]interface{}{
			"collection_name":                   "tp-cluster-data-test",
			"project_id":                        "tp-testproj",
			"endpoint":                          "localhost:8618",
			"purgeExpiredDocumentsPollInterval": time.Second,
		})
	}
	bk, err := newBackend()
	c.Assert(err, check.IsNil)
	s.bk = bk.(*Backend)
	s.suite.B = s.bk
	s.suite.NewBackend = newBackend
	clock := clockwork.NewFakeClock()
	s.bk.clock = clock
	s.suite.Clock = clock
}

func emulatorRunning() bool {
	con, err := net.Dial("tcp", "localhost:8618")
	if err != nil {
		return false
	}
	con.Close()
	return true
}

func (s *FirestoreSuite) TearDownTest(c *check.C) {
	// Delete all documents.
	ctx := context.Background()
	docSnaps, err := s.bk.svc.Collection(s.bk.CollectionName).Documents(ctx).GetAll()
	c.Assert(err, check.IsNil)
	if len(docSnaps) == 0 {
		return
	}
	batch := s.bk.svc.Batch()
	for _, docSnap := range docSnaps {
		batch.Delete(docSnap.Ref)
	}
	_, err = batch.Commit(ctx)
	c.Assert(err, check.IsNil)
}

func (s *FirestoreSuite) TearDownSuite(c *check.C) {
	if s.bk != nil {
		s.bk.Close()
	}
}

func (s *FirestoreSuite) TestCRUD(c *check.C) {
	s.suite.CRUD(c)
}

func (s *FirestoreSuite) TestRange(c *check.C) {
	s.suite.Range(c)
}

func (s *FirestoreSuite) TestDeleteRange(c *check.C) {
	s.suite.DeleteRange(c)
}

func (s *FirestoreSuite) TestCompareAndSwap(c *check.C) {
	s.suite.CompareAndSwap(c)
}

func (s *FirestoreSuite) TestExpiration(c *check.C) {
	s.suite.Expiration(c)
}

func (s *FirestoreSuite) TestKeepAlive(c *check.C) {
	s.suite.KeepAlive(c)
}

func (s *FirestoreSuite) TestEvents(c *check.C) {
	s.suite.Events(c)
}

func (s *FirestoreSuite) TestWatchersClose(c *check.C) {
	s.suite.WatchersClose(c)
}

func (s *FirestoreSuite) TestLocking(c *check.C) {
	s.suite.Locking(c, s.bk)
}

func (s *FirestoreSuite) TestReadLegacyRecord(c *check.C) {
	item := backend.Item{
		Key:     []byte("legacy-record"),
		Value:   []byte("foo"),
		Expires: s.bk.clock.Now().Add(time.Minute).Round(time.Second).UTC(),
		ID:      s.bk.clock.Now().UTC().UnixNano(),
	}

	// Write using legacy record format, emulating data written by an older
	// version of this backend.
	ctx := context.Background()
	rl := legacyRecord{
		Key:       string(item.Key),
		Value:     string(item.Value),
		Expires:   item.Expires.UTC().Unix(),
		Timestamp: s.bk.clock.Now().UTC().Unix(),
		ID:        item.ID,
	}
	_, err := s.bk.svc.Collection(s.bk.CollectionName).Doc(s.bk.keyToDocumentID(item.Key)).Set(ctx, rl)
	c.Assert(err, check.IsNil)

	// Read the data back and make sure it matches the original item.
	got, err := s.bk.Get(ctx, item.Key)
	c.Assert(err, check.IsNil)
	c.Assert(got.Key, check.DeepEquals, item.Key)
	c.Assert(got.Value, check.DeepEquals, item.Value)
	c.Assert(got.ID, check.DeepEquals, item.ID)
	c.Assert(got.Expires.Equal(item.Expires), check.Equals, true)

	// Read the data back using a range query too.
	gotRange, err := s.bk.GetRange(ctx, item.Key, item.Key, 1)
	c.Assert(err, check.IsNil)
	c.Assert(len(gotRange.Items), check.Equals, 1)
	got = &gotRange.Items[0]
	c.Assert(got.Key, check.DeepEquals, item.Key)
	c.Assert(got.Value, check.DeepEquals, item.Value)
	c.Assert(got.ID, check.DeepEquals, item.ID)
	c.Assert(got.Expires.Equal(item.Expires), check.Equals, true)
}

type mockFirestoreServer struct {
	// Embed for forward compatibility.
	// Tests will keep working if more methods are added
	// in the future.
	firestorepb.FirestoreServer

	reqs []proto.Message

	// If set, Commit returns this error.
	commitErr error
}

func (s *mockFirestoreServer) Commit(ctx context.Context, req *firestorepb.CommitRequest) (*firestorepb.CommitResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if xg := md["x-goog-api-client"]; len(xg) == 0 || !strings.Contains(xg[0], "gl-go/") {
		return nil, fmt.Errorf("x-goog-api-client = %v, expected gl-go key", xg)
	}

	if len(req.Writes) > commitLimit {
		return nil, status.Errorf(codes.InvalidArgument, "too many writes in a transaction")
	}

	s.reqs = append(s.reqs, req)
	if s.commitErr != nil {
		return nil, s.commitErr
	}
	return &firestorepb.CommitResponse{
		WriteResults: []*firestorepb.WriteResult{{
			UpdateTime: timestamppb.Now(),
		}},
	}, nil
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
			name:      "commit less than limit",
			assertion: require.NoError,
			documents: commitLimit - 123,
		},
		{
			name:      "commit limit",
			assertion: require.NoError,
			documents: commitLimit,
		},
		{
			name:      "commit more than limit",
			assertion: require.NoError,
			documents: (commitLimit * 3) + 173,
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			docs := make([]*firestore.DocumentSnapshot, 0, tt.documents)
			for i := 0; i < tt.documents; i++ {
				docs = append(docs, &firestore.DocumentSnapshot{
					Ref: &firestore.DocumentRef{
						Path: fmt.Sprintf("projects/test-project/databases/test-db/documents/test/%d", i+1),
					},
					CreateTime: time.Now(),
					UpdateTime: time.Now(),
				})
			}

			mockFirestore := &mockFirestoreServer{
				commitErr: tt.commitErr,
			}
			srv := grpc.NewServer()
			firestorepb.RegisterFirestoreServer(srv, mockFirestore)

			lis, err := net.Listen("tcp", "localhost:0")
			require.NoError(t, err)
			go func() { require.NoError(t, srv.Serve(lis)) }()
			t.Cleanup(srv.Stop)

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
			require.NoError(t, err)

			client, err := firestore.NewClient(ctx, "test-project", option.WithGRPCConn(conn))
			require.NoError(t, err)

			b := &Backend{
				svc:           client,
				Entry:         utils.NewLoggerForTests().WithFields(logrus.Fields{trace.Component: BackendName}),
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
			for _, req := range mockFirestore.reqs {
				switch r := req.(type) {
				case *firestorepb.CommitRequest:
					committed += len(r.Writes)
				}
			}

			require.Equal(t, tt.documents, committed)

		})
	}
}
