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

package athena

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
)

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
			url:  "athena://db.tbl/?getQueryResultsInterval=200ms&limiterRate=0.642&limiterBurst=3",
			want: Config{
				TableName:               "tbl",
				Database:                "db",
				GetQueryResultsInterval: 200 * time.Millisecond,
				LimiterRate:             0.642,
				LimiterBurst:            3,
			},
		},
		{
			name: "params to batcher",
			url:  "athena://db.tbl/?queueURL=https://queueURL&batchMaxItems=1000&batchMaxInterval=10s",
			want: Config{
				TableName:        "tbl",
				Database:         "db",
				QueueURL:         "https://queueURL",
				BatchMaxItems:    1000,
				BatchMaxInterval: 10 * time.Second,
			},
		},
		{
			name:    "invalid database/table format",
			url:     "athena://dsa.dsa.dsa",
			wantErr: "invalid athena address, supported format is 'athena://database.table'",
		},
		{
			name:    "invalid limiterRate format",
			url:     "athena://db.tbl/?limiterRate=abc",
			wantErr: "invalid limiterRate value (it must be float32)",
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
	validConfig := Config{
		Database:      "db",
		TableName:     "tbl",
		TopicARN:      "arn:topic",
		LargeEventsS3: "s3://large-payloads-bucket",
		LocationS3:    "s3://events-bucket",
		QueueURL:      "https://queue-url",
		AWSConfig:     &aws.Config{},
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
				Database:                "db",
				TableName:               "tbl",
				TopicARN:                "arn:topic",
				LargeEventsS3:           "s3://large-payloads-bucket",
				largeEventsBucket:       "large-payloads-bucket",
				LocationS3:              "s3://events-bucket",
				QueueURL:                "https://queue-url",
				GetQueryResultsInterval: 100 * time.Millisecond,
				BatchMaxItems:           20000,
				BatchMaxInterval:        1 * time.Minute,
				AWSConfig:               &aws.Config{},
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
			wantErr: "TableName can contains only alphanumeric or underscore character",
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
			wantErr: "LocationS3 must be valid url and start with s3",
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
			name: "invalid LimiterBurst and LimiterRate combination",
			input: func() Config {
				cfg := validConfig
				cfg.LimiterBurst = 0
				cfg.LimiterRate = 2.5
				return cfg
			},
			wantErr: "LimiterBurst must be greater than 0 if LimiterRate is used",
		},
		{
			name: "invalid LimiterRate and LimiterBurst combination",
			input: func() Config {
				cfg := validConfig
				cfg.LimiterBurst = 3
				cfg.LimiterRate = 0
				return cfg
			},
			wantErr: "LimiterRate must be greater than 0 if LimiterBurst is used",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.input()
			err := cfg.CheckAndSetDefaults(context.Background())
			if tt.wantErr == "" {
				require.NoError(t, err, "CheckAndSetDefaults return unexpected err")
				require.Empty(t, cmp.Diff(tt.want, cfg, cmpopts.EquateApprox(0, 0.0001), cmpopts.IgnoreFields(Config{}, "Clock", "UIDGenerator", "LogEntry"), cmp.AllowUnexported(Config{})))
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}
