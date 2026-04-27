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

package awsoidc

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	eksTypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/coreos/go-semver/semver"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/usertasks"
	apiutils "github.com/gravitational/teleport/api/utils"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/aws/tags"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/integrations/awsoidc/internal/kubeagent"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// https://docs.aws.amazon.com/eks/latest/userguide/access-policies.html
	// We use cluster admin policy to create namespace and cluster role.
	eksClusterAdminPolicy = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"

	agentNamespace              = "teleport-agent"
	agentName                   = "teleport-kube-agent"
	awsKubePrefix               = "k8s-aws-v1."
	awsHeaderClusterName        = "x-k8s-aws-id"
	awsHeaderExpires            = "X-Amz-Expires" // Header required by AWS when creating presigned URL.
	concurrentEKSEnrollingLimit = 5
)

// EnrollEKSClusterResult contains result for a single EKS cluster enrollment, if it was successful 'Error' will be nil
// otherwise it will contain an error happened during enrollment.
type EnrollEKSClusterResult struct {
	// ClusterName is the name of an EKS cluster.
	ClusterName string
	// ResourceId is resource ID for the cluster, it is taken from the join token used to enroll the cluster.
	ResourceId string
	// Error contains an error that happened during enrollment, if there was one.
	Error error
	// IssueType contains the UserTask issue type for well-known errors.
	IssueType string
}

// EnrollEKSClusterResponse contains result for enrollment .
type EnrollEKSClusterResponse struct {
	// Results contain an error per a cluster enrollment if there was one.
	Results []EnrollEKSClusterResult
}

// EnrollEKSClusterClient defines functions required for EKS cluster enrollment.
type EnrollEKSClusterClient interface {
	// CreateAccessEntry creates an access entry. An access entry allows an IAM principal to access an EKS cluster.
	CreateAccessEntry(ctx context.Context, params *eks.CreateAccessEntryInput, optFns ...func(*eks.Options)) (*eks.CreateAccessEntryOutput, error)

	// AssociateAccessPolicy associates an access policy and its scope to an access entry.
	AssociateAccessPolicy(ctx context.Context, params *eks.AssociateAccessPolicyInput, optFns ...func(*eks.Options)) (*eks.AssociateAccessPolicyOutput, error)

	// ListAccessEntries lists the access entries for an EKS cluster.
	ListAccessEntries(ctx context.Context, params *eks.ListAccessEntriesInput, optFns ...func(*eks.Options)) (*eks.ListAccessEntriesOutput, error)

	// DeleteAccessEntry deletes an access entry from an EKS cluster.
	DeleteAccessEntry(ctx context.Context, params *eks.DeleteAccessEntryInput, optFns ...func(*eks.Options)) (*eks.DeleteAccessEntryOutput, error)

	// DescribeCluster returns detailed information about an EKS cluster.
	DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)

	// GetCallerIdentity returns details about the IAM user or role whose credentials are used to call the operation.
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)

	// CheckAgentAlreadyInstalled checks if teleport-kube-agent Helm chart is already installed on the EKS cluster.
	CheckAgentAlreadyInstalled(context.Context, genericclioptions.RESTClientGetter) (bool, error)

	// InstallKubeAgent installs teleport-kube-agent Helm chart to the EKS cluster.
	InstallKubeAgent(context.Context, *eksTypes.Cluster, string, string, string, genericclioptions.RESTClientGetter, EnrollEKSClustersRequest) error

	// CreateToken creates provisioning token on the auth server. That token can be used to install kube agent to an EKS cluster.
	CreateToken(context.Context, types.ProvisionToken) error

	// PresignGetCallerIdentityURL creates a presigned URL for the GetCallerIdentity action, that can be used for accessing EKS cluster.
	PresignGetCallerIdentityURL(context.Context, string) (string, error)
}

type defaultEnrollEKSClustersClient struct {
	*eks.Client
	stsClient    *sts.Client
	tokenCreator TokenCreatorFn
}

// GetCallerIdentity returns details about the IAM user or role whose credentials are used to call the operation.
func (d *defaultEnrollEKSClustersClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return d.stsClient.GetCallerIdentity(ctx, params, optFns...)
}

func (d *defaultEnrollEKSClustersClient) PresignGetCallerIdentityURL(ctx context.Context, clusterName string) (string, error) {
	return presignCallerIdentityURL(ctx, d.stsClient, clusterName)
}

