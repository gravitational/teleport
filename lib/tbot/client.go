package tbot

import (
	"context"
	"sync"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	trustv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"google.golang.org/grpc"
)

type Client interface {
	Close() error
	GenerateHostCert(ctx context.Context, in *trustv1.GenerateHostCertRequest, opts ...grpc.CallOption) (*trustv1.GenerateHostCertResponse, error)
	GenerateUserCerts(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error)
	GetAuthPreference(context.Context) (types.AuthPreference, error)
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)
	GetCertAuthority(ctx context.Context, id types.CertAuthID, includeSigningKeys bool) (types.CertAuthority, error)
	GetClusterCACert(ctx context.Context) (*proto.GetClusterCACertResponse, error)
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)
	GetRemoteClusters(ctx context.Context) ([]types.RemoteCluster, error)
	GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error)
	GetRole(ctx context.Context, name string) (types.Role, error)
	GetTrustCertAuthority(ctx context.Context, in *trustv1.GetCertAuthorityRequest, opts ...grpc.CallOption) (*types.CertAuthorityV2, error)
	IssueWorkloadIdentities(ctx context.Context, in *workloadidentityv1pb.IssueWorkloadIdentitiesRequest, opts ...grpc.CallOption) (*workloadidentityv1pb.IssueWorkloadIdentitiesResponse, error)
	IssueWorkloadIdentity(ctx context.Context, in *workloadidentityv1pb.IssueWorkloadIdentityRequest, opts ...grpc.CallOption) (*workloadidentityv1pb.IssueWorkloadIdentityResponse, error)
	ListSPIFFEFederations(ctx context.Context, in *machineidv1pb.ListSPIFFEFederationsRequest, opts ...grpc.CallOption) (*machineidv1pb.ListSPIFFEFederationsResponse, error)
	ListUnifiedResources(ctx context.Context, req *proto.ListUnifiedResourcesRequest) (*proto.ListUnifiedResourcesResponse, error)
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)
	Ping(ctx context.Context) (proto.PingResponse, error)
	ResolveSSHTarget(ctx context.Context, req *proto.ResolveSSHTargetRequest) (*proto.ResolveSSHTargetResponse, error)
	SignJWTSVIDs(ctx context.Context, in *machineidv1pb.SignJWTSVIDsRequest, opts ...grpc.CallOption) (*machineidv1pb.SignJWTSVIDsResponse, error)
	SignX509SVIDs(ctx context.Context, in *machineidv1pb.SignX509SVIDsRequest, opts ...grpc.CallOption) (*machineidv1pb.SignX509SVIDsResponse, error)
	StreamSignedCRL(ctx context.Context, in *workloadidentityv1pb.StreamSignedCRLRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[workloadidentityv1pb.StreamSignedCRLResponse], error)
	SubmitHeartbeat(ctx context.Context, in *machineidv1pb.SubmitHeartbeatRequest, opts ...grpc.CallOption) (*machineidv1pb.SubmitHeartbeatResponse, error)
}

type fallableClient struct {
	mu     sync.Mutex
	client *apiclient.Client
	err    error
}

var _ Client = (*fallableClient)(nil)

func (f *fallableClient) setClient(client *apiclient.Client) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.client = client
	f.err = nil
}

func (f *fallableClient) getClient() (*apiclient.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.client, f.err
}

func (c *fallableClient) Close() error {
	if client, _ := c.getClient(); client != nil {
		return client.Close()
	}
	return nil
}

func (c *fallableClient) GenerateHostCert(ctx context.Context, in *trustv1.GenerateHostCertRequest, opts ...grpc.CallOption) (*trustv1.GenerateHostCertResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.TrustClient().GenerateHostCert(ctx, in, opts...)
}

func (c *fallableClient) GenerateUserCerts(ctx context.Context, in proto.UserCertsRequest) (*proto.Certs, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.GenerateUserCerts(ctx, in)
}

func (c *fallableClient) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.GetAuthPreference(ctx)
}

func (c *fallableClient) GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.GetCertAuthorities(ctx, caType, loadKeys)
}

