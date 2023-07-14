// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubev1

import (
	"context"
	"errors"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	proto "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

// errDone indicates that resource iteration is complete
var errDone = errors.New("done iterating")

// Server implements KubeService gRPC server.
type Server struct {
	proto.UnimplementedKubeServiceServer
	cfg          Config
	proxyAddress string
	kubeProxySNI string
}

// New creates a new instance of Kube gRPC handler.
func New(cfg Config) (*Server, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	sni, addr, err := getWebAddrAndKubeSNI(cfg.KubeProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Server{cfg: cfg, proxyAddress: addr, kubeProxySNI: sni}, nil
}

// Config specifies configuration for Kube gRPC server.
type Config struct {
	// Signer is a auth server client to sign Kubernetes Certificates.
	Signer CertificateSigner
	// AccessPoint is caching access point to retrieve search and preview roles
	// from the backend.
	AccessPoint services.RoleGetter
	// Authz authenticates user.
	Authz authz.Authorizer
	// Log is the logger function.
	Log logrus.FieldLogger
	// Emitter is used to emit audit events.
	Emitter apievents.Emitter
	// Component name to include in log output.
	Component string
	// KubeProxyAddr is the address that can be used to reach the Kubernetes Proxy.
	KubeProxyAddr string
	// ClusterName is the name of the cluster that this server is running in.
	ClusterName string
}

// CertificateSigner is an interface for signing Kubernetes certificates.
type CertificateSigner interface {
	// ProcessKubeCSR processes CSR request against Kubernetes CA, returns
	// signed certificate if successful.
	ProcessKubeCSR(req auth.KubeCSR) (*auth.KubeCSRResponse, error)
}

// CheckAndSetDefaults checks and sets default values.
func (c *Config) CheckAndSetDefaults() error {
	if c.Signer == nil {
		return trace.BadParameter("missing parameter Signer")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing parameter AccessPoint")
	}
	if c.Authz == nil {
		return trace.BadParameter("missing parameter Authz")
	}
	if c.Emitter == nil {
		return trace.BadParameter("missing parameter Emitter")
	}
	if c.KubeProxyAddr == "" {
		return trace.BadParameter("missing parameter KubeProxyAddr")
	}
	if c.ClusterName == "" {
		return trace.BadParameter("missing parameter ClusterName")
	}
	if c.Component == "" {
		c.Component = "kube.grpc"
	}
	if c.Log == nil {
		c.Log = logrus.New()
	}
	c.Log = c.Log.WithFields(logrus.Fields{trace.Component: c.Component})
	return nil
}

// ListKubernetesResources returns the list of pods available for the user for
// the specified Kubernetes cluster and namespace.
func (s *Server) ListKubernetesResources(ctx context.Context, req *proto.ListKubernetesResourcesRequest) (*proto.ListKubernetesResourcesResponse, error) {
	userContext, err := s.authorize(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if req.UseSearchAsRoles || req.UsePreviewAsRoles {
		var extraRoles []string
		if req.UseSearchAsRoles {
			extraRoles = append(extraRoles, userContext.Checker.GetAllowedSearchAsRoles()...)
		}
		if req.UsePreviewAsRoles {
			extraRoles = append(extraRoles, userContext.Checker.GetAllowedPreviewAsRoles()...)
		}

		extendedContext, err := userContext.WithExtraRoles(s.cfg.AccessPoint, s.cfg.ClusterName, extraRoles)
		if err != nil {
			return nil, trail.ToGRPC(err)
		}
		if len(extendedContext.Checker.RoleNames()) != len(userContext.Checker.RoleNames()) {
			if err := s.emitAuditEvent(ctx, userContext, req); err != nil {
				return nil, trail.ToGRPC(err)
			}
		}
		userContext = extendedContext
	}
	// We use the unmapped identity here because Kube Proxy will handle
	// the forwarding of the request to the correct leaf cluster if that's the case
	// and it handles the mapping of the identity to the leaf cluster.
	identity := userContext.UnmappedIdentity.GetIdentity()
	identity.KubernetesCluster = req.KubernetesCluster
	identity.Groups = userContext.Checker.RoleNames()
	identity.RouteToCluster = req.TeleportCluster
	switch {
	case requiresFakePagination(req):
		rsp, err := s.listResourcesUsingFakePagination(ctx, identity, req)
		return rsp, trail.ToGRPC(err)
	case req.ResourceType == types.KindKubePod:
		rsp, err := s.listKubernetesPods(ctx, identity, true, req)
		return rsp, trail.ToGRPC(err)
	default:
		return nil, trail.ToGRPC(trace.BadParameter("unsupported resource type %q", req.ResourceType))
	}
}

// authorize checks if the user is authorized to connect to the cluster.
func (s *Server) authorize(ctx context.Context) (*authz.Context, error) {
	authCtx, err := s.cfg.Authz.Authorize(ctx)
	if err != nil {
		return nil, authz.ConvertAuthorizerError(ctx, s.cfg.Log, err)
	}
	return authCtx, nil
}

// emitAuditEvent emits an audit event for a resource search action and logs
// the roles used to perform the search.
func (s *Server) emitAuditEvent(ctx context.Context, userContext *authz.Context, req *proto.ListKubernetesResourcesRequest) error {
	err := s.cfg.Emitter.EmitAuditEvent(
		ctx,
		&apievents.AccessRequestResourceSearch{
			Metadata: apievents.Metadata{
				Type: events.AccessRequestResourceSearch,
				Code: events.AccessRequestResourceSearchCode,
			},
			UserMetadata:        authz.ClientUserMetadata(ctx),
			SearchAsRoles:       userContext.Checker.RoleNames(),
			ResourceType:        req.ResourceType,
			Namespace:           defaults.Namespace,
			Labels:              req.Labels,
			PredicateExpression: req.PredicateExpression,
			SearchKeywords:      req.SearchKeywords,
		})
	return trace.Wrap(err)
}

// listKubernetesPods returns the list of pods available for the user for
// the specified Kubernetes cluster and namespace. If respectLimit is true,
// the limit will be respected, otherwise the limit will be ignored and we return
// all pods available to the user. If any search parameters are specified, the
// only pods returned will be those that match the search parameters.
func (s *Server) listKubernetesPods(
	ctx context.Context,
	identity tlsca.Identity,
	respectLimit bool,
	req *proto.ListKubernetesResourcesRequest,
) (*proto.ListKubernetesResourcesResponse, error) {
	if req.KubernetesCluster == "" {
		return nil, trace.BadParameter("missing parameter KubernetesCluster")
	}
	if req.TeleportCluster == "" {
		return nil, trace.BadParameter("missing parameter TeleportCluster")
	}

	limit := int(req.Limit)
	filter := services.MatchResourceFilter{
		ResourceKind:        req.ResourceType,
		Labels:              req.Labels,
		SearchKeywords:      req.SearchKeywords,
		PredicateExpression: req.PredicateExpression,
	}

	rsp := &proto.ListKubernetesResourcesResponse{}
	err := s.iterateKubernetesPods(
		ctx, identity, req, respectLimit,
		func(r *types.KubernetesResourceV1, continueKey string) (int, error) {
			switch match, err := services.MatchResourceByFilters(r, filter, nil /* ignore dup matches  */); {
			case err != nil:
				return len(rsp.Resources), trace.Wrap(err)
			case match:
				rsp.Resources = append(rsp.Resources, r)
			}
			// repectLimit is true only if we do not require the fake pagination field.
			if len(rsp.Resources) == limit && respectLimit {
				rsp.NextKey = continueKey
				return len(rsp.Resources), errDone
			}
			return len(rsp.Resources), nil
		},
	)
	return rsp, trace.Wrap(err)
}

// iterateKubernetesPods creates a new Kubernetes Client with temporary user
// certificates and iterates through the returned Kubernetes Pods.
// For each Pod discovered, the fn function is called to decide the action.
// Kubernetes continue key is a base64 encoded json payload with the resource
// version of the request. In order to resume the operation when using the paginated
// mode, Teleport respects the Kubernetes Continue Key and will return it to the client
// as a NextKey.
// In order to have the expected behavior Teleport must respect the ContinueKey and
// cannot manipulate it. It means that Teleport needs to manipulate the number of
// requested items from the Kubernetes Cluster in order to have the expected behavior.
func (s *Server) iterateKubernetesPods(
	ctx context.Context,
	identity tlsca.Identity,
	req *proto.ListKubernetesResourcesRequest,
	respectLimit bool,
	fn func(*types.KubernetesResourceV1, string) (int, error),
) error {
	kubeClient, err := s.newKubernetesClient(s.cfg.ClusterName, identity)
	if err != nil {
		s.cfg.Log.WithError(err).Warnf("unable to create a Kubernetes client for user %q", identity.Username)
		// Hide the root cause of the error from the client.
		return trace.Errorf("unable to create a Kubernetes client for user %q", identity.Username)
	}
	continueKey := req.StartKey
	itemsAppended := 0
	for {
		podList, err := kubeClient.CoreV1().Pods(req.KubernetesNamespace).List(ctx, metav1.ListOptions{
			Limit:    decideLimit(int64(req.Limit), int64(itemsAppended), respectLimit),
			Continue: continueKey,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		for _, pod := range podList.Items {
			resource, err := types.NewKubernetesPodV1(
				types.Metadata{
					Name:   pod.Name,
					Labels: pod.Labels,
				},
				types.KubernetesResourceSpecV1{
					Namespace: pod.Namespace,
				},
			)
			if err != nil {
				return trace.Wrap(err)
			}

			itemsAppended, err = fn(resource, podList.Continue)
			if errors.Is(err, errDone) {
				return nil
			} else if err != nil {
				return trace.Wrap(err)
			}
		}

		if len(podList.Continue) == 0 {
			return nil
		}
		continueKey = podList.Continue
	}
}

// listResourcesUsingFakePagination is a helper function that lists Kubernetes
// resources using fake pagination. It is used when the client requires
// the total count or sorting.
func (s *Server) listResourcesUsingFakePagination(
	ctx context.Context, identity tlsca.Identity,
	req *proto.ListKubernetesResourcesRequest,
) (*proto.ListKubernetesResourcesResponse, error) {
	var (
		rsp *proto.ListKubernetesResourcesResponse
		err error
	)
	switch {
	case req.ResourceType == types.KindKubePod:
		rsp, err = s.listKubernetesPods(ctx, identity, false /* do not respect the limit value */, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("unsupported resource type %q", req.ResourceType)
	}

	sortedClusters := types.KubeResources(rsp.Resources)
	if req.SortBy != nil {
		if err := sortedClusters.SortByCustom(*req.SortBy); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	// Apply request filters and get pagination info.
	fakeRsp, err := local.FakePaginate(
		sortedClusters.AsResources(),
		// map the request to the fake pagination request.
		apiproto.ListResourcesRequest{
			StartKey:            req.StartKey,
			Limit:               req.Limit,
			ResourceType:        req.ResourceType,
			Labels:              req.Labels,
			PredicateExpression: req.PredicateExpression,
			SearchKeywords:      req.SearchKeywords,
			NeedTotalCount:      req.NeedTotalCount,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resources, err := resourcesToKubeResources(fakeRsp.Resources)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.ListKubernetesResourcesResponse{
		Resources:  resources,
		NextKey:    fakeRsp.NextKey,
		TotalCount: int32(fakeRsp.TotalCount),
	}, nil
}

// requiresFakePagination returns true if the request requires the fake pagination.
func requiresFakePagination(req *proto.ListKubernetesResourcesRequest) bool {
	return req.SortBy != nil && req.SortBy.Field != "" || req.NeedTotalCount
}

// resourcesToKubeResources converts a list of resources to a list of Kubernetes resources.
func resourcesToKubeResources(resources types.ResourcesWithLabels) ([]*types.KubernetesResourceV1, error) {
	kubeResources := make(types.KubeResources, 0, len(resources))
	for _, resource := range resources {
		kubeResource, ok := resource.(*types.KubernetesResourceV1)
		if !ok {
			return nil, trace.BadParameter("expected resource type %T, got %T", types.KubernetesResourceV1{}, resource)
		}
		kubeResources = append(kubeResources, kubeResource)
	}
	return kubeResources, nil
}
