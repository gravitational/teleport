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

	"github.com/gravitational/trace"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

// TODO(russjones): All functions within this file are only called by tests.
// Since CI does not run with the +dynamodb build tag and therefore does not
// build the test files, the linter complains about unused code.
//
// To fix this, and to make tests easier to run, add support for local
// DynamoDB [1]. Once that's done the +dynamodb build flag can be run as it will
// allow OSS users to run tests without incurring any costs.
//
// [1] https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/DynamoDBLocal.html

// getContinuousBackups gets the state of continuous backups.
func (b *Backend) getContinuousBackups(ctx context.Context) (bool, error) {
	resp, err := b.svc.DescribeContinuousBackupsWithContext(ctx, &dynamodb.DescribeContinuousBackupsInput{
		TableName: aws.String(b.TableName),
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

type autoScalingResponse struct {
	readMaxCapacity  int
	readMinCapacity  int
	readTargetValue  float64
	writeMaxCapacity int
	writeMinCapacity int
	writeTargetValue float64
}

// getAutoScaling gets the state of auto scaling.
func (b *Backend) getAutoScaling(ctx context.Context) (*autoScalingResponse, error) {
	svc := applicationautoscaling.New(b.session)

	var resp autoScalingResponse

	// Get scaling targets.
	targetResponse, err := svc.DescribeScalableTargets(&applicationautoscaling.DescribeScalableTargetsInput{
		ServiceNamespace: aws.String("ecs"),
	})
	if err != nil {
		return nil, convertError(err)
	}
	for _, target := range targetResponse.ScalableTargets {
		switch *target.ScalableDimension {
		case applicationautoscaling.ScalableDimensionDynamodbTableReadCapacityUnits:
			resp.readMinCapacity = int(*target.MinCapacity)
			resp.readMaxCapacity = int(*target.MinCapacity)
		case applicationautoscaling.ScalableDimensionDynamodbTableWriteCapacityUnits:
			resp.writeMinCapacity = int(*target.MinCapacity)
			resp.writeMaxCapacity = int(*target.MinCapacity)
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
		case fmt.Sprintf("%v-%v", b.TableName, readScalingPolicySuffix):
			resp.readTargetValue = *policy.TargetTrackingScalingPolicyConfiguration.TargetValue
		case fmt.Sprintf("%v-%v", b.TableName, writeScalingPolicySuffix):
			resp.writeTargetValue = *policy.TargetTrackingScalingPolicyConfiguration.TargetValue
		}
	}

	return &resp, nil
}
