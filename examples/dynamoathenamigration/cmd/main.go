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

package main

import (
	"context"
	"flag"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gravitational/teleport/examples/dynamoathenamigration"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	timeStr := flag.String("exportTime", "", "exportTime is time (RFC3339 format) in the past from which to export table data, empty for the current time")
	exportARN := flag.String("exportARN", "", "exportARN allows to reuse already finished export without triggering new")
	dynamoARN := flag.String("dynamoARN", "", "ARN of DynamoDB table to export")
	s3exportPath := flag.String("exportPath", "", "S3 address where export should be placed, in format s3://bucket/optional_prefix")
	snsTopicARN := flag.String("snsTopicARN", "", "SNS topic ARN configured in athena logger")
	s3largePayloadsPath := flag.String("largePayloadsPath", "", "S3 address configured in athena logger for placing large events payloads, in format s3://bucket/optional_prefix")
	dryRun := flag.Bool("dryRun", false, "dryRun means export will be triggered and verified, however events won't be published on SNS topic")
	noOfEmitWorker := flag.Int("noOfEmitWorker", 5, "noOfEmitWorker defines number of workers emitting events to athena logger")
	checkpointPath := flag.String("checkpointPath", "", "checkpointPath defines where checkpoint file will be stored")
	exportLocalDir := flag.String("exportLocalDir", "", "exportLocalDir defines directory where export will be downloaded")
	maxMemoryUseDuringSort := flag.Int("maxMem", dynamoathenamigration.DefaultMaxMemoryUsedForSortingExportInMB, "maximum memory used during sorting of events in MB")
	debug := flag.Bool("d", false, "debug logs")
	flag.Parse()

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	logger := slog.New(logutils.NewSlogTextHandler(os.Stdout, logutils.SlogTextHandlerConfig{Level: level}))

	cfg := dynamoathenamigration.Config{
		ExportARN:                         *exportARN,
		DynamoTableARN:                    *dynamoARN,
		DryRun:                            *dryRun,
		NoOfEmitWorkers:                   *noOfEmitWorker,
		TopicARN:                          *snsTopicARN,
		ExportLocalDir:                    *exportLocalDir,
		MaxMemoryUsedForSortingExportInMB: *maxMemoryUseDuringSort,
		Logger:                            logger,
	}
	var err error
	if *timeStr != "" {
		cfg.ExportTime, err = time.Parse(time.RFC3339, *timeStr)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to parse export time", "error", err)
			os.Exit(1)
		}
	}

	if *s3exportPath != "" {
		u, err := url.Parse(*s3exportPath)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to parse s3 export path", "error", err)
			os.Exit(1)
		}

		if u.Scheme != "s3" {
			logger.ErrorContext(ctx, "invalid scheme for s3 export path", "error", err)
			os.Exit(1)
		}
		cfg.Bucket = u.Host
		cfg.Prefix = strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/")
	}

	if *s3largePayloadsPath != "" {
		u, err := url.Parse(*s3largePayloadsPath)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to parse s3 large payloads path", "error", err)
			os.Exit(1)
		}

		if u.Scheme != "s3" {
			logger.ErrorContext(ctx, "invalid scheme for s3 large payloads path", "error", err)
			os.Exit(1)
		}
		cfg.LargePayloadBucket = u.Host
		cfg.LargePayloadPrefix = strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/")
	}

	if *checkpointPath != "" {
		cfg.CheckpointPath = *checkpointPath
	}

	if err := dynamoathenamigration.Migrate(ctx, cfg); err != nil {
		logger.ErrorContext(ctx, "migration failed", "error", err)
		os.Exit(1)
	}
}
