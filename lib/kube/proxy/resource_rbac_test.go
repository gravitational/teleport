/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package proxy

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	authv1 "k8s.io/api/rbac/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/streaming"
	kubetypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	restclientwatch "k8s.io/client-go/rest/watch"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
)

func TestListPodRBAC(t *testing.T) {
	const (
		usernameWithFullAccess      = "full_user"
		usernameWithNamespaceAccess = "default_user"
		usernameWithLimitedAccess   = "limited_user"
		usernameWithDenyRule        = "denied_user"
		testPodName                 = "test"
	)
	// kubeMock is a Kubernetes API mock for the session tests.
	// Once a new session is created, this mock will write to
	// stdout and stdin (if available) the pod name, followed
	// by copying the contents of stdin into both streams.
	kubeMock, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })

	// creates a Kubernetes service with a configured cluster pointing to mock api server
	testCtx := SetupTestContext(
		context.Background(),
		t,
		TestConfig{
			Clusters: []KubeClusterConfig{{Name: kubeCluster, APIEndpoint: kubeMock.URL}},
		},
	)
	// close tests
	t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

	// create a user with full access to kubernetes Pods.
	// (kubernetes_user and kubernetes_groups specified)
	userWithFullAccess, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameWithFullAccess,
		RoleSpec{
			Name:       usernameWithFullAccess,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,

			SetupRoleFunc: func(r types.Role) {
				r.SetKubeResources(types.Allow, []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Name:      types.Wildcard,
						Namespace: types.Wildcard,
					},
				})
			},
		},
	)
	// create a user with full access to kubernetes Pods.
	// (kubernetes_user and kubernetes_groups specified)
	userWithNamespaceAccess, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameWithNamespaceAccess,
		RoleSpec{
			Name:       usernameWithNamespaceAccess,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,
			SetupRoleFunc: func(r types.Role) {
				r.SetKubeResources(types.Allow,
					[]types.KubernetesResource{
						{
							Kind:      types.KindKubePod,
							Name:      types.Wildcard,
							Namespace: metav1.NamespaceDefault,
						},
					})
			},
		},
	)

	// create a moderator user with access to kubernetes
	// (kubernetes_user and kubernetes_groups specified)
	userWithLimitedAccess, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameWithLimitedAccess,
		RoleSpec{
			Name:       usernameWithLimitedAccess,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,
			SetupRoleFunc: func(r types.Role) {
				r.SetKubeResources(types.Allow,
					[]types.KubernetesResource{
						{
							Kind:      types.KindKubePod,
							Name:      "nginx-*",
							Namespace: metav1.NamespaceDefault,
						},
					},
				)
			},
		},
	)

	// create a user allowed to access all namespaces except the default namespace.
	// (kubernetes_user and kubernetes_groups specified)
	userWithDenyRule, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameWithDenyRule,
		RoleSpec{
			Name:       usernameWithDenyRule,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,
			SetupRoleFunc: func(r types.Role) {
				r.SetKubeResources(types.Allow,
					[]types.KubernetesResource{
						{
							Kind:      types.KindKubePod,
							Name:      types.Wildcard,
							Namespace: types.Wildcard,
						},
					},
				)
				r.SetKubeResources(types.Deny,
					[]types.KubernetesResource{
						{
							Kind:      types.KindKubePod,
							Name:      types.Wildcard,
							Namespace: metav1.NamespaceDefault,
						},
					},
				)
			},
		},
	)

	type args struct {
		user      types.User
		namespace string
		opts      []GenTestKubeClientTLSCertOptions
	}
	type want struct {
		listPodsResult   []string
		getTestPodResult error
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "list default namespace pods for user with full access",
			args: args{
				user:      userWithFullAccess,
				namespace: metav1.NamespaceDefault,
			},
			want: want{
				listPodsResult: []string{
					"default/nginx-1",
					"default/nginx-2",
					"default/test",
				},
			},
		},
		{
			name: "list pods in every namespace for user with full access",
			args: args{
				user:      userWithFullAccess,
				namespace: metav1.NamespaceAll,
			},
			want: want{
				listPodsResult: []string{
					"default/nginx-1",
					"default/nginx-2",
					"default/test",
					"dev/nginx-1",
					"dev/nginx-2",
				},
			},
		},
		{
			name: "list default namespace pods for user with default namespace",
			args: args{
				user:      userWithNamespaceAccess,
				namespace: metav1.NamespaceDefault,
			},
			want: want{
				listPodsResult: []string{
					"default/nginx-1",
					"default/nginx-2",
					"default/test",
				},
			},
		},
		{
			name: "list pods in every namespace for user with default namespace",
			args: args{
				user:      userWithNamespaceAccess,
				namespace: metav1.NamespaceAll,
			},
			want: want{
				listPodsResult: []string{
					"default/nginx-1",
					"default/nginx-2",
					"default/test",
				},
			},
		},
		{
			name: "list default namespace pods for user with limited access",
			args: args{
				user:      userWithLimitedAccess,
				namespace: metav1.NamespaceDefault,
			},
			want: want{
				listPodsResult: []string{
					"default/nginx-1",
					"default/nginx-2",
				},
				getTestPodResult: &kubeerrors.StatusError{
					ErrStatus: metav1.Status{
						Status:  "Failure",
						Message: "pods \"test\" is forbidden: User \"limited_user\" cannot get resource \"pods\" in API group \"\" in the namespace \"default\"",
						Code:    403,
						Reason:  metav1.StatusReasonForbidden,
					},
				},
			},
		},
		{
			name: "list pods in every namespace for user with default namespace deny rule",
			args: args{
				user: userWithDenyRule,
			},
			want: want{
				listPodsResult: []string{
					"dev/nginx-1",
					"dev/nginx-2",
				},
				getTestPodResult: &kubeerrors.StatusError{
					ErrStatus: metav1.Status{
						Status:  "Failure",
						Message: "pods \"test\" is forbidden: User \"denied_user\" cannot get resource \"pods\" in API group \"\" in the namespace \"default\"",
						Code:    403,
						Reason:  metav1.StatusReasonForbidden,
					},
				},
			},
		},
		{
			name: "list default namespace pods for user with limited access and a resource access request",
			args: args{
				user:      userWithNamespaceAccess,
				namespace: metav1.NamespaceDefault,
				opts: []GenTestKubeClientTLSCertOptions{
					WithResourceAccessRequests(
						types.ResourceID{
							ClusterName:     testCtx.ClusterName,
							Kind:            types.KindKubePod,
							Name:            kubeCluster,
							SubResourceName: "default/nginx-1",
						},
					),
				},
			},
			want: want{
				listPodsResult: []string{
					// Users roles allow access to all pods in default namespace
					// but the access request only allows access to default/nginx-1.
					"default/nginx-1",
				},
				getTestPodResult: &kubeerrors.StatusError{
					ErrStatus: metav1.Status{
						Status:  "Failure",
						Message: "pods \"test\" is forbidden: User \"default_user\" cannot get resource \"pods\" in API group \"\" in the namespace \"default\"",
						Code:    403,
						Reason:  metav1.StatusReasonForbidden,
					},
				},
			},
		},
		{
			name: "user with pod access request that no longer fullfills the role requirements",
			args: args{
				user:      userWithLimitedAccess,
				namespace: metav1.NamespaceDefault,
				opts: []GenTestKubeClientTLSCertOptions{
					WithResourceAccessRequests(
						types.ResourceID{
							ClusterName:     testCtx.ClusterName,
							Kind:            types.KindKubePod,
							Name:            kubeCluster,
							SubResourceName: fmt.Sprintf("%s/%s", metav1.NamespaceDefault, testPodName),
						},
					),
				},
			},
			want: want{
				listPodsResult: []string{},
				getTestPodResult: &kubeerrors.StatusError{
					ErrStatus: metav1.Status{
						Status:  "Failure",
						Message: "pods \"test\" is forbidden: User \"limited_user\" cannot get resource \"pods\" in API group \"\" in the namespace \"default\"",
						Code:    403,
						Reason:  metav1.StatusReasonForbidden,
					},
				},
			},
		},
	}
	getPodsFromPodList := func(items []corev1.Pod) []string {
		pods := make([]string, 0, len(items))
		for _, item := range items {
			pods = append(pods, path.Join(item.Namespace, item.Name))
		}
		return pods
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// generate a kube client with user certs for auth
			client, _ := testCtx.GenTestKubeClientTLSCert(
				t,
				tt.args.user.GetName(),
				kubeCluster,
				tt.args.opts...,
			)

			rsp, err := client.CoreV1().Pods(tt.args.namespace).List(
				testCtx.Context,
				metav1.ListOptions{},
			)
			require.NoError(t, err)

			require.Equal(t, tt.want.listPodsResult, getPodsFromPodList(rsp.Items))

			_, err = client.CoreV1().Pods(metav1.NamespaceDefault).Get(
				testCtx.Context,
				testPodName,
				metav1.GetOptions{},
			)

			if tt.want.getTestPodResult == nil {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.want.getTestPodResult.Error())
			}
		})
	}
}

