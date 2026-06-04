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
	"cmp"
	"context"
	"encoding/base64"
	"log/slog"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	rbacv1 "k8s.io/api/rbac/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/fixtures"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
)

const (
	// teleportKubernetesGroup is the Kubernetes group that exists in the EKS cluster and is used to grant access to the cluster
	// for the specified ARN.
	teleportKubernetesGroup = "teleport:kube-service:eks"
	// eksClusterAdminPolicy is the EKS access policy granting cluster-admin. It is
	// associated to the bootstrap ARN only temporarily, to create the Teleport role
	// and binding, then revoked.
	// https://docs.aws.amazon.com/eks/latest/userguide/access-policies.html
	eksClusterAdminPolicy = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"
)

// eksDiscoveryPermissions is used for logging to list all the permissions that
// the discovery service may need to discover EKS clusters and configure access.
var eksDiscoveryPermissions = []string{
	"eks:AssociateAccessPolicy",
	"eks:CreateAccessEntry",
	"eks:DeleteAccessEntry",
	"eks:DescribeAccessEntry",
	"eks:DescribeCluster",
	"eks:DisassociateAccessPolicy",
	"eks:ListClusters",
	"eks:TagResource",
	"eks:UpdateAccessEntry",
}

// EKSAccessManager owns the write side of EKS discovery. It provisions EKS access
// entries, computes the Status.Discovery.Aws cleanup metadata, and removes the
// access entry when a cluster is deleted. It is the only writer of EKS access
// entries and the only producer of AWS discovery status.
type EKSAccessManager struct {
	clientGetter AWSClientGetter
	logger       *slog.Logger
}

// NewEKSAccessManager creates an EKSAccessManager from the AWS client getter the
// fetchers use.
func NewEKSAccessManager(clientGetter AWSClientGetter, logger *slog.Logger) (*EKSAccessManager, error) {
	if clientGetter == nil {
		return nil, trace.BadParameter("missing ClientGetter field")
	}
	if logger == nil {
		logger = slog.With(teleport.ComponentKey, "eks:access")
	}
	return &EKSAccessManager{clientGetter: clientGetter, logger: logger}, nil
}

// EKSAccessConfig is the access-relevant subset of a discovery matcher: the
// principal to grant long-term access to, and the role the manager assumes to
// provision it. It keeps the manager decoupled from discovery configuration.
// Integration-enrolled clusters are provisioned by their integration, not by
// the manager, so there is no integration field here.
type EKSAccessConfig struct {
	// AssumeRole, when set, is assumed for AWS and Kubernetes calls and is the
	// bootstrap principal when SetupAccessForARN is unset.
	AssumeRole *types.AssumeRole
	// SetupAccessForARN is the principal granted long-term access; it falls back
	// to the bootstrap ARN when unset.
	SetupAccessForARN string
}

func (c EKSAccessConfig) credentialOpts() []awsconfig.OptionsFn {
	var assumeRole types.AssumeRole
	if c.AssumeRole != nil {
		assumeRole = *c.AssumeRole
	}
	return getAWSOpts(assumeRole, "")
}

// resolvedAccess is an EKSAccessConfig with its identity resolved: the principal
// granted long-term access and the bootstrap principal used to provision it.
// Resolving once per config avoids an sts:GetCallerIdentity per cluster.
type resolvedAccess struct {
	config       EKSAccessConfig
	principalARN string
	bootstrapARN string
}

// resolveAccess resolves the principal and bootstrap ARNs for an access config.
// principalARN is SetupAccessForARN, falling back to bootstrapARN; bootstrapARN
// is AssumeRole, falling back to sts:GetCallerIdentity. Either may be empty when
// STS fails and the relevant field is unset.
func (m *EKSAccessManager) resolveAccess(ctx context.Context, access EKSAccessConfig, regions []string) resolvedAccess {
	var bootstrapARN string
	if access.AssumeRole != nil && access.AssumeRole.RoleARN != "" {
		bootstrapARN = access.AssumeRole.RoleARN
	} else {
		bootstrapARN = m.getCallerIdentityARN(ctx, access, regions)
	}
	return resolvedAccess{
		config:       access,
		principalARN: cmp.Or(access.SetupAccessForARN, bootstrapARN),
		bootstrapARN: bootstrapARN,
	}
}

