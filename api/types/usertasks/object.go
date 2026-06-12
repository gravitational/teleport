/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package usertasks

import (
	"encoding/binary"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
)

// UserTaskOption defines a function that mutates a User Task.
type UserTaskOption func(ut *usertasksv1.UserTask)

// WithExpiration sets the expiration of the UserTask resource.
func WithExpiration(t time.Time) func(ut *usertasksv1.UserTask) {
	return func(ut *usertasksv1.UserTask) {
		ut.Metadata.Expires = timestamppb.New(t)
	}
}

// NewDiscoverEC2UserTask creates a new DiscoverEC2 User Task Type.
func NewDiscoverEC2UserTask(spec *usertasksv1.UserTaskSpec, opts ...UserTaskOption) (*usertasksv1.UserTask, error) {
	taskName := TaskNameForDiscoverEC2(TaskNameForDiscoverEC2Parts{
		Integration:     spec.GetIntegration(),
		IssueType:       spec.GetIssueType(),
		AccountID:       spec.GetDiscoverEc2().GetAccountId(),
		Region:          spec.GetDiscoverEc2().GetRegion(),
		SSMDocument:     spec.GetDiscoverEc2().GetSsmDocument(),
		InstallerScript: spec.GetDiscoverEc2().GetInstallerScript(),
	})

	ut := &usertasksv1.UserTask{
		Kind:    types.KindUserTask,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: taskName,
		},
		Spec: spec,
	}
	for _, o := range opts {
		o(ut)
	}

	if err := ValidateUserTask(ut); err != nil {
		return nil, trace.Wrap(err)
	}

	return ut, nil
}

// NewDiscoverEKSUserTask creates a new DiscoverEKS User Task Type.
func NewDiscoverEKSUserTask(spec *usertasksv1.UserTaskSpec, opts ...UserTaskOption) (*usertasksv1.UserTask, error) {
	taskName := TaskNameForDiscoverEKS(TaskNameForDiscoverEKSParts{
		Integration:     spec.GetIntegration(),
		IssueType:       spec.GetIssueType(),
		AccountID:       spec.GetDiscoverEks().GetAccountId(),
		Region:          spec.GetDiscoverEks().GetRegion(),
		AppAutoDiscover: spec.GetDiscoverEks().GetAppAutoDiscover(),
	})

	ut := &usertasksv1.UserTask{
		Kind:    types.KindUserTask,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: taskName,
		},
		Spec: spec,
	}
	for _, o := range opts {
		o(ut)
	}

	if err := ValidateUserTask(ut); err != nil {
		return nil, trace.Wrap(err)
	}

	return ut, nil
}

// NewDiscoverRDSUserTask creates a new DiscoverRDS User Task Type.
func NewDiscoverRDSUserTask(spec *usertasksv1.UserTaskSpec, opts ...UserTaskOption) (*usertasksv1.UserTask, error) {
	taskName := TaskNameForDiscoverRDS(TaskNameForDiscoverRDSParts{
		Integration: spec.GetIntegration(),
		IssueType:   spec.GetIssueType(),
		AccountID:   spec.GetDiscoverRds().GetAccountId(),
		Region:      spec.GetDiscoverRds().GetRegion(),
	})

	ut := &usertasksv1.UserTask{
		Kind:    types.KindUserTask,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: taskName,
		},
		Spec: spec,
	}
	for _, o := range opts {
		o(ut)
	}

	if err := ValidateUserTask(ut); err != nil {
		return nil, trace.Wrap(err)
	}

	return ut, nil
}

const (
	// TaskStateOpen identifies an issue with an instance that is not yet resolved.
	TaskStateOpen = "OPEN"
	// TaskStateResolved identifies an issue with an instance that is resolved.
	TaskStateResolved = "RESOLVED"
)

var validTaskStates = []string{TaskStateOpen, TaskStateResolved}

