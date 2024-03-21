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
	"bytes"
	"io"
	"net/http"

	"github.com/gravitational/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
	"github.com/gravitational/teleport/lib/utils"
)

// listResources forwards the pod list request to the target server, captures
// all output and filters accordingly to user roles resource access rules.
func (f *Forwarder) listResources(sess *clusterSession, w http.ResponseWriter, req *http.Request) (resp any, err error) {
	ctx, span := f.cfg.tracer.Start(
		req.Context(),
		"kube.Forwarder/listResources",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
			semconv.RPCMethodKey.String("listResources"),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	defer span.End()

	req = req.WithContext(ctx)

	isLocalKubeCluster := f.isLocalKubeCluster(sess.teleportCluster.isRemote, sess.kubeClusterName)
	supportsType := false
	resourceKind := ""
	if isLocalKubeCluster {
		resourceKind, supportsType = sess.rbacSupportedResources.getTeleportResourceKindFromAPIResource(sess.apiResource)
	}

	// status holds the returned response code.
	var status int
	// Check if the target Kubernetes cluster is not served by the current service.
	// If it's the case, forward the request to the target Kube Service where the
	// filtering logic will be applied.
	if !isLocalKubeCluster || !supportsType {
		rw := httplib.NewResponseStatusRecorder(w)
		sess.forwarder.ServeHTTP(rw, req)
		status = rw.Status()
	} else {
		allowedResources, deniedResources := sess.Checker.GetKubeResources(sess.kubeCluster)

		shouldBeAllowed, err := matchListRequestShouldBeAllowed(sess, resourceKind, allowedResources, deniedResources)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !shouldBeAllowed {
			notFoundMessage := f.kubeResourceDeniedAccessMsg(
				sess.User.GetName(),
				sess.requestVerb,
				sess.apiResource,
			)
			return nil, trace.AccessDenied(notFoundMessage)
		}
		// isWatch identifies if the request is long-lived watch stream based on
		// HTTP connection.
		isWatch := isKubeWatchRequest(req, sess.authContext.apiResource)
		if !isWatch {
			// List resources.
			status, err = f.listResourcesList(req, w, sess, allowedResources, deniedResources)
		} else {
			// Creates a watch stream to the upstream target and applies filtering
			// for each new frame that is received to exclude resources the user doesn't
			// have access to.
			status, err = f.listResourcesWatcher(req, w, sess, allowedResources, deniedResources)
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	f.emitAuditEvent(req, sess, status)
	return nil, nil
}

// listResourcesList forwards the request into the target cluster and accumulates the
// response into the memory. Once the request finishes, the memory buffer
// data is parsed and resources the user does not have access to are excluded from
// the response. Finally, the filtered response is serialized and sent back to
// the user with the appropriate headers.
func (f *Forwarder) listResourcesList(req *http.Request, w http.ResponseWriter, sess *clusterSession, allowedResources, deniedResources []types.KubernetesResource) (int, error) {
	// Creates a memory response writer that collects the response status, headers
	// and payload into memory.
	memBuffer := responsewriters.NewMemoryResponseWriter()
	// Forward the request to the target cluster.
	sess.forwarder.ServeHTTP(memBuffer, req)
	resourceKind, ok := sess.rbacSupportedResources.getTeleportResourceKindFromAPIResource(sess.apiResource)
	if !ok {
		return http.StatusBadRequest, trace.BadParameter("unknown resource kind %q", sess.apiResource.resourceKind)
	}
	verb := sess.requestVerb
	// filterBuffer filters the response to exclude resources the user doesn't have access to.
	// The filtered payload will be written into memBuffer again.
	if err := filterBuffer(
		newResourceFilterer(resourceKind, verb, sess.codecFactory, allowedResources, deniedResources, f.log),
		memBuffer,
	); err != nil {
		return memBuffer.Status(), trace.Wrap(err)
	}
	// Copy the filtered payload into target http.ResponseWriter.
	err := memBuffer.CopyInto(w)

	// Returns the status and any filter error.
	return memBuffer.Status(), trace.Wrap(err)
}

// matchListRequestShouldBeAllowed assess whether the user is permitted to perform its request
// based on the defined kubernetes_resource rules. The aim is to catch cases when the user
// has no access and present then a more user-friendly error message instead of returning
// an empty list.
// This function is not responsible for enforcing access rules.
func matchListRequestShouldBeAllowed(sess *clusterSession, resourceKind string, allowedResources, deniedResources []types.KubernetesResource) (bool, error) {
	resource := types.KubernetesResource{
		Kind:      resourceKind,
		Namespace: sess.apiResource.namespace,
		Verbs:     []string{sess.requestVerb},
	}
	result, err := utils.KubeResourceCouldMatchRules(resource, deniedResources, types.Deny)
	if err != nil {
		return false, trace.Wrap(err)
	} else if result {
		return false, nil
	}

	result, err = utils.KubeResourceCouldMatchRules(resource, allowedResources, types.Allow)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return result, nil
}

// listResourcesWatcher handles a long lived connection to the upstream server where
// the Kubernetes API returns frames with events.
// This handler creates a WatcherResponseWriter that spins a new goroutine once
// the API server writes the status code and headers.
// The goroutine waits for new events written into the response body and
// decodes each event. Once decoded, we validate if the Pod name matches
// any Pod specified in `kubernetes_resources` and if included, the event is
// forwarded to the user's response writer.
// If it does not match, the watcher ignores the event and continues waiting
// for the next event.
func (f *Forwarder) listResourcesWatcher(req *http.Request, w http.ResponseWriter, sess *clusterSession, allowedResources, deniedResources []types.KubernetesResource) (int, error) {
	negotiator := newClientNegotiator(sess.codecFactory)
	resourceKind, ok := sess.rbacSupportedResources.getTeleportResourceKindFromAPIResource(sess.apiResource)
	if !ok {
		return http.StatusBadRequest, trace.BadParameter("unknown resource kind %q", sess.apiResource.resourceKind)
	}
	verb := sess.requestVerb
	rw, err := responsewriters.NewWatcherResponseWriter(
		w,
		negotiator,
		newResourceFilterer(
			resourceKind,
			verb,
			sess.codecFactory,
			allowedResources,
			deniedResources,
			f.log,
		),
	)
	if err != nil {
		return http.StatusInternalServerError, trace.Wrap(err)
	}
	// Forwards the request to the target cluster.
	sess.forwarder.ServeHTTP(rw, req)
	// Once the request terminates, close the watcher and waits for resources
	// cleanup.
	err = rw.Close()
	return rw.Status(), trace.Wrap(err)
}

// decompressInplace decompresses the response into the same buffer it was
// written to.
// If the response is not compressed, it does nothing.
func decompressInplace(memoryRW *responsewriters.MemoryResponseWriter) error {
	switch memoryRW.Header().Get(contentEncodingHeader) {
	case contentEncodingGZIP:
		_, decompressor, err := getResponseCompressorDecompressor(memoryRW.Header())
		if err != nil {
			return trace.Wrap(err)
		}
		newBuf := bytes.NewBuffer(nil)
		_, err = io.Copy(newBuf, memoryRW.Buffer())
		if err != nil {
			return trace.Wrap(err)
		}
		memoryRW.Buffer().Reset()
		err = decompressor(memoryRW.Buffer(), newBuf)
		return trace.Wrap(err)
	default:
		return nil
	}
}