func TestWatcherResponseWriter(t *testing.T) {
	defaultNamespace := "default"
	devNamespace := "dev"
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	t.Parallel()
	statusErr := &metav1.Status{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Status",
			APIVersion: "v1",
		},
		Status:  metav1.StatusFailure,
		Message: "error",
		Code:    http.StatusForbidden,
	}
	fakeEvents := []*metav1.WatchEvent{
		{
			Type:   string(watch.Added),
			Object: newRawExtension("podAdded", devNamespace),
		},
		{
			Type:   string(watch.Modified),
			Object: newRawExtension("podAdded", defaultNamespace),
		},
		{
			Type:   string(watch.Modified),
			Object: newRawExtension("otherPod", defaultNamespace),
		},
	}

	type args struct {
		allowed []types.KubernetesResource
		denied  []types.KubernetesResource
	}
	tests := []struct {
		name       string
		args       args
		wantEvents []*metav1.WatchEvent
		wantStatus *metav1.Status
	}{
		{
			name: "receive every event",
			args: args{
				allowed: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: "*",
						Name:      "*",
					},
				},
			},
			wantEvents: fakeEvents,
		},
		{
			name: "receive events for default namespace",
			args: args{
				allowed: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: defaultNamespace,
						Name:      "*",
					},
				},
			},
			wantEvents: fakeEvents[1:],
		},
		{
			name: "receive events for default namespace but with denied pod",
			args: args{
				allowed: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: defaultNamespace,
						Name:      "*",
					},
				},
				denied: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: defaultNamespace,
						Name:      "otherPod",
					},
				},
			},
			wantEvents: fakeEvents[1:2],
		},
		{
			name: "receive receives no events for default namespace",
			args: args{
				allowed: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: defaultNamespace,
						Name:      "rand*",
					},
				},
			},
			wantStatus: statusErr,
			wantEvents: []*metav1.WatchEvent{
				{
					Type: string(watch.Error),
					Object: runtime.RawExtension{
						Object: statusErr,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			userReader, userWriter := io.Pipe()
			negotiator := newClientNegotiator()
			filterWrapper := newResourceFilterer(types.KindKubePod, tt.args.allowed, tt.args.denied, log)
			// watcher parses the data written into itself and if the user is allowed to
			// receive the update, it writes the event into target.
			watcher, err := responsewriters.NewWatcherResponseWriter(newFakeResponseWriter(userWriter) /*target*/, negotiator, filterWrapper)
			require.NoError(t, err)

			// create the encoder that writes frames into watcher ResponseWriter and
			// a decoder that parses the events written into userWriter pipe.
			watchEncoder, decoder := newWatchSerializers(
				t,
				responsewriters.DefaultContentType,
				negotiator,
				watcher,
				userReader,
			)
			// Set the content type header to use `json`.
			watcher.Header().Set(
				responsewriters.ContentTypeHeader, responsewriters.DefaultContentType,
			)
			// Write the status to spin the goroutine that filters the requests.
			watcher.WriteHeader(http.StatusOK)

			var collectedEvents []*metav1.WatchEvent
			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					// collects filtered events.
					event, err := decoder.decodeStreamingMessage()
					if err != nil {
						break
					}
					collectedEvents = append(collectedEvents, event)
				}
			}()

			for _, event := range fakeEvents {
				// writes frames into watcher ResponseWrite.
				err := watchEncoder.Encode(&watch.Event{
					Type:   watch.EventType(event.Type),
					Object: event.Object.Object,
				})
				require.NoError(t, err)
			}
			// Write the metav1.Status to make sure it's always forwarded.
			if tt.wantStatus != nil {
				// writes frames into watcher ResponseWrite.
				err := watchEncoder.Encode(&watch.Event{
					Type:   watch.Error,
					Object: tt.wantStatus,
				})
				require.NoError(t, err)
			}

			watcher.Close()
			userReader.CloseWithError(io.EOF)
			userWriter.CloseWithError(io.EOF)
			// Waits until collector finishes.
			wg.Wait()
			// verify events.
			require.Empty(t,
				cmp.Diff(tt.wantEvents, collectedEvents,
					cmp.FilterPath(func(path cmp.Path) bool {
						if field, ok := path.Last().(cmp.StructField); ok {
							// Ignore Raw fields that contain the Object encoded.
							return strings.EqualFold(field.Name(), "Raw")
						}
						return false
					}, cmp.Ignore()),
				),
			)
		})

	}
}