const (
	// TaskTypeDiscoverEC2 identifies a User Tasks that is created
	// when an auto-enrollment of an EC2 instance fails.
	// UserTasks that have this Task Type must include the DiscoverEC2 field.
	TaskTypeDiscoverEC2 = "discover-ec2"

	// TaskTypeDiscoverEKS identifies a User Tasks that is created
	// when an auto-enrollment of an EKS cluster fails.
	// UserTasks that have this Task Type must include the DiscoverEKS field.
	TaskTypeDiscoverEKS = "discover-eks"

	// TaskTypeDiscoverRDS identifies a User Tasks that is created
	// when an auto-enrollment of an RDS database fails or needs attention.
	// UserTasks that have this Task Type must include the DiscoverRDS field.
	TaskTypeDiscoverRDS = "discover-rds"
)

// List of Auto Discover EC2 issues identifiers.
// This value is used to populate the UserTasks.Spec.IssueType for Discover EC2 tasks.
// The Web UI will then use those identifiers to show detailed instructions on how to fix the issue.
const (
	// AutoDiscoverEC2IssueSSMInstanceNotRegistered is used to identify instances that failed to auto-enroll
	// because they are not present in Amazon Systems Manager.
	// This usually means that the Instance does not have the SSM Agent running,
	// or that the instance's IAM Profile does not allow have the managed IAM Policy AmazonSSMManagedInstanceCore assigned to it.
	AutoDiscoverEC2IssueSSMInstanceNotRegistered = "ec2-ssm-agent-not-registered"

	// AutoDiscoverEC2IssueSSMInstanceConnectionLost is used to identify instances that failed to auto-enroll
	// because the agent lost connection to Amazon Systems Manager.
	// This can happen if the user changed some setting in the instance's network or IAM profile.
	AutoDiscoverEC2IssueSSMInstanceConnectionLost = "ec2-ssm-agent-connection-lost"

	// AutoDiscoverEC2IssueSSMInstanceUnsupportedOS is used to identify instances that failed to auto-enroll
	// because its OS is not supported by teleport.
	// This can happen if the instance is running Windows.
	AutoDiscoverEC2IssueSSMInstanceUnsupportedOS = "ec2-ssm-unsupported-os"

	// AutoDiscoverEC2IssueSSMScriptFailure is used to identify instances that failed to auto-enroll
	// because the installation script failed.
	// The invocation url must be included in the report, so that users can see what was wrong.
	AutoDiscoverEC2IssueSSMScriptFailure = "ec2-ssm-script-failure"

	// AutoDiscoverEC2IssueSSMInvocationFailure is used to identify instances that failed to auto-enroll
	// because the SSM Script Run (also known as Invocation) failed.
	// This happens when there's a failure with permissions or an invalid configuration (eg, invalid document name).
	AutoDiscoverEC2IssueSSMInvocationFailure = "ec2-ssm-invocation-failure"
)

// DiscoverEC2IssueTypes is a list of issue types that can occur when trying to auto enroll EC2 instances.
var DiscoverEC2IssueTypes = []string{
	AutoDiscoverEC2IssueSSMInstanceNotRegistered,
	AutoDiscoverEC2IssueSSMInstanceConnectionLost,
	AutoDiscoverEC2IssueSSMInstanceUnsupportedOS,
	AutoDiscoverEC2IssueSSMScriptFailure,
	AutoDiscoverEC2IssueSSMInvocationFailure,
}

// List of Auto Discover EKS issues identifiers.
// This value is used to populate the UserTasks.Spec.IssueType for Discover EKS tasks.
const (
	// AutoDiscoverEKSIssueStatusNotActive is used to identify clusters that failed to auto-enroll
	// because their Status is not Active.
	AutoDiscoverEKSIssueStatusNotActive = "eks-status-not-active"
	// AutoDiscoverEKSIssueMissingEndpoingPublicAccess is used to identify clusters that failed to auto-enroll
	// because they don't have a public endpoint and this Teleport Cluster is running in Teleport Cloud.
	AutoDiscoverEKSIssueMissingEndpoingPublicAccess = "eks-missing-endpoint-public-access"
	// AutoDiscoverEKSIssueAuthenticationModeUnsupported is used to identify clusters that failed to auto-enroll
	// because their Authentication Mode is not supported.
	// Accepted values are API and API_AND_CONFIG_MAP.
	AutoDiscoverEKSIssueAuthenticationModeUnsupported = "eks-authentication-mode-unsupported"
	// AutoDiscoverEKSIssueClusterUnreachable is used to identify clusters that failed to auto-enroll
	// because Teleport Cluster is not able to reach the cluster's API.
	// Similar to AutoDiscoverEKSIssueMissingEndpoingPublicAccess, which is only used when Teleport is running in Teleport Cloud.
	AutoDiscoverEKSIssueClusterUnreachable = "eks-cluster-unreachable"
	// AutoDiscoverEKSIssueAgentNotConnecting is used to identify clusters that Teleport tried to
	// install the HELM chart but the Kube Agent is not connecting to Teleport.
	// This can be a transient issue (eg kube agent is in the process of joining), or some non-recoverable issue.
	// To get more information, users can follow the following link:
	// https://<region>.console.aws.amazon.com/eks/home?#/clusters/<cluster-name>/statefulsets/teleport-kube-agent?namespace=teleport-agent
	AutoDiscoverEKSIssueAgentNotConnecting = "eks-agent-not-connecting"
)

