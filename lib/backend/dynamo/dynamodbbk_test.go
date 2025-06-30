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

package dynamo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/smithy-go/middleware"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/integrations/lib/backoff"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func ensureTestsEnabled(t *testing.T) {
	t.Helper()
	const varName = "TELEPORT_DYNAMODB_TEST"
	if os.Getenv(varName) == "" {
		t.Skipf("DynamoDB tests are disabled. Enable by defining the %v environment variable", varName)
	}
}

func dynamoDBTestTable() string {
	if t := os.Getenv("TELEPORT_DYNAMODB_TEST_TABLE"); t != "" {
		return t
	}

	return "teleport.dynamo.test"
}

func TestDynamoDB(t *testing.T) {
	ensureTestsEnabled(t)

	dynamoCfg := map[string]any{
		"table_name":         dynamoDBTestTable(),
		"poll_stream_period": 300 * time.Millisecond,
	}

	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clocki.FakeClock, error) {
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
	dynamoClient
	expectedProvisionedthroughput *types.ProvisionedThroughput
	expectedTableName             string
	expectedBillingMode           types.BillingMode
}

func (d *dynamoDBAPIMock) CreateTable(ctx context.Context, input *dynamodb.CreateTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error) {
	if d.expectedTableName != aws.ToString(input.TableName) {
		return nil, trace.BadParameter("table names do not match")
	}

	if d.expectedBillingMode != input.BillingMode {
		return nil, trace.BadParameter("billing mode does not match")
	}

	if d.expectedProvisionedthroughput != nil {
		if input.BillingMode == types.BillingModePayPerRequest {
			return nil, trace.BadParameter("pthroughput should be nil if on demand is true")
		}

		if aws.ToInt64(d.expectedProvisionedthroughput.ReadCapacityUnits) != aws.ToInt64(input.ProvisionedThroughput.ReadCapacityUnits) ||
			aws.ToInt64(d.expectedProvisionedthroughput.WriteCapacityUnits) != aws.ToInt64(input.ProvisionedThroughput.WriteCapacityUnits) {

			return nil, trace.BadParameter("pthroughput values were not equal")
		}
	}

	return nil, nil
}

func (d *dynamoDBAPIMock) DescribeTable(ctx context.Context, input *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	if d.expectedTableName != aws.ToString(input.TableName) {
		return nil, trace.BadParameter("table names do not match")
	}
	return &dynamodb.DescribeTableOutput{
		Table: &types.TableDescription{
			TableName:   input.TableName,
			TableStatus: types.TableStatusActive,
		},
		ResultMetadata: middleware.Metadata{},
	}, nil
}

func TestCreateTable(t *testing.T) {
	const tableName = "table"

	errIsNil := func(err error) bool { return err == nil }

	for _, tc := range []struct {
		errorIsFn                     func(error) bool
		expectedProvisionedThroughput *types.ProvisionedThroughput
		name                          string
		expectedBillingMode           types.BillingMode
		billingMode                   billingMode
		readCapacityUnits             int
		writeCapacityUnits            int
	}{
		{
			name:                "table creation succeeds",
			errorIsFn:           errIsNil,
			billingMode:         billingModePayPerRequest,
			expectedBillingMode: types.BillingModePayPerRequest,
		},
		{
			name:                "read/write capacity units are ignored if on demand is on",
			readCapacityUnits:   10,
			writeCapacityUnits:  10,
			errorIsFn:           errIsNil,
			billingMode:         billingModePayPerRequest,
			expectedBillingMode: types.BillingModePayPerRequest,
		},
		{
			name:               "bad parameter when provisioned throughput is set",
			readCapacityUnits:  10,
			writeCapacityUnits: 10,
			errorIsFn:          trace.IsBadParameter,
			expectedProvisionedThroughput: &types.ProvisionedThroughput{
				ReadCapacityUnits:  aws.Int64(10),
				WriteCapacityUnits: aws.Int64(10),
			},
			billingMode:         billingModePayPerRequest,
			expectedBillingMode: types.BillingModePayPerRequest,
		},
		{
			name:               "bad parameter when the incorrect billing mode is set",
			readCapacityUnits:  10,
			writeCapacityUnits: 10,
			errorIsFn:          trace.IsBadParameter,
			expectedProvisionedThroughput: &types.ProvisionedThroughput{
				ReadCapacityUnits:  aws.Int64(10),
				WriteCapacityUnits: aws.Int64(10),
			},
			billingMode:         billingModePayPerRequest,
			expectedBillingMode: types.BillingModePayPerRequest,
		},
		{
			name:               "create table succeeds",
			readCapacityUnits:  10,
			writeCapacityUnits: 10,
			errorIsFn:          errIsNil,
			expectedProvisionedThroughput: &types.ProvisionedThroughput{
				ReadCapacityUnits:  aws.Int64(10),
				WriteCapacityUnits: aws.Int64(10),
			},
			billingMode:         billingModeProvisioned,
			expectedBillingMode: types.BillingModeProvisioned,
		},
	} {

		ctx := context.Background()
		t.Run(tc.name, func(t *testing.T) {
			mock := dynamoDBAPIMock{
				expectedBillingMode:           tc.expectedBillingMode,
				expectedTableName:             tableName,
				expectedProvisionedthroughput: tc.expectedProvisionedThroughput,
			}
			b := &Backend{
				logger: slog.With(teleport.ComponentKey, BackendName),
				Config: Config{
					BillingMode:        tc.billingMode,
					ReadCapacityUnits:  int64(tc.readCapacityUnits),
					WriteCapacityUnits: int64(tc.writeCapacityUnits),
				},

				svc: &mock,
			}

			err := b.createTable(ctx, aws.String(tableName), "_")
			require.True(t, tc.errorIsFn(err), err)
		})
	}
}

