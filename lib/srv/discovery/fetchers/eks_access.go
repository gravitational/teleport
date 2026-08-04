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
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"
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
	// teleportKubernetesGroup is the Kubernetes group an EKS access entry maps the
	// principal ARN into.
	teleportKubernetesGroup = "teleport:kube-service:eks"
	// eksClusterAdminPolicy is the EKS access policy granting cluster-admin. It is
	// associated to the bootstrap ARN to create the Teleport role and binding, and
	// is removed together with the access entry when this code created the entry.
	// https://docs.aws.amazon.com/eks/latest/userguide/access-policies.html
	eksClusterAdminPolicy = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"
	provisionConcurrency  = 10
)

// eksDiscoveryPermissions lists the IAM permissions discovery needs to find EKS
// clusters and configure access; logged when a call is denied.
var eksDiscoveryPermissions = []string{
	"eks:AssociateAccessPolicy",
	"eks:CreateAccessEntry",
	"eks:DeleteAccessEntry",
	"eks:DescribeAccessEntry",
	"eks:DescribeCluster",
	"eks:ListClusters",
	"eks:TagResource",
	"eks:UpdateAccessEntry",
}

// EKSAccessManager handles access provisioning for discovered EKS clusters.
// Discovery provisions access only when not using an integration.
type EKSAccessManager struct {
	clientGetter AWSClientGetter
	logger       *slog.Logger
	// mu guards callerIdentityARN against concurrent population by ProvisionAll's
	// workers.
	mu sync.Mutex
	// callerIdentityARN is the discovery service's own IAM role ARN, the bootstrap
	// principal for matchers with no assume role. It is cached only within a cycle.
	callerIdentityARN string
}

// NewEKSAccessManager builds an EKSAccessManager from the fetchers' AWS client getter.
func NewEKSAccessManager(clientGetter AWSClientGetter, logger *slog.Logger) (*EKSAccessManager, error) {
	if clientGetter == nil {
		return nil, trace.BadParameter("missing ClientGetter field")
	}
	if logger == nil {
		logger = slog.With(teleport.ComponentKey, "eks:access")
	}
	return &EKSAccessManager{clientGetter: clientGetter, logger: logger}, nil
}

// Provision sets up EKS access for a discovered cluster and returns the status to
// record, or nil when no principal resolves or there is no describe output to act
// on.
func (m *EKSAccessManager) Provision(ctx context.Context, cluster *DiscoveredEKSCluster) *types.KubernetesClusterStatus {
	bootstrapARN := cluster.GetAssumeRoleARN()
	if bootstrapARN == "" {
		bootstrapARN = m.ambientIdentity(ctx, cluster.GetAWSConfig().Region)
	}
	principalARN := cmp.Or(cluster.GetSetupAccessForARN(), bootstrapARN)
	if principalARN == "" {
		return nil
	}

	awsCluster := cluster.awsCluster
	if awsCluster == nil || awsCluster.AccessConfig == nil {
		return nil
	}

	// Record the status before provisioning RBAC, so a cluster whose setup fails
	// midway still carries the metadata Cleanup needs to remove its access entry.
	status := &types.KubernetesClusterStatus{
		Discovery: &types.KubernetesClusterDiscoveryStatus{
			Aws: &types.KubernetesClusterAWSStatus{
				SetupAccessForArn:    principalARN,
				DiscoveryAssumedRole: cluster.GetAssumeRole(),
			},
		},
	}

	// Only API-enabled auth modes can be provisioned; a ConfigMap-only cluster must
	// be configured manually, so record the intent and skip setup.
	switch mode := awsCluster.AccessConfig.AuthenticationMode; mode {
	case ekstypes.AuthenticationModeApiAndConfigMap, ekstypes.AuthenticationModeApi:
	default:
		m.logger.InfoContext(ctx, "EKS cluster must be configured manually due to its authentication mode",
			"cluster", cluster.GetName(),
			"authentication_mode", mode,
			"access_arn", principalARN,
		)
		return status
	}

	region := cluster.GetAWSConfig().Region
	cfg, err := m.clientGetter.GetConfig(ctx, region, accessCredentialOpts(cluster.GetAssumeRole())...)
	if err != nil {
		m.logger.WarnContext(ctx, "Failed to initialize AWS config for EKS access, will retry next cycle",
			"cluster", cluster.GetName(), "region", region, "error", err)
		return nil
	}

	setup := &eksAccessSetup{
		eks:          m.clientGetter.GetAWSEKSClient(cfg),
		stsPresign:   m.clientGetter.GetAWSSTSPresignClient(cfg),
		principalARN: principalARN,
		bootstrapARN: bootstrapARN,
		logger:       m.logger,
	}
	if err := setup.checkOrSetupAccessForARN(ctx, awsCluster); err != nil {
		m.logger.WarnContext(ctx, "Failed to provision EKS access; keeping recorded status for later cleanup",
			"cluster", cluster.GetName(),
			"error", err,
		)
	}
	return status
}

