/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package fetchers

import (
	"errors"
	"fmt"

	"github.com/gravitational/trace"
)

const (
	// EKSDiscoveryOperationDescribeCluster identifies discovery of one known cluster.
	EKSDiscoveryOperationDescribeCluster = "eks:DescribeCluster"
)

// EKSDiscoveryPermissionError describes an IAM permission failure encountered
// while discovering EKS clusters. Region and cluster are empty when the
// failure happened before that scope was known.
type EKSDiscoveryPermissionError struct {
	// Integration is the AWS integration used by discovery.
	Integration string
	// AccountID is the AWS account ID, or "unknown" when it cannot be derived.
	AccountID string
	// Region is the affected AWS region, when known.
	Region string
	// Cluster is the affected EKS cluster, when known.
	Cluster string
	// Operation is the denied AWS API operation.
	Operation string
	// DiscoveryConfigName is the DiscoveryConfig that produced the error.
	DiscoveryConfigName string
	// Err is the underlying access-denied error.
	Err error
}

func (e *EKSDiscoveryPermissionError) Error() string {
	integrationPrefix := ""
	if e.Integration != "" {
		integrationPrefix = fmt.Sprintf("integration %s: ", e.Integration)
	}
	scope := "AWS account"
	if e.AccountID != "" {
		scope += " " + e.AccountID
	}
	if e.Region != "" {
		scope += fmt.Sprintf(" in region %s", e.Region)
	}
	if e.Cluster != "" {
		scope += fmt.Sprintf(" for cluster %s", e.Cluster)
	}
	return fmt.Sprintf("%slacks %s permission for %s: %v", integrationPrefix, e.Operation, scope, e.Err)
}

// Unwrap returns the AWS access-denied error that caused discovery to fail.
func (e *EKSDiscoveryPermissionError) Unwrap() error {
	return e.Err
}

// EKSDiscoveryPermissionErrors returns all structured EKS permission errors
// contained in err, including errors joined in a trace aggregate.
func EKSDiscoveryPermissionErrors(err error) []*EKSDiscoveryPermissionError {
	if err == nil {
		return nil
	}

	var aggregate trace.Aggregate
	if errors.As(err, &aggregate) {
		var permissionErrors []*EKSDiscoveryPermissionError
		for _, aggregateErr := range aggregate.Errors() {
			permissionErrors = append(permissionErrors, EKSDiscoveryPermissionErrors(aggregateErr)...)
		}
		return permissionErrors
	}

	var permissionError *EKSDiscoveryPermissionError
	if errors.As(err, &permissionError) {
		return []*EKSDiscoveryPermissionError{permissionError}
	}
	return nil
}
