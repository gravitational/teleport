/*
Copyright 2015-2018 Gravitational, Inc.

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

package dynamo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

const tableName = "teleport.dynamo.test"

func ensureTestsEnabled(t *testing.T) {
	const varName = "TELEPORT_DYNAMODB_TEST"
	if os.Getenv(varName) == "" {
		t.Skipf("DynamoDB tests are disabled. Enable by defining the %v environment variable", varName)
	}
}

func TestDynamoDB(t *testing.T) {
	ensureTestsEnabled(t)

	dynamoCfg := map[string]interface{}{
		"table_name":         tableName,
		"poll_stream_period": 300 * time.Millisecond,
	}

	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clockwork.FakeClock, error) {
		testCfg, err := test.ApplyOptions(options)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		if testCfg.MirrorMode {
			return nil, nil, test.ErrMirrorNotSupported
		}

		// This would seem to be a bad thing for dynamo to omit
		if testCfg.ConcurrentBackend != nil {
			return nil, nil, test.ErrConcurrentAccessNotSupported
		}

		uut, err := New(context.Background(), dynamoCfg)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		clock := clockwork.NewFakeClockAt(time.Now())
		uut.clock = clock
		return uut, clock, nil
	}

	test.RunBackendComplianceSuite(t, newBackend)
}

type dynamoDBAPIMock struct {
	dynamodbiface.DynamoDBAPI

	expectedTableName   string
	expectedBillingMode string
	expectedPthroughput *dynamodb.ProvisionedThroughput
}

func (d *dynamoDBAPIMock) CreateTableWithContext(_ aws.Context, input *dynamodb.CreateTableInput, opts ...request.Option) (*dynamodb.CreateTableOutput, error) {

	if d.expectedTableName != aws.StringValue(input.TableName) {
		return nil, trace.BadParameter("table names do not match")
	}

	if d.expectedBillingMode != aws.StringValue(input.BillingMode) {
		return nil, trace.BadParameter("billing mode does not match")
	}

	if d.expectedPthroughput != nil {
		if aws.StringValue(input.BillingMode) == dynamodb.BillingModePayPerRequest {
			return nil, trace.BadParameter("pthroughput should be nil if on demand is true")
		}

		if aws.Int64Value(d.expectedPthroughput.ReadCapacityUnits) != aws.Int64Value(input.ProvisionedThroughput.ReadCapacityUnits) ||
			aws.Int64Value(d.expectedPthroughput.WriteCapacityUnits) != aws.Int64Value(input.ProvisionedThroughput.WriteCapacityUnits) {

			return nil, trace.BadParameter("pthroughput values were not equal")
		}
	}

	return nil, nil
}

func (d *dynamoDBAPIMock) WaitUntilTableExistsWithContext(_ aws.Context, input *dynamodb.DescribeTableInput, _ ...request.WaiterOption) error {
	if d.expectedTableName != aws.StringValue(input.TableName) {
		return trace.BadParameter("table names do not match")
	}
	return nil
}

func TestCreateTable(t *testing.T) {
	mock := dynamoDBAPIMock{
		expectedBillingMode: dynamodb.BillingModePayPerRequest,
		expectedTableName:   "table",
	}
	backend := &Backend{
		Entry: log.NewEntry(log.New()),
		Config: Config{
			OnDemand: aws.Bool(true),
		},
		svc: &mock,
	}

	// passes as all fields are correct
	err := backend.createTable(context.Background(), "table", "hello")
	require.NoError(t, err)

	// pass as pthroughput should not get set even if the capacity
	// units are set
	backend.ReadCapacityUnits = 10
	backend.WriteCapacityUnits = 10
	err = backend.createTable(context.Background(), "table", "hello")
	require.NoError(t, err)

	backend.OnDemand = aws.Bool(false)
	mock.expectedPthroughput = &dynamodb.ProvisionedThroughput{
		ReadCapacityUnits:  aws.Int64(10),
		WriteCapacityUnits: aws.Int64(10),
	}

	err = backend.createTable(context.Background(), "table", "hello")
	require.True(t, trace.IsBadParameter(err))

	mock.expectedBillingMode = dynamodb.BillingModeProvisioned

	err = backend.createTable(context.Background(), "table", "hello")
	require.NoError(t, err)

}
