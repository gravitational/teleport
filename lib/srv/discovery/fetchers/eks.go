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

package fetchers

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
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
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/fixtures"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

const (
	concurrencyLimit = 5
)

type eksFetcher struct {
	EKSFetcherConfig

	mu               sync.Mutex
	client           EKSClient
	stsPresignClient STSPresignClient
	callerIdentity   string
}

// EKSClient is the subset of the EKS interface we use in fetchers.
type EKSClient interface {
	eks.DescribeClusterAPIClient
	eks.ListClustersAPIClient

	AssociateAccessPolicy(ctx context.Context, params *eks.AssociateAccessPolicyInput, optFns ...func(*eks.Options)) (*eks.AssociateAccessPolicyOutput, error)
	CreateAccessEntry(ctx context.Context, params *eks.CreateAccessEntryInput, optFns ...func(*eks.Options)) (*eks.CreateAccessEntryOutput, error)
	DeleteAccessEntry(ctx context.Context, params *eks.DeleteAccessEntryInput, optFns ...func(*eks.Options)) (*eks.DeleteAccessEntryOutput, error)
	DescribeAccessEntry(ctx context.Context, params *eks.DescribeAccessEntryInput, optFns ...func(*eks.Options)) (*eks.DescribeAccessEntryOutput, error)
	UpdateAccessEntry(ctx context.Context, params *eks.UpdateAccessEntryInput, optFns ...func(*eks.Options)) (*eks.UpdateAccessEntryOutput, error)
}

// STSClient is the subset of the STS interface we use in fetchers.
type STSClient interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
	stscreds.AssumeRoleAPIClient
}

// STSPresignClient is the subset of the STS presign interface we use in fetchers.
type STSPresignClient = kubeutils.STSPresignClient

// AWSClientGetter is an interface for getting an EKS client and an STS client.
type AWSClientGetter interface {
	awsconfig.Provider
	// GetAWSEKSClient returns AWS EKS client for the specified config.
	GetAWSEKSClient(aws.Config) EKSClient
	// GetAWSSTSClient returns AWS STS client for the specified config.
	GetAWSSTSClient(aws.Config) STSClient
	// GetAWSSTSPresignClient returns AWS STS presign client for the specified config.
	GetAWSSTSPresignClient(aws.Config) STSPresignClient
}

// EKSFetcherConfig configures the EKS fetcher.
type EKSFetcherConfig struct {
	// ClientGetter retrieves an EKS client and an STS client.
	ClientGetter AWSClientGetter
	// AssumeRole provides a role ARN and ExternalID to assume an AWS role
	// when fetching clusters.
	AssumeRole types.AssumeRole
	// Integration is the integration name to be used to fetch credentials.
	// When present, it will use this integration and discard any local credentials.
	Integration string
	// DiscoveryConfigName is the name of the discovery config which originated the resource.
	// Might be empty when the fetcher is using static matchers:
	// ie teleport.yaml/discovery_service.<cloud>.<matcher>
	DiscoveryConfigName string
	// KubeAppDiscovery specifies if Kubernetes App Discovery should be enabled for the
	// discovered cluster. We don't use this information for fetching itself, but we need it for
	// correct enrollment of the clusters returned from this fetcher.
	KubeAppDiscovery bool
	// Region is the region where the clusters should be located.
	Region string
	// FilterLabels are the filter criteria.
	FilterLabels types.Labels
	// Log is the logger.
	Logger *slog.Logger
	// SetupAccessForARN is the ARN to setup access for.
	SetupAccessForARN string
	// Clock is the clock.
	Clock clockwork.Clock
}

// CheckAndSetDefaults validates and sets the defaults values.
func (c *EKSFetcherConfig) CheckAndSetDefaults() error {
	if c.ClientGetter == nil {
		return trace.BadParameter("missing ClientGetter field")
	}
	if len(c.Region) == 0 {
		return trace.BadParameter("missing Region field")
	}

	if len(c.FilterLabels) == 0 {
		return trace.BadParameter("missing FilterLabels field")
	}

	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "fetcher:eks")
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	return nil
}

