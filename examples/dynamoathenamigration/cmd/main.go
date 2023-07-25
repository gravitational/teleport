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
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/examples/dynamoathenamigration"
)

func main() {
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
	debug := flag.Bool("d", false, "debug logs")
	flag.Parse()

	level := log.InfoLevel
	if *debug {
		level = log.DebugLevel
	}
	logger := log.New()
	logger.SetLevel(level)

	cfg := dynamoathenamigration.Config{
		ExportARN:       *exportARN,
		DynamoTableARN:  *dynamoARN,
		DryRun:          *dryRun,
		NoOfEmitWorkers: *noOfEmitWorker,
		TopicARN:        *snsTopicARN,
		ExportLocalDir:  *exportLocalDir,
		Logger:          logger,
	}
	var err error
	if *timeStr != "" {
		cfg.ExportTime, err = time.Parse(time.RFC3339, *timeStr)
		if err != nil {
			logger.Fatal(err)
		}
	}

	if *s3exportPath != "" {
		u, err := url.Parse(*s3exportPath)
		if err != nil {
			logger.Fatal(err)
		}

		if u.Scheme != "s3" {
			logger.Fatal("invalid scheme for s3 export path")
		}
		cfg.Bucket = u.Host
		cfg.Prefix = strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/")
	}

	if *s3largePayloadsPath != "" {
		u, err := url.Parse(*s3largePayloadsPath)
		if err != nil {
			logger.Fatal(err)
		}

		if u.Scheme != "s3" {
			logger.Fatal("invalid scheme for s3 large payloads path")
		}
		cfg.LargePayloadBucket = u.Host
		cfg.LargePayloadPrefix = strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/")
	}

	if *checkpointPath != "" {
		cfg.CheckpointPath = *checkpointPath
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := dynamoathenamigration.Migrate(ctx, cfg); err != nil {
		logger.Fatal(err)
	}
}