// CheckAgentAlreadyInstalled reports whether the teleport-kube-agent
// StatefulSet is already present in the agent namespace.
func (d *defaultEnrollEKSClustersClient) CheckAgentAlreadyInstalled(ctx context.Context, clientGetter genericclioptions.RESTClientGetter) (bool, error) {
	c, err := newKubeClient(clientGetter)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return checkAgentAlreadyInstalled(ctx, c)
}

func getToken(ctx context.Context, clock clockwork.Clock, tokenCreator TokenCreatorFn) (string, string, error) {
	const eksJoinTokenTTL = 30 * time.Minute

	tokenName, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	expires := clock.Now().UTC().Add(eksJoinTokenTTL)

	resourceId := uuid.NewString()
	req := types.ProvisionTokenSpecV2{
		SuggestedLabels: types.Labels{
			types.InternalResourceIDLabel: apiutils.Strings{resourceId},
		},
		Roles: []types.SystemRole{types.RoleKube, types.RoleApp, types.RoleDiscovery},
	}

	provisionToken, err := types.NewProvisionTokenFromSpec(tokenName, expires, req)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	err = tokenCreator(ctx, provisionToken)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	return provisionToken.GetName(), resourceId, trace.Wrap(err)
}

// InstallKubeAgent applies the teleport-kube-agent objects to the EKS cluster.
func (d *defaultEnrollEKSClustersClient) InstallKubeAgent(ctx context.Context, eksCluster *eksTypes.Cluster, proxyAddr, joinToken, resourceId string, clientGetter genericclioptions.RESTClientGetter, req EnrollEKSClustersRequest) error {
	c, err := newKubeClient(clientGetter)
	if err != nil {
		return trace.Wrap(err)
	}
	return installKubeAgent(ctx, c, installKubeAgentParams{
		eksCluster: eksCluster,
		proxyAddr:  proxyAddr,
		joinToken:  joinToken,
		resourceID: resourceId,
		req:        req,
	})
}

// CreateToken creates provisioning token on the auth server. That token can be used to install kube agent to an EKS cluster.
func (d *defaultEnrollEKSClustersClient) CreateToken(ctx context.Context, token types.ProvisionToken) error {
	return d.tokenCreator(ctx, token)
}

// TokenCreatorFn creates join token on the auth server.
type TokenCreatorFn func(ctx context.Context, token types.ProvisionToken) error

