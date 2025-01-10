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

package athena

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/protoadapt"

	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestConfig_SetFromURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    Config
		wantErr string
	}{
		{
			name: "params to emiter",
			url:  "athena://db.tbl/?topicArn=arn:topic&largeEventsS3=s3://large-events-bucket",
			want: Config{
				TableName:     "tbl",
				Database:      "db",
				TopicARN:      "arn:topic",
				LargeEventsS3: "s3://large-events-bucket",
			},
		},
		{
			name: "params to querier - part 1",
			url:  "athena://db.tbl/?locationS3=s3://events-bucket&queryResultsS3=s3://results-bucket&workgroup=default",
			want: Config{
				TableName:      "tbl",
				Database:       "db",
				LocationS3:     "s3://events-bucket",
				QueryResultsS3: "s3://results-bucket",
				Workgroup:      "default",
			},
		},
		{
			name: "params to querier - part 2",
			url:  "athena://db.tbl/?getQueryResultsInterval=200ms&limiterRefillAmount=2&&limiterRefillTime=2s&limiterBurst=3&disableSearchCostOptimization=true",
			want: Config{
				TableName:                     "tbl",
				Database:                      "db",
				GetQueryResultsInterval:       200 * time.Millisecond,
				LimiterRefillAmount:           2,
				LimiterRefillTime:             2 * time.Second,
				LimiterBurst:                  3,
				DisableSearchCostOptimization: true,
			},
		},
		{
			name: "params to batcher",
			url:  "athena://db.tbl/?queueURL=https://queueURL&batchMaxItems=1000&batchMaxInterval=10s&consumerLockName=mylock&consumerDisabled=true",
			want: Config{
				TableName:        "tbl",
				Database:         "db",
				QueueURL:         "https://queueURL",
				BatchMaxItems:    1000,
				BatchMaxInterval: 10 * time.Second,
				ConsumerLockName: "mylock",
				ConsumerDisabled: true,
			},
		},
		{
			name:    "invalid database/table format",
			url:     "athena://dsa.dsa.dsa",
			wantErr: "invalid athena address, supported format is 'athena://database.table'",
		},
		{
			name:    "invalid limiterRefillAmount format",
			url:     "athena://db.tbl/?limiterRefillAmount=abc",
			wantErr: "invalid limiterRefillAmount value (it must be int)",
		},
		{
			name: "region param",
			url:  "athena://db.tbl/?region=fake-region",
			want: Config{
				TableName: "tbl",
				Database:  "db",
				Region:    "fake-region",
			},
		},
		{
			name: "no region param",
			url:  "athena://db.tbl",
			want: Config{
				TableName: "tbl",
				Database:  "db",
				Region:    "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			u, err := url.Parse(tt.url)
			require.NoError(t, err, "Failed to parse url")
			err = cfg.SetFromURL(u)
			if tt.wantErr == "" {
				require.NoError(t, err, "SetFromURL return unexpected err")
				require.Empty(t, cmp.Diff(tt.want, *cfg, cmpopts.EquateApprox(0, 0.0001), cmpopts.IgnoreUnexported(Config{})))
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestConfig_CheckAndSetDefaults(t *testing.T) {
	type mockBackend struct {
		backend.Backend
	}

	dummyAWSCfg := &aws.Config{}
	validConfig := Config{
		Database:                   "db",
		TableName:                  "tbl",
		TopicARN:                   "arn:topic",
		LargeEventsS3:              "s3://large-payloads-bucket",
		LocationS3:                 "s3://events-bucket",
		QueueURL:                   "https://queue-url",
		PublisherConsumerAWSConfig: dummyAWSCfg,
		Backend:                    mockBackend{},
		metrics:                    &athenaMetrics{},
	}
	tests := []struct {
		name    string
		input   func() Config
		want    Config
		wantErr string
	}{
		{
			name: "minimum config with defaults",
			input: func() Config {
				return validConfig
			},
			want: Config{
				Database:                   "db",
				TableName:                  "tbl",
				TopicARN:                   "arn:topic",
				LargeEventsS3:              "s3://large-payloads-bucket",
				largeEventsBucket:          "large-payloads-bucket",
				LocationS3:                 "s3://events-bucket",
				locationS3Bucket:           "events-bucket",
				QueueURL:                   "https://queue-url",
				GetQueryResultsInterval:    100 * time.Millisecond,
				BatchMaxItems:              20000,
				BatchMaxInterval:           1 * time.Minute,
				ConsumerLockName:           "",
				PublisherConsumerAWSConfig: dummyAWSCfg,
				StorerQuerierAWSConfig:     dummyAWSCfg,
				Backend:                    mockBackend{},
			},
		},
		{
			name: "valid config with limiter, check defaults refillTime",
			input: func() Config {
				cfg := validConfig
				cfg.LimiterBurst = 10
				cfg.LimiterRefillAmount = 5
				return cfg
			},
			want: Config{
				Database:                   "db",
				TableName:                  "tbl",
				TopicARN:                   "arn:topic",
				LargeEventsS3:              "s3://large-payloads-bucket",
				largeEventsBucket:          "large-payloads-bucket",
				LocationS3:                 "s3://events-bucket",
				locationS3Bucket:           "events-bucket",
				QueueURL:                   "https://queue-url",
				GetQueryResultsInterval:    100 * time.Millisecond,
				BatchMaxItems:              20000,
				BatchMaxInterval:           1 * time.Minute,
				ConsumerLockName:           "",
				PublisherConsumerAWSConfig: dummyAWSCfg,
				StorerQuerierAWSConfig:     dummyAWSCfg,
				Backend:                    mockBackend{},
				LimiterRefillTime:          1 * time.Second,
				LimiterBurst:               10,
				LimiterRefillAmount:        5,
			},
		},
		{
			name: "missing table name",
			input: func() Config {
				cfg := validConfig
				cfg.TableName = ""
				return cfg
			},
			wantErr: "TableName is not specified",
		},
		{
			name: "invalid table name",
			input: func() Config {
				cfg := validConfig
				cfg.TableName = "table with space"
				return cfg
			},
			wantErr: "TableName can only contain alphanumeric or underscore character",
		},
		{
			name: "missing topicARN",
			input: func() Config {
				cfg := validConfig
				cfg.TopicARN = ""
				return cfg
			},
			wantErr: "TopicARN is not specified",
		},
		{
			name: "missing LocationS3",
			input: func() Config {
				cfg := validConfig
				cfg.LocationS3 = ""
				return cfg
			},
			wantErr: "LocationS3 is not specified",
		},
		{
			name: "invalid LocationS3",
			input: func() Config {
				cfg := validConfig
				cfg.LocationS3 = "https://abc"
				return cfg
			},
			wantErr: "LocationS3 must starts with s3://",
		},
		{
			name: "missing QueueURL",
			input: func() Config {
				cfg := validConfig
				cfg.QueueURL = ""
				return cfg
			},
			wantErr: "QueueURL is not specified",
		},
		{
			name: "invalid QueueURL",
			input: func() Config {
				cfg := validConfig
				cfg.QueueURL = "s3://abc"
				return cfg
			},
			wantErr: "QueueURL must be valid url and start with https",
		},
		{
			name: "invalid LimiterBurst and LimiterRefillAmount combination",
			input: func() Config {
				cfg := validConfig
				cfg.LimiterBurst = 0
				cfg.LimiterRefillAmount = 2
				return cfg
			},
			wantErr: "LimiterBurst must be greater than 0 if LimiterRefillAmount is used",
		},
		{
			name: "invalid LimiterRefillAmount and LimiterBurst combination",
			input: func() Config {
				cfg := validConfig
				cfg.LimiterBurst = 3
				cfg.LimiterRefillAmount = 0
				return cfg
			},
			wantErr: "LimiterRefillAmount must be greater than 0 if LimiterBurst is used",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.input()
			err := cfg.CheckAndSetDefaults(context.Background())
			if tt.wantErr == "" {
				require.NoError(t, err, "CheckAndSetDefaults return unexpected err")
				require.Empty(t, cmp.Diff(tt.want, cfg, cmpopts.EquateApprox(0, 0.0001), cmpopts.IgnoreFields(Config{}, "Clock", "UIDGenerator", "Logger", "Tracer", "metrics", "ObserveWriteEventsError"), cmp.AllowUnexported(Config{})))
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestPublisherConsumer(t *testing.T) {
	smallEvent := &apievents.AppCreate{
		Metadata: apievents.Metadata{
			ID:   uuid.NewString(),
			Time: time.Now().UTC(),
			Type: events.AppCreateEvent,
		},
		AppMetadata: apievents.AppMetadata{
			AppName: "app-small",
		},
	}

	largeEvent := &apievents.AppCreate{
		Metadata: apievents.Metadata{
			ID:   uuid.NewString(),
			Time: time.Now().UTC(),
			Type: events.AppCreateEvent,
			Code: strings.Repeat("d", 2*maxDirectMessageSize),
		},
		AppMetadata: apievents.AppMetadata{
			AppName: "app-large",
		},
	}

	eventWithoutID := &apievents.AppCreate{
		Metadata: apievents.Metadata{
			Time: time.Now().UTC(),
			Type: events.AppCreateEvent,
		},
		AppMetadata: apievents.AppMetadata{
			AppName: "app-small",
		},
	}
	eventWithoutTime := &apievents.AppCreate{
		Metadata: apievents.Metadata{
			ID:   uuid.NewString(),
			Type: events.AppCreateEvent,
		},
		AppMetadata: apievents.AppMetadata{
			AppName: "app-small",
		},
	}
	tests := []struct {
		name  string
		input interface {
			apievents.AuditEvent
			protoadapt.MessageV1
		}
		assertFn func(t *testing.T, got apievents.AuditEvent, s3fake *fakeS3manager)
	}{
		{
			name:  "standard event via sns",
			input: smallEvent,
			assertFn: func(t *testing.T, got apievents.AuditEvent, s3fake *fakeS3manager) {
				require.Empty(t, cmp.Diff(smallEvent, got))
				// no calls via s3.
				require.Equal(t, 0, s3fake.uploadCount)
			},
		},
		{
			name:  "large via s3",
			input: largeEvent,
			assertFn: func(t *testing.T, got apievents.AuditEvent, s3fake *fakeS3manager) {
				require.Empty(t, cmp.Diff(largeEvent, got))
				// S3 for uplodad should be called only once.
				require.Equal(t, 1, s3fake.uploadCount)
			},
		},
		{
			name: "missing event id",
			// Input event is modified during emitting even if some fields are missing.
			// We are using clone to be able to do proper assertion.
			input: eventWithoutID,
			assertFn: func(t *testing.T, got apievents.AuditEvent, s3fake *fakeS3manager) {
				require.Empty(t, cmp.Diff(eventWithoutID, got, cmpopts.IgnoreFields(apievents.Metadata{}, "ID")))
				// ID should be set by emitting.
				require.NotEmpty(t, got.GetID())
				// no calls via s3.
				require.Equal(t, 0, s3fake.uploadCount)
			},
		},
		{
			name:  "missing event time",
			input: eventWithoutTime,
			assertFn: func(t *testing.T, got apievents.AuditEvent, s3fake *fakeS3manager) {
				require.Empty(t, cmp.Diff(eventWithoutTime, got, cmpopts.IgnoreFields(apievents.Metadata{}, "Time")))
				// Time should be set by emitting.
				require.NotEmpty(t, got.GetTime())
				// no calls via s3.
				require.Equal(t, 0, s3fake.uploadCount)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fS3 := newFakeS3manager()
			fq := newFakeQueue()
			p := &publisher{
				PublisherConfig: PublisherConfig{
					MessagePublisher: fq,
					Uploader:         fS3,
				},
			}
			cfg := validCollectCfgForTests(t)
			cfg.sqsReceiver = fq
			cfg.payloadDownloader = fS3
			cfg.batchMaxItems = 1
			require.NoError(t, cfg.CheckAndSetDefaults())
			c := newSqsMessagesCollector(cfg)

			eventsChan := c.getEventsChan()

			ctx := context.Background()
			readSQSCtx, readCancel := context.WithCancel(ctx)
			defer readCancel()

			go c.fromSQS(readSQSCtx)

			// receiver is used to read messages from eventsChan.
			r := &receiver{}
			go r.Do(eventsChan)

			// we need to clone input because it's used for assertions and
			// EmitAuditEvent modifies input in case id/time is empty.
			clonedInput := apiutils.CloneProtoMsg(tt.input)
			err := p.EmitAuditEvent(ctx, clonedInput)
			require.NoError(t, err)

			// wait for event to be propagated.
			require.Eventually(t, func() bool {
				return len(r.GetMsgs()) == 1
			}, 1*time.Second, 10*time.Millisecond, "missing events, got %d", len(r.GetMsgs()))

			tt.assertFn(t, eventAndAckIDToAuditEvents(r.GetMsgs())[0], fS3)
		})
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

type fakeS3manager struct {
	objects     map[string][]byte
	uploadCount int
}

func newFakeS3manager() *fakeS3manager {
	return &fakeS3manager{
		objects: map[string][]byte{},
	}
}

func (f *fakeS3manager) Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error) {
	data, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	f.objects[*input.Key] = data
	f.uploadCount++
	return &manager.UploadOutput{Key: input.Key}, nil
}

func (f *fakeS3manager) Download(ctx context.Context, w io.WriterAt, input *s3.GetObjectInput, options ...func(*manager.Downloader)) (int64, error) {
	data, ok := f.objects[*input.Key]
	if !ok {
		return 0, errors.New("object not found")
	}
	n, err := w.WriteAt(data, 0)
	if err != nil {
		return 0, err
	}
	return int64(n), nil
}
