// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package firestore

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adminpb "google.golang.org/genproto/googleapis/firestore/admin/v1"
	"google.golang.org/protobuf/proto"
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

var commonFirestoreParams = map[string]interface{}{
	"collection_name":                   "tp-cluster-data-test",
	"project_id":                        "tp-testproj",
	"endpoint":                          "localhost:8618",
	"purgeExpiredDocumentsPollInterval": time.Second,
}

func TestFirestoreDB(t *testing.T) {
	ensureEmulatorRunning(t)

	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clockwork.FakeClock, error) {
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

		uut, err := New(context.Background(), commonFirestoreParams)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		clock := clockwork.NewFakeClock()
		uut.clock = clock
		return uut, clock, nil
	}

	test.RunBackendComplianceSuite(t, newBackend)
}

func ensureEmulatorRunning(t *testing.T) {
	con, err := net.Dial("tcp", "localhost:8618")
	if err != nil {
		t.Skip("Firestore emulator is not running, start it with: gcloud beta emulators firestore start --host-port=localhost:8618")
	}
	con.Close()
}

// newBackend creates a self-closing firestore backend
func newBackend(t *testing.T) *Backend {
	uut, err := New(context.Background(), commonFirestoreParams)
	require.NoError(t, err)
	t.Cleanup(func() { uut.Close() })

	clock := clockwork.NewFakeClock()
	uut.clock = clock

	return uut
}

func TestReadLegacyRecord(t *testing.T) {
	ensureEmulatorRunning(t)

	uut := newBackend(t)

	item := backend.Item{
		Key:     []byte("legacy-record"),
		Value:   []byte("foo"),
		Expires: uut.clock.Now().Add(time.Minute).Round(time.Second).UTC(),
		ID:      uut.clock.Now().UTC().UnixNano(),
	}

	// Write using legacy record format, emulating data written by an older
	// version of this backend.
	ctx := context.Background()
	rl := legacyRecord{
		Key:       string(item.Key),
		Value:     string(item.Value),
		Expires:   item.Expires.UTC().Unix(),
		Timestamp: uut.clock.Now().UTC().Unix(),
		ID:        item.ID,
	}
	_, err := uut.svc.Collection(uut.CollectionName).Doc(uut.keyToDocumentID(item.Key)).Set(ctx, rl)
	require.NoError(t, err)

	// Read the data back and make sure it matches the original item.
	got, err := uut.Get(ctx, item.Key)
	require.NoError(t, err)
	require.Equal(t, item.Key, got.Key)
	require.Equal(t, item.Value, got.Value)
	require.Equal(t, item.ID, got.ID)
	require.Equal(t, item.Expires, got.Expires)

	// Read the data back using a range query too.
	gotRange, err := uut.GetRange(ctx, item.Key, item.Key, 1)
	require.NoError(t, err)
	require.Len(t, gotRange.Items, 1)

	got = &gotRange.Items[0]
	require.Equal(t, item.Key, got.Key)
	require.Equal(t, item.Value, got.Value)
	require.Equal(t, item.ID, got.ID)
	require.Equal(t, item.Expires, got.Expires)
}
