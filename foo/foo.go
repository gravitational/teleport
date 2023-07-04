package main

import (
	"context"
	"path/filepath"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	backendv1alpha "github.com/gravitational/teleport/api/gen/proto/go/backend/v1alpha"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
)

func main() {
	ctx := context.Background()

	addr := "main.wet-dry.world:3025"
	dataDir := "/Users/espadolini/teleport-local/datadir_aio"
	hostUUID, err := utils.ReadHostUUID(dataDir)
	if err != nil {
		panic(err)
	}

	id, err := auth.ReadLocalIdentity(filepath.Join(dataDir, teleport.ComponentProcess), auth.IdentityID{Role: types.RoleAdmin, HostUUID: hostUUID})
	if err != nil {
		panic(err)
	}
	t, err := id.TLSConfig(nil)
	if err != nil {
		panic(err)
	}

	clt, err := client.New(ctx, client.Config{
		Addrs:       []string{addr},
		Credentials: []client.Credentials{client.LoadTLS(t)},

		ALPNSNIAuthDialClusterName: id.ClusterName,
	})
	if err != nil {
		panic(err)
	}

	startKey := backend.ExactKey()
	if _, err := clt.UnstableBackendClient().DeleteRange(ctx, &backendv1alpha.DeleteRangeRequest{
		StartKey: startKey,
		EndKey:   backend.RangeEnd(startKey),
	}); err != nil {
		panic(err)
	}
}
