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
	"log"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

func main() {
	ctx := context.Background()
	log.Printf("Starting Teleport client...")

	creds, err := client.ProfileCreds()
	if err != nil {
		log.Fatalf("Failed to read credentials: %v", err)
	}

	clt, err := client.New(client.Config{
		Addrs: []string{"proxy.example.com:3025"},
		Creds: creds,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer clt.Close()

	demoClient(ctx, clt)
}

func demoClient(ctx context.Context, clt *client.Client) (err error) {
	// Create a new access request for api-admin to use the admin role.
	accessReq, err := types.NewAccessRequest(uuid.New(), "access-admin", "admin")
	if err != nil {
		return trace.Wrap(err, "Failed to make new access request: %v")
	}
	if err = clt.CreateAccessRequest(ctx, accessReq); err != nil {
		return trace.Wrap(err, "Failed to create access request: %v")
	}
	log.Printf("Created access request: %v", accessReq)

	defer func() {
		if err = clt.DeleteAccessRequest(ctx, accessReq.GetName()); err != nil {
			log.Fatalf("Failed to delete access request: %v", err)
		}
		log.Println("Deleted access request")
	}()

	// Approve the access request.
	if err = clt.SetAccessRequestState(ctx, types.AccessRequestUpdate{
		RequestID: accessReq.GetName(),
		State:     types.RequestState_APPROVED,
	}); err != nil {
		return trace.Wrap(err, "Failed to accept request: %v")
	}
	log.Printf("Approved access request")

	return nil
}
