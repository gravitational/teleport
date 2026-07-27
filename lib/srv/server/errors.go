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

package server

import (
	"errors"
	"fmt"

	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/gravitational/trace"

	utilsaws "github.com/gravitational/teleport/lib/utils/aws"
)

// EC2IAMPermissionError is a structured IAM permission error from EC2 discovery.
type EC2IAMPermissionError struct {
	// Integration is the AWS integration used by discovery.
	Integration string
	// AccountID is the AWS account ID associated with the permission error.
	AccountID string
	// Region is the AWS region associated with the permission error, if known.
	Region string
	// IssueType is the UserTask issue type for the permission error.
	IssueType string
	// DiscoveryConfigName is the DiscoveryConfig name that produced the permission error.
	DiscoveryConfigName string
	// CallerARN is the AWS caller identity ARN used for the failed discovery call.
	CallerARN string
	// Err is the underlying AWS or Teleport error.
	Err error
}

// Error formats the permission error as a human-readable string, including the
// integration and region (if set), issue type, account ID, and underlying error.
func (e *EC2IAMPermissionError) Error() string {
	integrationPrefix := ""
	if e.Integration != "" {
		integrationPrefix = fmt.Sprintf("integration %s: ", e.Integration)
	}

	if e.Region != "" {
		return fmt.Sprintf("%sIAM permission error (%s) for account %s in region %s: %v",
			integrationPrefix, e.IssueType, e.AccountID, e.Region, e.Err)
	}

	return fmt.Sprintf("%sIAM permission error (%s) for account %s: %v",
		integrationPrefix, e.IssueType, e.AccountID, e.Err)
}

// Unwrap returns the underlying error that triggered the permission failure.
// This is typically an AWS SDK request error.
func (e *EC2IAMPermissionError) Unwrap() error {
	return e.Err
}

// EC2IAMPermissionErrors returns EC2 IAM permission errors found in err.
func EC2IAMPermissionErrors(err error) []*EC2IAMPermissionError {
	if err == nil {
		return nil
	}

	var aggregate trace.Aggregate
	if errors.As(err, &aggregate) {
		var permissionErrors []*EC2IAMPermissionError
		for _, err := range aggregate.Errors() {
			permissionErrors = append(permissionErrors, EC2IAMPermissionErrors(err)...)
		}
		return permissionErrors
	}

	var permissionError *EC2IAMPermissionError
	if errors.As(err, &permissionError) {
		return []*EC2IAMPermissionError{permissionError}
	}

	return nil
}

func isEC2DiscoveryPermissionError(err error) bool {
	if trace.IsAccessDenied(err) {
		return true
	}

	var invalidIdentityTokenErr *ststypes.InvalidIdentityTokenException
	return errors.As(err, &invalidIdentityTokenErr)
}

// accountIDFromRoleARN extracts the AWS account ID from a role ARN.
// It returns "unknown" when the account ID cannot be determined.
func accountIDFromRoleARN(roleARN string) string {
	parsed, err := utilsaws.ParseRoleARN(roleARN)
	if err != nil {
		return "unknown"
	}

	return parsed.AccountID
}