// MakeEKSFetchersFromAWSMatchers creates fetchers from the provided matchers. Returned fetchers are separated
// by their reliance on the integration.
func MakeEKSFetchersFromAWSMatchers(logger *slog.Logger, clients AWSClientGetter, matchers []types.AWSMatcher, discoveryConfigName string) (kubeFetchers []common.Fetcher, _ error) {
	for _, matcher := range matchers {
		var matcherAssumeRole types.AssumeRole
		if matcher.AssumeRole != nil {
			matcherAssumeRole = *matcher.AssumeRole
		}

		for _, t := range matcher.Types {
			for _, region := range matcher.Regions {
				switch t {
				case types.AWSMatcherEKS:
					fetcher, err := NewEKSFetcher(
						EKSFetcherConfig{
							ClientGetter:        clients,
							AssumeRole:          matcherAssumeRole,
							Region:              region,
							Integration:         matcher.Integration,
							KubeAppDiscovery:    matcher.KubeAppDiscovery,
							FilterLabels:        matcher.Tags,
							Logger:              logger,
							SetupAccessForARN:   matcher.SetupAccessForARN,
							DiscoveryConfigName: discoveryConfigName,
						},
					)
					if err != nil {
						logger.WarnContext(context.Background(), "Could not initialize EKS fetcher, skipping",
							"error", err,
							"region", region,
							"labels", matcher.Tags,
							"assume_role", matcherAssumeRole.RoleARN,
						)
						continue
					}
					kubeFetchers = append(kubeFetchers, fetcher)
				}
			}
		}
	}
	return kubeFetchers, nil
}

// NewEKSFetcher creates a new EKS fetcher configuration.
func NewEKSFetcher(cfg EKSFetcherConfig) (common.Fetcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	fetcher := &eksFetcher{EKSFetcherConfig: cfg}

	if err := fetcher.setCallerIdentity(context.Background()); err != nil {
		cfg.Logger.WarnContext(context.Background(), "Failed to set caller identity", "error", err)
	}

	// If the fetcher SetupAccessForARN isn't set, use the caller identity.
	// This is useful to setup access for the caller identity itself
	// without having to specify the ARN.
	// If the current caller identity doesn't have access to setup access entries,
	// the fetcher will log a warning and skip the setup access process.
	if fetcher.SetupAccessForARN == "" {
		fetcher.SetupAccessForARN = fetcher.callerIdentity
	}

	return fetcher, nil
}

