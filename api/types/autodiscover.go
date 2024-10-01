/*
Copyright 2024 Gravitational, Inc.

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

package types

// List of Auto Discover EC2 issues identifiers.
// This value is used to populate the UserTasks.Spec.IssueType for Discover EC2 tasks.
// The Web UI will then use those identifiers to show detailed instructions on how to fix the issue.
const (
	// AutoDiscoverEC2IssueEICEFailedToCreateNode is used when the EICE flow fails to create a node.
	// This can happen when the Node does not have a valid PrivateIPAddress.
	// This is very unlekly and should only happen if the AWS API returns an unexpected response.
	AutoDiscoverEC2IssueEICEFailedToCreateNode = "ec2-eice-create-node"

	// AutoDiscoverEC2IssueEICEFailedToUpsertNode is used when the EICE flow fails to upsert a node into the cluster.
	// This is very unlekly and should only happen
	// - if the Discovery system role was changed
	// - if the Node resource validation was changed on the Auth and not on the DiscoveryService
	// - or because of a network error
	AutoDiscoverEC2IssueEICEFailedToUpsertNode = "ec2-eice-upsert-node"

	// AutoDiscoverEC2IssueScriptInstanceNotRegistered is used to identify instances that failed to auto-enroll
	// because they are not present in Amazon Systems Manager.
	// This usually means that the Instance does not have the SSM Agent running,
	// or that the instance's IAM Profile does not allow have the managed IAM Policy AmazonSSMManagedInstanceCore assigned to it.
	AutoDiscoverEC2IssueScriptInstanceNotRegistered = "ec2-ssm-agent-not-registered"

	// AutoDiscoverEC2IssueScriptInstanceConnectionLost is used to identify instances that failed to auto-enroll
	// because the agent lost connection to Amazon Systems Manager.
	// This can happen if the user changed some setting in the instance's network or IAM profile.
	AutoDiscoverEC2IssueScriptInstanceConnectionLost = "ec2-ssm-agent-connection-lost"

	// AutoDiscoverEC2IssueScriptInstanceUnsupportedOS is used to identify instances that failed to auto-enroll
	// because its OS is not supported by teleport.
	// This can happen if the instance is running Windows.
	AutoDiscoverEC2IssueScriptInstanceUnsupportedOS = "ec2-ssm-unsupported-os"

	// AutoDiscoverEC2IssueScriptFailure is used to identify instances that failed to auto-enroll
	// because the installation script failed.
	// The invocation url must be included in the report, so that users can see what was wrong.
	AutoDiscoverEC2IssueScriptFailure = "ec2-ssm-script-failure"

	// AutoDiscoverEC2IssueInvocationFailure is used to identify instances that failed to auto-enroll
	// because the SSM Script Run (also known as Invocation) failed.
	// This happens when there's a failure with permissions or an invalid configuration (eg, invalid document name).
	AutoDiscoverEC2IssueInvocationFailure = "ec2-ssm-invocation-failure"
)