// DiscoverEKSIssueTypes is a list of issue types that can occur when trying to auto enroll EKS clusters.
var DiscoverEKSIssueTypes = []string{
	AutoDiscoverEKSIssueStatusNotActive,
	AutoDiscoverEKSIssueMissingEndpoingPublicAccess,
	AutoDiscoverEKSIssueAuthenticationModeUnsupported,
	AutoDiscoverEKSIssueClusterUnreachable,
	AutoDiscoverEKSIssueAgentNotConnecting,
}

// List of Auto Discover RDS issues identifiers.
// This value is used to populate the UserTasks.Spec.IssueType for Discover RDS tasks.
const (
	// AutoDiscoverRDSIssueIAMAuthenticationDisabled is used to identify databases that won't be
	// accessible because IAM Authentication is not enabled.
	AutoDiscoverRDSIssueIAMAuthenticationDisabled = "rds-iam-auth-disabled"
)

// DiscoverRDSIssueTypes is a list of issue types that can occur when trying to auto enroll RDS databases.
var DiscoverRDSIssueTypes = []string{
	AutoDiscoverRDSIssueIAMAuthenticationDisabled,
}

// ValidateUserTask validates the UserTask object without modifying it.
func ValidateUserTask(ut *usertasksv1.UserTask) error {
	switch {
	case ut.GetKind() != types.KindUserTask:
		return trace.BadParameter("invalid kind")
	case ut.GetVersion() != types.V1:
		return trace.BadParameter("invalid version")
	case ut.GetSubKind() != "":
		return trace.BadParameter("invalid sub kind, must be empty")
	case ut.GetMetadata() == nil:
		return trace.BadParameter("user task metadata is nil")
	case ut.Metadata.GetName() == "":
		return trace.BadParameter("user task name is empty")
	case ut.GetSpec() == nil:
		return trace.BadParameter("user task spec is nil")
	case !slices.Contains(validTaskStates, ut.GetSpec().State):
		return trace.BadParameter("invalid task state, allowed values: %v", validTaskStates)
	}

	switch ut.Spec.TaskType {
	case TaskTypeDiscoverEC2:
		if err := validateDiscoverEC2TaskType(ut); err != nil {
			return trace.Wrap(err)
		}
	case TaskTypeDiscoverEKS:
		if err := validateDiscoverEKSTaskType(ut); err != nil {
			return trace.Wrap(err)
		}
	case TaskTypeDiscoverRDS:
		if err := validateDiscoverRDSTaskType(ut); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("task type %q is not valid", ut.Spec.TaskType)
	}

	return nil
}

