// Copyright 2023 Gravitational, Inc
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

package dynamoathenamigration

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func TestMigrateProcessDataObjects(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// testDataObjects represents how dynamo export data using JSON lines format.
	testDataObjects := map[string]string{
		"testdata/dataObj1.json.gz": `{ "Item": { "EventIndex": { "N": "2147483647" }, "SessionID": { "S": "4298bd54-a747-4d53-b850-83ba17caae5a" }, "CreatedAtDate": { "S": "2023-05-22" }, "FieldsMap": { "M": { "cluster_name": { "S": "test.example.local" }, "uid": { "S": "850d0dd5-7d6b-415e-a404-c4f79cdf27c9" }, "code": { "S": "T2005I" }, "ei": { "N": "2147483647" }, "time": { "S": "2023-05-22T12:12:21.966Z" }, "event": { "S": "session.upload" }, "sid": { "S": "4298bd54-a747-4d53-b850-83ba17caae5a" } } }, "EventType": { "S": "session.upload" }, "EventNamespace": { "S": "default" }, "CreatedAt": { "N": "1684757541" } } }
{ "Item": { "EventIndex": { "N": "2147483647" }, "SessionID": { "S": "f81a2fda-4ede-4abc-86f9-7a58e189038e" }, "CreatedAtDate": { "S": "2023-05-22" }, "FieldsMap": { "M": { "cluster_name": { "S": "test.example.local" }, "uid": { "S": "19ab6e90-602c-4dcc-88b3-de5f28753f88" }, "code": { "S": "T2005I" }, "ei": { "N": "2147483647" }, "time": { "S": "2023-05-22T12:12:21.966Z" }, "event": { "S": "session.upload" }, "sid": { "S": "f81a2fda-4ede-4abc-86f9-7a58e189038e" } } }, "EventType": { "S": "session.upload" }, "EventNamespace": { "S": "default" }, "CreatedAt": { "N": "1684757541" } } }`,
		"testdata/dataObj2.json.gz": `{ "Item": { "EventIndex": { "N": "2147483647" }, "SessionID": { "S": "35f35254-92f9-46a2-9b05-8c13f712389b" }, "CreatedAtDate": { "S": "2023-05-22" }, "FieldsMap": { "M": { "cluster_name": { "S": "test.example.local" }, "uid": { "S": "46c03b4f-4ef5-4d86-80aa-4b53c7efc28f" }, "code": { "S": "T2005I" }, "ei": { "N": "2147483647" }, "time": { "S": "2023-05-22T12:12:21.966Z" }, "event": { "S": "session.upload" }, "sid": { "S": "35f35254-92f9-46a2-9b05-8c13f712389b" } } }, "EventType": { "S": "session.upload" }, "EventNamespace": { "S": "default" }, "CreatedAt": { "N": "1684757541" } } }`,
	}
	emitter := &mockEmitter{}
	mt := &task{
		s3Downloader: &fakeDownloader{
			dataObjects: testDataObjects,
		},
		eventsEmitter: emitter,
		Config: Config{
			Logger:          utils.NewSlogLoggerForTests(),
			NoOfEmitWorkers: 5,
			bufferSize:      10,
			CheckpointPath:  filepath.Join(t.TempDir(), "migration-tests.json"),
		},
	}
	err := mt.ProcessDataObjects(ctx, &exportInfo{
		ExportARN: "export-arn",
		DataObjectsInfo: []dataObjectInfo{
			{DataFileS3Key: "testdata/dataObj1.json.gz", ItemCount: 2},
			{DataFileS3Key: "testdata/dataObj2.json.gz", ItemCount: 1},
		},
	})
	require.NoError(t, err)
	wantEventTimestamp := time.Date(2023, 5, 22, 12, 12, 21, 966000000, time.UTC)
	requireEventsEqualInAnyOrder(t, []apievents.AuditEvent{
		&apievents.SessionUpload{
			Metadata: apievents.Metadata{
				Index:       2147483647,
				Type:        "session.upload",
				ID:          "850d0dd5-7d6b-415e-a404-c4f79cdf27c9",
				Code:        "T2005I",
				Time:        wantEventTimestamp,
				ClusterName: "test.example.local",
			},
			SessionMetadata: apievents.SessionMetadata{
				SessionID: "4298bd54-a747-4d53-b850-83ba17caae5a",
			},
		},
		&apievents.SessionUpload{
			Metadata: apievents.Metadata{
				Index:       2147483647,
				Type:        "session.upload",
				ID:          "19ab6e90-602c-4dcc-88b3-de5f28753f88",
				Code:        "T2005I",
				Time:        wantEventTimestamp,
				ClusterName: "test.example.local",
			},
			SessionMetadata: apievents.SessionMetadata{
				SessionID: "f81a2fda-4ede-4abc-86f9-7a58e189038e",
			},
		},
		&apievents.SessionUpload{
			Metadata: apievents.Metadata{
				Index:       2147483647,
				Type:        "session.upload",
				ID:          "46c03b4f-4ef5-4d86-80aa-4b53c7efc28f",
				Code:        "T2005I",
				Time:        wantEventTimestamp,
				ClusterName: "test.example.local",
			},
			SessionMetadata: apievents.SessionMetadata{
				SessionID: "35f35254-92f9-46a2-9b05-8c13f712389b",
			},
		},
	}, emitter.events)
}

