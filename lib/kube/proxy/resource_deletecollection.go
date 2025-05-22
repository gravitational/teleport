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
	"reflect"
	"strings"

	"github.com/gravitational/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
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

	// status holds the returned response code.
	var status int
	switch {
	// Check if the target Kubernetes cluster is not served by the current service.
	// If it's the case, forward the request to the target Kube Service where the
	// filtering logic will be applied.
	case !sess.isLocalKubernetesCluster:
		rw := httplib.NewResponseStatusRecorder(w)
		sess.forwarder.ServeHTTP(rw, req)
		status = rw.Status()
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
	case *unstructured.Unstructured:
		if !o.IsList() {
			return internalErrStatus, trace.BadParameter("unexpected CRD type")
		}
		list, err := o.ToList()
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		items, err := deleteResources(
			params,
			sess.metaResource.requestedResource.resourceKind,
			sess.metaResource.requestedResource.apiGroup,
			o.GetAPIVersion(),
			slices.ToPointers(list.Items),
			deleteOptions,
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		objList := make([]any, 0, len(items))
		for _, item := range items {
			objList = append(objList, item.Object)
		}

		o.Object["items"] = objList
	default:
		output, err := getItemsUsingReflection(obj)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}

		if len(output.items) == 0 {
			break
		}

		apiVersion, itemsR, objs, underlyingType := output.apiVersion, output.underlyingValue, output.items, output.underlyingType
		items, err := deleteResources(
			params,
			sess.metaResource.requestedResource.resourceKind,
			sess.metaResource.requestedResource.apiGroup,
			apiVersion,
			objs,
			deleteOptions,
		)
		if err != nil {
			return internalErrStatus, trace.Wrap(err)
		}
		setItemsUsingReflection(itemsR, underlyingType, items)
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

type getItemsUsingReflectionOutput struct {
	items           []kubeObjectInterface
	apiVersion      string
	underlyingType  reflect.Type
	underlyingValue reflect.Value
}

func getItemsUsingReflection(obj runtime.Object) (getItemsUsingReflectionOutput, error) {
	// itemsFieldName is the field name of the items in the list
	// object. This is used to get the items from the list object.
	// We use reflection to get the items field name since
	// the list object can be of any type.
	const itemsFieldName = "Items"
	objReflect := reflect.ValueOf(obj).Elem()
	itemsR := objReflect.FieldByName(itemsFieldName)
	if itemsR.Type().Kind() != reflect.Slice {
		return getItemsUsingReflectionOutput{}, trace.BadParameter("unexpected type %T, Items is not a slice", obj)
	}
	if itemsR.Len() == 0 {
		return getItemsUsingReflectionOutput{}, nil
	}

	var (
		underlyingType = itemsR.Index(0).Type()
		apiVersion, _  = obj.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
		objs           = make([]kubeObjectInterface, 0, itemsR.Len())
	)
	for i := range itemsR.Len() {
		item := itemsR.Index(i).Addr().Interface()
		if item, ok := item.(kubeObjectInterface); ok {
			objs = append(objs, item)
		} else {
			return getItemsUsingReflectionOutput{}, trace.BadParameter("unexpected type %T", itemsR.Interface())
		}
	}

	return getItemsUsingReflectionOutput{
		items:           objs,
		apiVersion:      apiVersion,
		underlyingType:  underlyingType,
		underlyingValue: itemsR,
	}, nil

}

func setItemsUsingReflection(itemsR reflect.Value, underlyingType reflect.Type, items []kubeObjectInterface) {
	// make a new slice of the same type as the original one.
	slice := reflect.MakeSlice(itemsR.Type(), len(items), len(items))
	for i := 0; i < len(items); i++ {
		item := items[i]
		// convert the item to the underlying type of the slice.
		// this is needed because items is a slice of pointers that
		// satisfy the kubeObjectInterface interface.
		// but the underlying type of the slice of elements is not
		// a pointer. We dereference the item and convert it to the
		// original slice element type.
		slice.Index(i).Set(reflect.ValueOf(item).Elem().Convert(underlyingType))
	}

	itemsR.Set(slice)
}

// newImpersonatedKubeClient creates a new Kubernetes Client that impersonates
// a username and the groups.
func newImpersonatedKubeClient(creds kubeCreds, username string, groups []string) (*dynamic.DynamicClient, error) {
	c := &rest.Config{}
	// clone cluster's rest config.
	*c = *creds.getKubeRestConfig()
	// change the impersonated headers.
	c.Impersonate = rest.ImpersonationConfig{
		UserName: username,
		Groups:   groups,
	}
	// TODO(tigrato): reuse the http client.
	client, err := dynamic.NewForConfig(c)
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

type deleteResourcesCommonParams struct {
	ctx         context.Context
	log         *slog.Logger
	authCtx     *authContext
	header      http.Header
	kubeDetails *kubeDetails
}

func deleteResources[T kubeObjectInterface](
	params deleteResourcesCommonParams,
	kind, group, apiVersion string,
	items []T,
	deleteOptions metav1.DeleteOptions,
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
				getKubeResource(kind, group, types.KubeVerbDeleteCollection, item),
				params.authCtx.metaResource.isClusterWideResource(),
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

		gvk := item.GroupVersionKind()
		if gvk.Group == "" || gvk.Version == "" {
			tmp := strings.Split(apiVersion, "/")
			if len(tmp) == 2 {
				gvk.Group = tmp[0]
				gvk.Version = tmp[1]
			} else {
				gvk.Version = apiVersion
			}
		}

		// delete each resource individually.
		err = client.Resource(schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: kind,
		}).Namespace(item.GetNamespace()).Delete(params.ctx, item.GetName(), deleteOptions)
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