func (c *fallableClient) GetCertAuthority(ctx context.Context, id types.CertAuthID, includeSigningKeys bool) (types.CertAuthority, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.GetCertAuthority(ctx, id, includeSigningKeys)
}

func (c *fallableClient) GetClusterCACert(ctx context.Context) (*proto.GetClusterCACertResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.GetClusterCACert(ctx)
}

func (c *fallableClient) GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.GetClusterNetworkingConfig(ctx)
}

func (c *fallableClient) GetRemoteClusters(ctx context.Context) ([]types.RemoteCluster, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.GetRemoteClusters(ctx)
}

func (c *fallableClient) GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.GetResources(ctx, req)
}

func (c *fallableClient) GetRole(ctx context.Context, name string) (types.Role, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.GetRole(ctx, name)
}

func (c *fallableClient) GetTrustCertAuthority(ctx context.Context, in *trustv1.GetCertAuthorityRequest, opts ...grpc.CallOption) (*types.CertAuthorityV2, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.TrustClient().GetCertAuthority(ctx, in, opts...)
}

func (c *fallableClient) IssueWorkloadIdentities(ctx context.Context, in *workloadidentityv1pb.IssueWorkloadIdentitiesRequest, opts ...grpc.CallOption) (*workloadidentityv1pb.IssueWorkloadIdentitiesResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.WorkloadIdentityIssuanceClient().IssueWorkloadIdentities(ctx, in, opts...)
}

func (c *fallableClient) IssueWorkloadIdentity(ctx context.Context, in *workloadidentityv1pb.IssueWorkloadIdentityRequest, opts ...grpc.CallOption) (*workloadidentityv1pb.IssueWorkloadIdentityResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.WorkloadIdentityIssuanceClient().IssueWorkloadIdentity(ctx, in, opts...)
}

func (c *fallableClient) ListSPIFFEFederations(ctx context.Context, in *machineidv1pb.ListSPIFFEFederationsRequest, opts ...grpc.CallOption) (*machineidv1pb.ListSPIFFEFederationsResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.SPIFFEFederationServiceClient().ListSPIFFEFederations(ctx, in, opts...)
}

func (c *fallableClient) ListUnifiedResources(ctx context.Context, req *proto.ListUnifiedResourcesRequest) (*proto.ListUnifiedResourcesResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.ListUnifiedResources(ctx, req)
}

func (c *fallableClient) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.NewWatcher(ctx, watch)
}

func (c *fallableClient) Ping(ctx context.Context) (proto.PingResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return proto.PingResponse{}, err
	}
	return client.Ping(ctx)
}

func (c *fallableClient) ResolveSSHTarget(ctx context.Context, req *proto.ResolveSSHTargetRequest) (*proto.ResolveSSHTargetResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.ResolveSSHTarget(ctx, req)
}

func (c *fallableClient) SignJWTSVIDs(ctx context.Context, in *machineidv1pb.SignJWTSVIDsRequest, opts ...grpc.CallOption) (*machineidv1pb.SignJWTSVIDsResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.WorkloadIdentityServiceClient().SignJWTSVIDs(ctx, in, opts...)
}

func (c *fallableClient) SignX509SVIDs(ctx context.Context, in *machineidv1pb.SignX509SVIDsRequest, opts ...grpc.CallOption) (*machineidv1pb.SignX509SVIDsResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.WorkloadIdentityServiceClient().SignX509SVIDs(ctx, in, opts...)
}

func (c *fallableClient) StreamSignedCRL(ctx context.Context, in *workloadidentityv1pb.StreamSignedCRLRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[workloadidentityv1pb.StreamSignedCRLResponse], error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.WorkloadIdentityRevocationServiceClient().StreamSignedCRL(ctx, in, opts...)
}

func (c *fallableClient) SubmitHeartbeat(ctx context.Context, in *machineidv1pb.SubmitHeartbeatRequest, opts ...grpc.CallOption) (*machineidv1pb.SubmitHeartbeatResponse, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}
	return client.BotInstanceServiceClient().SubmitHeartbeat(ctx, in, opts...)
}