// NewEnrollEKSClustersClient returns new client that can be used to enroll EKS clusters into Teleport.
func NewEnrollEKSClustersClient(ctx context.Context, req *AWSClientRequest, tokenCreator TokenCreatorFn) (EnrollEKSClusterClient, error) {
	eksClient, err := newEKSClient(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	stsClient, err := newSTSClient(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt := defaultEnrollEKSClustersClient{
		Client:       eksClient,
		stsClient:    stsClient,
		tokenCreator: tokenCreator,
	}

	return &clt, nil
}

// EnrollEKSClustersRequest contains the required fields to enroll EKS cluster to Teleport.
type EnrollEKSClustersRequest struct {
	// Region is the AWS Region.
	Region string

	// ClusterNames is name of the EKS cluster to enroll.
	ClusterNames []string

	// TeleportClusterName is the name of the Teleport cluster.
	// Used to tag resources created during enrollment.
	TeleportClusterName string

	// IntegrationName is the name of the integration.
	// Used to tag resources created during enrollment.
	IntegrationName string

	// EnableAppDiscovery specifies if we should enable Kubernetes App Discovery inside the enrolled EKS cluster.
	EnableAppDiscovery bool

	// EnableAutoUpgrades specifies if we should enable agent auto upgrades.
	EnableAutoUpgrades bool

	// IsCloud specifies if enrollment is done for the Teleport Cloud client.
	IsCloud bool

	// AgentVersion specifies the teleport version of the teleport-kube-agent
	// containers installed during enrollment. The chart structure itself is
	// fixed at the Teleport binary's build time.
	AgentVersion string

	// ExtraLabels added to the enrolled clusters.
	ExtraLabels map[string]string
}

// CheckAndSetDefaults checks if the required fields are present.
func (e *EnrollEKSClustersRequest) CheckAndSetDefaults() error {
	if e.Region == "" {
		return trace.BadParameter("region is required")
	}

	if len(e.ClusterNames) == 0 {
		return trace.BadParameter("non-empty cluster names is required")
	}

	if e.AgentVersion == "" {
		return trace.BadParameter("agent version is required")
	}

	if e.TeleportClusterName == "" {
		return trace.BadParameter("teleport cluster name is required")
	}

	if e.IntegrationName == "" {
		return trace.BadParameter("integration name is required")
	}

	return nil
}

// EnrollEKSClusters enrolls EKS clusters into Teleport by installing teleport-kube-agent chart on the clusters.
// It returns list of result individually for each EKS cluster. Clusters are enrolled concurrently. If an error occurs
// during a cluster enrollment an error message will be present in the result for this cluster. Otherwise result will
// contain resource ID - this is ID from the join token used by the enrolled cluster and can be used by UI to check
// when agent joins Teleport cluster.
//
// During enrollment we create access entry for an EKS cluster if needed and cluster admin policy is associated with that entry,
// so our AWS integration can access the target EKS cluster during the chart installation. After enrollment is done we remove
// the access entry (if it was created by us), since we don't need it anymore.
func EnrollEKSClusters(ctx context.Context, log *slog.Logger, clock clockwork.Clock, proxyAddr string, clt EnrollEKSClusterClient, req EnrollEKSClustersRequest) (*EnrollEKSClusterResponse, error) {
	var mu sync.Mutex
	var results []EnrollEKSClusterResult

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(concurrentEKSEnrollingLimit)

	for _, eksClusterName := range req.ClusterNames {

		group.Go(func() error {
			resourceId, issueType, err := enrollEKSCluster(ctx, log, clock, clt, proxyAddr, eksClusterName, req)
			if err != nil {
				log.WarnContext(ctx, "Failed to enroll EKS cluster",
					"error", err,
					"cluster", eksClusterName,
					"issue_type", issueType,
				)
			}

			mu.Lock()
			defer mu.Unlock()
			results = append(results, EnrollEKSClusterResult{
				ClusterName: eksClusterName,
				ResourceId:  resourceId,
				Error:       trace.Wrap(err),
				IssueType:   issueType,
			})

			return nil
		})
	}
	// We don't return error from individual group goroutines, they are gathered in the returned value.
	_ = group.Wait()

	return &EnrollEKSClusterResponse{Results: results}, nil
}

func presignCallerIdentityURL(ctx context.Context, stsClient *sts.Client, clusterName string) (string, error) {
	presignClient := sts.NewPresignClient(stsClient)

	// This function adds required headers for accessing an EKS cluster to the presigned URL.
	// Header "x-k8s-aws-id" specifies EKS cluster name and header "X-Amz-Expires" is just required for compatibility reasons.
	addEKSHeaders := func(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (
		out middleware.BuildOutput, metadata middleware.Metadata, err error,
	) {
		req, ok := in.Request.(*smithyhttp.Request)
		if !ok {
			return out, metadata, fmt.Errorf("unknown transport type %T", req)
		}

		req.Header.Add(awsHeaderClusterName, clusterName)
		// 60 is put for compatibility reasons, in reality it is ignored and real expiration time is 15 minutes.
		req.Header.Add(awsHeaderExpires, "60")

		return next.HandleBuild(ctx, in)
	}

	presigned, err := presignClient.PresignGetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}, func(options *sts.PresignOptions) {
		options.ClientOptions = append(options.ClientOptions,
			sts.WithAPIOptions(func(stack *middleware.Stack) error {
				return stack.Build.Add(middleware.BuildMiddlewareFunc("AddEKSHeaders", addEKSHeaders), 0)
			}))
	})
	if err != nil {
		return "", trace.Wrap(err, "failed to presign caller identity")
	}

	return presigned.URL, nil
}

// enrollEKSCluster tries to enroll a single EKS cluster using the EnrollEKSClusterClient.
// Returns the resource id or an error and an issue type which identifies the class of the error that occurred.
func enrollEKSCluster(ctx context.Context, log *slog.Logger, clock clockwork.Clock, clt EnrollEKSClusterClient, proxyAddr, clusterName string, req EnrollEKSClustersRequest) (string, string, error) {
	eksClusterInfo, err := clt.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return "", "", trace.Wrap(err, "unable to describe EKS cluster")
	}
	eksCluster := eksClusterInfo.Cluster

	if eksCluster.Status != eksTypes.ClusterStatusActive {
		return "",
			usertasks.AutoDiscoverEKSIssueStatusNotActive,
			trace.BadParameter(`can't enroll EKS cluster %q - expected "ACTIVE" state, got %q.`, clusterName, eksCluster.Status)
	}

	// We can't discover private EKS clusters for cloud clients, since we know that auth server is running in our VPC.
	if req.IsCloud && !eksCluster.ResourcesVpcConfig.EndpointPublicAccess {
		return "",
			usertasks.AutoDiscoverEKSIssueMissingEndpoingPublicAccess,
			trace.AccessDenied("can't enroll %q because it is not accessible from Teleport Cloud, please enable endpoint public access in your EKS cluster and try again.", clusterName)
	}

	// When clusters are using CONFIG_MAP, API is not acessible and thus Teleport can't install the Teleport's Helm chart.
	// You can read more about the Authentication Modes here: https://aws.amazon.com/blogs/containers/a-deep-dive-into-simplified-amazon-eks-access-management-controls/
	allowedAuthModes := []eksTypes.AuthenticationMode{
		eksTypes.AuthenticationModeApi,
		eksTypes.AuthenticationModeApiAndConfigMap,
	}
	if !slices.Contains(allowedAuthModes, eksCluster.AccessConfig.AuthenticationMode) {
		return "",
			usertasks.AutoDiscoverEKSIssueAuthenticationModeUnsupported,
			trace.BadParameter("can't enroll %q because its access config's authentication mode is %q, only %v are supported", clusterName, eksCluster.AccessConfig.AuthenticationMode, allowedAuthModes)
	}

	principalArn, err := getAccessEntryPrincipalArn(ctx, clt.GetCallerIdentity)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	ownershipTags := defaultResourceCreationTags(req.TeleportClusterName, req.IntegrationName)

	wasAdded, err := maybeAddAccessEntry(ctx, log, clusterName, principalArn, clt, ownershipTags)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	if wasAdded {
		// If we added access entry, we'll clean it up when function stops executing.
		defer func() {
			if _, err := clt.DeleteAccessEntry(ctx, &eks.DeleteAccessEntryInput{
				ClusterName:  aws.String(clusterName),
				PrincipalArn: aws.String(principalArn),
			}); err != nil {
				log.WarnContext(ctx, "Could not delete access entry for principal %q on cluster %q",
					"error", err,
					"principal", principalArn,
					"cluster", clusterName,
				)
			}
		}()
	}

	_, err = clt.AssociateAccessPolicy(ctx, &eks.AssociateAccessPolicyInput{
		AccessScope: &eksTypes.AccessScope{
			Namespaces: nil,
			Type:       eksTypes.AccessScopeTypeCluster,
		},
		ClusterName:  aws.String(clusterName),
		PolicyArn:    aws.String(eksClusterAdminPolicy),
		PrincipalArn: aws.String(principalArn),
	})
	if err != nil {
		return "", "", trace.Wrap(err, "unable to associate EKS Access Policy to cluster %q", clusterName)
	}

	presignedURL, err := clt.PresignGetCallerIdentityURL(ctx, clusterName)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	kubeClientGetter, err := getKubeClientGetter(presignedURL,
		aws.ToString(eksCluster.CertificateAuthority.Data), aws.ToString(eksCluster.Endpoint))
	if err != nil {
		return "", "", trace.Wrap(err, "unable to build kubernetes client for EKS cluster %q", clusterName)
	}

	if alreadyInstalled, err := clt.CheckAgentAlreadyInstalled(ctx, kubeClientGetter); err != nil {
		return "",
			issueTypeFromCheckAgentInstalledError(err),
			trace.Wrap(err, "could not check if teleport-kube-agent is already installed.")
	} else if alreadyInstalled {
		return "",
			// When using EKS Auto Discovery, after the Kube Agent connects to the Teleport cluster, it is ignored in next discovery iterations.
			// Given that this iteration is still hitting this EKS Cluster, it means that the agent can't connect to the Teleport Cluster or is taking too long.
			usertasks.AutoDiscoverEKSIssueAgentNotConnecting,
			// Web UI relies on the text of this error message. If changed, sync with EnrollEksCluster.tsx
			trace.AlreadyExists("teleport-kube-agent is already installed on the cluster %q", clusterName)
	}

	joinToken, resourceId, err := getToken(ctx, clock, clt.CreateToken)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	if err := clt.InstallKubeAgent(ctx, eksCluster, proxyAddr, joinToken, resourceId, kubeClientGetter, req); err != nil {
		return "", "", trace.Wrap(err)
	}

	return resourceId, "", nil
}