// getCallerIdentityARN calls sts:GetCallerIdentity through any region whose
// config initializes successfully.
func (m *EKSAccessManager) getCallerIdentityARN(ctx context.Context, access EKSAccessConfig, regions []string) string {
	opts := access.credentialOpts()
	for _, region := range regions {
		cfg, err := m.clientGetter.GetConfig(ctx, region, opts...)
		if err != nil {
			m.logger.WarnContext(ctx, "Failed to initialize AWS config for STS",
				"region", region, "error", err)
			continue
		}
		out, err := m.clientGetter.GetAWSSTSClient(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		if err != nil {
			m.logger.WarnContext(ctx, "Failed to resolve AWS caller identity, EKS access bootstrap will be skipped this cycle", "error", err)
			return ""
		}
		iamARN, err := resolveIAMRoleARN(ctx, m.clientGetter.GetAWSIAMClient(cfg), aws.ToString(out.Arn))
		if err != nil {
			m.logger.WarnContext(ctx, "Failed to parse AWS caller identity ARN, EKS access bootstrap will be skipped this cycle", "error", err)
			return ""
		}
		return iamARN
	}
	return ""
}

// Ensure provisions EKS access for one described cluster and returns the AWS
// status to record for it. It records status before provisioning
// the Kubernetes RBAC, so a cluster whose access setup fails midway keeps the
// metadata Cleanup needs to remove its access entry.
func (m *EKSAccessManager) Ensure(ctx context.Context, access resolvedAccess, cluster *ekstypes.Cluster) (*types.KubernetesClusterAWSStatus, error) {
	// When a principal role ARN couldn't be resolved, or we can't inspect the
	// existing access configuration, skip and the next cycle will retry.
	if access.principalARN == "" || cluster.AccessConfig == nil {
		return nil, nil
	}

	var assumedRole *types.AssumeRole
	if access.config.AssumeRole != nil && access.config.AssumeRole.RoleARN != "" {
		assumedRole = access.config.AssumeRole
	}
	status := &types.KubernetesClusterAWSStatus{
		SetupAccessForArn:    access.principalARN,
		DiscoveryAssumedRole: assumedRole,
	}

	// Only auth modes that allow API access can be provisioned. A ConfigMap-only
	// cluster must be configured manually, so record the intent but skip setup.
	switch st := cluster.AccessConfig.AuthenticationMode; st {
	case ekstypes.AuthenticationModeApiAndConfigMap, ekstypes.AuthenticationModeApi:
	default:
		m.logger.InfoContext(ctx, "EKS cluster must be configured manually due to its authentication mode",
			"cluster", aws.ToString(cluster.Name),
			"authentication_mode", st,
			"access_arn", access.principalARN,
		)
		return status, nil
	}

	setup, err := m.newAccessSetup(ctx, access, aws.ToString(cluster.Arn))
	if err != nil {
		return status, trace.Wrap(err)
	}
	if err := setup.checkOrSetupAccessForARN(ctx, cluster); err != nil {
		return status, trace.Wrap(err, "unable to setup access for EKS cluster %q", aws.ToString(cluster.Name))
	}
	return status, nil
}

// newAccessSetup builds the per-region write context Ensure uses to provision a
// single cluster, with the EKS and STS-presign clients for the cluster's region.
func (m *EKSAccessManager) newAccessSetup(ctx context.Context, access resolvedAccess, clusterARN string) (*eksAccessSetup, error) {
	parsed, err := arn.Parse(clusterARN)
	if err != nil {
		return nil, trace.Wrap(err, "unable to parse EKS cluster ARN %q", clusterARN)
	}
	cfg, err := m.clientGetter.GetConfig(ctx, parsed.Region, access.config.credentialOpts()...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &eksAccessSetup{
		eks:          m.clientGetter.GetAWSEKSClient(cfg),
		stsPresign:   m.clientGetter.GetAWSSTSPresignClient(cfg),
		principalARN: access.principalARN,
		bootstrapARN: access.bootstrapARN,
		logger:       m.logger,
	}, nil
}

// Cleanup deletes the EKS access entry recorded on the cluster's AWS discovery
// status. It is a no-op for non-EKS clusters and clusters without AWS status.
func (m *EKSAccessManager) Cleanup(ctx context.Context, cluster types.KubeCluster) error {
	if !cluster.IsAWS() || cluster.GetStatus() == nil || cluster.GetStatus().Discovery == nil || cluster.GetStatus().Discovery.Aws == nil {
		return nil
	}

	region := cluster.GetAWSConfig().Region
	clusterName := cluster.GetAWSConfig().Name
	awsStatus := cluster.GetStatus().Discovery.Aws

	// Reconstruct the AWS configuration used during cluster discovery so cleanup
	// uses the same credentials/role/integration.
	assumeRole := types.AssumeRole{}
	if awsStatus.DiscoveryAssumedRole != nil {
		assumeRole = *awsStatus.DiscoveryAssumedRole
	}

	awsConfig, err := m.clientGetter.GetConfig(ctx, region, getAWSOpts(assumeRole, awsStatus.Integration)...)
	if err != nil {
		return trace.Wrap(err)
	}

	client := m.clientGetter.GetAWSEKSClient(awsConfig)

	m.logger.InfoContext(ctx, "Deleting dangling access entry for EKS cluster",
		"cluster_name", clusterName,
		"region", region,
		"aws_account_id", cluster.GetAWSConfig().AccountID,
		"principal_arn", awsStatus.SetupAccessForArn,
	)

	_, err = convertAWSError(
		client.DeleteAccessEntry(ctx, &eks.DeleteAccessEntryInput{
			ClusterName:  aws.String(clusterName),
			PrincipalArn: aws.String(awsStatus.SetupAccessForArn),
		}),
	)

	switch {
	case trace.IsNotFound(err):
		// Already cleaned up or never created.
		return nil
	case trace.IsAccessDenied(err):
		return trace.Wrap(err, "access denied when attempting to delete access entry for cluster %q. Ensure the required permissions are set", clusterName)
	default:
		return trace.Wrap(err, "failed to delete access entry for cluster %q", clusterName)
	}
}

// eksAccessSetup provisions access for a single EKS cluster in one region using
// the regional EKS and STS-presign clients and the resolved principal/bootstrap
// ARNs.
type eksAccessSetup struct {
	eks          EKSClient
	stsPresign   STSPresignClient
	principalARN string
	bootstrapARN string
	logger       *slog.Logger
}

// checkOrSetupAccessForARN ensures the principal ARN has an access entry mapping it to
// teleportKubernetesGroup, provisioning the entry and the cluster RBAC when it is missing or misconfigured.
func (s *eksAccessSetup) checkOrSetupAccessForARN(ctx context.Context, cluster *ekstypes.Cluster) error {
	entry, err := convertAWSError(
		s.eks.DescribeAccessEntry(ctx,
			&eks.DescribeAccessEntryInput{
				ClusterName:  cluster.Name,
				PrincipalArn: aws.String(s.principalARN),
			},
		),
	)

	switch {
	case trace.IsAccessDenied(err):
		s.logger.WarnContext(ctx, "Access denied to setup access for EKS cluster, ensure the required permissions are set",
			"error", err,
			"cluster", aws.ToString(cluster.Name),
			"required_permissions", eksDiscoveryPermissions,
		)
		return nil
	case err == nil:
		// Entry already grants teleportKubernetesGroup.
		if entry.AccessEntry != nil && slices.Contains(entry.AccessEntry.KubernetesGroups, teleportKubernetesGroup) {
			return nil
		}
		fallthrough
	case trace.IsNotFound(err):
		// Entry missing or misconfigured: temporarily gain admin access to install
		// the role and binding. Skip when bootstrapARN is unresolved and retry next
		// cycle, leaving status intact so cleanup metadata is preserved.
		if s.bootstrapARN == "" {
			s.logger.WarnContext(ctx, "Skipping EKS access setup because bootstrap identity is unresolved, will retry next discovery cycle",
				"cluster", aws.ToString(cluster.Name),
				"principal_arn", s.principalARN,
			)
			return nil
		}
		if err := s.temporarilyGainAdminAccessAndCreateRole(ctx, cluster); trace.IsAccessDenied(err) {
			s.logger.WarnContext(ctx, "Access denied to setup access for EKS cluster, ensure the required permissions are set",
				"error", err,
				"cluster", aws.ToString(cluster.Name),
				"required_permissions", eksDiscoveryPermissions,
			)
			return nil
		} else if err != nil {
			return trace.Wrap(err, "unable to setup access for EKS cluster %q", aws.ToString(cluster.Name))
		}

		err = s.upsertAccessEntry(ctx, cluster)
		if trace.IsAccessDenied(err) {
			s.logger.WarnContext(ctx, "Access denied to setup access for EKS cluster, ensure the required permissions are set",
				"error", err,
				"cluster", aws.ToString(cluster.Name),
				"required_permissions", eksDiscoveryPermissions,
			)
			return nil
		}
		return trace.Wrap(err, "unable to setup access for EKS cluster %q", aws.ToString(cluster.Name))
	default:
		return trace.Wrap(err)
	}
}

// temporarilyGainAdminAccessAndCreateRole grants the bootstrap ARN cluster-admin long enough to install
// the Teleport role and binding for teleportKubernetesGroup. On exit it deletes the access entry if it
// created one, or disassociates the admin policy if the entry already existed, so the temporary
// cluster-admin grant never lingers.
func (s *eksAccessSetup) temporarilyGainAdminAccessAndCreateRole(ctx context.Context, cluster *ekstypes.Cluster) error {
	rsp, err := convertAWSError(
		s.eks.CreateAccessEntry(ctx,
			&eks.CreateAccessEntryInput{
				ClusterName:  cluster.Name,
				PrincipalArn: aws.String(s.bootstrapARN),
			},
		),
	)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	// rsp is non-nil when we created the access entry; nil when it already existed.
	createdEntry := rsp != nil
	defer func() {
		if createdEntry {
			// Deleting the entry also removes its admin policy association.
			if _, err := convertAWSError(
				s.eks.DeleteAccessEntry(
					ctx,
					&eks.DeleteAccessEntryInput{
						ClusterName:  cluster.Name,
						PrincipalArn: aws.String(s.bootstrapARN),
					}),
			); err != nil {
				s.logger.WarnContext(ctx, "Failed to delete access entry for EKS cluster",
					"error", err,
					"cluster", aws.ToString(cluster.Name),
				)
			}
			return
		}
		// The entry pre-existed, so only revoke the admin policy we associated to it.
		if _, err := convertAWSError(
			s.eks.DisassociateAccessPolicy(
				ctx,
				&eks.DisassociateAccessPolicyInput{
					ClusterName:  cluster.Name,
					PolicyArn:    aws.String(eksClusterAdminPolicy),
					PrincipalArn: aws.String(s.bootstrapARN),
				}),
		); err != nil {
			s.logger.WarnContext(ctx, "Failed to disassociate temporary admin policy from EKS cluster",
				"error", err,
				"cluster", aws.ToString(cluster.Name),
			)
		}
	}()

	_, err = convertAWSError(
		s.eks.AssociateAccessPolicy(ctx, &eks.AssociateAccessPolicyInput{
			AccessScope: &ekstypes.AccessScope{
				Namespaces: nil,
				Type:       ekstypes.AccessScopeTypeCluster,
			},
			ClusterName:  cluster.Name,
			PolicyArn:    aws.String(eksClusterAdminPolicy),
			PrincipalArn: aws.String(s.bootstrapARN),
		}),
	)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err, "unable to associate EKS Access Policy to cluster %q", aws.ToString(cluster.Name))
	}

	timeout := time.NewTimer(60 * time.Second)
	defer timeout.Stop()
forLoop:
	for {

		// EKS Access Entries are eventually consistent, so we need to wait for the access to be granted.
		// AWS API recommends to wait for 5 seconds before checking the access.
		err = s.upsertRoleAndBinding(ctx, cluster)
		if err == nil || !kubeerrors.IsForbidden(err) && !kubeerrors.IsUnauthorized(err) {
			break
		}
		select {
		case <-timeout.C:
			break forLoop
		case <-time.After(5 * time.Second):

		}

	}
	return trace.Wrap(err, "unable to upsert role and binding for cluster %q", aws.ToString(cluster.Name))
}

