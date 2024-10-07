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

package authorizedkeys

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/constants"
	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types/accessgraph"
)

func TestAuthorizedKeys(t *testing.T) {
	hostID := "hostID"

	dir := createFSData(t)
	clock := clockwork.NewFakeClockAt(time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC))
	client := &fakeClient{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher, err := NewWatcher(ctx, WatcherConfig{
		Client: client,
		getHostUsers: func() ([]user.User, error) {
			return exampleUsers(dir)
		},
		HostID: hostID,
		Clock:  clock,
		Logger: slog.Default(),
		getRuntimeOS: func() string {
			return constants.LinuxOS
		},
	})
	require.NoError(t, err)

	// Start the watcher
	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return trace.Wrap(watcher.Run(ctx))
	})

	// Wait for the watcher to start and to block on the initial spread.
	clock.BlockUntil(2) // wait for clock to blocked at supervisor and initial delay routine
	// Advance the clock to trigger the first scan
	clock.Advance(5 * time.Minute)

	// Wait for the watcher to start
	require.Eventually(t, func() bool {
		return len(client.getReqReceived()) == 2
	}, 1*time.Second, 10*time.Millisecond, "expected watcher to start, but it did not")

	// Check the requests
	got := client.getReqReceived()
	require.Len(t, got, 2)
	expected := []*accessgraphsecretsv1pb.ReportAuthorizedKeysRequest{
		{
			Keys:      createKeysForUsers(t, hostID),
			Operation: accessgraphsecretsv1pb.OperationType_OPERATION_TYPE_ADD,
		},
		{
			Operation: accessgraphsecretsv1pb.OperationType_OPERATION_TYPE_SYNC,
		},
	}
	require.Empty(t, cmp.Diff(got, expected,
		protocmp.Transform(),
		protocmp.SortRepeated(
			func(a, b *accessgraphsecretsv1pb.AuthorizedKey) bool {
				return a.Metadata.Name < b.Metadata.Name
			},
		),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "expires"),
	),
	)
	// Clear the requests
	client.clear()

	cancel()
	err = group.Wait()
	require.NoError(t, err)
}

func createFSData(t *testing.T) string {
	dir := t.TempDir()

	createUsersAndAuthorizedKeys(t, dir)
	return dir
}

func createFile(t *testing.T, dir, name, content string) {
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)
	path := fmt.Sprintf("%s/%s", dir, name)
	err = os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
}

func exampleUsers(dir string) ([]user.User, error) {
	return []user.User{
		{
			Name:     "root",
			Username: "root",
			Uid:      "0",
			Gid:      "0",
			HomeDir:  fmt.Sprintf("%s/root", dir),
		},
		{
			Name:     "bin",
			Username: "bin",
			Uid:      "1",
			Gid:      "1",
			HomeDir:  "/",
		},
		{
			Name:     "user",
			Username: "user",
			Uid:      "1000",
			Gid:      "1000",
			HomeDir:  fmt.Sprintf("%s/user", dir),
		},
	}, nil

}

const authorizedFileExample = `
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQClwXUKOp/S4XEtFjgr8mfaCy4OyI7N9ZMibdCGxvk2VHP9+Vn8Al1lUSVwuBxHI7EHiq42RCTBetIpTjzn6yiPNAeGNL5cfl9i6r+P5k7og1hz+2oheWveGodx6Dp+Z4o2dw65NGf5EPaotXF8AcHJc3+OiMS5yp/x2A3tu2I1SPQ6dtPa067p8q1L49BKbFwrFRBCVwkr6kpEQAIjnMESMPGD5Buu/AtyAdEZQSLTt8RZajJZDfXFKMEtQm2UF248NFl3hSMAcbbTxITBbZxX7THbwQz22Yuw7422G5CYBPf6WRXBY84Rs6jCS4I4GMxj+3rF4mGtjvuz0wOE32s3w4eMh9h3bPuEynufjE8henmPCIW49+kuZO4LZut7Zg5BfVDQnZYclwokEIMz+gR02YpyflxQOa98t/0mENu+t4f0LNAdkQEBpYtGKKDth5kLphi2Sdi9JpGO2sTivlxMsGyBqdd0wT9VwQpWf4wro6t09HdZJX1SAuEi/0tNI10= friel@test
# comment
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGtqQKEkGIY5+Bc4EmEv7NeSn6aA7KMl5eiNEAOqwTBl friel@test
invalidLine
# comment
`

func createUsersAndAuthorizedKeys(t *testing.T, dir string) {
	for _, user := range []string{"root", "user"} {
		dir := filepath.Join(dir, user, ".ssh")
		createFile(t, dir, "authorized_keys", authorizedFileExample)
	}
}

type fakeClient struct {
	accessgraphsecretsv1pb.SecretsScannerServiceClient
	accessgraphsecretsv1pb.SecretsScannerService_ReportAuthorizedKeysClient
	mu          sync.Mutex
	reqReceived []*accessgraphsecretsv1pb.ReportAuthorizedKeysRequest
}

func (f *fakeClient) GetClusterAccessGraphConfig(_ context.Context) (*clusterconfigpb.AccessGraphConfig, error) {
	return &clusterconfigpb.AccessGraphConfig{
		Enabled: true,
		SecretsScanConfig: &clusterconfigpb.AccessGraphSecretsScanConfiguration{
			SshScanEnabled: true,
		},
	}, nil
}
func (f *fakeClient) AccessGraphSecretsScannerClient() accessgraphsecretsv1pb.SecretsScannerServiceClient {
	return f
}

func (f *fakeClient) ReportAuthorizedKeys(_ context.Context, _ ...grpc.CallOption) (accessgraphsecretsv1pb.SecretsScannerService_ReportAuthorizedKeysClient, error) {
	return f, nil
}

func (f *fakeClient) Send(req *accessgraphsecretsv1pb.ReportAuthorizedKeysRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqReceived = append(f.reqReceived, req)
	return nil
}

func (f *fakeClient) CloseSend() error {
	return nil
}

func (f *fakeClient) Recv() (*accessgraphsecretsv1pb.ReportAuthorizedKeysResponse, error) {
	return nil, nil
}

func (f *fakeClient) clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqReceived = nil
}

func (f *fakeClient) getReqReceived() []*accessgraphsecretsv1pb.ReportAuthorizedKeysRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.reqReceived)
}

func createKeysForUsers(t *testing.T, hostID string) []*accessgraphsecretsv1pb.AuthorizedKey {
	var keys []*accessgraphsecretsv1pb.AuthorizedKey
	for _, k := range []struct {
		fingerprint string
		keyType     string
	}{
		{
			fingerprint: "SHA256:GbJlTLeQgZhvGoklWGXHo0AinGgGEcldllgYExoSy+s",
			keyType:     "ssh-ed25519",
		},
		{
			fingerprint: "SHA256:ewwMB/nCAYurNrYFXYZuxLZv7T7vgpPd7QuIo0d5n+U",
			keyType:     "ssh-rsa",
		},
	} {
		for _, user := range []string{"root", "user"} {
			at, err := accessgraph.NewAuthorizedKey(&accessgraphsecretsv1pb.AuthorizedKeySpec{
				HostId:         hostID,
				HostUser:       user,
				KeyFingerprint: k.fingerprint,
				KeyComment:     "friel@test",
				KeyType:        k.keyType,
			})
			require.NoError(t, err)
			keys = append(keys, at)
		}
	}
	return keys
}