func issueTypeFromCheckAgentInstalledError(checkErr error) string {
	// When the Auth Service fails to reach the EKS Cluster, it usually means that, either:
	// - EKS does not have EndpointPublicAccess
	// - EKS is not reachable by the Teleport Auth Service
	// In the first case, it should be handled in a pre-install check, however, for the second one, we'll get the following message:
	// > Kubernetes cluster unreachable: Get \"https://<longid>.gr7.<region>.eks.amazonaws.com/version\": dial tcp: lookup <longid>.gr7.<region>.eks.amazonaws.com: no such host"
	if strings.Contains(checkErr.Error(), "Kubernetes cluster unreachable: Get") && strings.Contains(checkErr.Error(), "eks.amazonaws.com: no such host") {
		return usertasks.AutoDiscoverEKSIssueClusterUnreachable
	}

	return ""
}

// IdentityGetter returns AWS identity of the caller.
type IdentityGetter func(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)

func getAccessEntryPrincipalArn(ctx context.Context, identityGetter IdentityGetter) (string, error) {
	ident, err := identityGetter(ctx, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}

	parsedIdentity, err := awslib.IdentityFromArn(aws.ToString(ident.Arn))
	if err != nil {
		return "", trace.Wrap(err)
	}

	return fmt.Sprintf("arn:aws:iam::%s:role/%s", parsedIdentity.GetAccountID(), parsedIdentity.GetName()), nil
}