func validateDiscoverEC2TaskType(ut *usertasksv1.UserTask) error {
	if ut.GetSpec().Integration == "" {
		return trace.BadParameter("integration is required")
	}
	if ut.GetSpec().DiscoverEc2 == nil {
		return trace.BadParameter("%s requires the discover_ec2 field", TaskTypeDiscoverEC2)
	}
	if ut.GetSpec().DiscoverEc2.AccountId == "" {
		return trace.BadParameter("%s requires the discover_ec2.account_id field", TaskTypeDiscoverEC2)
	}
	if ut.GetSpec().DiscoverEc2.Region == "" {
		return trace.BadParameter("%s requires the discover_ec2.region field", TaskTypeDiscoverEC2)
	}

	expectedTaskName := TaskNameForDiscoverEC2(TaskNameForDiscoverEC2Parts{
		Integration:     ut.Spec.Integration,
		IssueType:       ut.Spec.IssueType,
		AccountID:       ut.Spec.DiscoverEc2.AccountId,
		Region:          ut.Spec.DiscoverEc2.Region,
		SSMDocument:     ut.Spec.DiscoverEc2.SsmDocument,
		InstallerScript: ut.Spec.DiscoverEc2.InstallerScript,
	})
	if ut.Metadata.GetName() != expectedTaskName {
		return trace.BadParameter("task name is pre-defined for discover-ec2 types, expected %q, got %q",
			expectedTaskName,
			ut.Metadata.GetName(),
		)
	}

	if !slices.Contains(DiscoverEC2IssueTypes, ut.GetSpec().IssueType) {
		return trace.BadParameter("invalid issue type state, allowed values: %v", DiscoverEC2IssueTypes)
	}

	if len(ut.Spec.DiscoverEc2.Instances) == 0 {
		return trace.BadParameter("at least one instance is required")
	}
	for instanceID, instanceIssue := range ut.Spec.DiscoverEc2.Instances {
		if instanceID == "" {
			return trace.BadParameter("instance id in discover_ec2.instances map is required")
		}
		if instanceIssue.InstanceId == "" {
			return trace.BadParameter("instance id in discover_ec2.instances field is required")
		}
		if instanceID != instanceIssue.InstanceId {
			return trace.BadParameter("instance id in discover_ec2.instances map and field are different")
		}
		if instanceIssue.DiscoveryConfig == "" {
			return trace.BadParameter("discovery config in discover_ec2.instances field is required")
		}
		if instanceIssue.DiscoveryGroup == "" {
			return trace.BadParameter("discovery group in discover_ec2.instances field is required")
		}
	}

	return nil
}

func validateDiscoverEKSTaskType(ut *usertasksv1.UserTask) error {
	if ut.GetSpec().Integration == "" {
		return trace.BadParameter("integration is required")
	}
	if ut.GetSpec().DiscoverEks == nil {
		return trace.BadParameter("%s requires the discover_eks field", TaskTypeDiscoverEKS)
	}
	if ut.GetSpec().DiscoverEks.AccountId == "" {
		return trace.BadParameter("%s requires the discover_eks.account_id field", TaskTypeDiscoverEKS)
	}
	if ut.GetSpec().DiscoverEks.Region == "" {
		return trace.BadParameter("%s requires the discover_eks.region field", TaskTypeDiscoverEKS)
	}

	expectedTaskName := TaskNameForDiscoverEKS(TaskNameForDiscoverEKSParts{
		Integration:     ut.Spec.Integration,
		IssueType:       ut.Spec.IssueType,
		AccountID:       ut.Spec.DiscoverEks.AccountId,
		Region:          ut.Spec.DiscoverEks.Region,
		AppAutoDiscover: ut.Spec.DiscoverEks.AppAutoDiscover,
	})
	if ut.Metadata.GetName() != expectedTaskName {
		return trace.BadParameter("task name is pre-defined for discover-eks types, expected %s, got %s",
			expectedTaskName,
			ut.Metadata.GetName(),
		)
	}

	if !slices.Contains(DiscoverEKSIssueTypes, ut.GetSpec().IssueType) {
		return trace.BadParameter("invalid issue type state, allowed values: %v", DiscoverEKSIssueTypes)
	}

	if len(ut.Spec.DiscoverEks.Clusters) == 0 {
		return trace.BadParameter("at least one cluster is required")
	}
	for clusterName, clusterIssue := range ut.Spec.DiscoverEks.Clusters {
		if clusterName == "" {
			return trace.BadParameter("cluster name in discover_eks.clusters map is required")
		}
		if clusterIssue.Name == "" {
			return trace.BadParameter("cluster name in discover_eks.clusters field is required")
		}
		if clusterName != clusterIssue.Name {
			return trace.BadParameter("cluster name in discover_eks.clusters map and field are different")
		}
		if clusterIssue.DiscoveryConfig == "" {
			return trace.BadParameter("discovery config in discover_eks.clusters field is required")
		}
		if clusterIssue.DiscoveryGroup == "" {
			return trace.BadParameter("discovery group in discover_eks.clusters field is required")
		}
	}

	return nil
}

