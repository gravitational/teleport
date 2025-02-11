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

package proxy

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/gravitational/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	authv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/slices"
)

// deleteResourcesCollection calls listResources method to list the resources the user
// has access to and calls their delete method using the allowed kube principals.
func (f *Forwarder) deleteResourcesCollection(sess *clusterSession, w http.ResponseWriter, req *http.Request) (resp any, err error) {
	ctx, span := f.cfg.tracer.Start(
		req.Context(),
		"kube.Forwarder/deleteResourcesCollection",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
			semconv.RPCMethodKey.String("deleteResourcesCollection"),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	defer span.End()
	req = req.WithContext(ctx)
	var (
		isLocalKubeCluster = sess.isLocalKubernetesCluster
		kubeObjType        string
		namespace          string
	)

	if isLocalKubeCluster {
		namespace = sess.apiResource.namespace
		kubeObjType, _ = sess.rbacSupportedResources.getTeleportResourceKindFromAPIResource(sess.apiResource)
	}
	// status holds the returned response code.
	var status int
	switch {
	// Check if the target Kubernetes cluster is not served by the current service.
	// If it's the case, forward the request to the target Kube Service where the
	// filtering logic will be applied.
	case !isLocalKubeCluster:
		rw := httplib.NewResponseStatusRecorder(w)
		sess.forwarder.ServeHTTP(rw, req)
		status = rw.Status()
	case kubeObjType == utils.KubeCustomResource && namespace != "":
		status, err = f.handleDeleteCustomResourceCollection(w, req, sess)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		memoryRW := responsewriters.NewMemoryResponseWriter()
		listReq := req.Clone(req.Context())
		// reset body and method since list does not need the body response.
		listReq.Body = nil
		listReq.Method = http.MethodGet
		_, err = f.listResources(sess, memoryRW, listReq)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// decompress the response body to be able to parse it.
		if err := decompressInplace(memoryRW); err != nil {
			return nil, trace.Wrap(err)
		}
		status, err = f.handleDeleteCollectionReq(req, sess, memoryRW, w)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	f.emitAuditEvent(req, sess, status)

	return nil, nil
}

func (f *Forwarder) handleDeleteCollectionReq(req *http.Request, sess *clusterSession, memWriter *responsewriters.MemoryResponseWriter, w http.ResponseWriter) (int, error) {
	ctx, span := f.cfg.tracer.Start(
		req.Context(),
		"kube.Forwarder/handleDeleteCollectionReq",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
			semconv.RPCMethodKey.String("deletePodsCollection"),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	defer span.End()

	const internalErrStatus = http.StatusInternalServerError
	// get content-type value
	deleteRequestContentType := responsewriters.GetContentTypeHeader(req.Header)
	deleteRequestEncoder, deleteRequestDecoder, err := newEncoderAndDecoderForContentType(
		deleteRequestContentType,
		newClientNegotiator(sess.codecFactory),
	)
	if err != nil {
		return internalErrStatus, trace.Wrap(err)
	}

	deleteOptions, err := parseDeleteCollectionBody(req.Body, deleteRequestDecoder)
	if err != nil {
		return internalErrStatus, trace.Wrap(err)
	}
	req.Body.Close()

	// decode memory rw body.
	// We are reading an API request and API honors the GVK in the request so we don't
	// need to set it.
	_, listRequestDecoder, err := newEncoderAndDecoderForContentType(
		responsewriters.GetContentTypeHeader(memWriter.Header()),
		newClientNegotiator(sess.codecFactory),
	)
	if err != nil {
		return internalErrStatus, trace.Wrap(err)
	}
	obj, err := decodeAndSetGVK(listRequestDecoder, memWriter.Buffer().Bytes(), nil /* defaults GVK */)
	if err != nil {
		return internalErrStatus, trace.Wrap(err)
	}

	details, err := f.findKubeDetailsByClusterName(sess.kubeClusterName)
	if err != nil {
		return internalErrStatus, trace.Wrap(err)
	}
	params := deleteResourcesCommonParams{
		ctx:         ctx,
		log:         f.log,
		authCtx:     &sess.authContext,
		header:      req.Header,
		kubeDetails: details,
	}

	// At this point, items already include the list of pods the filtered pods the
	// user has access to.
	// For each Pod, we compute the kubernetes_groups and kubernetes_labels
	// that are applicable and we will forward them as the delete request.
	// If request is a dry-run.
	// TODO (tigrato):
	//  - parallelize loop
	//  -  check if the request should stop at the first fail.

	switch o := obj.(type) {
	case *metav1.Status:
		// Do nothing.
	case *corev1.PodList:
		items, err := deleteResources(
			params,
			types.KindKubePod,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.CoreV1().Pods(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *corev1.SecretList:
		items, err := deleteResources(
			params,
			types.KindKubeSecret,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.CoreV1().Secrets(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *corev1.ConfigMapList:
		items, err := deleteResources(
			params,
			types.KindKubeConfigmap,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.CoreV1().ConfigMaps(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *corev1.NamespaceList:
		items, err := deleteResources(
			params,
			types.KindKubeNamespace,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, _ string) error {
				return trace.Wrap(client.CoreV1().Namespaces().Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *corev1.ServiceAccountList:
		items, err := deleteResources(
			params,
			types.KindKubeServiceAccount,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.CoreV1().ServiceAccounts(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *corev1.PersistentVolumeList:
		items, err := deleteResources(
			params,
			types.KindKubePersistentVolume,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, _ string) error {
				return trace.Wrap(client.CoreV1().PersistentVolumes().Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)

	case *corev1.PersistentVolumeClaimList:
		items, err := deleteResources(
			params,
			types.KindKubePersistentVolumeClaim,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *appsv1.DeploymentList:
		items, err := deleteResources(
			params,
			types.KindKubeDeployment,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.AppsV1().Deployments(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *appsv1.ReplicaSetList:
		items, err := deleteResources(
			params,
			types.KindKubeReplicaSet,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.AppsV1().ReplicaSets(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)

	case *appsv1.StatefulSetList:
		items, err := deleteResources(
			params,
			types.KindKubeStatefulset,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.AppsV1().StatefulSets(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *appsv1.DaemonSetList:
		items, err := deleteResources(
			params,
			types.KindKubeDaemonSet,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.AppsV1().DaemonSets(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)

	case *authv1.ClusterRoleList:
		items, err := deleteResources(
			params,
			types.KindKubeClusterRole,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, _ string) error {
				return trace.Wrap(client.RbacV1().ClusterRoles().Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *authv1.RoleList:
		items, err := deleteResources(
			params,
			types.KindKubeRole,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.RbacV1().Roles(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *authv1.ClusterRoleBindingList:
		items, err := deleteResources(
			params,
			types.KindKubeClusterRoleBinding,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, _ string) error {
				return trace.Wrap(client.RbacV1().ClusterRoleBindings().Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *authv1.RoleBindingList:
		items, err := deleteResources(
			params,
			types.KindKubeRoleBinding,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.RbacV1().RoleBindings(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *batchv1.CronJobList:
		items, err := deleteResources(
			params,
			types.KindKubeCronjob,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.BatchV1().CronJobs(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *batchv1.JobList:
		items, err := deleteResources(
			params,
			types.KindKubeJob,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.BatchV1().Jobs(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *certificatesv1.CertificateSigningRequestList:
		items, err := deleteResources(
			params,
			types.KindKubeCertificateSigningRequest,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, _ string) error {
				return trace.Wrap(client.CertificatesV1().CertificateSigningRequests().Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *networkingv1.IngressList:
		items, err := deleteResources(
			params,
			types.KindKubeIngress,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.NetworkingV1().Ingresses(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *extensionsv1beta1.IngressList:
		items, err := deleteResources(
			params,
			types.KindKubeIngress,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.ExtensionsV1beta1().Ingresses(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *extensionsv1beta1.DaemonSetList:
		items, err := deleteResources(
			params,
			types.KindKubeDaemonSet,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.ExtensionsV1beta1().DaemonSets(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *extensionsv1beta1.DeploymentList:
		items, err := deleteResources(
			params,
			types.KindKubeDeployment,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.ExtensionsV1beta1().Deployments(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	case *extensionsv1beta1.ReplicaSetList:
		items, err := deleteResources(
			params,
			types.KindKubeReplicaSet,
			slices.ToPointers(o.Items),
			func(ctx context.Context, client kubernetes.Interface, name, namespace string) error {
				return trace.Wrap(client.ExtensionsV1beta1().ReplicaSets(namespace).Delete(ctx, name, deleteOptions))
			},
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		o.Items = slices.FromPointers(items)
	default:
		return internalErrStatus, trace.BadParameter("unexpected type %T", obj)
	}
	// reset the memory buffer.
	memWriter.Buffer().Reset()
	// set the content type to the response writer to match the delete
	// request content type instead of the list request content type.
	memWriter.Header().Set(
		responsewriters.ContentTypeHeader,
		deleteRequestContentType,
	)
	// encode the filtered response into the memory buffer.
	if err := deleteRequestEncoder.Encode(obj, memWriter.Buffer()); err != nil {
		return internalErrStatus, trace.Wrap(err)
	}
	// copy the output into the user's ResponseWriter and return.
	return memWriter.Status(), trace.Wrap(memWriter.CopyInto(w))
}

// newImpersonatedKubeClient creates a new Kubernetes Client that impersonates
// a username and the groups.
func newImpersonatedKubeClient(creds kubeCreds, username string, groups []string) (*kubernetes.Clientset, error) {
	c := &rest.Config{}
	// clone cluster's rest config.
	*c = *creds.getKubeRestConfig()
	// change the impersonated headers.
	c.Impersonate = rest.ImpersonationConfig{
		UserName: username,
		Groups:   groups,
	}
	// TODO(tigrato): reuse the http client.
	client, err := kubernetes.NewForConfig(c)
	return client, trace.Wrap(err)
}

// parseDeleteCollectionBody parses the request body targeted to pod collection
// endpoints.
func parseDeleteCollectionBody(r io.Reader, decoder runtime.Decoder) (metav1.DeleteOptions, error) {
	into := metav1.DeleteOptions{}
	data, err := io.ReadAll(r)
	if err != nil {
		return into, trace.Wrap(err)
	}
	if len(data) == 0 {
		return into, nil
	}
	_, _, err = decoder.Decode(data, nil, &into)
	return into, trace.Wrap(err)
}

// handleDeleteCustomResourceCollection handles the DELETE Collection request
// for custom resources. It checks if the user is allowed to execute
// delete collection requests on the desired namespace and forwards the request
// to the Kubernetes API server.
// This process is different from the other delete collection requests because
// we don't have to check if the user is allowed to delete each individual
// resource. Instead, we just have to check if the user is allowed to delete
// collections within the namespace.
func (f *Forwarder) handleDeleteCustomResourceCollection(w http.ResponseWriter, req *http.Request, sess *clusterSession) (status int, err error) {
	// Access to custom resources is controlled by KindKubeNamespace.
	// We need to check if the user is allowed to delete the custom resource within
	// the namespace by using the CheckKubeGroupsAndUsers method.
	r := types.KubernetesResource{
		Kind:  types.KindKubeNamespace,
		Name:  sess.apiResource.namespace,
		Verbs: []string{types.KubeVerbDeleteCollection},
	}

	// Check if the user is allowed to delete the custom resource within the namespace
	// and get the list of groups and users that the request impersonates.
	allowedKubeGroups, allowedKubeUsers, err := sess.Checker.CheckKubeGroupsAndUsers(
		sess.sessionTTL,
		false, /* overrideTTL */
		services.NewKubernetesClusterLabelMatcher(
			sess.kubeClusterLabels,
			sess.Checker.Traits(),
		),
		services.NewKubernetesResourceMatcher(r),
	)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	// fillDefaultKubePrincipalDetails fills the default details in order to keep
	// the correct behavior when forwarding the request to the Kubernetes API.
	kubeUsers, kubeGroups := fillDefaultKubePrincipalDetails(allowedKubeGroups, allowedKubeUsers, sess.User.GetName())
	sess.kubeUsers = utils.StringsSet(kubeUsers)
	sess.kubeGroups = utils.StringsSet(kubeGroups)
	if err := setupImpersonationHeaders(sess, req.Header); err != nil {
		return 0, trace.Wrap(err)
	}

	// forward the request to the Kubernetes API.
	rw := httplib.NewResponseStatusRecorder(w)
	sess.forwarder.ServeHTTP(rw, req)
	return rw.Status(), nil
}

type deleteResourcesCommonParams struct {
	ctx         context.Context
	log         *slog.Logger
	authCtx     *authContext
	header      http.Header
	kubeDetails *kubeDetails
}

func deleteResources[T kubeObjectInterface](
	params deleteResourcesCommonParams,
	kind string,
	items []T,
	deleteOP func(ctx context.Context, client kubernetes.Interface, name, namespace string) error,
) ([]T, error) {
	deletedItems := make([]T, 0, len(items))
	for _, item := range items {
		// Compute users and groups from available roles that match the
		// cluster labels and kubernetes resources.
		allowedKubeGroups, allowedKubeUsers, err := params.authCtx.Checker.CheckKubeGroupsAndUsers(
			params.authCtx.sessionTTL,
			false,
			services.NewKubernetesClusterLabelMatcher(
				params.authCtx.kubeClusterLabels,
				params.authCtx.Checker.Traits(),
			),
			services.NewKubernetesResourceMatcher(
				getKubeResource(kind, types.KubeVerbDeleteCollection, item),
			),
		)
		// no match was found, we ignore the request.
		if err != nil {
			continue
		}
		allowedKubeUsers, allowedKubeGroups = fillDefaultKubePrincipalDetails(allowedKubeUsers, allowedKubeGroups, params.authCtx.User.GetName())

		impersonatedUsers, impersonatedGroups, err := computeImpersonatedPrincipals(
			utils.StringsSet(allowedKubeUsers), utils.StringsSet(allowedKubeGroups),
			params.header,
		)
		if err != nil {
			continue
		}

		// create a new kubernetes.Client using the impersonated users and groups
		// that matched the current pod.
		client, err := newImpersonatedKubeClient(params.kubeDetails.kubeCreds, impersonatedUsers, impersonatedGroups)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// delete each pod individually.
		err = deleteOP(params.ctx, client, item.GetName(), item.GetNamespace())
		if err != nil {
			// TODO(tigrato): check what should we do when delete returns an error.
			// Should we check if it's permission error?
			// Check if the Pod has already been deleted by a concurrent request
			continue
		}
		deletedItems = append(deletedItems, item)
	}
	return deletedItems, nil
}
