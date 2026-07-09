// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package linuxdesktop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
)

type mockClient struct {
	linuxdesktopv1.LinuxDesktopServiceClient

	listRequest   *linuxdesktopv1.ListLinuxDesktopsRequest
	createRequest *linuxdesktopv1.CreateLinuxDesktopRequest
	getRequest    *linuxdesktopv1.GetLinuxDesktopRequest
	updateRequest *linuxdesktopv1.UpdateLinuxDesktopRequest
	upsertRequest *linuxdesktopv1.UpsertLinuxDesktopRequest
	deleteRequest *linuxdesktopv1.DeleteLinuxDesktopRequest

	listResponse []*linuxdesktopv1.LinuxDesktop
}

func (m *mockClient) ListLinuxDesktops(_ context.Context, in *linuxdesktopv1.ListLinuxDesktopsRequest, _ ...grpc.CallOption) (*linuxdesktopv1.ListLinuxDesktopsResponse, error) {
	m.listRequest = in
	rsp := &linuxdesktopv1.ListLinuxDesktopsResponse{}
	rsp.SetLinuxDesktops(m.listResponse)
	rsp.SetNextPageToken("next")
	return rsp, nil
}

func (m *mockClient) CreateLinuxDesktop(_ context.Context, in *linuxdesktopv1.CreateLinuxDesktopRequest, _ ...grpc.CallOption) (*linuxdesktopv1.LinuxDesktop, error) {
	m.createRequest = in
	return in.GetLinuxDesktop(), nil
}

func (m *mockClient) GetLinuxDesktop(_ context.Context, in *linuxdesktopv1.GetLinuxDesktopRequest, _ ...grpc.CallOption) (*linuxdesktopv1.LinuxDesktop, error) {
	m.getRequest = in
	return newLinuxDesktop(in.GetName()), nil
}

func (m *mockClient) UpdateLinuxDesktop(_ context.Context, in *linuxdesktopv1.UpdateLinuxDesktopRequest, _ ...grpc.CallOption) (*linuxdesktopv1.LinuxDesktop, error) {
	m.updateRequest = in
	return in.GetLinuxDesktop(), nil
}

func (m *mockClient) UpsertLinuxDesktop(_ context.Context, in *linuxdesktopv1.UpsertLinuxDesktopRequest, _ ...grpc.CallOption) (*linuxdesktopv1.LinuxDesktop, error) {
	m.upsertRequest = in
	return in.GetLinuxDesktop(), nil
}

func (m *mockClient) DeleteLinuxDesktop(_ context.Context, in *linuxdesktopv1.DeleteLinuxDesktopRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	m.deleteRequest = in
	return &emptypb.Empty{}, nil
}

func TestClientListLinuxDesktops(t *testing.T) {
	t.Parallel()

	mockClient := &mockClient{
		listResponse: []*linuxdesktopv1.LinuxDesktop{
			newLinuxDesktop("desktop-1"),
			newLinuxDesktop("desktop-2"),
		},
	}
	client := NewClient(mockClient)

	desktops, next, err := client.ListLinuxDesktops(t.Context(), 10, "token")
	require.NoError(t, err)
	require.Equal(t, mockClient.listResponse, desktops)
	require.Equal(t, "next", next)
	require.Equal(t, int32(10), mockClient.listRequest.GetPageSize())
	require.Equal(t, "token", mockClient.listRequest.GetPageToken())
}

func TestClientCreateLinuxDesktop(t *testing.T) {
	t.Parallel()

	mockClient := &mockClient{}
	client := NewClient(mockClient)
	desktop := newLinuxDesktop("desktop-1")

	resp, err := client.CreateLinuxDesktop(t.Context(), desktop)
	require.NoError(t, err)
	require.Equal(t, desktop, resp)
	require.Equal(t, desktop, mockClient.createRequest.GetLinuxDesktop())
}

func TestClientGetLinuxDesktop(t *testing.T) {
	t.Parallel()

	mockClient := &mockClient{}
	client := NewClient(mockClient)

	resp, err := client.GetLinuxDesktop(t.Context(), "desktop-1")
	require.NoError(t, err)
	require.Equal(t, "desktop-1", mockClient.getRequest.GetName())
	require.Equal(t, newLinuxDesktop("desktop-1"), resp)
}

func TestClientUpdateLinuxDesktop(t *testing.T) {
	t.Parallel()

	mockClient := &mockClient{}
	client := NewClient(mockClient)
	desktop := newLinuxDesktop("desktop-1")

	resp, err := client.UpdateLinuxDesktop(t.Context(), desktop)
	require.NoError(t, err)
	require.Equal(t, desktop, resp)
	require.Equal(t, desktop, mockClient.updateRequest.GetLinuxDesktop())
}

func TestClientUpsertLinuxDesktop(t *testing.T) {
	t.Parallel()

	mockClient := &mockClient{}
	client := NewClient(mockClient)
	desktop := newLinuxDesktop("desktop-1")

	resp, err := client.UpsertLinuxDesktop(t.Context(), desktop)
	require.NoError(t, err)
	require.Equal(t, desktop, resp)
	require.Equal(t, desktop, mockClient.upsertRequest.GetLinuxDesktop())
}

func TestClientDeleteLinuxDesktop(t *testing.T) {
	t.Parallel()

	mockClient := &mockClient{}
	client := NewClient(mockClient)

	err := client.DeleteLinuxDesktop(t.Context(), "desktop-1")
	require.NoError(t, err)
	require.Equal(t, "desktop-1", mockClient.deleteRequest.GetName())
}

func newLinuxDesktop(name string) *linuxdesktopv1.LinuxDesktop {
	spec := &linuxdesktopv1.LinuxDesktopSpec{}
	spec.SetAddr("127.0.0.1:22")
	spec.SetHostname("host")
	l := &linuxdesktopv1.LinuxDesktop{}
	l.SetKind(types.KindLinuxDesktop)
	l.SetVersion(types.V1)
	l.SetMetadata(&headerv1.Metadata{
		Name: name,
	})
	l.SetSpec(spec)
	return l
}
