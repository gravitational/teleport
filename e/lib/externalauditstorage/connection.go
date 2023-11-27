/*
Copyright 2023 Gravitational, Inc.

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

package externalauditstorage

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/externalauditstorage"
)

const (
	dummyFileName = "_connection-test"
	//empty string md5 base64 encoded
	contentMD5                  = "1B2M2Y8AsgTpgAmY7PhCfg=="
	queryExecutionTimeout       = time.Minute
	queryExecutionInterval      = 100 * time.Millisecond
	queryExecutionIntervalDelay = 300 * time.Millisecond
)

// ConnectionTestAthenaClient is a subset of [athena.Client] methods needed for athena test connection.
type ConnectionTestAthenaClient interface {
	// Runs the SQL query statements contained in the Query
	StartQueryExecution(ctx context.Context, params *athena.StartQueryExecutionInput, optFns ...func(*athena.Options)) (*athena.StartQueryExecutionOutput, error)
	// Returns information about a single execution of a query if you have access to the workgroup in which the query ran.
	GetQueryExecution(ctx context.Context, params *athena.GetQueryExecutionInput, optFns ...func(*athena.Options)) (*athena.GetQueryExecutionOutput, error)
	// Streams the results of a single query execution specified by QueryExecutionId from the Athena query results location in Amazon S3.
	GetQueryResults(ctx context.Context, params *athena.GetQueryResultsInput, optFns ...func(*athena.Options)) (*athena.GetQueryResultsOutput, error)
}

// ConnectionTestGlueClient is a subset of [glue.Client] methods needed for athena test connection.
type ConnectionTestGlueClient interface {
	// Retrieves the Table definition in a Data Catalog for a specified table.
	GetTable(ctx context.Context, params *glue.GetTableInput, optFns ...func(*glue.Options)) (*glue.GetTableOutput, error)
}

// ConnectionTestS3Client is a subset of [s3.Client] methods needed for athena test connection.
type ConnectionTestS3Client interface {
	// Adds an object to a bucket.
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	//Retrieves objects from Amazon S3.
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// ConnectionTestBuckets performs tests against External Audit Storage buckets.
// * Upload dummy file to S3 for both storage buckets.
// * Download dummy file from S3 for session storage bucket.
func ConnectionTestBuckets(ctx context.Context, clt ConnectionTestS3Client, spec *externalauditstorage.ExternalAuditStorageSpec) error {
	if spec == nil {
		return trace.BadParameter("spec is required parameter")
	}

	sessionBucket, sessionPrefix, err := parseS3URI(spec.SessionRecordingsURI)
	if err != nil {
		return trace.Wrap(err, "failed to parse session recordings URI")
	}

	// Test Session Recording Bucket
	if _, err = clt.PutObject(ctx, &s3.PutObjectInput{
		Bucket:     &sessionBucket,
		Key:        aws.String(sessionPrefix + dummyFileName),
		ContentMD5: aws.String(contentMD5),
	}); err != nil {
		return trace.Wrap(err, "failed to write dummy file to sessions bucket")
	}

	if _, err = clt.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &sessionBucket,
		Key:    aws.String(sessionPrefix + dummyFileName),
	}); err != nil {
		return trace.Wrap(err, "failed to retrieve dummy file from sessions bucket")
	}

	auditBucket, auditPrefix, err := parseS3URI(spec.AuditEventsLongTermURI)
	if err != nil {
		return trace.Wrap(err, "failed to parse audit events URI")
	}

	// Test Audit Events Bucket
	if _, err = clt.PutObject(ctx, &s3.PutObjectInput{
		Bucket:     &auditBucket,
		Key:        aws.String(auditPrefix + dummyFileName),
		ContentMD5: aws.String(contentMD5), //empty string md5 base
	}); err != nil {
		return trace.Wrap(err, "failed to write dummy file to events bucket")
	}

	return nil
}

// ConnectionTestGlue checks that the glue table exists and we have permission to access it.
func ConnectionTestGlue(ctx context.Context, clt ConnectionTestGlueClient, spec *externalauditstorage.ExternalAuditStorageSpec) error {
	// Verify glue table exists
	_, err := clt.GetTable(ctx, &glue.GetTableInput{
		DatabaseName: &spec.GlueDatabase,
		Name:         &spec.GlueTable,
	})
	return trace.Wrap(err, "failed to get glue table")
}

// ConnectionTestAthena performs a small query against the provided athena workgroup and glue database and table.
// The query polls for the results and returns them. This tests that audit events bucket can be accessed through an athena
// query.
func ConnectionTestAthena(ctx context.Context, clt ConnectionTestAthenaClient, spec *externalauditstorage.ExternalAuditStorageSpec) error {
	// Test Athena Queries
	startQueryExecutionOutput, err := clt.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
		QueryString: aws.String(fmt.Sprintf("select uid, event_time, event_data FROM %s limit 1;", spec.GlueTable)),
		QueryExecutionContext: &athenatypes.QueryExecutionContext{
			Database: &spec.GlueDatabase,
		},
		WorkGroup: &spec.AthenaWorkgroup,
		ResultConfiguration: &athenatypes.ResultConfiguration{
			OutputLocation: &spec.AthenaResultsURI,
		},
	})
	if err != nil {
		return trace.Wrap(err, "failed to start query execution")
	}

	if err := waitForResults(ctx, clt, *startQueryExecutionOutput.QueryExecutionId); err != nil {
		return trace.Wrap(err, "error waiting for results to be ready")
	}

	if _, err := clt.GetQueryResults(ctx, &athena.GetQueryResultsInput{
		QueryExecutionId: startQueryExecutionOutput.QueryExecutionId,
		MaxResults:       aws.Int32(1),
	}); err != nil {
		return trace.Wrap(err, "error getting query results")
	}

	return nil
}

// waitForResults of athena query for [queryExecutionTimeout] minutes checking the status every [queryExecutionInterval]
// before downloading the results.
func waitForResults(ctx context.Context, clt ConnectionTestAthenaClient, executionID string) error {
	ctx, cancel := context.WithTimeout(ctx, queryExecutionTimeout)
	defer cancel()
	for i := 0; ; i++ {
		interval := queryExecutionInterval
		if i == 0 {
			interval = queryExecutionIntervalDelay
		}
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-time.After(interval):
		}

		resp, err := clt.GetQueryExecution(ctx, &athena.GetQueryExecutionInput{QueryExecutionId: &executionID})
		if err != nil {
			return trace.Wrap(err)
		}
		state := resp.QueryExecution.Status.State
		switch state {
		case athenatypes.QueryExecutionStateSucceeded:
			return nil
		case athenatypes.QueryExecutionStateQueued, athenatypes.QueryExecutionStateRunning:
			continue
		default:
			return trace.Errorf("got unexpected state: %s from queryID: %s", state, executionID)
		}
	}
}

// parseS3URI parses and extracts the provided s3 url bucket and prefix names
// the prefix returned is an empty string unless the URL path isn't empty in which case
// the results are trimmed and a / is appended to the suffix.
func parseS3URI(uri string) (host string, prefix string, err error) {
	parsedURI, err := url.Parse(uri)
	if err != nil {
		return "", "", trace.Wrap(err, "parsing provided uri")
	}

	prefix = ""
	if parsedURI.Path != "" {
		prefix = strings.Trim(parsedURI.Path, "/") + "/"
	}
	return parsedURI.Host, prefix, nil
}