func validateDiscoverRDSTaskType(ut *usertasksv1.UserTask) error {
	if ut.GetSpec().Integration == "" {
		return trace.BadParameter("integration is required")
	}
	if ut.GetSpec().GetDiscoverRds() == nil {
		return trace.BadParameter("%s requires the discover_rds field", TaskTypeDiscoverRDS)
	}
	if ut.GetSpec().GetDiscoverRds().AccountId == "" {
		return trace.BadParameter("%s requires the discover_rds.account_id field", TaskTypeDiscoverRDS)
	}
	if ut.GetSpec().GetDiscoverRds().Region == "" {
		return trace.BadParameter("%s requires the discover_rds.region field", TaskTypeDiscoverRDS)
	}

	expectedTaskName := TaskNameForDiscoverRDS(TaskNameForDiscoverRDSParts{
		Integration: ut.GetSpec().Integration,
		IssueType:   ut.GetSpec().IssueType,
		AccountID:   ut.GetSpec().GetDiscoverRds().AccountId,
		Region:      ut.GetSpec().GetDiscoverRds().Region,
	})
	if ut.GetMetadata().GetName() != expectedTaskName {
		return trace.BadParameter("task name is pre-defined for discover-rds types, expected %s, got %s",
			expectedTaskName,
			ut.Metadata.GetName(),
		)
	}

	if !slices.Contains(DiscoverRDSIssueTypes, ut.GetSpec().GetIssueType()) {
		return trace.BadParameter("invalid issue type state, allowed values: %v", DiscoverRDSIssueTypes)
	}

	if len(ut.GetSpec().GetDiscoverRds().GetDatabases()) == 0 {
		return trace.BadParameter("at least one database is required")
	}
	for databaseIdentifier, databaseIssue := range ut.GetSpec().GetDiscoverRds().GetDatabases() {
		if databaseIdentifier == "" {
			return trace.BadParameter("database identifier in discover_rds.databases map is required")
		}
		if databaseIssue.Name == "" {
			return trace.BadParameter("database identifier in discover_rds.databases field is required")
		}
		if databaseIdentifier != databaseIssue.Name {
			return trace.BadParameter("database identifier in discover_rds.databases map and field are different")
		}
		if databaseIssue.DiscoveryConfig == "" {
			return trace.BadParameter("discovery config in discover_rds.databases field is required")
		}
		if databaseIssue.DiscoveryGroup == "" {
			return trace.BadParameter("discovery group in discover_rds.databases field is required")
		}
	}

	return nil
}

// TaskNameForDiscoverEC2Parts are the fields that deterministically compute a Discover EC2 task name.
// To be used with TaskNameForDiscoverEC2 function.
type TaskNameForDiscoverEC2Parts struct {
	Integration     string
	IssueType       string
	AccountID       string
	Region          string
	SSMDocument     string
	InstallerScript string
}

// TaskNameForDiscoverEC2 returns a deterministic name for the DiscoverEC2 task type.
// This method is used to ensure a single UserTask is created to report issues in enrolling EC2 instances for a given integration, issue type, account id and region.
func TaskNameForDiscoverEC2(parts TaskNameForDiscoverEC2Parts) string {
	var bs []byte
	bs = append(bs, binary.LittleEndian.AppendUint64(nil, uint64(len(parts.Integration)))...)
	bs = append(bs, []byte(parts.Integration)...)
	bs = append(bs, binary.LittleEndian.AppendUint64(nil, uint64(len(parts.IssueType)))...)
	bs = append(bs, []byte(parts.IssueType)...)
	bs = append(bs, binary.LittleEndian.AppendUint64(nil, uint64(len(parts.AccountID)))...)
	bs = append(bs, []byte(parts.AccountID)...)
	bs = append(bs, binary.LittleEndian.AppendUint64(nil, uint64(len(parts.Region)))...)
	bs = append(bs, []byte(parts.Region)...)
	bs = append(bs, binary.LittleEndian.AppendUint64(nil, uint64(len(parts.SSMDocument)))...)
	bs = append(bs, []byte(parts.SSMDocument)...)
	bs = append(bs, binary.LittleEndian.AppendUint64(nil, uint64(len(parts.InstallerScript)))...)
	bs = append(bs, []byte(parts.InstallerScript)...)
	return uuid.NewSHA1(discoverEC2Namespace, bs).String()
}