func newRawExtension(name, namespace string) runtime.RawExtension {
	return runtime.RawExtension{
		Object: newFakePod(name, namespace),
	}
}

func newFakePod(name, namespace string) *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func newWatchSerializers(
	t *testing.T,
	contentType string,
	negotiator runtime.ClientNegotiator,
	writer io.Writer, reader io.ReadCloser,
) (*restclientwatch.Encoder, *streamDecoder) {
	// parse mime type.
	mediaType, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	// create a stream decoder based on mediaType.s
	objectDecoder, streamingSerializer, framer, err := negotiator.StreamDecoder(mediaType, params)
	require.NoError(t, err)
	// create a encoder to encode filtered requests to the user.
	encoder, err := negotiator.Encoder(mediaType, params)
	require.NoError(t, err)
	// create a frameReader that waits until the Kubernetes API sends the full
	// event frame.
	frameReader := framer.NewFrameReader(reader)
	t.Cleanup(func() {
		frameReader.Close()
	})
	// create a frameWriter that writes event frames into the user's connection.
	frameWriter := framer.NewFrameWriter(writer)
	// streamingDecoder is the decoder that parses metav1.WatchEvents from the
	// long-lived connection.
	streamingDecoder := streaming.NewDecoder(frameReader, streamingSerializer)
	t.Cleanup(func() {
		streamingDecoder.Close()
	})
	// create encoders
	watchEventEncoder := streaming.NewEncoder(frameWriter, streamingSerializer)
	watchEncoder := restclientwatch.NewEncoder(watchEventEncoder, encoder)

	return watchEncoder,
		&streamDecoder{streamDecoder: streamingDecoder, embeddedEncoder: objectDecoder}
}