func TestLargeEventsParse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	emitter := &mockEmitter{}
	mt := &task{
		s3Downloader: &fakeDownloader{
			dataObjects: map[string]string{
				"large.json.gz": generateLargeEventLine(),
			},
		},
		eventsEmitter: emitter,
		Config: Config{
			Logger:          utils.NewSlogLoggerForTests(),
			NoOfEmitWorkers: 5,
			bufferSize:      10,
			CheckpointPath:  filepath.Join(t.TempDir(), "migration-tests.json"),
		},
	}
	err := mt.ProcessDataObjects(ctx, &exportInfo{
		ExportARN: "export-arn",
		DataObjectsInfo: []dataObjectInfo{
			{DataFileS3Key: "large.json.gz"},
		},
	})
	require.NoError(t, err)
	require.Len(t, emitter.events, 1)
}

type fakeDownloader struct {
	dataObjects map[string]string
}

func (f *fakeDownloader) Download(ctx context.Context, w io.WriterAt, input *s3.GetObjectInput, options ...func(*manager.Downloader)) (int64, error) {
	data, ok := f.dataObjects[*input.Key]
	if !ok {
		return 0, errors.New("object does not exists")
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write([]byte(data))
	if err != nil {
		return 0, err
	}
	if err := zw.Close(); err != nil {
		return 0, err
	}

	n, err := w.WriteAt(buf.Bytes(), 0)
	return int64(n), err
}

type mockEmitter struct {
	mu     sync.Mutex
	events []apievents.AuditEvent

	// failAfterNCalls if greater than 0, will cause failure of emitter after n calls
	failAfterNCalls int
}

func (m *mockEmitter) EmitAuditEvent(ctx context.Context, in apievents.AuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failAfterNCalls > 0 && len(m.events) >= m.failAfterNCalls {
		return errors.New("emitter failure")
	}
	m.events = append(m.events, in)
	// Simulate some work by sleeping during emitting.
	// It helps redistribute task processing among all workers and requires
	// less iterations in tests to generate checkpoint file from all workers.
	// Without it, in rare cases after 50 events still some worker were not able
	// to read message because of other processing it immediately.
	select {
	case <-time.After(retryutils.HalfJitter(100 * time.Microsecond)):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// requireEventsEqualInAnyOrder compares slices of auditevents ignoring order.
// It's useful in tests because consumer does not guarantee order.
func requireEventsEqualInAnyOrder(t *testing.T, want, got []apievents.AuditEvent) {
	sort.Slice(want, func(i, j int) bool {
		return want[i].GetID() < want[j].GetID()
	})
	sort.Slice(got, func(i, j int) bool {
		return got[i].GetID() < got[j].GetID()
	})
	require.Empty(t, cmp.Diff(want, got))
}

func TestMigrationCheckpoint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// There is confirmation prompt in migration when reusing checkpoint, that's why
	// Stdin is overwritten in tests.
	oldStdin := prompt.Stdin()
	t.Cleanup(func() { prompt.SetStdin(oldStdin) })

	noOfWorkers := 3
	defaultConfig := Config{
		Logger:          utils.NewSlogLoggerForTests(),
		NoOfEmitWorkers: noOfWorkers,
		bufferSize:      noOfWorkers * 5,
		CheckpointPath:  filepath.Join(t.TempDir(), "migration-tests.json"),
	}

	t.Run("no migration checkpoint, emit every event", func(t *testing.T) {
		prompt.SetStdin(prompt.NewFakeReader())
		testDataObjects := map[string]string{
			"testdata/dataObj1.json.gz": generateDynamoExportData(100),
			"testdata/dataObj2.json.gz": generateDynamoExportData(100),
		}
		emitter := &mockEmitter{}
		mt := &task{
			s3Downloader: &fakeDownloader{
				dataObjects: testDataObjects,
			},
			eventsEmitter: emitter,
			Config:        defaultConfig,
		}
		err := mt.ProcessDataObjects(ctx, &exportInfo{
			ExportARN: uuid.NewString(),
			DataObjectsInfo: []dataObjectInfo{
				{DataFileS3Key: "testdata/dataObj1.json.gz"},
				{DataFileS3Key: "testdata/dataObj2.json.gz"},
			},
		})
		require.NoError(t, err)
		require.Len(t, emitter.events, 200, "unexpected number of emitted events")
	})

	t.Run("failure after 50 calls, reuse checkpoint", func(t *testing.T) {
		// y to prompt on if reuse checkpoint
		prompt.SetStdin(prompt.NewFakeReader().AddString("y"))
		exportARN := uuid.NewString()
		testDataObjects := map[string]string{
			"testdata/dataObj1.json.gz": generateDynamoExportData(100),
			"testdata/dataObj2.json.gz": generateDynamoExportData(100),
		}
		emitter := &mockEmitter{
			failAfterNCalls: 50,
		}
		mt := &task{
			s3Downloader: &fakeDownloader{
				dataObjects: testDataObjects,
			},
			eventsEmitter: emitter,
			Config:        defaultConfig,
		}
		err := mt.ProcessDataObjects(ctx, &exportInfo{
			ExportARN: exportARN,
			DataObjectsInfo: []dataObjectInfo{
				{DataFileS3Key: "testdata/dataObj1.json.gz"},
				{DataFileS3Key: "testdata/dataObj2.json.gz"},
			},
		})
		require.Error(t, err)
		require.Len(t, emitter.events, 50, "unexpected number of emitted events")

		newEmitter := &mockEmitter{}
		newMigration := task{
			s3Downloader: &fakeDownloader{
				dataObjects: testDataObjects,
			},
			eventsEmitter: newEmitter,
			Config:        defaultConfig,
		}
		err = newMigration.ProcessDataObjects(ctx, &exportInfo{
			ExportARN: exportARN,
			DataObjectsInfo: []dataObjectInfo{
				{DataFileS3Key: "testdata/dataObj1.json.gz"},
				{DataFileS3Key: "testdata/dataObj2.json.gz"},
			},
		})
		require.NoError(t, err)
		// There was 200 events, first migration finished after 50, so this one should emit at least 150.
		// We are using range (150,199) to check because of checkpoint is stored per worker and we are using
		// first from list so we expect up to noOfWorkers-1 more events, but in some condition it can be more (on worker processing faster).
		require.GreaterOrEqual(t, len(newEmitter.events), 150, "unexpected number of emitted events")
		require.Less(t, len(newEmitter.events), 199, "unexpected number of emitted events")
	})
	t.Run("failure after 150 calls (from 2nd export file), reuse checkpoint", func(t *testing.T) {
		// y to prompt on if reuse checkpoint
		prompt.SetStdin(prompt.NewFakeReader().AddString("y"))
		exportARN := uuid.NewString()
		testDataObjects := map[string]string{
			"testdata/dataObj1.json.gz": generateDynamoExportData(100),
			"testdata/dataObj2.json.gz": generateDynamoExportData(100),
		}
		emitter := &mockEmitter{
			failAfterNCalls: 150,
		}
		mt := &task{
			s3Downloader: &fakeDownloader{
				dataObjects: testDataObjects,
			},
			eventsEmitter: emitter,
			Config:        defaultConfig,
		}
		err := mt.ProcessDataObjects(ctx, &exportInfo{
			ExportARN: exportARN,
			DataObjectsInfo: []dataObjectInfo{
				{DataFileS3Key: "testdata/dataObj1.json.gz"},
				{DataFileS3Key: "testdata/dataObj2.json.gz"},
			},
		})
		require.Error(t, err)
		require.Len(t, emitter.events, 150, "unexpected number of emitted events")

		newEmitter := &mockEmitter{}
		newMigration := task{
			s3Downloader: &fakeDownloader{
				dataObjects: testDataObjects,
			},
			eventsEmitter: newEmitter,
			Config:        defaultConfig,
		}
		err = newMigration.ProcessDataObjects(ctx, &exportInfo{
			ExportARN: exportARN,
			DataObjectsInfo: []dataObjectInfo{
				{DataFileS3Key: "testdata/dataObj1.json.gz"},
				{DataFileS3Key: "testdata/dataObj2.json.gz"},
			},
		})
		require.NoError(t, err)
		// There was 200 events, first migration finished after 150, so this one should emit at least 50.
		// We are using range (50,99) to check because of checkpoint is stored per worker and we are using
		// first from list so we expect up to noOfWorkers-1 more events, but in some condition it can be more (on worker processing faster).
		require.GreaterOrEqual(t, len(newEmitter.events), 50, "unexpected number of emitted events")
		require.Less(t, len(newEmitter.events), 99, "unexpected number of emitted events")
	})
	t.Run("checkpoint from export1 is not reused on export2", func(t *testing.T) {
		prompt.SetStdin(prompt.NewFakeReader())
		exportARN1 := uuid.NewString()
		testDataObjects1 := map[string]string{
			"testdata/dataObj11.json.gz": generateDynamoExportData(100),
			"testdata/dataObj12.json.gz": generateDynamoExportData(100),
		}
		emitter := &mockEmitter{
			// To use checkpoint.
			failAfterNCalls: 50,
		}
		mt := &task{
			s3Downloader: &fakeDownloader{
				dataObjects: testDataObjects1,
			},
			eventsEmitter: emitter,
			Config:        defaultConfig,
		}
		err := mt.ProcessDataObjects(ctx, &exportInfo{
			ExportARN: exportARN1,
			DataObjectsInfo: []dataObjectInfo{
				{DataFileS3Key: "testdata/dataObj11.json.gz"},
				{DataFileS3Key: "testdata/dataObj12.json.gz"},
			},
		})
		require.Error(t, err)
		require.Len(t, emitter.events, 50, "unexpected number of emitted events")

		exportARN2 := uuid.NewString()
		testDataObjects2 := map[string]string{
			"testdata/dataObj21.json.gz": generateDynamoExportData(100),
			"testdata/dataObj22.json.gz": generateDynamoExportData(100),
		}
		newEmitter := &mockEmitter{}
		newMigration := task{
			s3Downloader: &fakeDownloader{
				dataObjects: testDataObjects2,
			},
			eventsEmitter: newEmitter,
			Config:        defaultConfig,
		}
		err = newMigration.ProcessDataObjects(ctx, &exportInfo{
			ExportARN: exportARN2,
			DataObjectsInfo: []dataObjectInfo{
				{DataFileS3Key: "testdata/dataObj21.json.gz"},
				{DataFileS3Key: "testdata/dataObj22.json.gz"},
			},
		})
		require.NoError(t, err)
		require.Len(t, newEmitter.events, 200, "unexpected number of emitted events")
	})
	t.Run("failure after 50 calls, refuse to reuse checkpoint", func(t *testing.T) {
		// y to prompt on if reuse checkpoint
		prompt.SetStdin(prompt.NewFakeReader().AddString("n"))
		exportARN := uuid.NewString()
		testDataObjects := map[string]string{
			"testdata/dataObj1.json.gz": generateDynamoExportData(100),
			"testdata/dataObj2.json.gz": generateDynamoExportData(100),
		}
		emitter := &mockEmitter{
			failAfterNCalls: 50,
		}
		mt := &task{
			s3Downloader: &fakeDownloader{
				dataObjects: testDataObjects,
			},
			eventsEmitter: emitter,
			Config:        defaultConfig,
		}
		err := mt.ProcessDataObjects(ctx, &exportInfo{
			ExportARN: exportARN,
			DataObjectsInfo: []dataObjectInfo{
				{DataFileS3Key: "testdata/dataObj1.json.gz"},
				{DataFileS3Key: "testdata/dataObj2.json.gz"},
			},
		})
		require.Error(t, err)
		require.Len(t, emitter.events, 50, "unexpected number of emitted events")

		newEmitter := &mockEmitter{}
		newMigration := task{
			s3Downloader: &fakeDownloader{
				dataObjects: testDataObjects,
			},
			eventsEmitter: newEmitter,
			Config:        defaultConfig,
		}
		err = newMigration.ProcessDataObjects(ctx, &exportInfo{
			ExportARN: exportARN,
			DataObjectsInfo: []dataObjectInfo{
				{DataFileS3Key: "testdata/dataObj1.json.gz"},
				{DataFileS3Key: "testdata/dataObj2.json.gz"},
			},
		})
		require.NoError(t, err)
		require.Len(t, newEmitter.events, 200, "unexpected number of emitted events")
	})
}

func generateLargeEventLine() string {
	// Generate event close to 400KB which is max of dynamoDB to test if
	// it can be processed.
	return fmt.Sprintf(
		`{
			"Item": {
				"EventIndex": {
					"N": "2147483647"
				},
				"SessionID": {
					"S": "4298bd54-a747-4d53-b850-83ba17caae5a"
				},
				"CreatedAtDate": {
					"S": "2023-05-22"
				},
				"FieldsMap": {
					"M": {
						"cluster_name": {
							"S": "%s"
						},
						"uid": {
							"S": "%s"
						},
						"code": {
							"S": "T2005I"
						},
						"ei": {
							"N": "2147483647"
						},
						"time": {
							"S": "2023-05-22T12:12:21.966Z"
						},
						"event": {
							"S": "session.upload"
						},
						"sid": {
							"S": "4298bd54-a747-4d53-b850-83ba17caae5a"
						}
					}
				},
				"EventType": {
					"S": "session.upload"
				},
				"EventNamespace": {
					"S": "default"
				},
				"CreatedAt": {
					"N": "1684757541"
				}
			}
		}`,
		strings.Repeat("a", 1024*400 /* 400 KB */), uuid.NewString())
}

func generateDynamoExportData(n int) string {
	if n < 1 {
		panic("number of events to generate must be > 0")
	}
	lineFmt := `{ "Item": { "EventIndex": { "N": "2147483647" }, "SessionID": { "S": "4298bd54-a747-4d53-b850-83ba17caae5a" }, "CreatedAtDate": { "S": "2023-05-22" }, "FieldsMap": { "M": { "cluster_name": { "S": "test.example.local" }, "uid": { "S": "%s" }, "code": { "S": "T2005I" }, "ei": { "N": "2147483647" }, "time": { "S": "2023-05-22T12:12:21.966Z" }, "event": { "S": "session.upload" }, "sid": { "S": "4298bd54-a747-4d53-b850-83ba17caae5a" } } }, "EventType": { "S": "session.upload" }, "EventNamespace": { "S": "default" }, "CreatedAt": { "N": "1684757541" } } }`
	sb := strings.Builder{}
	for range n {
		sb.WriteString(fmt.Sprintf(lineFmt+"\n", uuid.NewString()))
	}
	return sb.String()
}

func TestMigrationDryRunValidation(t *testing.T) {
	validEvent := func() apievents.AuditEvent {
		return &apievents.AppCreate{
			Metadata: apievents.Metadata{
				Time: time.Date(2023, 5, 1, 12, 15, 0, 0, time.UTC),
				ID:   uuid.NewString(),
			},
		}
	}
	tests := []struct {
		name    string
		events  func() []apievents.AuditEvent
		wantLog string
		wantErr string
	}{
		{
			name: "valid events",
			events: func() []apievents.AuditEvent {
				return []apievents.AuditEvent{
					validEvent(), validEvent(),
				}
			},
		},
		{
			name: "event without time",
			events: func() []apievents.AuditEvent {
				eventWithoutTime := validEvent()
				eventWithoutTime.SetTime(time.Time{})
				return []apievents.AuditEvent{
					validEvent(), eventWithoutTime,
				}
			},
			wantLog: "empty event time",
			wantErr: "1 invalid",
		},
		{
			name: "event with wrong uuid",
			events: func() []apievents.AuditEvent {
				eventWithInvalidUUID := validEvent()
				eventWithInvalidUUID.SetID("invalid-uuid")
				return []apievents.AuditEvent{
					validEvent(), eventWithInvalidUUID,
				}
			},
			wantLog: "invalid uid format: invalid UUID length",
			wantErr: "1 invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Migration cli logs output from validation to logger.
			var logBuffer bytes.Buffer
			log := slog.New(logutils.NewSlogJSONHandler(&logBuffer, logutils.SlogJSONHandlerConfig{
				Level: slog.LevelDebug,
			}))

			tr := &task{
				Config: Config{
					Logger: log,
					DryRun: true,
				},
			}
			c := make(chan apievents.AuditEvent, 10)
			for _, e := range tt.events() {
				c <- e
			}
			close(c)
			err := tr.emitEvents(context.Background(), c, "" /* exportARN not used in dryRun */)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			if tt.wantLog != "" {
				require.Contains(t, logBuffer.String(), tt.wantLog)
			}
		})
	}
}

func TestSortingExportFile(t *testing.T) {
	t.Run("sorting 1MB of data by loading max 200KB into memory", func(t *testing.T) {
		// Create 1MB of Data.
		exportSize := 1024 * 1024
		generatedExport, noOfEvents := generateExportFileOfSize(t, exportSize, "2022-03-01", "2023-03-01")
		defer generatedExport.Close()
		// Allow loading max of 200KB data.
		maxSingleBatchSize := 200 * 1024
		sortedFile, err := createSortedExport(generatedExport, t.TempDir(), noOfEvents, maxSingleBatchSize)
		require.NoError(t, err)
		defer sortedFile.Close()

		// Aeert that file is sorted and it contains all events.
		dec := json.NewDecoder(sortedFile)
		// let's take first firstEvent and use it later for comparisons.
		var firstEvent dynamoEventPart
		err = dec.Decode(&firstEvent)
		require.NoError(t, err)
		// start with 1 because we already read first item
		gotEvents := 1
		minDate := firstEvent.Item.CreatedAtDate.Value
		require.NotEmpty(t, minDate)
		for dec.More() {
			var event dynamoEventPart
			err = dec.Decode(&event)
			require.NoError(t, err)
			require.LessOrEqual(t, minDate, event.Item.CreatedAtDate.Value, "events are not sorted")
			gotEvents++
		}
		require.Equal(t, noOfEvents, gotEvents)
	})
	t.Run("equality check, less data then maxSize, make sure it is stil sorted", func(t *testing.T) {
		line1 := eventLineFromTimeWithUID("2023-05-06", "id1")
		line2 := eventLineFromTimeWithUID("2023-05-06", "id2")
		line3 := eventLineFromTimeWithUID("2023-05-03", "id3")
		line4 := eventLineFromTimeWithUID("2023-05-08", "id4")
		line5 := eventLineFromTimeWithUID("2023-05-04", "id5")
		line6 := eventLineFromTimeWithUID("2023-05-01", "id6")
		unsortedLines := []string{line1, line2, line3, line4, line5, line6}
		sortedLines := []string{line6, line3, line5, line1, line2, line4}
		generatedExport, size := generateExportFilesFromLines(t, unsortedLines)
		defer generatedExport.Close()

		// Make sure that size is bigger, we want to be sure that we will sort also
		// if file will fit in one batch.
		maxSingleBatchSize := 10 * size
		sortedFile, err := createSortedExport(generatedExport, t.TempDir(), len(unsortedLines), maxSingleBatchSize)
		require.NoError(t, err)
		defer sortedFile.Close()

		bb, err := io.ReadAll(sortedFile)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(
			strings.Join(sortedLines, "\n")+"\n", /* add new line at the end, join does not add it */
			string(bb)))
	})
	t.Run("equality check, more data then maxSize", func(t *testing.T) {
		line1 := eventLineFromTimeWithUID("2023-05-06", "id1")
		line2 := eventLineFromTimeWithUID("2023-05-06", "id2")
		line3 := eventLineFromTimeWithUID("2023-05-03", "id3")
		line4 := eventLineFromTimeWithUID("2023-05-08", "id4")
		line5 := eventLineFromTimeWithUID("2023-05-04", "id5")
		line6 := eventLineFromTimeWithUID("2023-05-01", "id6")
		unsortedLines := []string{line1, line2, line3, line4, line5, line6}
		sortedLines := []string{line6, line3, line5, line1, line2, line4}
		generatedExport, size := generateExportFilesFromLines(t, unsortedLines)
		defer generatedExport.Close()

		// create at least 3 temporary files for external sorting.
		maxSingleBatchSize := size / 3
		sortedFile, err := createSortedExport(generatedExport, t.TempDir(), len(unsortedLines), maxSingleBatchSize)
		require.NoError(t, err)
		defer sortedFile.Close()

		bb, err := io.ReadAll(sortedFile)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(
			strings.Join(sortedLines, "\n")+"\n", /* add new line at the end, join does not add it */
			string(bb)))
	})
}

