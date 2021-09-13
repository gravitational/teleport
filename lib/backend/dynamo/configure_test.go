// +build dynamodb

/*
Copyright 2020 Gravitational, Inc.

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
	"fmt"
	"testing"

	"github.com/gravitational/trace"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
)

// TestContinuousBackups verifies that the continuous backup state is set upon
// startup of DynamoDB.
func TestContinuousBackups(t *testing.T) {
	// Create new backend with continuous backups enabled.
	b, err := New(context.Background(), map[string]interface{}{
		"table_name":         uuid.New() + "-test",
		"continuous_backups": true,
	})
	require.NoError(t, err)

	// Remove table after tests are done.
	t.Cleanup(func() {
		deleteTable(context.Background(), b.svc, b.Config.TableName)
	})

	// Check status of continuous backups.
	ok, err := getContinuousBackups(context.Background(), b.svc, b.Config.TableName)
	require.NoError(t, err)
	require.True(t, ok)
}

// TestAutoScaling verifies that auto scaling is enabled upon startup of DynamoDB.
func TestAutoScaling(t *testing.T) {
	// Create new backend with auto scaling enabled.
	b, err := New(context.Background(), map[string]interface{}{
		"table_name":         uuid.New() + "-test",
		"auto_scaling":       true,
		"read_min_capacity":  10,
		"read_max_capacity":  20,
		"read_target_value":  50.0,
		"write_min_capacity": 10,
		"write_max_capacity": 20,
		"write_target_value": 50.0,
	})
	require.NoError(t, err)

	// Remove table after tests are done.
	t.Cleanup(func() {
		deleteTable(context.Background(), b.svc, b.Config.TableName)
	})

	// Check auto scaling values match.
	resp, err := getAutoScaling(context.Background(), applicationautoscaling.New(b.session), b.Config.TableName)
	require.NoError(t, err)
	require.Equal(t, resp, &AutoScalingParams{
		ReadMinCapacity:  10,
		ReadMaxCapacity:  20,
		ReadTargetValue:  50.0,
		WriteMinCapacity: 10,
		WriteMaxCapacity: 20,
		WriteTargetValue: 50.0,
	})
}

// getContinuousBackups gets the state of continuous backups.
func getContinuousBackups(ctx context.Context, svc *dynamodb.DynamoDB, tableName string) (bool, error) {
	resp, err := svc.DescribeContinuousBackupsWithContext(ctx, &dynamodb.DescribeContinuousBackupsInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return false, convertError(err)
	}

	switch *resp.ContinuousBackupsDescription.PointInTimeRecoveryDescription.PointInTimeRecoveryStatus {
	case string(dynamodb.ContinuousBackupsStatusEnabled):
		return true, nil
	case string(dynamodb.ContinuousBackupsStatusDisabled):
		return false, nil
	default:
		return false, trace.BadParameter("dynamo returned unknown state for continuous backups: %v",
			*resp.ContinuousBackupsDescription.PointInTimeRecoveryDescription.PointInTimeRecoveryStatus)
	}
}

// getAutoScaling gets the state of auto scaling.
func getAutoScaling(ctx context.Context, svc *applicationautoscaling.ApplicationAutoScaling, tableName string) (*AutoScalingParams, error) {
	var resp AutoScalingParams

	// Get scaling targets.
	targetResponse, err := svc.DescribeScalableTargets(&applicationautoscaling.DescribeScalableTargetsInput{
		ServiceNamespace: aws.String(applicationautoscaling.ServiceNamespaceDynamodb),
	})
	if err != nil {
		return nil, convertError(err)
	}
	for _, target := range targetResponse.ScalableTargets {
		switch *target.ScalableDimension {
		case applicationautoscaling.ScalableDimensionDynamodbTableReadCapacityUnits:
			resp.ReadMinCapacity = *target.MinCapacity
			resp.ReadMaxCapacity = *target.MaxCapacity
		case applicationautoscaling.ScalableDimensionDynamodbTableWriteCapacityUnits:
			resp.WriteMinCapacity = *target.MinCapacity
			resp.WriteMaxCapacity = *target.MaxCapacity
		}
	}

	// Get scaling policies.
	policyResponse, err := svc.DescribeScalingPolicies(&applicationautoscaling.DescribeScalingPoliciesInput{
		ServiceNamespace: aws.String(applicationautoscaling.ServiceNamespaceDynamodb),
	})
	if err != nil {
		return nil, convertError(err)
	}
	for _, policy := range policyResponse.ScalingPolicies {
		switch *policy.PolicyName {
		case fmt.Sprintf("%v-%v", tableName, readScalingPolicySuffix):
			resp.ReadTargetValue = *policy.TargetTrackingScalingPolicyConfiguration.TargetValue
		case fmt.Sprintf("%v-%v", tableName, writeScalingPolicySuffix):
			resp.WriteTargetValue = *policy.TargetTrackingScalingPolicyConfiguration.TargetValue
		}
	}

	return &resp, nil
}

// deleteTable will remove a table.
func deleteTable(ctx context.Context, svc *dynamodb.DynamoDB, tableName string) error {
	_, err := svc.DeleteTableWithContext(ctx, &dynamodb.DeleteTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return convertError(err)
	}
	err = svc.WaitUntilTableNotExistsWithContext(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return convertError(err)
	}
	return nil
}

const (
	readScalingPolicySuffix  = "read-target-tracking-scaling-policy"
	writeScalingPolicySuffix = "write-target-tracking-scaling-policy"
	resourcePrefix           = "table"
)
