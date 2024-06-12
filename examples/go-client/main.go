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
	"log"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
)

func main() {
	ctx := context.Background()
	log.Printf("Starting Teleport client...")

	cfg := client.Config{
		Credentials: []client.Credentials{
			client.LoadProfile("", ""),
		},
	}

	clt, err := client.New(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer clt.Close()

	if err := demoClient(ctx, clt); err != nil {
		log.Printf("error in demoClient: %v", err)
	}
}

func demoClient(ctx context.Context, clt *client.Client) (err error) {
	// Create a new access request for the `access-admin` user to use the `admin` role.
	accessReq, err := types.NewAccessRequest("", "access-admin", "admin")
	if err != nil {
		return fmt.Errorf("failed to make new access request: %w", err)
	}
	if _, err := clt.CreateAccessRequestV2(ctx, accessReq); err != nil {
		return fmt.Errorf("failed to create access request: %w", err)
	}
	log.Printf("Created access request: %v", accessReq)

	// Approve the access request as if this was another party.
	if err = clt.SetAccessRequestState(ctx, types.AccessRequestUpdate{
		RequestID: accessReq.GetName(),
		State:     types.RequestState_APPROVED,
	}); err != nil {
		return fmt.Errorf("failed to accept request: %w", err)
	}
	log.Printf("Approved access request")

	if err := clt.DeleteAccessRequest(ctx, accessReq.GetName()); err != nil {
		return fmt.Errorf("failed to delete access request: %w", err)
	}
	log.Println("Deleted access request")

	return nil
}