// ProvisionAll provisions the clusters concurrently, returning statuses index-aligned
// with clusters.
func (m *EKSAccessManager) ProvisionAll(ctx context.Context, clusters []*DiscoveredEKSCluster) []*types.KubernetesClusterStatus {
	// Clear the cached ambient identity each cycle so a changed deployment identity is
	// picked up rather than pinned for the process lifetime.
	m.mu.Lock()
	m.callerIdentityARN = ""
	m.mu.Unlock()

	statuses := make([]*types.KubernetesClusterStatus, len(clusters))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(provisionConcurrency)
	for i, cluster := range clusters {
		group.Go(func() error {
			statuses[i] = m.Provision(groupCtx, cluster)
			return nil
		})
	}
	_ = group.Wait()
	return statuses
}

// ambientIdentity resolves the discovery service's own IAM identity and caches it for
// the cycle. It returns empty until the lookup succeeds, so a cluster that needs it
// defers provisioning to a later cycle. The region keeps the lookup in the same AWS
// partition as the cluster.
func (m *EKSAccessManager) ambientIdentity(ctx context.Context, region string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.callerIdentityARN != "" {
		return m.callerIdentityARN
	}
	arn, err := m.resolveCallerIdentity(ctx, region)
	if err != nil {
		m.logger.WarnContext(ctx, "Failed to resolve discovery service identity; ambient-credential clusters will retry next cycle", "error", err)
		return ""
	}
	m.callerIdentityARN = arn
	return arn
}

// resolveCallerIdentity returns the discovery service's own IAM role ARN, looked up
// via STS in the given region.
func (m *EKSAccessManager) resolveCallerIdentity(ctx context.Context, region string) (string, error) {
	cfg, err := m.clientGetter.GetConfig(ctx, region, accessCredentialOpts(nil)...)
	if err != nil {
		return "", trace.Wrap(err)
	}
	out, err := m.clientGetter.GetAWSSTSClient(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return resolveIAMRoleARN(ctx, m.clientGetter.GetAWSIAMClient(cfg), aws.ToString(out.Arn))
}

// accessCredentialOpts builds AWS config options for provisioning a cluster.
func accessCredentialOpts(assumeRole *types.AssumeRole) []awsconfig.OptionsFn {
	role := types.AssumeRole{}
	if assumeRole != nil {
		role = *assumeRole
	}
	return getAWSOpts(role, "")
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

// eksAccessSetup provisions access for a single EKS cluster in one region.
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
		if entry.AccessEntry != nil && slices.Contains(entry.AccessEntry.KubernetesGroups, teleportKubernetesGroup) {
			return nil
		}
		fallthrough
	case trace.IsNotFound(err):
		// Entry missing or misconfigured. Skip when the bootstrap identity is
		// unresolved and retry next cycle.
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
// the Teleport role and binding.
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

	// rsp is nil when CreateAccessEntry hit the AlreadyExists error. The entry is
	// deleted only when this call created it.
	if rsp != nil {
		defer func() {
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
		}()
	}

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
		// EKS access entries are eventually consistent, so retry until the grant propagates.
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
