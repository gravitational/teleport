package cloud

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"log/slog"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/transport"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/kube/internal/creds"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
)

type KubernetesCredentials interface {
	GetTLSConfig() *tls.Config
	GetTransportConfig() *transport.Config
	GetTargetAddr() string
	GetKubeRestConfig() *rest.Config
	GetKubeClient() kubernetes.Interface
	GetTransport() http.RoundTripper
	WrapTransport(http.RoundTripper) (http.RoundTripper, error)
	Close() error
}

type AzureClientGetter interface {
	// GetAzureKubernetesClient returns an Azure AKS client for the specified subscription.
	GetAzureKubernetesClient(subscription string) (azure.AKSClient, error)
}

type GCPClientGetter interface {
	// GetGCPGKEClient returns GKE client.
	GetGCPGKEClient(context.Context) (gcp.GKEClient, error)
}

type CredentialsGetter struct {
	clock       clockwork.Clock
	azureClient AzureClientGetter
	awsClient   AWSClientGetter
	gcpGlient   GCPClientGetter

	checker          servicecfg.ImpersonationPermissionsChecker
	resourceMatchers []services.ResourceMatcher
}

type STSPresignClient = kubeutils.STSPresignClient

// EKSClient is the subset of the EKS Client interface we use.
type EKSClient interface {
	eks.DescribeClusterAPIClient
}

// AWSClientGetter is an interface for getting an EKS client and an STS client.
type AWSClientGetter interface {
	awsconfig.Provider
	// GetAWSEKSClient returns AWS EKS client for the specified config.
	GetAWSEKSClient(aws.Config) EKSClient
	// GetAWSSTSPresignClient returns AWS STS presign client for the specified config.
	GetAWSSTSPresignClient(aws.Config) STSPresignClient
}

// getKubeClusterCredentials generates kube credentials for dynamic clusters.
func (c CredentialsGetter) GetKubeClusterCredentials(ctx context.Context, cluster types.KubeCluster) (KubernetesCredentials, error) {
	switch {
	case cluster.IsKubeconfig():
		return c.getStaticCredentialsFromKubeconfig(ctx, cluster)
	case cluster.IsAzure():
		return c.getAzureCredentials(ctx, cluster)
	case cluster.IsAWS():
		return c.getAWSCredentials(ctx, cluster)
	case cluster.IsGCP():
		return c.getGCPCredentials(ctx, cluster)
	default:
		return nil, trace.BadParameter("authentication method provided for cluster %q not supported", cluster.GetName())
	}
}

func (c CredentialsGetter) getStaticCredentialsFromKubeconfig(ctx context.Context, cluster types.KubeCluster) (KubernetesCredentials, error) {
	config, err := clientcmd.Load(cluster.GetKubeconfig())
	if err != nil {
		return nil, trace.WrapWithMessage(err, "unable to parse kubeconfig for cluster %q", cluster.GetName())
	}
	if len(config.CurrentContext) == 0 && len(config.Contexts) > 0 {
		// select the first context key as default context
		for k := range config.Contexts {
			config.CurrentContext = k
			break
		}
	}
	restConfig, err := clientcmd.NewDefaultClientConfig(*config, nil).ClientConfig()
	if err != nil {
		return nil, trace.WrapWithMessage(err, "unable to create client from kubeconfig for cluster %q", cluster.GetName())
	}

	creds, err := creds.ExtractKubeCreds(ctx, "", cluster.GetName(), restConfig, slog.Default(), c.checker)
	return creds, trace.Wrap(err)
}

// azureRestConfigClient creates a dynamicCredsClient that returns credentials to a AKS cluster.
func (c CredentialsGetter) azureRestConfigClient() creds.DynamicCredsClient {
	return func(ctx context.Context, cluster types.KubeCluster) (*rest.Config, time.Time, error) {
		aksClient, err := c.azureClient.GetAzureKubernetesClient(cluster.GetAzureConfig().SubscriptionID)
		if err != nil {
			return nil, time.Time{}, trace.Wrap(err)
		}
		cfg, exp, err := aksClient.ClusterCredentials(ctx, azure.ClusterCredentialsConfig{
			ResourceGroup:                   cluster.GetAzureConfig().ResourceGroup,
			ResourceName:                    cluster.GetAzureConfig().ResourceName,
			TenantID:                        cluster.GetAzureConfig().TenantID,
			ImpersonationPermissionsChecker: creds.CheckImpersonationPermissions,
		})
		return cfg, exp, trace.Wrap(err)
	}
}

func (c CredentialsGetter) getAzureCredentials(ctx context.Context, cluster types.KubeCluster) (KubernetesCredentials, error) {
	creds, err := creds.NewDynamicKubeCreds(ctx, creds.DynamicCredsConfig{
		KubeCluster:      cluster,
		Log:              slog.Default(),
		Checker:          c.checker,
		ResourceMatchers: c.resourceMatchers,
		Clock:            c.clock,
		Client:           c.azureRestConfigClient(),
	})
	return creds, trace.Wrap(err)
}

