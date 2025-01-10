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

package kubev1

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	proto "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

// errDone indicates that resource iteration is complete
var errDone = errors.New("done iterating")

// Server implements KubeService gRPC server.
type Server struct {
	proto.UnimplementedKubeServiceServer
	cfg          Config
	proxyAddress string
	kubeProxySNI string
	kubeClient   kubernetes.Interface
}

// New creates a new instance of Kube gRPC handler.
func New(cfg Config) (*Server, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	var sni, addr string
	var err error
	if cfg.KubeProxyAddr != "" {
		sni, addr, err = getWebAddrAndKubeSNI(cfg.KubeProxyAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	s := &Server{cfg: cfg, proxyAddress: addr, kubeProxySNI: sni}

	if s.kubeClient, err = s.buildKubeClient(); err != nil {
		return nil, trace.Wrap(err, "unable to create kubeClient")
	}

	return s, nil
}

// Config specifies configuration for Kube gRPC server.
type Config struct {
	// AccessPoint is caching access point to retrieve roles and the cluster
	// auth preference.
	AccessPoint AccessPoint
	// Authz authenticates user.
	Authz authz.Authorizer
	// Log is the logger function.
	Log *slog.Logger
	// Emitter is used to emit audit events.
	Emitter apievents.Emitter
	// Component name to include in log output.
	Component string
	// KubeProxyAddr is the address that can be used to reach the Kubernetes Proxy.
	KubeProxyAddr string
	// ClusterName is the name of the cluster that this server is running in.
	ClusterName string
	// GetConnTLSCertificate returns the TLS kubeClient certificate to use when
	// connecting to the upstream Teleport proxy or Kubernetes service when
	// forwarding requests using the forward identity (i.e. proxy impersonating
	// a user) method. Paired with GetConnTLSRoots and ConnTLSCipherSuites to
	// generate the correct [*tls.Config] on demand.
	GetConnTLSCertificate utils.GetCertificateFunc
	// GetConnTLSRoots returns the [*x509.CertPool] used to validate TLS
	// connections to the upstream Teleport proxy or Kubernetes service.
	GetConnTLSRoots utils.GetRootsFunc
	// ConnTLSCipherSuites optionally contains a list of TLS ciphersuites to use
	// when connecting to the upstream Teleport Proxy or Kubernetes service.
	ConnTLSCipherSuites []uint16
}

// AccessPoint is caching access point to retrieve roles and the cluster
// auth preference.
type AccessPoint interface {
	services.RoleGetter
}

// CheckAndSetDefaults checks and sets default values.
func (c *Config) CheckAndSetDefaults() error {
	if c.GetConnTLSCertificate == nil {
		return trace.BadParameter("missing parameter GetConnTLSCertificate")
	}
	if c.GetConnTLSRoots == nil {
		return trace.BadParameter("missing parameter GetConnTLSRoots")
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

	if c.ClusterName == "" {
		return trace.BadParameter("missing parameter ClusterName")
	}
	if c.Component == "" {
		c.Component = "kube.grpc"
	}
	if c.Log == nil {
		c.Log = slog.Default()
	}
	c.Log = c.Log.With(teleport.ComponentKey, c.Component)
	return nil
}

// ListKubernetesResources returns the list of resources available for the user for
// the specified Resource kind, Kubernetes cluster and namespace.
func (s *Server) ListKubernetesResources(ctx context.Context, req *proto.ListKubernetesResourcesRequest) (*proto.ListKubernetesResourcesResponse, error) {
	if s.proxyAddress == "" {
		return nil, trail.ToGRPC(trace.ConnectionProblem(nil, "Kubernetes API is disabled"))
	}
	userContext, err := s.authorize(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if req.UseSearchAsRoles || req.UsePreviewAsRoles {
		var extraRoles []string
		if req.UseSearchAsRoles {
			allowedSearchAsRoles := userContext.Checker.GetAllowedSearchAsRolesForKubeResourceKind(req.ResourceType)
			extraRoles = append(extraRoles, allowedSearchAsRoles...)
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
	ctx = authz.ContextWithUser(ctx, authz.WrapIdentity(identity)) // wrap the identity in the context

	switch {
	case requiresFakePagination(req):
		rsp, err := s.listResourcesUsingFakePagination(ctx, req)
		return rsp, trail.ToGRPC(err)
	case slices.Contains(types.KubernetesResourcesKinds, req.ResourceType):
		rsp, err := s.listKubernetesResources(ctx, true, req)
		return rsp, trail.ToGRPC(err)
	default:
		return nil, trail.ToGRPC(trace.BadParameter("unsupported resource type %q", req.ResourceType))
	}
}

// authorize checks if the user is authorized to connect to the cluster.
func (s *Server) authorize(ctx context.Context) (*authz.Context, error) {
	authCtx, err := s.cfg.Authz.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
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

// listKubernetesResources returns the list of resources available for the user for
// the specified Kubernetes cluster and namespace. If respectLimit is true,
// the limit will be respected, otherwise the limit will be ignored and we return
// all resources from type=req.ResourceType available to the user.
// If any search parameters are specified, the only resources returned will be
// those that match the search parameters.
func (s *Server) listKubernetesResources(
	ctx context.Context,
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
		ResourceKind:   req.ResourceType,
		Labels:         req.Labels,
		SearchKeywords: req.SearchKeywords,
	}

	if req.PredicateExpression != "" {
		expression, err := services.NewResourceExpression(req.PredicateExpression)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		filter.PredicateExpression = expression
	}

	rsp := &proto.ListKubernetesResourcesResponse{}
	err := s.iterateKubernetesResources(
		ctx, req, respectLimit,
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

// iterateKubernetesResources creates a new Kubernetes Client with temporary user
// certificates and iterates through the returned Kubernetes resources.
// For each resources discovered, the fn function is called to decide the action.
// Kubernetes continue key is a base64 encoded json payload with the resource
// version of the request. In order to resume the operation when using the paginated
// mode, Teleport respects the Kubernetes Continue Key and will return it to the kubeClient
// as a NextKey.
// In order to have the expected behavior Teleport must respect the ContinueKey and
// cannot manipulate it. It means that Teleport needs to manipulate the number of
// requested items from the Kubernetes Cluster in order to have the expected behavior.
func (s *Server) iterateKubernetesResources(
	ctx context.Context,
	req *proto.ListKubernetesResourcesRequest,
	respectLimit bool,
	fn func(*types.KubernetesResourceV1, string) (int, error),
) error {
	kubeClient := s.kubeClient

	continueKey := req.StartKey
	itemsAppended := 0
	for {
		var (
			items           []kObject
			nextContinueKey string
			listOpts        = metav1.ListOptions{
				Limit:    decideLimit(int64(req.Limit), int64(itemsAppended), respectLimit),
				Continue: continueKey,
			}
		)

		switch req.ResourceType {
		case types.KindKubePod:
			lItems, err := kubeClient.CoreV1().Pods(req.KubernetesNamespace).List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeSecret:
			lItems, err := kubeClient.CoreV1().Secrets(req.KubernetesNamespace).List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeConfigmap:
			lItems, err := kubeClient.CoreV1().ConfigMaps(req.KubernetesNamespace).List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeNamespace:
			lItems, err := kubeClient.CoreV1().Namespaces().List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeService:
			lItems, err := kubeClient.CoreV1().Services(req.KubernetesNamespace).List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeServiceAccount:
			lItems, err := kubeClient.CoreV1().ServiceAccounts(req.KubernetesNamespace).List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeNode:
			lItems, err := kubeClient.CoreV1().Nodes().List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubePersistentVolume:
			lItems, err := kubeClient.CoreV1().PersistentVolumes().List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubePersistentVolumeClaim:
			lItems, err := kubeClient.CoreV1().PersistentVolumeClaims(req.KubernetesNamespace).List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeDeployment:
			lItems, err := kubeClient.AppsV1().Deployments(req.KubernetesNamespace).List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeReplicaSet:
			lItems, err := kubeClient.AppsV1().ReplicaSets(req.KubernetesNamespace).List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeStatefulset:
			lItems, err := kubeClient.AppsV1().StatefulSets(req.KubernetesNamespace).List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeDaemonSet:
			lItems, err := kubeClient.AppsV1().DaemonSets(req.KubernetesNamespace).List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeClusterRole:
			lItems, err := kubeClient.RbacV1().ClusterRoles().List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeRole:
			lItems, err := kubeClient.RbacV1().Roles(req.KubernetesNamespace).List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeClusterRoleBinding:
			lItems, err := kubeClient.RbacV1().ClusterRoleBindings().List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeRoleBinding:
			lItems, err := kubeClient.RbacV1().RoleBindings(req.KubernetesNamespace).List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeCronjob:
			lItems, err := kubeClient.BatchV1().CronJobs(req.KubernetesNamespace).List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeJob:
			lItems, err := kubeClient.BatchV1().Jobs(req.KubernetesNamespace).List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeCertificateSigningRequest:
			lItems, err := kubeClient.CertificatesV1().CertificateSigningRequests().List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		case types.KindKubeIngress:
			lItems, err := kubeClient.NetworkingV1().Ingresses(req.KubernetesNamespace).List(ctx, listOpts)
			if err != nil {
				return trace.Wrap(err)
			}
			items = itemListToKObjectList(itemListToItemListPtr(lItems.Items))
			nextContinueKey = lItems.Continue
		default:
			return trace.BadParameter("unsupported resource type: %q", req.ResourceType)
		}

		for _, resource := range items {
			resource, err := getKubernetesResourceFromKObject(resource, req.ResourceType)
			if err != nil {
				return trace.Wrap(err)
			}

			itemsAppended, err = fn(resource, nextContinueKey)
			if errors.Is(err, errDone) {
				return nil
			} else if err != nil {
				return trace.Wrap(err)
			}
		}

		if len(nextContinueKey) == 0 {
			return nil
		}
		continueKey = nextContinueKey
	}
}

// kObject is an interface that all Kubernetes objects implement.
type kObject interface {
	GetName() string
	GetNamespace() string
	GetLabels() map[string]string
}

// getKubernetesResourceFromKObject converts a Kubernetes object to a
// KubernetesResourceV1.
func getKubernetesResourceFromKObject(
	kObj kObject,
	resourceType string,
) (*types.KubernetesResourceV1, error) {
	return types.NewKubernetesResourceV1(
		resourceType,
		types.Metadata{
			Name:   kObj.GetName(),
			Labels: kObj.GetLabels(),
		},
		types.KubernetesResourceSpecV1{
			Namespace: kObj.GetNamespace(),
		},
	)
}

// itemListToItemListPtr is a helper function that converts a list of items
// to a list of pointers to items.
// This is needed because the Kubernetes API returns a list of items, but
// only a list of pointers to items satisfies the kObject interface.
func itemListToItemListPtr[T any](items []T) []*T {
	kObjects := make([]*T, len(items))
	for i := range items {
		kObjects[i] = &(items[i])
	}
	return kObjects
}

// itemListToKObjectList is a helper function that converts a list of items
// to a list of kObjects.
func itemListToKObjectList[T kObject](items []T) []kObject {
	kObjects := make([]kObject, len(items))
	for i, item := range items {
		kObjects[i] = item
	}
	return kObjects
}

// listResourcesUsingFakePagination is a helper function that lists Kubernetes
// resources using fake pagination. It is used when the kubeClient requires
// the total count or sorting.
func (s *Server) listResourcesUsingFakePagination(
	ctx context.Context,
	req *proto.ListKubernetesResourcesRequest,
) (*proto.ListKubernetesResourcesResponse, error) {
	var (
		rsp *proto.ListKubernetesResourcesResponse
		err error
	)
	switch {
	case slices.Contains(types.KubernetesResourcesKinds, req.ResourceType):
		rsp, err = s.listKubernetesResources(ctx, false /* do not respect the limit value */, req)
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

	// map the request to the fake pagination request.
	params := local.FakePaginateParams{
		StartKey:       req.StartKey,
		Limit:          req.Limit,
		ResourceType:   req.ResourceType,
		Labels:         req.Labels,
		SearchKeywords: req.SearchKeywords,
	}

	if req.PredicateExpression != "" {
		expression, err := services.NewResourceExpression(req.PredicateExpression)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		params.PredicateExpression = expression
	}

	// Apply request filters and get pagination info.
	fakeRsp, err := local.FakePaginate(sortedClusters.AsResources(), params)
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