type streamDecoder struct {
	streamDecoder   streaming.Decoder
	embeddedEncoder runtime.Decoder
}

func (s *streamDecoder) decodeStreamingMessage() (*metav1.WatchEvent, error) {
	var event metav1.WatchEvent
	res, gvk, err := s.streamDecoder.Decode(nil, &event)
	if err != nil {
		return nil, err
	}
	if gvk != nil {
		res.GetObjectKind().SetGroupVersionKind(*gvk)
	}
	switch res.(type) {
	case *metav1.Status:
		return nil, trace.BadParameter("expected metav1.WatchEvent; got *metav1.Status")
	default:
		switch watch.EventType(event.Type) {
		case watch.Added, watch.Modified, watch.Deleted, watch.Error, watch.Bookmark:
		default:
			return nil, trace.BadParameter("got invalid watch event type: %v", event.Type)
		}
		obj, gvk, err := s.embeddedEncoder.Decode(event.Object.Raw, nil /* defaults */, nil /* into */)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if gvk != nil {
			obj.GetObjectKind().SetGroupVersionKind(*gvk)
		}
		event.Object.Object = obj
		return &event, nil
	}
}

func newFakeResponseWriter(writer *io.PipeWriter) *fakeResponseWriter {
	return &fakeResponseWriter{
		writer: writer,
		header: http.Header{},
	}
}

type fakeResponseWriter struct {
	writer *io.PipeWriter
	header http.Header
	status int
}

func (f *fakeResponseWriter) Header() http.Header {
	return f.header
}

func (f *fakeResponseWriter) WriteHeader(status int) {
	f.status = status
}

func (f *fakeResponseWriter) Write(b []byte) (int, error) {
	return f.writer.Write(b)
}