func (s *eksAccessSetup) upsertRoleAndBinding(ctx context.Context, cluster *ekstypes.Cluster) error {
	client, err := s.createKubeClient(ctx, cluster)
	if err != nil {
		return trace.Wrap(err, "unable to create Kubernetes client for cluster %q", aws.ToString(cluster.Name))
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	if err := s.upsertClusterRoleWithAdminCredentials(ctx, client); err != nil {
		return trace.Wrap(err, "unable to upsert ClusterRole for group %q", teleportKubernetesGroup)
	}

	if err := s.upsertClusterRoleBindingWithAdminCredentials(ctx, client, teleportKubernetesGroup); err != nil {
		return trace.Wrap(err, "unable to upsert ClusterRoleBinding for group %q", teleportKubernetesGroup)
	}

	return nil
}

func (s *eksAccessSetup) createKubeClient(ctx context.Context, cluster *ekstypes.Cluster) (*kubernetes.Clientset, error) {
	if s.stsPresign == nil {
		return nil, trace.BadParameter("STS presign client is not set")
	}
	token, _, err := kubeutils.GenAWSEKSToken(ctx, s.stsPresign, aws.ToString(cluster.Name), clockwork.NewRealClock())
	if err != nil {
		return nil, trace.Wrap(err, "unable to generate EKS token for cluster %q", aws.ToString(cluster.Name))
	}

	if cluster.CertificateAuthority == nil {
		return nil, trace.BadParameter("missing certificate authority for cluster %q", aws.ToString(cluster.Name))
	}
	ca, err := base64.StdEncoding.DecodeString(aws.ToString(cluster.CertificateAuthority.Data))
	if err != nil {
		return nil, trace.Wrap(err, "unable to decode EKS cluster %q certificate authority", aws.ToString(cluster.Name))
	}

	apiEndpoint := aws.ToString(cluster.Endpoint)
	if len(apiEndpoint) == 0 {
		return nil, trace.BadParameter("invalid api endpoint for cluster %q", aws.ToString(cluster.Name))
	}

	client, err := kubernetes.NewForConfig(
		&rest.Config{
			Host:        apiEndpoint,
			BearerToken: token,
			TLSClientConfig: rest.TLSClientConfig{
				CAData: ca,
			},
		},
	)
	return client, trace.Wrap(err, "unable to create Kubernetes client for cluster %q", aws.ToString(cluster.Name))
}

func (s *eksAccessSetup) upsertClusterRoleWithAdminCredentials(ctx context.Context, client *kubernetes.Clientset) error {
	clusterRole := &rbacv1.ClusterRole{}

	if err := yaml.Unmarshal([]byte(fixtures.KubeClusterRoleTemplate), clusterRole); err != nil {
		return trace.Wrap(err)
	}

	_, err := client.RbacV1().ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{})
	if err == nil {
		return nil
	}

	if kubeerrors.IsAlreadyExists(err) {
		_, err := client.RbacV1().ClusterRoles().Update(ctx, clusterRole, metav1.UpdateOptions{})
		return trace.Wrap(err)
	}

	return trace.Wrap(err)
}

