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

	var creds client.Credentials
	var err error

	clt, err := client.New(client.Config{
		// Addrs is the Auth Server address, only works locally.
		// TODO (Joerger): add support to connect via proxy.
		Addrs: []string{"localhost:3025"},
		// Multiple credentials can be provided in order to fall back to to
		// later credentials if the previous ones fail to authenticate the client.
		Credentials: client.CredentialsList{
			creds,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer clt.Close()

	if err := demoClient(ctx, clt); err != nil {
		log.Printf("error(s) in demoClient: %v", err)
	}
}

func demoClient(ctx context.Context, clt *client.Client) (err error) {
	// Create a new access request for the `access-admin` user to use the `admin` role.
	accessReq, err := types.NewAccessRequest(uuid.New(), "access-admin", "admin")
	if err != nil {
		return trace.Wrap(err, "failed to make new access request: %v")
	}
	if err = clt.CreateAccessRequest(ctx, accessReq); err != nil {
		return trace.Wrap(err, "failed to create access request: %v")
	}
	log.Printf("Created access request: %v", accessReq)

	defer func() {
		if err2 := clt.DeleteAccessRequest(ctx, accessReq.GetName()); err2 != nil {
			err = trace.NewAggregate([]error{err, err2}...)
			log.Println("Failed to delete access request")
			return
		}
		log.Println("Deleted access request")
	}()

	// Approve the access request as if this was another party.
	if err = clt.SetAccessRequestState(ctx, types.AccessRequestUpdate{
		RequestID: accessReq.GetName(),
		State:     types.RequestState_APPROVED,
	}); err != nil {
		return trace.Wrap(err, "failed to accept request: %v")
	}
	log.Printf("Approved access request")

	return nil
}