func TestDeletePodCollectionRBAC(t *testing.T) {
	const (
		usernameWithFullAccess      = "full_user"
		usernameWithNamespaceAccess = "default_user"
		usernameWithLimitedAccess   = "limited_user"
	)
	// kubeMock is a Kubernetes API mock for the session tests.
	// Once a new session is created, this mock will write to
	// stdout and stdin (if available) the pod name, followed
	// by copying the contents of stdin into both streams.
	kubeMock, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })

	// creates a Kubernetes service with a configured cluster pointing to mock api server
	testCtx := SetupTestContext(
		context.Background(),
		t,
		TestConfig{
			Clusters: []KubeClusterConfig{{Name: kubeCluster, APIEndpoint: kubeMock.URL}},
		},
	)
	// close tests
	t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

	// create a user with full access to kubernetes Pods.
	// (kubernetes_user and kubernetes_groups specified)
	userWithFullAccess, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameWithFullAccess,
		RoleSpec{
			Name:       usernameWithFullAccess,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,

			SetupRoleFunc: func(r types.Role) {
				r.SetKubeResources(types.Allow, []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Name:      types.Wildcard,
						Namespace: types.Wildcard,
					},
				})
			},
		},
	)
	// create a user with full access to kubernetes Pods.
	// (kubernetes_user and kubernetes_groups specified)
	userWithNamespaceAccess, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameWithNamespaceAccess,
		RoleSpec{
			Name:       usernameWithNamespaceAccess,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,
			SetupRoleFunc: func(r types.Role) {
				r.SetKubeResources(types.Allow,
					[]types.KubernetesResource{
						{
							Kind:      types.KindKubePod,
							Name:      types.Wildcard,
							Namespace: metav1.NamespaceDefault,
						},
					})
			},
		},
	)

	// create a moderator user with access to kubernetes
	// (kubernetes_user and kubernetes_groups specified)
	userWithLimitedAccess, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameWithLimitedAccess,
		RoleSpec{
			Name:       usernameWithLimitedAccess,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,
			SetupRoleFunc: func(r types.Role) {
				r.SetKubeResources(types.Allow,
					[]types.KubernetesResource{
						{
							Kind:      types.KindKubePod,
							Name:      "nginx-*",
							Namespace: metav1.NamespaceDefault,
						},
					},
				)
			},
		},
	)

	type args struct {
		user      types.User
		namespace string
	}
	tests := []struct {
		name        string
		args        args
		deletedPods []string
	}{
		{
			name: "delete pods in default namespace for user with full access",
			args: args{
				user:      userWithFullAccess,
				namespace: metav1.NamespaceDefault,
			},

			deletedPods: []string{
				"default/nginx-1",
				"default/nginx-2",
				"default/test",
			},
		},
		{
			name: "delete pods for user limited to default namespace",
			args: args{
				user:      userWithNamespaceAccess,
				namespace: metav1.NamespaceDefault,
			},
			deletedPods: []string{
				"default/nginx-1",
				"default/nginx-2",
				"default/test",
			},
		},
		{
			name: "delete pods in dev namespace for user limited to default",
			args: args{
				user:      userWithNamespaceAccess,
				namespace: "dev",
			},
			deletedPods: []string{},
		},
		{
			name: "delete pods in default namespace for user with limited access",
			args: args{
				user:      userWithLimitedAccess,
				namespace: metav1.NamespaceDefault,
			},

			deletedPods: []string{
				"default/nginx-1",
				"default/nginx-2",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			requestID := kubetypes.UID(uuid.NewString())
			// generate a kube client with user certs for auth
			client, _ := testCtx.GenTestKubeClientTLSCert(
				t,
				tt.args.user.GetName(),
				kubeCluster,
			)
			err := client.CoreV1().Pods(tt.args.namespace).DeleteCollection(
				testCtx.Context,
				metav1.DeleteOptions{
					// We send the requestID as precondition to identify the request where it came
					// from. kubemock receives this metav1.DeleteOptions and
					// accumulates the deleted pods per Preconditions.UID.
					Preconditions: &metav1.Preconditions{
						UID: &requestID,
					},
				},
				metav1.ListOptions{},
			)
			require.NoError(t, err)

			require.Equal(t, tt.deletedPods, kubeMock.DeletedPods(string(requestID)))
		})
	}
	require.Empty(t, kubeMock.DeletedPods(""), "a request as received without metav1.DeleteOptions.Preconditions.UID")
}

