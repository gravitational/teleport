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

package proxy

import (
	"bytes"
	"context"
	"net/http"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	"github.com/gravitational/teleport/api/types/kubewaitingcontainer"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
	"github.com/gravitational/teleport/lib/utils"
)

// ephemeralContainers handles ephemeral container creation requests.
// If a user that is required to be moderated attempts to create an
// ephemeral container, the creation of that container will be delayed
// until the requirements for the moderated session are met.
func (f *Forwarder) ephemeralContainers(authCtx *authContext, w http.ResponseWriter, req *http.Request, p httprouter.Params) (resp any, err error) {
	ctx, span := f.cfg.tracer.Start(
		req.Context(),
		"kube.Forwarder/ephemeralContainers",
		oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		oteltrace.WithAttributes(
			semconv.RPCServiceKey.String(f.cfg.KubeServiceType),
			semconv.RPCMethodKey.String("ephemeralContainers"),
			semconv.RPCSystemKey.String("kube"),
		),
	)
	req = req.WithContext(ctx)
	defer span.End()

	// If the user can start a session by themselves, proxy the ephemeral
	// container creation request. Otherwise if the user requires
	// moderation reply with fake data so kubectl will attempt to start
	// a session with this ephemeral container. Then we will wait to
	// create the ephemeral container until the requirements for the
	// moderated session are met. If we wait here kubectl will timeout,
	// so make it wait to establish a session instead.
	canStart, err := f.canStartSessionAlone(authCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if canStart {
		return f.catchAll(authCtx, w, req)
	}

	sess, err := f.newClusterSession(req.Context(), *authCtx)
	if err != nil {
		// This error goes to kubernetes client and is not visible in the logs
		// of the teleport server if not logged here.
		f.log.ErrorContext(req.Context(), "Failed to create cluster session", "error", err)
		return nil, trace.Wrap(err)
	}
	// sess.Close cancels the connection monitor context to release it sooner.
	// When the server is under heavy load it can take a while to identify that
	// the underlying connection is gone. This change prevents that and releases
	// the resources as soon as we know the session is no longer active.
	defer sess.close()

	sess.upgradeToHTTP2 = true
	sess.forwarder, err = f.makeSessionForwarder(sess)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := f.setupForwardingHeaders(sess, req, true /* withImpersonationHeaders */); err != nil {
		// This error goes to kubernetes client and is not visible in the logs
		// of the teleport server if not logged here.
		f.log.ErrorContext(req.Context(), "Failed to set up forwarding headers", "error", err)
		return nil, trace.Wrap(err)
	}
	if !sess.isLocalKubernetesCluster {
		sess.forwarder.ServeHTTP(w, req)
		return nil, nil
	}

	err = f.ephemeralContainersLocal(authCtx, sess, w, req)
	return nil, trace.Wrap(err)
}

// ephemeralContainersLocal handles ephemeral container creation requests for
// users that require moderation.
func (f *Forwarder) ephemeralContainersLocal(authCtx *authContext, sess *clusterSession, w http.ResponseWriter, req *http.Request) (err error) {
	// Fetch information on the requested pod and apply the patch
	// so kubectl will think the ephemeral container has been created.
	podPatch, err := utils.ReadAtMost(req.Body, teleport.MaxHTTPRequestSize)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := req.Body.Close(); err != nil {
		return trace.Wrap(err)
	}

	reqContentType := responsewriters.GetContentTypeHeader(req.Header)
	// Remove "; charset=" if included in header.
	if idx := strings.Index(reqContentType, ";"); idx > 0 {
		reqContentType = reqContentType[:idx]
	}

	reqPatchType := apimachinerytypes.PatchType(reqContentType)
	contentType, err := patchTypeToContentType(reqPatchType)
	if err != nil {
		return trace.Wrap(err)
	}
	encoder, decoder, err := newEncoderAndDecoderForContentType(
		contentType,
		newClientNegotiator(sess.codecFactory))
	if err != nil {
		return trace.Wrap(err, "failed to create encoder and decoder")
	}

	patchedPod, ephemeralContName, err := f.mergeEphemeralPatchWithCurrentPod(
		req.Context(),
		mergeEphemeralPatchWithCurrentPodConfig{
			kubeCluster:   sess.kubeClusterName,
			kubeNamespace: authCtx.kubeResource.Namespace,
			podName:       authCtx.kubeResource.Name,
			decoder:       decoder,
			encoder:       encoder,
			podPatch:      podPatch,
			patchType:     reqPatchType,
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := f.createWaitingContainer(req.Context(), ephemeralContName, authCtx, podPatch, reqPatchType); err != nil {
		return trace.Wrap(err)
	}

	responsewriters.SetContentTypeHeader(w, w.Header())
	w.WriteHeader(http.StatusOK)

	if err := encoder.Encode(patchedPod, w); err != nil {
		return trace.Wrap(err)
	}

	f.emitAuditEvent(req, sess, http.StatusOK)
	return trace.Wrap(err)
}

// mergeEphemeralPatchWithCurrentPodConfig is a configuration struct for
// mergeEphemeralPatchWithCurrentPod.
type mergeEphemeralPatchWithCurrentPodConfig struct {
	kubeCluster   string
	kubeNamespace string
	podName       string
	decoder       runtime.Decoder
	encoder       runtime.Encoder
	podPatch      []byte
	patchType     apimachinerytypes.PatchType
}

// mergeEphemeralPatchWithCurrentPod merges the provided patch with the
// current pod and returns the patched pod.
// This function gets the current pod from the Kubernetes API server and
// merges the provided patch with it. The patch is expected to be a strategic
// merge patch that adds an ephemeral container to the pod.
func (f *Forwarder) mergeEphemeralPatchWithCurrentPod(
	ctx context.Context,
	cfg mergeEphemeralPatchWithCurrentPodConfig,
) (*corev1.Pod, string, error) {
	details, err := f.findKubeDetailsByClusterName(cfg.kubeCluster)
	if err != nil {
		return nil, "", trace.NotFound("kubernetes cluster %q not found", cfg.kubeCluster)
	}

	pod, err := details.getKubeClient().CoreV1().
		Pods(cfg.kubeNamespace).
		Get(ctx, cfg.podName, metav1.GetOptions{})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	podSerializedBuf := &bytes.Buffer{}
	if err := cfg.encoder.Encode(pod, podSerializedBuf); err != nil {
		return nil, "", trace.Wrap(err)
	}

	patchedPod, ephemeralContName, err := patchPodWithDebugContainer(cfg.decoder, podSerializedBuf.Bytes(), cfg.podPatch, *pod, cfg.patchType)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return patchedPod, ephemeralContName, nil
}

func (f *Forwarder) createWaitingContainer(ctx context.Context, ephemeralContName string, authCtx *authContext, podPatch []byte, patchType apimachinerytypes.PatchType) error {
	waitingCont, err := kubewaitingcontainer.NewKubeWaitingContainer(
		ephemeralContName,
		&kubewaitingcontainerpb.KubernetesWaitingContainerSpec{
			Username:      authCtx.User.GetName(),
			Cluster:       authCtx.kubeClusterName,
			Namespace:     authCtx.kubeResource.Namespace,
			PodName:       authCtx.kubeResource.Name,
			ContainerName: ephemeralContName,
			Patch:         podPatch,
			PatchType:     string(patchType),
		})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = f.cfg.AuthClient.CreateKubernetesWaitingContainer(ctx, waitingCont)
	return trace.Wrap(err)
}

// impersonatedKubeClient returns a Kubernetes client that is impersonating
// the identity in the provided authCtx.
func (f *Forwarder) impersonatedKubeClient(authCtx *authContext, headers http.Header) (*kubernetes.Clientset, *kubeDetails, error) {
	details, err := f.findKubeDetailsByClusterName(authCtx.kubeClusterName)
	if err != nil {
		return nil, nil, trace.NotFound("kubernetes cluster %q not found", authCtx.kubeClusterName)
	}
	restConfig := details.getKubeRestConfig()
	kubeUser, kubeGroups, err := computeImpersonatedPrincipals(authCtx.kubeUsers, authCtx.kubeGroups, headers)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	restConfig.Impersonate = rest.ImpersonationConfig{
		UserName: kubeUser,
		Groups:   kubeGroups,
	}
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return clientSet, details, nil
}

// patchPodWithDebugContainer adds an ephemeral container to the provided spec of pod and
// returns the patched result.
func patchPodWithDebugContainer(decoder runtime.Decoder, podJson, podPatch []byte, pod corev1.Pod, patchType apimachinerytypes.PatchType) (*corev1.Pod, string, error) {
	patchResult, err := patchPod(podJson, podPatch, pod, patchType)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	gvk := corev1.SchemeGroupVersion.WithKind("Pod")
	decodedObj, _, err := decoder.Decode(patchResult, &gvk, nil)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	decodedObj.GetObjectKind().SetGroupVersionKind(gvk)

	patchedPod, ok := decodedObj.(*corev1.Pod)
	if !ok {
		return nil, "", trace.CompareFailed("expected *corev1.Pod, got %T", decodedObj)
	}

	// The last container in the list is the one we just added.
	ephemeralCont := patchedPod.Spec.EphemeralContainers[len(patchedPod.Spec.EphemeralContainers)-1]
	if !ephemeralCont.TTY {
		return nil, "", trace.AccessDenied("only interactive ephemeral containers are supported")
	}

	// Add the container to the status so kubectl will think it has started.
	patchedPod.Status.EphemeralContainerStatuses = append(
		pod.Status.EphemeralContainerStatuses,
		corev1.ContainerStatus{
			Name: ephemeralCont.Name,
			State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{
					StartedAt: metav1.Now(),
				},
			},
			Ready: true,
		},
	)

	return patchedPod, ephemeralCont.Name, nil
}

// pushPodEvent writes a fake event that shows that an ephemeral container
// started running on a given pod. This is so kubectl will attempt to start
// a session which can be safely waiting on until the moderated session
// is approved.
func (f *Forwarder) getPatchedPodEvent(ctx context.Context, sess *clusterSession, waitingCont *kubewaitingcontainerpb.KubernetesWaitingContainer) (*watch.Event, error) {
	contentType, err := patchTypeToContentType(apimachinerytypes.PatchType(waitingCont.Spec.PatchType))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	encoder, decoder, err := newEncoderAndDecoderForContentType(
		contentType,
		newClientNegotiator(sess.codecFactory),
	)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create encoder and decoder")
	}

	patchedPod, _, err := f.mergeEphemeralPatchWithCurrentPod(
		ctx,
		mergeEphemeralPatchWithCurrentPodConfig{
			kubeCluster:   waitingCont.Spec.Cluster,
			kubeNamespace: waitingCont.Spec.Namespace,
			podName:       waitingCont.Spec.PodName,
			decoder:       decoder,
			encoder:       encoder,
			podPatch:      waitingCont.Spec.Patch,
			patchType:     apimachinerytypes.PatchType(waitingCont.Spec.PatchType),
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &watch.Event{
		Type:   watch.Modified,
		Object: patchedPod,
	}, nil
}

// getUserEphemeralContainersForPod returns a list of ephemeral containers
// created by the username and are waiting to be created for a given pod.
func (f *Forwarder) getUserEphemeralContainersForPod(ctx context.Context, username, kubeCluster, namespace, pod string) ([]*kubewaitingcontainerpb.KubernetesWaitingContainer, error) {
	var (
		list      []*kubewaitingcontainerpb.KubernetesWaitingContainer
		startPage string
	)
	for {
		waitingContainers, nextPage, err := f.cfg.CachingAuthClient.ListKubernetesWaitingContainers(ctx, apidefaults.DefaultChunkSize, startPage)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, cont := range waitingContainers {
			if cont.Spec.Username != username ||
				cont.Spec.Cluster != kubeCluster ||
				cont.Spec.Namespace != namespace ||
				cont.Spec.PodName != pod {
				continue
			}

			list = append(list, cont)
		}
		if nextPage == "" {
			break
		}
		startPage = nextPage
	}
	return list, nil
}

func getEphemeralContainerStatusByName(pod *corev1.Pod, containerName string) *corev1.ContainerStatus {
	for _, status := range pod.Status.EphemeralContainerStatuses {
		if status.Name == containerName {
			return &status
		}
	}
	return nil
}

func patchTypeToContentType(reqPatchType apimachinerytypes.PatchType) (string, error) {
	var contentType string
	switch reqPatchType {
	case apimachinerytypes.JSONPatchType,
		apimachinerytypes.MergePatchType,
		apimachinerytypes.StrategicMergePatchType:
		contentType = responsewriters.JSONContentType
	case apimachinerytypes.ApplyPatchType:
		contentType = responsewriters.YAMLContentType
	default:
		return "", trace.BadParameter("unsupported content type %q", reqPatchType)
	}
	return contentType, nil
}

// patchPod applies the provided patch to the pod and returns the patched pod data.
// The patch type is used to determine how the patch should be applied.
func patchPod(podData, patchData []byte, pod corev1.Pod, pt apimachinerytypes.PatchType) ([]byte, error) {
	switch pt {
	case apimachinerytypes.JSONPatchType:
		patchObj, err := jsonpatch.DecodePatch(patchData)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		patchedObj, err := patchObj.Apply(podData)
		return patchedObj, trace.Wrap(err)
	case apimachinerytypes.MergePatchType:
		patchedObj, err := jsonpatch.MergePatch(podData, patchData)
		return patchedObj, trace.Wrap(err)
	case apimachinerytypes.StrategicMergePatchType:
		patchedObj, err := strategicpatch.StrategicMergePatch(podData, patchData, pod)
		return patchedObj, trace.Wrap(err)
	default:
		return nil, trace.BadParameter("unsupported patch type %q", pt)
	}
}