func generateExportFilesFromLines(t *testing.T, lines []string) (file *os.File, size int) {
	f, err := os.CreateTemp(t.TempDir(), "*")
	require.NoError(t, err)
	zw := gzip.NewWriter(f)
	for _, line := range lines {
		size += len(line)
		_, err = zw.Write([]byte(line + "\n"))
		require.NoError(t, err)
	}
	err = zw.Close()
	require.NoError(t, err)
	_, err = f.Seek(0, io.SeekStart)
	require.NoError(t, err)
	return f, size
}

func generateExportFileOfSize(t *testing.T, wantSize int, minDate, maxDate string) (*os.File, int) {
	f, err := os.CreateTemp(t.TempDir(), "*")
	require.NoError(t, err)
	require.NoError(t, err)
	zw := gzip.NewWriter(f)

	var noOfEvents, size int
	for size < wantSize {
		noOfEvents++
		line := eventLineFromTime(randomTime(t, minDate, maxDate).Format(time.DateOnly))
		size += len(line)
		_, err = zw.Write([]byte(line + "\n"))
		require.NoError(t, err)
	}
	err = zw.Close()
	require.NoError(t, err)
	_, err = f.Seek(0, io.SeekStart)
	require.NoError(t, err)
	return f, noOfEvents
}

func randomTime(t *testing.T, minStr, maxStr string) time.Time {
	min, err := time.Parse(time.DateOnly, minStr)
	require.NoError(t, err)
	max, err := time.Parse(time.DateOnly, maxStr)
	require.NoError(t, err)
	return min.Add(rand.N(max.Sub(min)))
}