// discoverEC2Namespace is an UUID that represents the name space to be used for generating UUIDs for DiscoverEC2 User Task names.
var discoverEC2Namespace = uuid.Must(uuid.Parse("6ba7b815-9dad-11d1-80b4-00c04fd430c8"))

// TaskNameForDiscoverEKSParts are the fields that deterministically compute a Discover EKS task name.
// To be used with TaskNameForDiscoverEKS function.
type TaskNameForDiscoverEKSParts struct {
	Integration     string
	IssueType       string
	AccountID       string
	Region          string
	AppAutoDiscover bool
}

// TaskNameForDiscoverEKS returns a deterministic name for the DiscoverEKS task type.
// This method is used to ensure a single UserTask is created to report issues in enrolling EKS clusters for a given integration, issue type, account id and region.
func TaskNameForDiscoverEKS(parts TaskNameForDiscoverEKSParts) string {
	var bs []byte
	bs = append(bs, binary.LittleEndian.AppendUint64(nil, uint64(len(parts.Integration)))...)
	bs = append(bs, []byte(parts.Integration)...)
	bs = append(bs, binary.LittleEndian.AppendUint64(nil, uint64(len(parts.IssueType)))...)
	bs = append(bs, []byte(parts.IssueType)...)
	bs = append(bs, binary.LittleEndian.AppendUint64(nil, uint64(len(parts.AccountID)))...)
	bs = append(bs, []byte(parts.AccountID)...)
	bs = append(bs, binary.LittleEndian.AppendUint64(nil, uint64(len(parts.Region)))...)
	bs = append(bs, []byte(parts.Region)...)
	appAutoDiscoverString := strconv.FormatBool(parts.AppAutoDiscover)
	bs = append(bs, binary.LittleEndian.AppendUint64(nil, uint64(len(appAutoDiscoverString)))...)
	bs = append(bs, []byte(appAutoDiscoverString)...)
	return uuid.NewSHA1(discoverEKSNamespace, bs).String()
}

// discoverEKSNamespace is an UUID that represents the name space to be used for generating UUIDs for DiscoverEKS User Task names.
var discoverEKSNamespace = uuid.NewSHA1(uuid.Nil, []byte("discover-eks"))

// TaskNameForDiscoverRDSParts are the fields that deterministically compute a Discover RDS task name.
// To be used with TaskNameForDiscoverRDS function.
type TaskNameForDiscoverRDSParts struct {
	Integration string
	IssueType   string
	AccountID   string
	Region      string
}

// TaskNameForDiscoverRDS returns a deterministic name for the DiscoverRDS task type.
// This method is used to ensure a single UserTask is created to report issues in enrolling RDS databases for a given integration, issue type, account id and region.
func TaskNameForDiscoverRDS(parts TaskNameForDiscoverRDSParts) string {
	var bs []byte
	bs = append(bs, binary.LittleEndian.AppendUint64(nil, uint64(len(parts.Integration)))...)
	bs = append(bs, []byte(parts.Integration)...)
	bs = append(bs, binary.LittleEndian.AppendUint64(nil, uint64(len(parts.IssueType)))...)
	bs = append(bs, []byte(parts.IssueType)...)
	bs = append(bs, binary.LittleEndian.AppendUint64(nil, uint64(len(parts.AccountID)))...)
	bs = append(bs, []byte(parts.AccountID)...)
	bs = append(bs, binary.LittleEndian.AppendUint64(nil, uint64(len(parts.Region)))...)
	bs = append(bs, []byte(parts.Region)...)
	return uuid.NewSHA1(discoverRDSNamespace, bs).String()
}

// discoverRDSNamespace is an UUID that represents the name space to be used for generating UUIDs for DiscoverRDS User Task names.
var discoverRDSNamespace = uuid.NewSHA1(uuid.Nil, []byte("discover-rds"))