func (a *eksFetcher) getClient(ctx context.Context) (EKSClient, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client != nil {
		return a.client, nil
	}

	cfg, err := a.ClientGetter.GetConfig(ctx, a.Region, a.getAWSOpts()...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a.client = a.ClientGetter.GetAWSEKSClient(cfg)
	return a.client, nil
}

// GetIntegration returns the integration name that is used for getting credentials of the fetcher.
func (a *eksFetcher) GetIntegration() string {
	return a.Integration
}

type DiscoveredEKSCluster struct {
	types.KubeCluster

	Integration            string
	EnableKubeAppDiscovery bool
}

func (d *DiscoveredEKSCluster) GetIntegration() string {
	return d.Integration
}

func (d *DiscoveredEKSCluster) GetKubeAppDiscovery() bool {
	return d.EnableKubeAppDiscovery
}

func (d *DiscoveredEKSCluster) GetKubeCluster() types.KubeCluster {
	return d.KubeCluster
}

func (a *eksFetcher) Get(ctx context.Context) (types.ResourcesWithLabels, error) {
	clusters, err := a.getEKSClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	a.rewriteKubeClusters(clusters)

	resources := make(types.ResourcesWithLabels, 0, len(clusters))
	for _, cluster := range clusters {
		resources = append(resources, &DiscoveredEKSCluster{
			KubeCluster:            cluster,
			Integration:            a.Integration,
			EnableKubeAppDiscovery: a.KubeAppDiscovery,
		})
	}
	return resources, nil
}

// rewriteKubeClusters rewrites the discovered kube clusters.
func (a *eksFetcher) rewriteKubeClusters(clusters types.KubeClusters) {
	for _, c := range clusters {
		common.ApplyEKSNameSuffix(c)
	}
}

func (a *eksFetcher) getEKSClusters(ctx context.Context) (types.KubeClusters, error) {
	var (
		clusters        types.KubeClusters
		mu              sync.Mutex
		group, groupCtx = errgroup.WithContext(ctx)
	)
	group.SetLimit(concurrencyLimit)

	client, err := a.getClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed getting AWS EKS client")
	}

	// For now we should only list EKS clusters so we use nil (default) input param.
	for p := eks.NewListClustersPaginator(client, nil); p.HasMorePages(); {
		out, err := p.NextPage(ctx)
		if err != nil {
			return clusters, trace.Wrap(err)
		}
		for _, clusterName := range out.Clusters {
			// group.Go will block if the concurrency limit is reached.
			// It will resume once any running function finishes.
			group.Go(func() error {
				cluster, err := a.getMatchingKubeCluster(groupCtx, clusterName)
				// trace.CompareFailed is returned if the cluster did not match the matcher filtering labels
				// or if the cluster is not yet active.
				if trace.IsCompareFailed(err) {
					a.Logger.DebugContext(groupCtx, "Cluster did not match the filtering criteria", "error", err, "cluster", clusterName)
					// never return an error otherwise we will impact discovery process
					return nil
				} else if err != nil {
					a.Logger.WarnContext(groupCtx, "Failed to discover EKS cluster", "error", err, "cluster", clusterName)
					// never return an error otherwise we will impact discovery process
					return nil
				}

				mu.Lock()
				defer mu.Unlock()
				clusters = append(clusters, cluster)
				return nil
			})
		}
	}

	// The error can be discarded since we do not return any error from group.Go closure.
	_ = group.Wait()
	return clusters, trace.Wrap(err)
}

func (a *eksFetcher) ResourceType() string {
	return types.KindKubernetesCluster
}

func (a *eksFetcher) FetcherType() string {
	return types.AWSMatcherEKS
}

func (a *eksFetcher) Cloud() string {
	return types.CloudAWS
}

func (a *eksFetcher) IntegrationName() string {
	return a.Integration
}

func (a *eksFetcher) GetDiscoveryConfigName() string {
	return a.DiscoveryConfigName
}

func (a *eksFetcher) String() string {
	return fmt.Sprintf("eksFetcher(Region=%v, FilterLabels=%v)",
		a.Region, a.FilterLabels)
}