func TestListClusterRoleRBAC(t *testing.T) {
	const (
		usernameWithFullAccess    = "full_user"
		usernameWithLimitedAccess = "limited_user"
		testPodName               = "test"
	)
	// kubeMock is a Kubernetes API mock for the session tests.
	// Once a new session is created, this mock will write to
	// stdout and stdin (if available) the pod name, followed
	// by copying the contents of stdin into both streams.
	kubeMock, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })

	// creates a Kubernetes service with a configured cluster pointing to mock api server
	testCtx := SetupTestContext(
		context.Background(),
		t,
		TestConfig{
			Clusters: []KubeClusterConfig{{Name: kubeCluster, APIEndpoint: kubeMock.URL}},
		},
	)
	// close tests
	t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

	// create a user with full access to kubernetes Pods.
	// (kubernetes_user and kubernetes_groups specified)
	userWithFullAccess, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameWithFullAccess,
		RoleSpec{
			Name:       usernameWithFullAccess,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,

			SetupRoleFunc: func(r types.Role) {
				r.SetKubeResources(types.Allow, []types.KubernetesResource{
					{
						Kind: types.KindKubeClusterRole,
						Name: types.Wildcard,
					},
				})
			},
		},
	)

	// create a moderator user with access to kubernetes
	// (kubernetes_user and kubernetes_groups specified)
	userWithLimitedAccess, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameWithLimitedAccess,
		RoleSpec{
			Name:       usernameWithLimitedAccess,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,
			SetupRoleFunc: func(r types.Role) {
				r.SetKubeResources(types.Allow,
					[]types.KubernetesResource{
						{
							Kind: types.KindKubeClusterRole,
							Name: "nginx-*",
						},
					},
				)
			},
		},
	)

	type args struct {
		user types.User
		opts []GenTestKubeClientTLSCertOptions
	}
	type want struct {
		listClusterRolesResult []string
		getTestResult          error
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "list cluster roles for user with full access",
			args: args{
				user: userWithFullAccess,
			},
			want: want{
				listClusterRolesResult: []string{
					"nginx-1",
					"nginx-2",
					"test",
				},
			},
		},
		{
			name: "list cluster roles for user with limited access",
			args: args{
				user: userWithLimitedAccess,
			},
			want: want{
				listClusterRolesResult: []string{
					"nginx-1",
					"nginx-2",
				},
				getTestResult: &kubeerrors.StatusError{
					ErrStatus: metav1.Status{
						Status:  "Failure",
						Message: "clusterroles \"test\" is forbidden: User \"limited_user\" cannot get resource \"clusterroles\" in API group \"rbac.authorization.k8s.io\"",
						Code:    403,
						Reason:  metav1.StatusReasonForbidden,
					},
				},
			},
		},

		{
			name: "user with cluster role access request that no longer fullfills the role requirements",
			args: args{
				user: userWithLimitedAccess,
				opts: []GenTestKubeClientTLSCertOptions{
					WithResourceAccessRequests(
						types.ResourceID{
							ClusterName:     testCtx.ClusterName,
							Kind:            types.KindKubePod,
							Name:            kubeCluster,
							SubResourceName: fmt.Sprintf("%s/%s", metav1.NamespaceDefault, testPodName),
						},
					),
				},
			},
			want: want{
				listClusterRolesResult: []string{},
				getTestResult: &kubeerrors.StatusError{
					ErrStatus: metav1.Status{
						Status:  "Failure",
						Message: "clusterroles \"test\" is forbidden: User \"limited_user\" cannot get resource \"clusterroles\" in API group \"rbac.authorization.k8s.io\"",
						Code:    403,
						Reason:  metav1.StatusReasonForbidden,
					},
				},
			},
		},
	}

	getClusterRolesFromList := func(items []authv1.ClusterRole) []string {
		clusterroles := make([]string, 0, len(items))
		for _, item := range items {
			clusterroles = append(clusterroles, item.Name)
		}
		return clusterroles
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// generate a kube client with user certs for auth
			client, _ := testCtx.GenTestKubeClientTLSCert(
				t,
				tt.args.user.GetName(),
				kubeCluster,
				tt.args.opts...,
			)

			rsp, err := client.RbacV1().ClusterRoles().List(
				testCtx.Context,
				metav1.ListOptions{},
			)
			require.NoError(t, err)

			require.Equal(t, tt.want.listClusterRolesResult, getClusterRolesFromList(rsp.Items))

			_, err = client.RbacV1().ClusterRoles().Get(
				testCtx.Context,
				testPodName,
				metav1.GetOptions{},
			)

			if tt.want.getTestResult == nil {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.want.getTestResult.Error())
			}
		})
	}
}
