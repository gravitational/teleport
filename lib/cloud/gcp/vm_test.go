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

package gcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/google/go-cmp/cmp"
	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/internalutils/stream"
	gcpimds "github.com/gravitational/teleport/lib/cloud/imds/gcp"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
)

func TestConvertAPIError(t *testing.T) {
	t.Parallel()
	fromError := func(t *testing.T, err error) error {
		outErr, ok := apierror.FromError(err)
		require.True(t, ok)
		return outErr
	}
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "google error",
			err: &googleapi.Error{
				Code:    http.StatusForbidden,
				Message: "abcd1234",
			},
		},
		{
			name: "api http error",
			err: fromError(t, &googleapi.Error{
				Code:    http.StatusForbidden,
				Message: "abcd1234",
			}),
		},
		{
			name: "api grpc error",
			err:  fromError(t, status.Error(codes.PermissionDenied, "abcd1234")),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := convertAPIError(tc.err)
			require.True(t, trace.IsAccessDenied(err), "unexpected error of type %T: %v", tc.err, err)
			require.Contains(t, tc.err.Error(), "abcd1234")
		})
	}
	t.Run("not a gcp error", func(t *testing.T) {
		inErr := trace.Errorf("abcd1234")
		outErr := convertAPIError(inErr)
		require.Same(t, inErr, outErr)
	})
}

type mockInstance struct {
	hostKeys      []ssh.PublicKey
	authorizedKey ssh.PublicKey
	sshServer     *sshutils.Server
	execCount     int

	instance *gcpimds.Instance
}

func newMockInstance(t *testing.T, hostSigner ssh.Signer, listener net.Listener) *mockInstance {
	mock := &mockInstance{
		hostKeys: []ssh.PublicKey{hostSigner.PublicKey()},
	}
	sshServer, err := sshutils.NewServer(
		"gcp-vm-server",
		utils.NetAddr{AddrNetwork: "tcp", Addr: listener.Addr().String()},
		mock,
		sshutils.StaticHostSigners(hostSigner),
		sshutils.AuthMethods{
			PublicKey: mock.userKeyAuth,
		},
		sshutils.SetInsecureSkipHostValidation(),
	)
	require.NoError(t, err)
	require.NoError(t, sshServer.SetListener(listener))
	mock.sshServer = sshServer
	return mock
}

func (m *mockInstance) Start() error {
	return trace.Wrap(m.sshServer.Start())
}

func (m *mockInstance) Stop() {
	m.sshServer.Close()
}

func (m *mockInstance) HandleNewChan(_ context.Context, ccx *sshutils.ConnectionContext, newChannel ssh.NewChannel) {
	channel, reqs, err := newChannel.Accept()
	if err != nil {
		ccx.ServerConn.Close()
		ccx.NetConn.Close()
		return
	}

	go m.handleChannel(channel, reqs)
}

func (m *mockInstance) handleChannel(channel ssh.Channel, reqs <-chan *ssh.Request) {
	defer channel.Close()

	for req := range reqs {
		if req.Type == "exec" {
			m.execCount++
			successPayload := ssh.Marshal(struct{ C uint32 }{C: uint32(0)})
			channel.SendRequest("exit-status", false, successPayload)
			if req.WantReply {
				req.Reply(true, nil)
			}
			return
		}
		if req.WantReply {
			req.Reply(true, nil)
		}
	}
}

func (m *mockInstance) userKeyAuth(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	if bytes.Equal(key.Marshal(), m.authorizedKey.Marshal()) {
		return nil, nil
	}
	return nil, trace.AccessDenied("unknown public key for user %q", conn.User())
}

func (m *mockInstance) ListInstances(ctx context.Context, projectID, location string) ([]*gcpimds.Instance, error) {
	return []*gcpimds.Instance{m.instance}, nil
}

func (m *mockInstance) StreamInstances(ctx context.Context, projectID, location string) stream.Stream[*gcpimds.Instance] {
	return stream.Once(m.instance)
}

func (m *mockInstance) GetInstance(ctx context.Context, req *gcpimds.InstanceRequest) (*gcpimds.Instance, error) {
	return m.instance, nil
}