// getMatchingKubeCluster extracts EKS cluster Tags and cluster status from EKS and checks if the cluster matches
// the AWS matcher filtering labels. It also excludes EKS clusters that are not ready.
// If any cluster does not match the filtering criteria, this function returns a “trace.CompareFailed“ error
// to distinguish filtering and operational errors.
func (a *eksFetcher) getMatchingKubeCluster(ctx context.Context, clusterName string) (types.KubeCluster, error) {
	client, err := a.getClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed getting AWS EKS client")
	}

	rsp, err := client.DescribeCluster(
		ctx,
		&eks.DescribeClusterInput{
			Name: aws.String(clusterName),
		},
	)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "Unable to describe EKS cluster %q", clusterName)
	}

	switch st := rsp.Cluster.Status; st {
	case ekstypes.ClusterStatusUpdating, ekstypes.ClusterStatusActive:
		a.Logger.DebugContext(ctx, "EKS cluster status is valid", "status", st, "cluster", clusterName)
	default:
		return nil, trace.CompareFailed("EKS cluster %q not enrolled due to its current status: %s", clusterName, st)
	}

	cluster, err := common.NewKubeClusterFromAWSEKS(aws.ToString(rsp.Cluster.Name), aws.ToString(rsp.Cluster.Arn), rsp.Cluster.Tags)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "Unable to convert eks.Cluster cluster into types.KubernetesClusterV3.")
	}

	if match, reason, err := services.MatchLabels(a.FilterLabels, cluster.GetAllLabels()); err != nil {
		return nil, trace.WrapWithMessage(err, "Unable to match EKS cluster labels against match labels.")
	} else if !match {
		return nil, trace.CompareFailed("EKS cluster %q labels does not match the selector: %s", clusterName, reason)
	}

	// If no access configuration is required, return the cluster.
	if a.SetupAccessForARN == "" || rsp.Cluster.AccessConfig == nil || a.Integration != "" {
		return cluster, nil
	}

	// If the fetcher should setup access for the specified ARN, first check if the cluster authentication mode
	// is set to either [eks.AuthenticationModeApi] or [eks.AuthenticationModeApiAndConfigMap].
	// If the authentication mode is set to [eks.AuthenticationModeConfigMap], the fetcher will ignore the cluster.
	switch st := rsp.Cluster.AccessConfig.AuthenticationMode; st {
	case ekstypes.AuthenticationModeApiAndConfigMap, ekstypes.AuthenticationModeApi:
		if err := a.checkOrSetupAccessForARN(ctx, client, rsp.Cluster); err != nil {
			return nil, trace.Wrap(err, "unable to setup access for EKS cluster %q", clusterName)
		}
		return cluster, nil
	default:
		a.Logger.InfoContext(ctx, "EKS cluster must be configured manually due to its authentication mode",
			"cluster", clusterName,
			"authentication_mode", st,
			"access_arn", a.SetupAccessForARN,
		)
		return cluster, nil
	}
}

const (
	// teleportKubernetesGroup is the Kubernetes group that exists in the EKS cluster and is used to grant access to the cluster
	// for the specified ARN.
	teleportKubernetesGroup = "teleport:kube-service:eks"
)

// eksDiscoveryPermissions is used for logging to list all the permissions that
// the discovery service may need to discover EKS clusters and configure access.
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