func eventLineFromTime(eventTime string) string {
	return eventLineFromTimeWithUID(eventTime, uuid.NewString())
}

func eventLineFromTimeWithUID(eventTime, uid string) string {
	// Generate event close to 1KB.
	event := fmt.Sprintf(
		`{
			"Item":{
				"EventIndex":{
					"N":"2147483647"
				},
				"SessionID":{
					"S":"4298bd54-a747-4d53-b850-83ba17caae5a"
				},
				"CreatedAtDate":{
					"S":"%s"
				},
				"FieldsMap":{
					"M":{
						"cluster_name":{
							"S":"%s"
						},
						"uid":{
							"S":"%s"
						},
						"code":{
							"S":"T2005I"
						},
						"ei":{
							"N":"2147483647"
						},
						"time":{
							"S":"2023-05-22T12:12:21.966Z"
						},
						"event":{
							"S":"session.upload"
						},
						"sid":{
							"S":"4298bd54-a747-4d53-b850-83ba17caae5a"
						}
					}
				},
				"EventType":{
					"S":"session.upload"
				},
				"EventNamespace":{
					"S":"default"
				},
				"CreatedAt":{
					"N":"1684757541"
				}
			}
		}`,
		eventTime, strings.Repeat("a", 1024), uid)
	return strings.ReplaceAll(strings.ReplaceAll(event, "\t", ""), "\n", "")
}