// maybeAddAccessEntry checks list of access entries for the EKS cluster and adds one for Teleport if it's missing.
// If access entry was added by this function it will return true as a first value.
func maybeAddAccessEntry(ctx context.Context, log *slog.Logger, clusterName, roleArn string, clt EnrollEKSClusterClient, ownershipTags tags.AWSTags) (bool, error) {
	entries, err := clt.ListAccessEntries(ctx, &eks.ListAccessEntriesInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		return false, trace.Wrap(err)
	}

	if slices.Contains(entries.AccessEntries, roleArn) {
		return false, nil
	}

	createAccessEntryReq := &eks.CreateAccessEntryInput{
		ClusterName:  aws.String(clusterName),
		PrincipalArn: aws.String(roleArn),
		Tags:         ownershipTags.ToMap(),
	}

	_, err = clt.CreateAccessEntry(ctx, createAccessEntryReq)
	if err != nil {
		convertedError := awslib.ConvertRequestFailureError(err)
		if !trace.IsAccessDenied(convertedError) {
			return false, trace.Wrap(err)
		}
		// Adding tags requires the `eks:TagResource` action.
		// This action is now part of the added policies, for previous set ups we didn't include the tag resource action in the policy document.
		// See lib/cloud/aws.StatementForEKSAccess
		// Instead of failing with an error, the Access Entry is created anyway without tags.
		// This resource is meant to be deleted right after the teleport agent is installed.
		createAccessEntryReq.Tags = nil

		log.WarnContext(ctx, "Failed to tag EKS Access Entry, please add eks:TagResource action in IAM Role. Continuing without tags.",
			"principal", roleArn,
			"cluster", clusterName,
		)
		_, err = clt.CreateAccessEntry(ctx, createAccessEntryReq)
	}
	return err == nil, trace.Wrap(err)
}

// getKubeClientGetter returns client getter for kube that can be used to access target EKS cluster
func getKubeClientGetter(presignedUrl, clusterCA, clusterEndpoint string) (*genericclioptions.ConfigFlags, error) {
	kubeToken := awsKubePrefix + base64.RawURLEncoding.EncodeToString([]byte(presignedUrl))

	eksClusterCA, err := base64.StdEncoding.DecodeString(clusterCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	restConfig := &rest.Config{
		Host:        clusterEndpoint,
		BearerToken: kubeToken,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: eksClusterCA,
		},
	}

	configFlags := genericclioptions.NewConfigFlags(false)
	configFlags.WithWrapConfigFn(func(*rest.Config) *rest.Config {
		return restConfig
	})

	return configFlags, nil
}

func newKubeClient(getter genericclioptions.RESTClientGetter) (client.Client, error) {
	cfg, err := getter.ToRESTConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c, nil
}