// TestContinuousBackups verifies that the continuous backup state is set upon
// startup of DynamoDB.
func TestContinuousBackups(t *testing.T) {
	ensureTestsEnabled(t)

	b, err := New(t.Context(), map[string]any{
		"table_name":         uuid.NewString() + "-test",
		"continuous_backups": true,
	})
	require.NoError(t, err)

	// Remove table after tests are done.
	t.Cleanup(func() {
		back := backoff.NewDecorr(500*time.Millisecond, 20*time.Second, clockwork.NewRealClock())
		for {
			err := deleteTable(context.Background(), b.svc, b.Config.TableName)
			if err == nil {
				return
			}
			inUse := &types.ResourceInUseException{}
			if errors.As(err, &inUse) {
				back.Do(context.Background())
			} else {
				assert.FailNow(t, "error deleting table", err)
			}
		}
	})

	// Check status of continuous backups.
	ok, err := getContinuousBackups(context.Background(), b.svc, b.Config.TableName)
	require.NoError(t, err)
	require.True(t, ok)
}

// TestAutoScaling verifies that auto scaling is enabled upon startup of DynamoDB.
func TestAutoScaling(t *testing.T) {
	ensureTestsEnabled(t)

	// Create new backend with auto scaling enabled.
	b, err := New(context.Background(), map[string]any{
		"table_name":         uuid.NewString() + "-test",
		"auto_scaling":       true,
		"read_min_capacity":  10,
		"read_max_capacity":  20,
		"read_target_value":  50.0,
		"write_min_capacity": 10,
		"write_max_capacity": 20,
		"write_target_value": 50.0,
		// Billing mode must be set to provisioned mode for the
		// auto-scaling option to be respected.
		"billing_mode": billingModeProvisioned,
	})
	require.NoError(t, err)

	// Remove table after tests are done.
	t.Cleanup(func() {
		require.NoError(t, deleteTable(context.Background(), b.svc, b.Config.TableName))
	})

	awsConfig, err := config.LoadDefaultConfig(context.Background())
	require.NoError(t, err)

	expected := &AutoScalingParams{
		ReadMinCapacity:  10,
		ReadMaxCapacity:  20,
		ReadTargetValue:  50.0,
		WriteMinCapacity: 10,
		WriteMaxCapacity: 20,
		WriteTargetValue: 50.0,
	}

	// Check auto scaling values match.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		resp, err := getAutoScaling(context.Background(), applicationautoscaling.NewFromConfig(awsConfig), b.Config.TableName)
		assert.NoError(t, err)
		assert.Equal(t, expected, resp)
	}, 10*time.Second, 500*time.Millisecond)
}

// getContinuousBackups gets the state of continuous backups.
func getContinuousBackups(ctx context.Context, svc dynamoClient, tableName string) (bool, error) {
	resp, err := svc.DescribeContinuousBackups(ctx, &dynamodb.DescribeContinuousBackupsInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return false, convertError(err)
	}

	switch resp.ContinuousBackupsDescription.PointInTimeRecoveryDescription.PointInTimeRecoveryStatus {
	case types.PointInTimeRecoveryStatusEnabled:
		return true, nil
	case types.PointInTimeRecoveryStatusDisabled:
		return false, nil
	default:
		return false, trace.BadParameter("dynamo returned unknown state for continuous backups: %v",
			resp.ContinuousBackupsDescription.PointInTimeRecoveryDescription.PointInTimeRecoveryStatus)
	}
}

