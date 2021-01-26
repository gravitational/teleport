/*
Copyright 2018-2021 Gravitational, Inc.

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
	"fmt"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/auth"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.Printf("Starting Teleport client...")
	config := client.Config{
		Addrs: []string{"127.0.0.1:3025"},
		// Credentials: client.PathCreds("certs/api-admin"),
		Credentials: client.ProfileCreds(),
		// Credentials: client.IdentityCreds("/home/bjoerger/dev"),
	}

	client, err := auth.NewClient(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	fmt.Println("")
	roleCRUD(ctx, client)

	fmt.Println("")
	tokenCRUD(ctx, client)

	fmt.Println("")
	accessWorkflow(ctx, client)
}
