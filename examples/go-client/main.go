/*
Copyright 2018 Gravitational, Inc.

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

package main

import (
	"context"
	"crypto/tls"
	"log"
	"path/filepath"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

func main() {
	log.Printf("Starting teleport client...")

	// Teleport HTTPS client uses TLS client authentication
	// so we have to set up certificates there
	tlsConfig, err := setupClientTLS(context.Background())
	if err != nil {
		log.Fatalf("Failed to parse TLS config: %v", err)
	}
	authServerAddr := []utils.NetAddr{*utils.MustParseAddr("127.0.0.1:3025")}
	clientConfig := auth.ClientConfig{Addrs: authServerAddr, TLS: tlsConfig}

	client, err := auth.NewTLSClient(clientConfig)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	// make an API call to generate a cluster join token for
	// adding another proxy to a cluster.
	token, err := client.GenerateToken(ctx, auth.GenerateTokenRequest{
		Token: "mytoken-proxy",
		Roles: teleport.Roles{teleport.RoleProxy},
		TTL:   time.Hour,
	})
	if err != nil {
		log.Fatalf("Failed to generate token: %v", err)
	}
	log.Printf("Generated token: %v\n", token)
}

// setupClientTLS sets up client TLS authentiction between TLS client
// and Teleport Auth server. This function uses hardcoded certificate paths,
// assuming program runs alongside auth server, but it can be ran
// on a remote location, assuming client has all the client certificates.
func setupClientTLS(ctx context.Context) (*tls.Config, error) {
	storage, err := auth.NewProcessStorage(ctx, filepath.Join("/var/lib/teleport", teleport.ComponentProcess))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer storage.Close()

	identity, err := storage.ReadIdentity(auth.IdentityCurrent, teleport.RoleAdmin)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return identity.TLSConfig(nil)
}