func (s *eksAccessSetup) upsertClusterRoleBindingWithAdminCredentials(ctx context.Context, client *kubernetes.Clientset, groupID string) error {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}

	if err := yaml.Unmarshal([]byte(fixtures.KubeClusterRoleBindingTemplate), clusterRoleBinding); err != nil {
		return trace.Wrap(err)
	}

	if len(clusterRoleBinding.Subjects) == 0 {
		return trace.BadParameter("Subjects field were not correctly unmarshaled")
	}

	clusterRoleBinding.Subjects[0].Name = groupID

	_, err := client.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{})
	if err == nil {
		return nil
	}

	if kubeerrors.IsAlreadyExists(err) {
		_, err := client.RbacV1().ClusterRoleBindings().Update(ctx, clusterRoleBinding, metav1.UpdateOptions{})
		return trace.Wrap(err)
	}

	return trace.Wrap(err)
}

func (s *eksAccessSetup) upsertAccessEntry(ctx context.Context, cluster *ekstypes.Cluster) error {
	_, err := convertAWSError(
		s.eks.CreateAccessEntry(ctx,
			&eks.CreateAccessEntryInput{
				ClusterName:      cluster.Name,
				PrincipalArn:     aws.String(s.principalARN),
				KubernetesGroups: []string{teleportKubernetesGroup},
			},
		))
	if err == nil || !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	_, err = convertAWSError(
		s.eks.UpdateAccessEntry(ctx,
			&eks.UpdateAccessEntryInput{
				ClusterName:      cluster.Name,
				PrincipalArn:     aws.String(s.principalARN),
				KubernetesGroups: []string{teleportKubernetesGroup},
			},
		))

	return trace.Wrap(err)
}