// checkOrSetupAccessForARN checks if the ARN has access to the cluster and sets up the access if needed.
// The check involves checking if the access entry exists and if the "teleport:kube-agent:eks" is part of the Kubernetes group.
// If the access entry doesn't exist or is misconfigured, the fetcher will temporarily gain admin access and create the role and binding.
// The fetcher will then upsert the access entry with the correct Kubernetes group.
func (a *eksFetcher) checkOrSetupAccessForARN(ctx context.Context, client EKSClient, cluster *ekstypes.Cluster) error {
	entry, err := convertAWSError(
		client.DescribeAccessEntry(ctx,
			&eks.DescribeAccessEntryInput{
				ClusterName:  cluster.Name,
				PrincipalArn: aws.String(a.SetupAccessForARN),
			},
		),
	)

	switch {
	case trace.IsAccessDenied(err):
		// Access denied means that the principal does not have access to setup access entries for the cluster.
		a.Logger.WarnContext(ctx, "Access denied to setup access for EKS cluster, ensure the required permissions are set",
			"error", err,
			"cluster", aws.ToString(cluster.Name),
			"required_permissions", eksDiscoveryPermissions,
		)
		return nil
	case err == nil:
		// If the access entry exists and the principal has access to the cluster, check if the teleportKubernetesGroup is part of the Kubernetes group.
		if entry.AccessEntry != nil && slices.Contains(entry.AccessEntry.KubernetesGroups, teleportKubernetesGroup) {
			return nil
		}
		fallthrough
	case trace.IsNotFound(err):
		// If the access entry does not exist or the teleportKubernetesGroup is not part of the Kubernetes group, temporarily gain admin access and create the role and binding.
		// This temporary access is granted to the identity that the Discovery service fetcher is running as (callerIdentity). If a role is assumed, the callerIdentity is the assumed role.
		if err := a.temporarilyGainAdminAccessAndCreateRole(ctx, client, cluster); trace.IsAccessDenied(err) {
			// Access denied means that the principal does not have access to setup access entries for the cluster.
			a.Logger.WarnContext(ctx, "Access denied to setup access for EKS cluster, ensure the required permissions are set",
				"error", err,
				"cluster", aws.ToString(cluster.Name),
				"required_permissions", eksDiscoveryPermissions,
			)
			return nil
		} else if err != nil {
			return trace.Wrap(err, "unable to setup access for EKS cluster %q", aws.ToString(cluster.Name))
		}

		// upsert the access entry with the correct Kubernetes group for the final
		err = a.upsertAccessEntry(ctx, client, cluster)
		if trace.IsAccessDenied(err) {
			// Access denied means that the principal does not have access to setup access entries for the cluster.
			a.Logger.WarnContext(ctx, "Access denied to setup access for EKS cluster, ensure the required permissions are set",
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

// temporarilyGainAdminAccessAndCreateRole temporarily gains admin access to the EKS cluster by associating the EKS Cluster Admin Policy
// to the callerIdentity. The fetcher will then create the role and binding for the teleportKubernetesGroup in the EKS cluster.
func (a *eksFetcher) temporarilyGainAdminAccessAndCreateRole(ctx context.Context, client EKSClient, cluster *ekstypes.Cluster) error {
	const (
		// https://docs.aws.amazon.com/eks/latest/userguide/access-policies.html
		// We use cluster admin policy to create namespace and cluster role.
		eksClusterAdminPolicy = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"
	)

	// Setup access for the ARN
	rsp, err := convertAWSError(
		client.CreateAccessEntry(ctx,
			&eks.CreateAccessEntryInput{
				ClusterName:  cluster.Name,
				PrincipalArn: aws.String(a.callerIdentity),
			},
		),
	)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	// rsp is not nil when the access entry was created and needs to be deleted after the role and binding are created.
	if rsp != nil {
		defer func() {
			_, err := convertAWSError(
				client.DeleteAccessEntry(
					ctx,
					&eks.DeleteAccessEntryInput{
						ClusterName:  cluster.Name,
						PrincipalArn: aws.String(a.callerIdentity),
					}),
			)
			if err != nil {
				a.Logger.WarnContext(ctx, "Failed to delete access entry for EKS cluster",
					"error", err,
					"cluster", aws.ToString(cluster.Name),
				)
			}
		}()
	}

	_, err = convertAWSError(
		client.AssociateAccessPolicy(ctx, &eks.AssociateAccessPolicyInput{
			AccessScope: &ekstypes.AccessScope{
				Namespaces: nil,
				Type:       ekstypes.AccessScopeTypeCluster,
			},
			ClusterName:  cluster.Name,
			PolicyArn:    aws.String(eksClusterAdminPolicy),
			PrincipalArn: aws.String(a.callerIdentity),
		}),
	)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err, "unable to associate EKS Access Policy to cluster %q", aws.ToString(cluster.Name))
	}

	timeout := a.Clock.NewTimer(60 * time.Second)
	defer timeout.Stop()
forLoop:
	for {

		// EKS Access Entries are eventually consistent, so we need to wait for the access to be granted.
		// AWS API recommends to wait for 5 seconds before checking the access.
		err = a.upsertRoleAndBinding(ctx, cluster)
		if err == nil || !kubeerrors.IsForbidden(err) && !kubeerrors.IsUnauthorized(err) {
			break
		}
		select {
		case <-timeout.Chan():
			break forLoop
		case <-a.Clock.After(5 * time.Second):

		}

	}
	return trace.Wrap(err, "unable to upsert role and binding for cluster %q", aws.ToString(cluster.Name))
}

// upsertRoleAndBinding upserts the ClusterRole and ClusterRoleBinding for the teleportKubernetesGroup in the EKS cluster.
func (a *eksFetcher) upsertRoleAndBinding(ctx context.Context, cluster *ekstypes.Cluster) error {
	client, err := a.createKubeClient(ctx, cluster)
	if err != nil {
		return trace.Wrap(err, "unable to create Kubernetes client for cluster %q", aws.ToString(cluster.Name))
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	if err := a.upsertClusterRoleWithAdminCredentials(ctx, client); err != nil {
		return trace.Wrap(err, "unable to upsert ClusterRole for group %q", teleportKubernetesGroup)
	}

	if err := a.upsertClusterRoleBindingWithAdminCredentials(ctx, client, teleportKubernetesGroup); err != nil {
		return trace.Wrap(err, "unable to upsert ClusterRoleBinding for group %q", teleportKubernetesGroup)
	}

	return nil
}

func (a *eksFetcher) createKubeClient(ctx context.Context, cluster *ekstypes.Cluster) (*kubernetes.Clientset, error) {
	if a.stsPresignClient == nil {
		return nil, trace.BadParameter("STS presign client is not set")
	}
	token, _, err := kubeutils.GenAWSEKSToken(ctx, a.stsPresignClient, aws.ToString(cluster.Name), a.Clock)
	if err != nil {
		return nil, trace.Wrap(err, "unable to generate EKS token for cluster %q", aws.ToString(cluster.Name))
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

// upsertClusterRoleWithAdminCredentials tries to upsert the ClusterRole using admin credentials.
func (a *eksFetcher) upsertClusterRoleWithAdminCredentials(ctx context.Context, client *kubernetes.Clientset) error {
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

// upsertClusterRoleBindingWithAdminCredentials tries to upsert the ClusterRoleBinding using admin credentials
// and maps it into the principal group.
func (a *eksFetcher) upsertClusterRoleBindingWithAdminCredentials(ctx context.Context, client *kubernetes.Clientset, groupID string) error {
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

// upsertAccessEntry upserts the access entry for the specified ARN with the teleportKubernetesGroup.
func (a *eksFetcher) upsertAccessEntry(ctx context.Context, client EKSClient, cluster *ekstypes.Cluster) error {
	_, err := convertAWSError(
		client.CreateAccessEntry(ctx,
			&eks.CreateAccessEntryInput{
				ClusterName:      cluster.Name,
				PrincipalArn:     aws.String(a.SetupAccessForARN),
				KubernetesGroups: []string{teleportKubernetesGroup},
			},
		))
	if err == nil || !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	_, err = convertAWSError(
		client.UpdateAccessEntry(ctx,
			&eks.UpdateAccessEntryInput{
				ClusterName:      cluster.Name,
				PrincipalArn:     aws.String(a.SetupAccessForARN),
				KubernetesGroups: []string{teleportKubernetesGroup},
			},
		))

	return trace.Wrap(err)
}

func (a *eksFetcher) setCallerIdentity(ctx context.Context) error {
	cfg, err := a.ClientGetter.GetConfig(ctx,
		a.Region,
		a.getAWSOpts()...,
	)
	if err != nil {
		return trace.Wrap(err)
	}
	a.stsPresignClient = a.ClientGetter.GetAWSSTSPresignClient(cfg)
	if a.AssumeRole.RoleARN != "" {
		a.callerIdentity = a.AssumeRole.RoleARN
		return nil
	}

	stsClient := a.ClientGetter.GetAWSSTSClient(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return trace.Wrap(err)
	}
	a.callerIdentity = convertAssumedRoleToIAMRole(aws.ToString(identity.Arn))
	return nil
}

func (a *eksFetcher) getAWSOpts() []awsconfig.OptionsFn {
	return []awsconfig.OptionsFn{
		awsconfig.WithAssumeRole(
			a.AssumeRole.RoleARN,
			a.AssumeRole.ExternalID,
		),
		awsconfig.WithCredentialsMaybeIntegration(a.Integration),
	}
}

func convertAWSError[T any](rsp T, err error) (T, error) {
	err = awslib.ConvertRequestFailureError(err)
	return rsp, trace.Wrap(err)
}

// convertAssumedRoleToIAMRole converts the assumed role ARN to an IAM role ARN.
// The assumed role ARN is in the format "arn:aws:sts::account-id:assumed-role/role-name/role-session-name".
// The IAM role ARN is in the format "arn:aws:iam::account-id:role/role-name".
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