// checkAgentAlreadyInstalled returns true when the agent's StatefulSet
// already exists in the agent namespace.
func checkAgentAlreadyInstalled(ctx context.Context, c client.Client) (bool, error) {
	const attempts = 6
	const retryDelay = 10 * time.Second
	var statefulSet appsv1.StatefulSet
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		err := c.Get(ctx, client.ObjectKey{Namespace: agentNamespace, Name: agentName}, &statefulSet)
		switch {
		case err == nil:
			return true, nil
		case apierrors.IsNotFound(err):
			return false, nil
		}
		lastErr = err
		if attempt == attempts {
			break
		}
		select {
		case <-time.After(retryDelay):
		case <-ctx.Done():
			return false, trace.NewAggregate(err, ctx.Err())
		}
	}
	return false, trace.Wrap(lastErr)
}

type installKubeAgentParams struct {
	eksCluster *eksTypes.Cluster
	proxyAddr  string
	joinToken  string
	resourceID string
	req        EnrollEKSClustersRequest
}

// installKubeAgent applies the teleport-kube-agent objects to the target
// EKS cluster via server-side apply. The agent namespace is created first
// if missing.
func installKubeAgent(ctx context.Context, c client.Client, cfg installKubeAgentParams) error {
	opts, err := optionsFromRequest(cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	objs, err := kubeagent.Manifests(opts)
	if err != nil {
		return trace.Wrap(err)
	}

	// Ensure the namespace exists. Applying a Namespace is idempotent and
	// safe to call every time.
	ns := &corev1.Namespace{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
		ObjectMeta: metav1.ObjectMeta{Name: opts.Namespace},
	}

	const installFieldOwner = "teleport-kube-agent-installer"

	if err := c.Patch(ctx, ns, client.Apply, client.ForceOwnership, client.FieldOwner(installFieldOwner)); err != nil {
		return trace.Wrap(err, "creating namespace %q", opts.Namespace)
	}

	for _, obj := range objs {
		if err := c.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner(installFieldOwner)); err != nil {
			return trace.Wrap(err, "applying %T %s/%s", obj, obj.GetNamespace(), obj.GetName())
		}
	}
	return nil
}

func optionsFromRequest(cfg installKubeAgentParams) (kubeagent.Options, error) {
	eksTags := make(map[string]string, len(cfg.eksCluster.Tags))
	maps.Copy(eksTags, cfg.eksCluster.Tags)
	eksTags[types.OriginLabel] = types.OriginCloud

	kubeCluster, err := common.NewKubeClusterFromAWSEKS(aws.ToString(cfg.eksCluster.Name), aws.ToString(cfg.eksCluster.Arn), eksTags)
	if err != nil {
		return kubeagent.Options{}, trace.Wrap(err)
	}
	common.ApplyEKSNameSuffix(kubeCluster)

	roles := kubeagent.RoleKube
	if cfg.req.EnableAppDiscovery {
		roles = kubeagent.RoleKubeAppDiscovery
	}

	agentVersion, err := semver.NewVersion(cfg.req.AgentVersion)
	if err != nil {
		return kubeagent.Options{}, trace.Wrap(err, "parsing agent version %q", cfg.req.AgentVersion)
	}

	opts := kubeagent.Options{
		Namespace:        agentNamespace,
		ProxyAddr:        cfg.proxyAddr,
		AuthToken:        cfg.joinToken,
		KubeClusterName:  kubeCluster.GetName(),
		Roles:            roles,
		Enterprise:       modules.GetModules().BuildType() == modules.BuildEnterprise,
		Labels:           kubeAgentLabels(kubeCluster, cfg.resourceID, cfg.req.ExtraLabels),
		RequestedVersion: agentVersion,
	}
	if cfg.req.IsCloud && cfg.req.EnableAutoUpgrades {
		opts.Updater = true
		opts.UpdaterChannel = "stable/cloud"
		opts.HighAvailability = true
	}
	return opts, nil
}

func kubeAgentLabels(kubeCluster types.KubeCluster, resourceID string, extraLabels map[string]string) map[string]string {
	labels := make(map[string]string, len(extraLabels)+len(kubeCluster.GetStaticLabels())+1)
	maps.Copy(labels, extraLabels)
	maps.Copy(labels, kubeCluster.GetStaticLabels())
	labels[types.InternalResourceIDLabel] = resourceID

	return labels
}