func (m *mockInstance) GetInstanceTags(ctx context.Context, req *gcpimds.InstanceRequest) (map[string]string, error) {
	return nil, nil
}

func (m *mockInstance) AddSSHKey(ctx context.Context, req *SSHKeyRequest) error {
	m.authorizedKey = req.PublicKey
	return nil
}

func (m *mockInstance) RemoveSSHKey(ctx context.Context, req *SSHKeyRequest) error {
	m.authorizedKey = nil
	return nil
}

func (m *mockInstance) setInstance(inst *gcpimds.Instance) {
	inst.HostKeys = m.hostKeys
	m.instance = inst
}

type mockListener struct {
	ctx context.Context
	net.Conn
	accepted atomic.Bool
}

func (m *mockListener) Accept() (net.Conn, error) {
	// Return the stored conn once.
	if m.accepted.CompareAndSwap(false, true) {
		return m, nil
	}
	// Block until the test is done.
	<-m.ctx.Done()
	return nil, m.ctx.Err()
}

func (m *mockListener) Addr() net.Addr {
	return &utils.NetAddr{
		Addr:        "teleport.cluster.local:22",
		AddrNetwork: "tcp",
	}
}

func TestRunCommand(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	signer, err := generateKeyPair(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	clientConn, serverConn, err := utils.DualPipeNetConn(
		&utils.NetAddr{Addr: "server", AddrNetwork: "tcp"},
		&utils.NetAddr{Addr: "client", AddrNetwork: "tcp"},
	)
	require.NoError(t, err)
	mock := newMockInstance(t, signer, &mockListener{Conn: serverConn, ctx: ctx})
	require.NoError(t, mock.Start())
	t.Cleanup(mock.Stop)
	mock.hostKeys = []ssh.PublicKey{signer.PublicKey()}

	inst := &gcpimds.Instance{
		Name:              "my-instance",
		Zone:              "my-zone",
		ProjectID:         "my-project-id",
		InternalIPAddress: "test.example.com",
		MetadataItems:     map[string]string{},
	}
	mock.setInstance(inst)

	require.NoError(t, RunCommand(ctx, &RunCommandRequest{
		Client: mock,
		InstanceRequest: gcpimds.InstanceRequest{
			ProjectID: "my-project-id",
			Zone:      "my-zone",
			Name:      "my-instance",
		},
		// Script value doesn't matter, it won't really be executed.
		Script: "echo hello",
		dialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return clientConn, nil
		},
		SSHKeyAlgo: cryptosuites.ECDSAP256,
	}))
	require.Equal(t, 1, mock.execCount)
}

func TestGetInstance(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/compute/v1/projects/{project}/zones/{zone}/instances/{id}", func(w http.ResponseWriter, r *http.Request) {
		out, err := json.Marshal(&computepb.Instance{
			Name:        googleapi.String("Test"),
			Zone:        googleapi.String("TestZone"),
			Fingerprint: googleapi.String("9876"),
			NetworkInterfaces: []*computepb.NetworkInterface{
				{NetworkIP: googleapi.String("10.20.30.40")},
			},
			Metadata: &computepb.Metadata{
				Fingerprint: googleapi.String("1234"),
				Items: []*computepb.Items{
					{Key: googleapi.String("animal"), Value: googleapi.String("llama")},
				},
			},
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Write(out)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	clt, err := compute.NewInstancesRESTClient(ctx, option.WithEndpoint(srv.URL), option.WithoutAuthentication(), option.WithHTTPClient(srv.Client()))
	require.NoError(t, err)

	instanceClient := instancesClient{InstancesClientConfig{InstanceClient: clt}}

	actual, err := instanceClient.GetInstance(ctx, &gcpimds.InstanceRequest{
		ProjectID:       "TestProject",
		Zone:            "TestZone",
		Name:            "llama",
		ID:              123,
		WithoutHostKeys: true,
	})
	require.NoError(t, err)

	expected := &gcpimds.Instance{
		Name:              "Test",
		ProjectID:         "TestProject",
		Zone:              "TestZone",
		Fingerprint:       "1234",
		InternalIPAddress: "10.20.30.40",
		MetadataItems:     map[string]string{"animal": "llama"},
	}
	require.Empty(t, cmp.Diff(expected, actual))

}