// resolveIAMRoleARN converts an STS caller identity ARN to the corresponding IAM role ARN.
// For assumed-role ARNs it calls iam:GetRole to retrieve the exact ARN (including any path),
// which is required for SSO roles that include a region in their path
// (e.g. /aws-reserved/sso.amazonaws.com/us-west-2/). Falls back to string conversion on error.
func resolveIAMRoleARN(ctx context.Context, iamClient IAMClient, callerARN string) (string, error) {
	parsed, err := arn.Parse(callerARN)
	if err != nil || !strings.HasPrefix(parsed.Resource, "assumed-role/") {
		return callerARN, nil
	}
	parts := strings.SplitN(parsed.Resource, "/", 3)
	if len(parts) < 2 {
		return callerARN, nil
	}
	roleName := parts[1]
	if iamClient == nil {
		return convertAssumedRoleToIAMRole(callerARN), nil
	}
	resp, err := iamClient.GetRole(ctx, &iam.GetRoleInput{RoleName: aws.String(roleName)})
	if err != nil {
		// Fall back to best-effort string conversion rather than failing entirely.
		return convertAssumedRoleToIAMRole(callerARN), nil
	}
	return aws.ToString(resp.Role.Arn), nil
}

// convertAssumedRoleToIAMRole converts the assumed role ARN to an IAM role ARN.
// The assumed role ARN is in the format "arn:aws:sts::account-id:assumed-role/role-name/role-session-name".
// The IAM role ARN is in the format "arn:aws:iam::account-id:role/role-name".
// Note: this does not handle roles with non-default paths (e.g. AWS SSO roles); use resolveIAMRoleARN instead.
func convertAssumedRoleToIAMRole(callerIdentity string) string {
	const (
		assumeRolePrefix = "assumed-role/"
		roleResource     = "role"
		serviceName      = "iam"
	)
	a, err := arn.Parse(callerIdentity)
	if err != nil {
		return callerIdentity
	}
	if !strings.HasPrefix(a.Resource, assumeRolePrefix) {
		return callerIdentity
	}
	a.Service = serviceName
	split := strings.Split(a.Resource, "/")
	if len(split) <= 2 {
		return callerIdentity
	}
	a.Resource = path.Join(roleResource, split[1])
	return a.String()
}