// getAWSClientRestConfig creates a dynamicCredsClient that generates returns credentials to EKS clusters.
func (c CredentialsGetter) getAWSClientRestConfig() creds.DynamicCredsClient {
	return func(ctx context.Context, cluster types.KubeCluster) (*rest.Config, time.Time, error) {
		region := cluster.GetAWSConfig().Region
		opts := []awsconfig.OptionsFn{
			awsconfig.WithAmbientCredentials(),
		}
		if awsAssume := getAWSResourceMatcherToCluster(cluster, c.resourceMatchers); awsAssume != nil {
			opts = append(opts, awsconfig.WithAssumeRole(awsAssume.AssumeRoleARN, awsAssume.ExternalID))
		}

		cfg, err := c.awsClient.GetConfig(ctx, region, opts...)
		if err != nil {
			return nil, time.Time{}, trace.Wrap(err)
		}

		regionalClient := c.awsClient.GetAWSEKSClient(cfg)

		eksCfg, err := regionalClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
			Name: aws.String(cluster.GetAWSConfig().Name),
		})
		if err != nil {
			return nil, time.Time{}, trace.Wrap(err)
		}

		ca, err := base64.StdEncoding.DecodeString(aws.ToString(eksCfg.Cluster.CertificateAuthority.Data))
		if err != nil {
			return nil, time.Time{}, trace.Wrap(err)
		}

		apiEndpoint := aws.ToString(eksCfg.Cluster.Endpoint)
		if len(apiEndpoint) == 0 {
			return nil, time.Time{}, trace.BadParameter("invalid api endpoint for cluster %q", cluster.GetAWSConfig().Name)
		}

		stsPresignClient := c.awsClient.GetAWSSTSPresignClient(cfg)

		token, exp, err := kubeutils.GenAWSEKSToken(ctx, stsPresignClient, cluster.GetAWSConfig().Name, c.clock)
		if err != nil {
			return nil, time.Time{}, trace.Wrap(err)
		}

		return &rest.Config{
			Host:        apiEndpoint,
			BearerToken: token,
			TLSClientConfig: rest.TLSClientConfig{
				CAData: ca,
			},
		}, exp, nil
	}
}

// getAWSResourceMatcherToCluster returns the AWS assume role ARN and external ID for the cluster that matches the kubeCluster.
// If no match is found, nil is returned, which means that we should not attempt to assume a role.
func getAWSResourceMatcherToCluster(kubeCluster types.KubeCluster, resourceMatchers []services.ResourceMatcher) *services.ResourceMatcherAWS {
	if !kubeCluster.IsAWS() {
		return nil
	}
	for _, matcher := range resourceMatchers {
		if len(matcher.Labels) == 0 || matcher.AWS.AssumeRoleARN == "" {
			continue
		}
		if match, _, _ := services.MatchLabels(matcher.Labels, kubeCluster.GetAllLabels()); !match {
			continue
		}
		return &matcher.AWS
	}
	return nil
}

// getAWSCredentials creates a dynamicKubeCreds that generates and updates the access credentials to a EKS kubernetes cluster.
func (c CredentialsGetter) getAWSCredentials(ctx context.Context, cluster types.KubeCluster) (KubernetesCredentials, error) {
	creds, err := creds.NewDynamicKubeCreds(ctx, creds.DynamicCredsConfig{
		KubeCluster:      cluster,
		Log:              slog.Default(),
		Checker:          c.checker,
		ResourceMatchers: c.resourceMatchers,
		Clock:            c.clock,
		Client:           c.getAWSClientRestConfig(),
	})
	return creds, trace.Wrap(err)
}

func (c CredentialsGetter) getGCPCredentials(ctx context.Context, cluster types.KubeCluster) (KubernetesCredentials, error) {
	// create a client that returns the credentials for kubeCluster
	creds, err := creds.NewDynamicKubeCreds(ctx, creds.DynamicCredsConfig{
		KubeCluster:      cluster,
		Log:              slog.Default(),
		Checker:          c.checker,
		ResourceMatchers: c.resourceMatchers,
		Clock:            c.clock,
		Client:           c.gcpRestConfigClient(),
	})
	return creds, trace.Wrap(err)
}

// gcpRestConfigClient creates a dynamicCredsClient that returns credentials to a GKE cluster.
func (c CredentialsGetter) gcpRestConfigClient() creds.DynamicCredsClient {
	return func(ctx context.Context, cluster types.KubeCluster) (*rest.Config, time.Time, error) {
		gkeClient, err := c.gcpGlient.GetGCPGKEClient(ctx)
		if err != nil {
			return nil, time.Time{}, trace.Wrap(err)
		}
		cfg, exp, err := gkeClient.GetClusterRestConfig(ctx,
			gcp.ClusterDetails{
				ProjectID: cluster.GetGCPConfig().ProjectID,
				Location:  cluster.GetGCPConfig().Location,
				Name:      cluster.GetGCPConfig().Name,
			},
		)
		return cfg, exp, trace.Wrap(err)
	}
}
