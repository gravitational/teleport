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
	"context"
	"io"
	"maps"
	"net/http"
	"strings"
	"sync"
	"time"

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

	isLocalKubeCluster := sess.isLocalKubernetesCluster
	supportsType := false
	if isLocalKubeCluster {
		_, supportsType = sess.rbacSupportedResources.getTeleportResourceKindFromAPIResource(sess.metaResource.requestedResource)
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

		shouldBeAllowed, err := matchListRequestShouldBeAllowed(sess.metaResource, allowedResources, deniedResources)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !shouldBeAllowed {
			notFoundMessage := f.kubeResourceDeniedAccessMsg(
				sess.User.GetName(),
				sess.metaResource.verb,
				sess.metaResource.requestedResource,
			)
			return nil, trace.AccessDenied("%s", notFoundMessage)
		}
		// Identify if the request is long-lived watch stream based on
		// HTTP connection.
		if !isKubeWatchRequest(req, sess.authContext.metaResource.requestedResource) {
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
	ctx, span := f.cfg.tracer.Start(
		req.Context(),
		"kube.Forwarder/listResourcesList",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	defer span.End()
	req = req.WithContext(ctx)

	if _, ok := sess.rbacSupportedResources.getTeleportResourceKindFromAPIResource(sess.metaResource.requestedResource); !ok {
		return http.StatusBadRequest, trace.BadParameter("unknown resource kind %q", sess.metaResource.requestedResource.resourceKind)
	}

	// Check if filtering is needed before buffering the entire response.
	// If the user has wildcard access and no denied resources, we can skip
	// buffering and directly forward the response for better performance.
	filterWrapper := newResourceFilterer(sess.metaResource, sess.codecFactory, allowedResources, deniedResources, f.log)
	if filterWrapper == nil {
		// No filtering needed - use direct forwarding with status recording only.
		// This avoids buffering the entire response in memory and the subsequent
		// deserialization/re-serialization overhead.
		rw := httplib.NewResponseStatusRecorder(w)
		sess.forwarder.ServeHTTP(rw, req)
		return rw.Status(), nil
	}

	// When filtering is needed and the client can accept JSON, force JSON on the upstream request.
	// This enables the streaming filter path which writes items incrementally without buffering,
	// significantly reducing latency and memory usage compared to the buffered path.
	// Protobuf is a length-prefixed format that requires knowing the total size before writing,
	// making true streaming impossible without buffering the entire filtered output.
	// kubectl prefers JSON (table format), but programmatic clients (client-go) prefer protobuf,
	// those will still benefit when they accept JSON as a fallback.
	// Set TELEPORT_UNSTABLE_DISABLE_KUBE_STREAMING_JSON=yes to preserve the original Accept header.
	if !disableKubeStreamingJSON && clientAcceptsJSON(req) {
		req.Header.Set("Accept", "application/json")
	}

	// Filtering is needed. Pipe the upstream response through so we can
	// inspect headers and choose the filter path without buffering the body.
	pipeReader, pipeWriter := io.Pipe()
	hc := newHeaderCapturer(pipeWriter)
	done := make(chan struct{})
	go func() {
		defer close(done)
		sess.forwarder.ServeHTTP(hc, req)
		pipeWriter.Close()
	}()
	defer func() {
		pipeReader.Close()
		<-done
	}()

	// Wait for upstream to send headers.
	select {
	case <-hc.wroteHeader:
	case <-done:
		return http.StatusBadGateway, trace.ConnectionProblem(nil, "upstream closed without response")
	}

	status := hc.status
	contentType := responsewriters.GetContentTypeHeader(hc.headers)
	contentEncoding := hc.headers.Get("Content-Encoding")

	// For successful list responses with a streaming filter implementation,
	// filter directly to the client without buffering the entire response.
	if status == http.StatusOK {
		matcher := newMatcher(sess.metaResource, allowedResources, deniedResources, f.log)
		sf := newStreamFilter(contentType, matcher)
		if sf != nil {
			src, dst, compErr := wrapContentEncoding(pipeReader, w, contentEncoding)
			if compErr != nil {
				// Kubernetes API servers only use gzip today.
				// If a new encoding appears, add support in wrapContentEncoding.
				f.log.WarnContext(ctx, "Unexpected Content-Encoding, falling back to buffered filter", "content_encoding", contentEncoding)
			} else {
				maps.Copy(w.Header(), hc.headers)
				w.Header().Del("Content-Length")
				w.WriteHeader(status)

				// Note: if the stream filter fails mid-write, the client receives a truncated
				// response with a 200 status (already sent above). There is no way to
				// signal an error to the client at this point. This is inherent to streaming.
				filterErr := sf.filter(src, dst)
				dst.Close()
				src.Close()
				return status, trace.Wrap(filterErr)
			}
		}
	}

	// Buffered fallback for non-200, unsupported content type, or unsupported encoding.
	memBuffer := responsewriters.NewMemoryResponseWriter()
	maps.Copy(memBuffer.Header(), hc.headers)
	memBuffer.WriteHeader(status)
	if _, copyErr := io.Copy(memBuffer.Buffer(), pipeReader); copyErr != nil {
		return status, trace.Wrap(copyErr)
	}

	// filterBuffer filters the response to exclude resources the user doesn't have access to.
	// The filtered payload will be written into memBuffer again.
	_, filterSpan := f.cfg.tracer.Start(ctx, "kube.Forwarder/listResourcesList/filterBuffer",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	if err := filterBuffer(filterWrapper, memBuffer); err != nil {
		filterSpan.End()
		return memBuffer.Status(), trace.Wrap(err)
	}
	filterSpan.End()

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
func matchListRequestShouldBeAllowed(mr metaResource, allowedResources, deniedResources []types.KubernetesResource) (bool, error) {
	resource := mr.rbacResource()
	if resource == nil {
		// Cluster is offline.
		return false, nil
	}

	result, err := utils.KubeResourceCouldMatchRules(*resource, mr.isClusterWideResource(), deniedResources, types.Deny)
	if err != nil {
		return false, trace.Wrap(err)
	} else if result {
		return false, nil
	}

	result, err = utils.KubeResourceCouldMatchRules(*resource, mr.isClusterWideResource(), allowedResources, types.Allow)
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
	ctx, span := f.cfg.tracer.Start(
		req.Context(),
		"kube.Forwarder/listResourcesWatcher",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	defer span.End()
	req = req.WithContext(ctx)

	negotiator := newClientNegotiator(sess.codecFactory)
	_, ok := sess.rbacSupportedResources.getTeleportResourceKindFromAPIResource(sess.metaResource.requestedResource)
	if !ok {
		return http.StatusBadRequest, trace.BadParameter("unknown resource kind %q", sess.metaResource.requestedResource.resourceKind)
	}
	rw, err := responsewriters.NewWatcherResponseWriter(
		w,
		negotiator,
		newResourceFilterer(
			sess.metaResource,
			sess.codecFactory,
			allowedResources,
			deniedResources,
			f.log,
		),
	)
	if err != nil {
		return http.StatusInternalServerError, trace.Wrap(err)
	}

	// if this pod watch request is for a specific pod, watch for and
	// push events that show ephemeral containers were started if there
	// are any ephemeral containers waiting to be created for this pod
	// by this user
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(req.Context())
	if podName := isRequestTargetedToPod(req, sess.metaResource.requestedResource); podName != "" && ok {
		wg.Add(1)
		go func() {
			defer wg.Done()

			f.sendEphemeralContainerEvents(ctx, rw, sess, podName)
		}()
	}
	// Forwards the request to the target cluster.
	sess.forwarder.ServeHTTP(rw, req)
	// Wait for the fake event pushing goroutine to finish
	cancel()
	wg.Wait()

	// Once the request terminates, close the watcher and waits for resources
	// cleanup.
	err = rw.Close()
	return rw.Status(), trace.Wrap(err)
}

// sendEphemeralContainerEvents will poll the list of ephemeral containers
// each 5s from cache and see if they match the user and pod and namespace.
// If any match exists, it will push a fake event to the watcher stream to trick
// kubectl into creating the exec session.
func (f *Forwarder) sendEphemeralContainerEvents(ctx context.Context, rw *responsewriters.WatcherResponseWriter, sess *clusterSession, podName string) {
	const backoff = 5 * time.Second
	sentDebugContainers := map[string]struct{}{}
	ticker := time.NewTicker(backoff)
	defer ticker.Stop()
	for {
		wcs, err := f.getUserEphemeralContainersForPod(
			ctx,
			sess.User.GetName(),
			sess.kubeClusterName,
			sess.metaResource.requestedResource.namespace,
			podName,
		)
		if err != nil {
			f.log.WarnContext(ctx, "error getting user ephemeral containers", "error", err)
			return
		}

		for _, wc := range wcs {
			if _, ok := sentDebugContainers[wc.Spec.ContainerName]; ok {
				continue
			}
			evt, err := f.getPatchedPodEvent(ctx, sess, wc)
			if err != nil {
				f.log.WarnContext(ctx, "error pushing pod event", "error", err)
				continue
			}
			sentDebugContainers[wc.Spec.ContainerName] = struct{}{}
			// push the event to the client
			// this will lock until the event is pushed or the
			// request context is done.
			rw.PushVirtualEventToClient(ctx, evt)
		}

		// wait a bit before querying the cache again, or return
		// if the request has finished
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
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

// isRequestTargetedToPod checks if the request is
// possibly targeted to an ephemeral container. If it is, it returns the
// name of the pod that the container is in.
// This function is used to determine if a watch request is for a specific pod
// because although the watch request is for a specific pod, the endpoint
// is the same as the endpoint for the pod list request.
// A request targeted to an ephemeral container will follow this template:
// GET api/v1/namespaces/<namespace>/pods?fieldSelector=metadata.name%3D<pod_name>
func isRequestTargetedToPod(req *http.Request, kube apiResource) string {
	const podsResource = "pods"
	if kube.resourceKind != podsResource {
		return ""
	}
	if kube.namespace == "" {
		return ""
	}
	if kube.resourceName != "" {
		return ""
	}

	q := req.URL.Query()
	fieldSel, ok := q["fieldSelector"]
	if !ok {
		return ""
	}

	for _, val := range fieldSel {
		if podName, ok := strings.CutPrefix(val, "metadata.name="); ok {
			return podName
		}
	}

	return ""
}
