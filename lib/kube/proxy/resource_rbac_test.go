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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	authv1 "k8s.io/api/rbac/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/streaming"
	kubetypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	restclientwatch "k8s.io/client-go/rest/watch"
	controllerclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
	tkm "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
	"github.com/gravitational/teleport/lib/utils"
)

func TestListPodRBAC(t *testing.T) {
	const (
		usernameWithFullAccess        = "full_user"
		usernameWithNamespaceAccess   = "default_user"
		usernameWithLimitedAccess     = "limited_user"
		usernameWithDenyRule          = "denied_user"
		usernameWithoutListVerbAccess = "no_list_user"
		usernameWithTraits            = "trait_user"
		testPodName                   = "test"
	)
	// kubeMock is a Kubernetes API mock for the session tests.
	// Once a new session is created, this mock will write to
	// stdout and stdin (if available) the pod name, followed
	// by copying the contents of stdin into both streams.
	kubeMock, err := tkm.NewKubeAPIMock()
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
						Verbs:     []string{types.Wildcard},
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
							Verbs:     []string{types.Wildcard},
						},
					})
			},
		},
	)

	userWithTraits, _ := testCtx.CreateUserWithTraitsAndRole(
		testCtx.Context,
		t,
		usernameWithTraits,
		map[string][]string{
			"namespaces": {metav1.NamespaceDefault},
		},
		RoleSpec{
			Name:       usernameWithTraits,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,
			SetupRoleFunc: func(r types.Role) {
				r.SetKubeResources(types.Allow,
					[]types.KubernetesResource{
						{
							Kind:      types.KindKubePod,
							Name:      types.Wildcard,
							Namespace: "{{external.namespaces}}",
							Verbs:     []string{types.Wildcard},
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
							Verbs:     []string{types.Wildcard},
						},
					},
				)
			},
		},
	)
	// create a moderator user with access to kubernetes
	// (kubernetes_user and kubernetes_groups specified)
	userWithoutListVerb, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameWithoutListVerbAccess,
		RoleSpec{
			Name:       usernameWithoutListVerbAccess,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,
			SetupRoleFunc: func(r types.Role) {
				r.SetKubeResources(types.Allow,
					[]types.KubernetesResource{
						{
							Kind:      types.KindKubePod,
							Name:      "*",
							Namespace: metav1.NamespaceDefault,
							Verbs:     []string{"get"},
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
							Verbs:     []string{types.Wildcard},
						},
					},
				)
				r.SetKubeResources(types.Deny,
					[]types.KubernetesResource{
						{
							Kind:      types.KindKubePod,
							Name:      types.Wildcard,
							Namespace: metav1.NamespaceDefault,
							Verbs:     []string{types.Wildcard},
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
		listPodErr       error
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
			name: "list default namespace pods for user with traits for default namespace",
			args: args{
				user:      userWithTraits,
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
			name: "list pods in every namespace for user with default namespace traits",
			args: args{
				user:      userWithTraits,
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
				listPodErr: &kubeerrors.StatusError{
					ErrStatus: metav1.Status{
						Status:  "Failure",
						Message: "pods is forbidden: User \"limited_user\" cannot list resource \"pods\" in API group \"\" in the namespace \"default\"",
						Code:    403,
						Reason:  metav1.StatusReasonForbidden,
					},
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
			name: "list default namespace pods for user with limited access",
			args: args{
				user:      userWithoutListVerb,
				namespace: metav1.NamespaceDefault,
			},
			want: want{
				listPodsResult: []string{},
				listPodErr: &kubeerrors.StatusError{
					ErrStatus: metav1.Status{
						Status:  "Failure",
						Message: "pods is forbidden: User \"no_list_user\" cannot list resource \"pods\" in API group \"\" in the namespace \"default\"",
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
			if tt.want.listPodErr != nil {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.want.listPodErr.Error())
			} else {
				require.NoError(t, err)
			}
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
						Verbs:     []string{types.Wildcard},
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
						Verbs:     []string{types.Wildcard},
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
						Verbs:     []string{types.Wildcard},
					},
				},
				denied: []types.KubernetesResource{
					{
						Kind:      types.KindKubePod,
						Namespace: defaultNamespace,
						Name:      "otherPod",
						Verbs:     []string{types.Wildcard},
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
						Verbs:     []string{types.Wildcard},
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
			negotiator := newClientNegotiator(&globalKubeCodecs)
			filterWrapper := newResourceFilterer(types.KindKubePod, types.KubeVerbWatch, &globalKubeCodecs, tt.args.allowed, tt.args.denied, utils.NewSlogLoggerForTests())
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
	kubeMock, err := tkm.NewKubeAPIMock()
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
						Verbs:     []string{types.Wildcard},
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
							Verbs:     []string{types.Wildcard},
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
							Verbs:     []string{types.Wildcard},
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
		wantErr     bool
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
			wantErr:     true,
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
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
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
	kubeMock, err := tkm.NewKubeAPIMock()
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
						Kind:  types.KindKubeClusterRole,
						Name:  types.Wildcard,
						Verbs: []string{types.Wildcard},
					},
				})
			},
		},
	)

	// Create a moderator user with access to kubernetes
	// (kubernetes_user and kubernetes_groups specified).
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
							Kind:  types.KindKubeClusterRole,
							Name:  "nginx-*",
							Verbs: []string{types.Wildcard},
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
		listClusterErr         error
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
				listClusterErr: &kubeerrors.StatusError{
					ErrStatus: metav1.Status{
						Status:  "Failure",
						Message: "clusterroles is forbidden: User \"limited_user\" cannot list resource \"clusterroles\" in API group \"rbac.authorization.k8s.io\" ",
						Code:    403,
						Reason:  metav1.StatusReasonForbidden,
					},
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
	}

	getClusterRolesFromList := func(items []authv1.ClusterRole) []string {
		clusterroles := make([]string, 0, len(items))
		for _, item := range items {
			clusterroles = append(clusterroles, item.Name)
		}
		return clusterroles
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Generate a kube client with user certs for auth.
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
			if tt.want.listClusterErr != nil {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.want.listClusterErr.Error())
			} else {
				require.NoError(t, err)
			}
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

func TestGenericCustomResourcesRBAC(t *testing.T) {
	const (
		usernameWithFullAccess     = "full_user"
		usernameWithLimitedAccess  = "limited_user"
		usernameWithSpecificAccess = "specific_user"
		testTeleportRoleName       = "telerole-test"
		testTeleportRoleNamespace  = "default"
	)

	kubeScheme, testCtx := newTestKubeCRDMock(t, tkm.WithTeleportRoleCRD)

	// create a user with full access to all namespaces.
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
						Kind:  types.KindKubeNamespace,
						Name:  types.Wildcard,
						Verbs: []string{types.Wildcard},
					},
				})
			},
		},
	)

	// create a user with limited access to kubernetes namespaces.
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
							Kind:  types.KindKubeNamespace,
							Name:  "dev",
							Verbs: []string{types.Wildcard},
						},
					},
				)
			},
		},
	)

	// create a user with limited access to kubernetes namespaces.
	userWithSpecificAccess, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameWithSpecificAccess,
		RoleSpec{
			Name:       usernameWithSpecificAccess,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,
			SetupRoleFunc: func(r types.Role) {
				r.SetKubeResources(types.Allow,
					[]types.KubernetesResource{
						{
							Kind:      "resources.teleport.dev/v6/teleportroles",
							Name:      types.Wildcard,
							Namespace: "dev",
							Verbs:     []string{types.Wildcard},
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
		listTeleportRolesResult []string
		wantListErr             bool
		getTestResult           error
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "list teleport roles for user with full access",
			args: args{
				user: userWithFullAccess,
			},
			want: want{
				listTeleportRolesResult: []string{
					"default/telerole-1",
					"default/telerole-1",
					"default/telerole-2",
					"default/telerole-test",
					"dev/telerole-1",
					"dev/telerole-2",
				},
			},
		},
		{
			name: "list teleport roles for user with specific crd access",
			args: args{
				user: userWithSpecificAccess,
			},
			want: want{
				listTeleportRolesResult: []string{
					"dev/telerole-1",
					"dev/telerole-2",
				},
				getTestResult: &kubeerrors.StatusError{
					ErrStatus: metav1.Status{
						Status: "Failure",
						Message: fmt.Sprintf(
							"teleportroles \"telerole-test\" is forbidden: User %q cannot get resource \"teleportroles\" in API group \"resources.teleport.dev\"",
							usernameWithSpecificAccess,
						),
						Code:   403,
						Reason: metav1.StatusReasonForbidden,
					},
				},
			},
		},
		{
			name: "list teleport roles for user with limited access",
			args: args{
				user: userWithLimitedAccess,
			},
			want: want{
				listTeleportRolesResult: []string{
					"dev/telerole-1",
					"dev/telerole-2",
				},
				getTestResult: &kubeerrors.StatusError{
					ErrStatus: metav1.Status{
						Status:  "Failure",
						Message: "teleportroles \"telerole-test\" is forbidden: User \"limited_user\" cannot get resource \"teleportroles\" in API group \"resources.teleport.dev\"",
						Code:    403,
						Reason:  metav1.StatusReasonForbidden,
					},
				},
			},
		},

		{
			name: "user with namespace access request that no longer fullfills the role requirements",
			args: args{
				user: userWithLimitedAccess,
				opts: []GenTestKubeClientTLSCertOptions{
					WithResourceAccessRequests(
						types.ResourceID{
							ClusterName:     testCtx.ClusterName,
							Kind:            types.KindKubeNamespace,
							Name:            kubeCluster,
							SubResourceName: "default",
						},
					),
				},
			},
			want: want{
				wantListErr: true,
				getTestResult: &kubeerrors.StatusError{
					ErrStatus: metav1.Status{
						Status:  "Failure",
						Message: "teleportroles \"telerole-test\" is forbidden: User \"limited_user\" cannot get resource \"teleportroles\" in API group \"resources.teleport.dev\"",
						Code:    403,
						Reason:  metav1.StatusReasonForbidden,
					},
				},
			},
		},
		{
			name: "user with namespace access request that restricts the role requirements",
			args: args{
				user: userWithFullAccess,
				opts: []GenTestKubeClientTLSCertOptions{
					WithResourceAccessRequests(
						types.ResourceID{
							ClusterName:     testCtx.ClusterName,
							Kind:            types.KindKubeNamespace,
							Name:            kubeCluster,
							SubResourceName: "dev",
						},
					),
				},
			},
			want: want{
				listTeleportRolesResult: []string{
					"dev/telerole-1",
					"dev/telerole-2",
				},
				getTestResult: &kubeerrors.StatusError{
					ErrStatus: metav1.Status{
						Status:  "Failure",
						Message: "teleportroles \"telerole-test\" is forbidden: User \"full_user\" cannot get resource \"teleportroles\" in API group \"resources.teleport.dev\"",
						Code:    403,
						Reason:  metav1.StatusReasonForbidden,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Generate a kube client with user certs for auth.
			_, rest := testCtx.GenTestKubeClientTLSCert(
				t,
				tt.args.user.GetName(),
				kubeCluster,
				tt.args.opts...,
			)

			client, err := controllerclient.New(rest, controllerclient.Options{
				Scheme: kubeScheme,
			})
			require.NoError(t, err)

			t.Run("list", func(t *testing.T) {
				t.Parallel()

				list := tkm.NewTeleportRoleCRD()

				if err := client.List(context.Background(), list); tt.want.wantListErr {
					require.Error(t, err)
					return
				} else {
					require.NoError(t, err)
				}

				require.True(t, list.IsList())

				var teleportRolesList []string
				// Iterate over the list of teleport roles and get the namespace and name
				// of each role in the format <namespace>/<name>.
				require.NoError(
					t,
					list.EachListItem(
						func(itemI runtime.Object) error {
							item := itemI.(*unstructured.Unstructured)
							teleportRolesList = append(teleportRolesList, item.GetNamespace()+"/"+item.GetName())
							return nil
						},
					))
				require.ElementsMatch(t, tt.want.listTeleportRolesResult, teleportRolesList)
			})

			t.Run("get", func(t *testing.T) {
				t.Parallel()

				get := tkm.NewTeleportRoleCRD()

				if err := client.Get(context.Background(),
					kubetypes.NamespacedName{
						Name:      testTeleportRoleName,
						Namespace: testTeleportRoleNamespace,
					},
					get,
				); tt.want.getTestResult == nil {
					require.NoError(t, err)
					require.Equal(t, testTeleportRoleName, get.GetName())
					require.Equal(t, testTeleportRoleNamespace, get.GetNamespace())
				} else {
					require.Error(t, err)
					require.ErrorContains(t, err, tt.want.getTestResult.Error())
				}
			})
		})
	}
}

func newTestKubeCRDMock(t *testing.T, opts ...tkm.Option) (*runtime.Scheme, *TestContext) {
	t.Helper()

	// kubeMock is a Kubernetes API mock for the session tests.
	// Once a new session is created, this mock will write to
	// stdout and stdin (if available) the pod name, followed
	// by copying the contents of stdin into both streams.
	kubeMock, err := tkm.NewKubeAPIMock(opts...)
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })

	// Register the custom resources with the scheme.
	kubeScheme := kubeMock.CRDScheme()
	require.NoError(t, registerDefaultKubeTypes(kubeScheme))

	// Creates a Kubernetes service with a configured cluster pointing to mock api server.
	testCtx := SetupTestContext(
		context.Background(),
		t,
		TestConfig{
			Clusters: []KubeClusterConfig{{Name: kubeCluster, APIEndpoint: kubeMock.URL}},
		},
	)
	// Close tests.
	t.Cleanup(func() { assert.NoError(t, testCtx.Close()) })

	return kubeScheme, testCtx
}

// Test cases:
// Given
//   - 1 CRD (namespaced)
//   - 3 NS: dev, staging, production
//   - Resources
//   - name: test-dev, ns: dev
//   - name: debug, ns: dev
//   - name: debug, ns: staging
//   - name: web, ns: dev
//   - name: web-dev, ns: dev
//   - name: web-dev, ns: production
//   - name:
//     name: *-dev, ns: *
//     name: *-dev, ns: dev
//     name: debug, ns: *
//     name: *, ns dev
//     name: *, ns dev, { pod name *, pod ns * }
//     name: *, ns dev, staging
func TestSpecificCustomResourcesRBAC(t *testing.T) {
	telerolev7 := tkm.NewCRD("resources.teleport.dev", "v7", "teleportroles", "TeleportRole", "TeleportRoleList", true)
	teleswagv1 := tkm.NewCRD("swag.teleport.dev", "v1", "teleswags", "TeleportSwag", "TeleportSwagList", true)
	clusterswagv0 := tkm.NewCRD("resources.teleport.dev", "v0", "clusterswags", "ClusterSwag", "ClusterSwagList", false)

	kubeScheme, testCtx := newTestKubeCRDMock(t,
		tkm.WithTeleportRoleCRD,
		tkm.WithCRD(telerolev7,
			tkm.NewObject("default", "telerole-1"),
			tkm.NewObject("default", "telerole-2"),
			tkm.NewObject("default", "telerole-test"),
			tkm.NewObject("dev", "telerole-1"),
			tkm.NewObject("dev", "telerole-2"),
		),
		tkm.WithCRD(teleswagv1,
			tkm.NewObject("default", "teleswag-1"),
		),
		tkm.WithCRD(clusterswagv0,
			tkm.NewObject("", "clusterswag-1"),
			tkm.NewObject("", "clusterswag-2"),
			tkm.NewObject("", "my-clusterswag"),
		),
	)

	newUser := func(name string, resources []types.KubernetesResource) types.User {
		u, _ := testCtx.CreateUserAndRole(
			testCtx.Context,
			t,
			name,
			RoleSpec{
				Name:          name,
				KubeUsers:     roleKubeUsers,
				KubeGroups:    roleKubeGroups,
				SetupRoleFunc: func(r types.Role) { r.SetKubeResources(types.Allow, resources) },
			},
		)
		return u
	}

	type args struct {
		user types.User
		crds []*tkm.CRD
	}
	type want struct {
		listTeleportRolesResult [][]string // One list per CRDs in args.
		wantListErr             []bool     // One per CRDs in args.
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "list crds on multiple versions",
			args: args{
				user: newUser("dev_access_two_versions", []types.KubernetesResource{
					{
						Kind:      tkm.NewTeleportRoleCRD().RoleKind(),
						Name:      types.Wildcard,
						Namespace: "dev",
						Verbs:     []string{types.Wildcard},
					},
					{
						Kind:      telerolev7.RoleKind(),
						Name:      types.Wildcard,
						Namespace: "dev",
						Verbs:     []string{types.Wildcard},
					},
				}),
				crds: []*tkm.CRD{tkm.NewTeleportRoleCRD(), telerolev7.Copy()},
			},
			want: want{
				listTeleportRolesResult: [][]string{
					{
						"dev/telerole-1",
						"dev/telerole-2",
					}, {
						"dev/telerole-1",
						"dev/telerole-2",
					},
				},
				wantListErr: []bool{false, false},
			},
		},
		{
			name: "access to multiple crds listing one without access",
			args: args{
				user: newUser("no_swag_access", []types.KubernetesResource{
					{
						Kind:      tkm.NewTeleportRoleCRD().RoleKind(),
						Name:      types.Wildcard,
						Namespace: "dev",
						Verbs:     []string{types.Wildcard},
					},
					{
						Kind:      telerolev7.RoleKind(),
						Name:      types.Wildcard,
						Namespace: "dev",
						Verbs:     []string{types.Wildcard},
					},
				}),
				crds: []*tkm.CRD{tkm.NewTeleportRoleCRD(), telerolev7.Copy(), teleswagv1.Copy()},
			},
			want: want{
				listTeleportRolesResult: [][]string{
					{
						"dev/telerole-1",
						"dev/telerole-2",
					},
					{
						"dev/telerole-1",
						"dev/telerole-2",
					},
					nil,
				},
				wantListErr: []bool{false, false, true},
			},
		},
		{
			name: "different valid kind format",
			args: args{
				user: newUser("diff_fmt_ok", []types.KubernetesResource{
					{
						Kind:      "resources.teleport.dev/v7/TeleportRole",
						Name:      types.Wildcard,
						Namespace: "dev",
						Verbs:     []string{types.Wildcard},
					},
				}),
				crds: []*tkm.CRD{telerolev7},
			},
			want: want{
				listTeleportRolesResult: [][]string{
					{
						"dev/telerole-1",
						"dev/telerole-2",
					},
				},
				wantListErr: []bool{false},
			},
		},
		{
			name: "different invalid kind format",
			args: args{
				user: newUser("diff_fmt_ko", []types.KubernetesResource{
					{
						Kind:      "TeleportRole",
						Name:      types.Wildcard,
						Namespace: types.Wildcard,
						Verbs:     []string{types.Wildcard},
					},
					{
						Kind:      "teleportrole",
						Name:      types.Wildcard,
						Namespace: types.Wildcard,
						Verbs:     []string{types.Wildcard},
					},
					{
						Kind:      "*/teleportrole",
						Name:      types.Wildcard,
						Namespace: types.Wildcard,
						Verbs:     []string{types.Wildcard},
					},
					{
						Kind:      "resources.teleport.dev/v7/teleportrole",
						Name:      types.Wildcard,
						Namespace: types.Wildcard,
						Verbs:     []string{types.Wildcard},
					},
					{
						Kind:      "resources.teleport.dev/v7/*",
						Name:      types.Wildcard,
						Namespace: types.Wildcard,
						Verbs:     []string{types.Wildcard},
					},
					{
						Kind:      "resources.teleport.dev/v7/",
						Name:      types.Wildcard,
						Namespace: types.Wildcard,
						Verbs:     []string{types.Wildcard},
					},
					{
						Kind:      "resources.teleport.dev/v7",
						Name:      types.Wildcard,
						Namespace: types.Wildcard,
						Verbs:     []string{types.Wildcard},
					},
					{
						Kind:      "resources.teleport.dev/",
						Name:      types.Wildcard,
						Namespace: types.Wildcard,
						Verbs:     []string{types.Wildcard},
					},
					{
						Kind:      "resources.teleport.dev",
						Name:      types.Wildcard,
						Namespace: types.Wildcard,
						Verbs:     []string{types.Wildcard},
					},
				}),
				crds: []*tkm.CRD{telerolev7},
			},
			want: want{
				wantListErr: []bool{true},
			},
		},
		{
			name: "cluster wide crd",
			args: args{
				user: newUser("cluster_crd_ok", []types.KubernetesResource{
					{
						Kind:      clusterswagv0.RoleKind(),
						Name:      "clusterswag-*",
						Namespace: types.Wildcard,
						Verbs:     []string{types.Wildcard},
					},
				}),
				crds: []*tkm.CRD{clusterswagv0},
			},
			want: want{
				listTeleportRolesResult: [][]string{
					{
						"clusterswag-1",
						"clusterswag-2",
					},
				},
				wantListErr: []bool{false},
			},
		},
		{
			name: "cluster wide crd no access",
			args: args{
				user: newUser("cluster_crd_ko", []types.KubernetesResource{
					{
						Kind:      telerolev7.RoleKind(),
						Name:      types.Wildcard,
						Namespace: types.Wildcard,
						Verbs:     []string{types.Wildcard},
					},
				}),
				crds: []*tkm.CRD{clusterswagv0},
			},
			want: want{
				wantListErr: []bool{true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Generate a kube client with user certs for auth.
			_, rest := testCtx.GenTestKubeClientTLSCert(t, tt.args.user.GetName(), kubeCluster)

			client, err := controllerclient.New(rest, controllerclient.Options{
				Scheme: kubeScheme,
			})
			require.NoError(t, err)

			for i, list := range tt.args.crds {
				list := list.Copy()
				if err := client.List(context.Background(), list); tt.want.wantListErr[i] {
					require.Error(t, err)
					continue
				} else {
					require.NoError(t, err)
				}
				require.True(t, list.IsList())

				// Iterate over the list of teleport roles and get the namespace and name
				// of each role in the format <namespace>/<name>.
				var retList []string
				require.NoError(
					t,
					list.EachListItem(
						func(itemI runtime.Object) error {
							item, ok := itemI.(*unstructured.Unstructured)
							if !ok {
								return fmt.Errorf("invalid item type %T", itemI)
							}
							retList = append(retList, path.Join(item.GetNamespace(), item.GetName()))
							return nil
						},
					),
				)
				require.ElementsMatch(t, tt.want.listTeleportRolesResult[i], retList)
			}
		})
	}
}