type AutoScalingParams struct {
	// ReadMaxCapacity is the maximum provisioned read capacity.
	ReadMaxCapacity int32
	// ReadMinCapacity is the minimum provisioned read capacity.
	ReadMinCapacity int32
	// ReadTargetValue is the ratio of consumed read to provisioned capacity.
	ReadTargetValue float64
	// WriteMaxCapacity is the maximum provisioned write capacity.
	WriteMaxCapacity int32
	// WriteMinCapacity is the minimum provisioned write capacity.
	WriteMinCapacity int32
	// WriteTargetValue is the ratio of consumed write to provisioned capacity.
	WriteTargetValue float64
}

// getAutoScaling gets the state of auto scaling.
func getAutoScaling(ctx context.Context, svc *applicationautoscaling.Client, tableName string) (*AutoScalingParams, error) {
	var resp AutoScalingParams
	tableResourceID := "table/" + tableName

	// Get scaling targets.
	targetResponse, err := svc.DescribeScalableTargets(ctx, &applicationautoscaling.DescribeScalableTargetsInput{
		ServiceNamespace: autoscalingtypes.ServiceNamespaceDynamodb,
		ResourceIds:      []string{tableResourceID},
	})
	if err != nil {
		return nil, convertError(err)
	}
	for _, target := range targetResponse.ScalableTargets {
		switch target.ScalableDimension {
		case autoscalingtypes.ScalableDimensionDynamoDBTableReadCapacityUnits:
			resp.ReadMinCapacity = aws.ToInt32(target.MinCapacity)
			resp.ReadMaxCapacity = aws.ToInt32(target.MaxCapacity)
		case autoscalingtypes.ScalableDimensionDynamoDBTableWriteCapacityUnits:
			resp.WriteMinCapacity = aws.ToInt32(target.MinCapacity)
			resp.WriteMaxCapacity = aws.ToInt32(target.MaxCapacity)
		}
	}

	// Get scaling policies.
	policyResponse, err := svc.DescribeScalingPolicies(ctx, &applicationautoscaling.DescribeScalingPoliciesInput{
		ServiceNamespace: autoscalingtypes.ServiceNamespaceDynamodb,
		ResourceId:       aws.String(tableResourceID),
	})
	if err != nil {
		return nil, convertError(err)
	}
	for i := range policyResponse.ScalingPolicies {
		policy := policyResponse.ScalingPolicies[i]
		switch aws.ToString(policy.PolicyName) {
		case fmt.Sprintf("%v-%v", tableName, readScalingPolicySuffix):
			resp.ReadTargetValue = aws.ToFloat64(policy.TargetTrackingScalingPolicyConfiguration.TargetValue)
		case fmt.Sprintf("%v-%v", tableName, writeScalingPolicySuffix):
			resp.WriteTargetValue = aws.ToFloat64(policy.TargetTrackingScalingPolicyConfiguration.TargetValue)
		}
	}

	return &resp, nil
}

// deleteTable will remove a table.
func deleteTable(ctx context.Context, svc dynamoClient, tableName string) error {
	_, err := svc.DeleteTable(ctx, &dynamodb.DeleteTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return convertError(err)
	}

	waiter := dynamodb.NewTableNotExistsWaiter(svc)
	if err := waiter.Wait(ctx,
		&dynamodb.DescribeTableInput{
			TableName: aws.String(tableName),
		},
		10*time.Minute,
	); err != nil {
		return convertError(err)
	}
	return nil
}

const (
	readScalingPolicySuffix  = "read-target-tracking-scaling-policy"
	writeScalingPolicySuffix = "write-target-tracking-scaling-policy"
)

func TestKeyPrefix(t *testing.T) {
	t.Run("leading separator in key", func(t *testing.T) {
		prefixed := prependPrefix(backend.NewKey("test", "llama"))
		assert.Equal(t, "teleport/test/llama", prefixed)

		key := trimPrefix(prefixed)
		assert.Equal(t, "/test/llama", key.String())
	})

	t.Run("no leading separator in key", func(t *testing.T) {
		prefixed := prependPrefix(backend.KeyFromString(".locks/test/llama"))
		assert.Equal(t, "teleport.locks/test/llama", prefixed)

		key := trimPrefix(prefixed)
		assert.Equal(t, ".locks/test/llama", key.String())
	})
}
