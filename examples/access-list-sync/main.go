/*
Copyright 2026 Gravitational, Inc.

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

// Package main demonstrates how to sync Access Lists and their members from an
// external identity system into Teleport using the Teleport API client.
//
// See: docs/pages/identity-governance/access-lists/sync-from-external-system.mdx
package main

import (
	"context"
	"log"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	accesslistclient "github.com/gravitational/teleport/api/client/accesslist"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	teleportClient, err := client.New(ctx, client.Config{
		Addrs: []string{"ice-berg.dev:3080"},
		Credentials: []client.Credentials{
			client.LoadProfile("", ""),
		},
	})
	if err != nil {
		log.Fatalf("Failed to create Teleport client: %v", err)
	}
	defer teleportClient.Close()

	if _, err := teleportClient.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping Teleport: %v", err)
	}

	alClient := teleportClient.AccessListClient()

	if err := syncFromUpstream(ctx, &mockUpstreamClient{}, alClient); err != nil {
		log.Fatalf("Sync failed: %v", err)
	}
}

// syncFromUpstream fetches both sides, builds the reconcilers, and runs them.
func syncFromUpstream(ctx context.Context, upstream UpstreamClient, alClient *accesslistclient.Client) error {
	existingLists, err := listAccessListsFromTeleport(ctx, alClient)
	if err != nil {
		return trace.Wrap(err)
	}
	newLists, err := accessListsFromUpstream(ctx, upstream)
	if err != nil {
		return trace.Wrap(err)
	}

	existingMembers, err := accessListMembersFromTeleport(ctx, accessListNames(existingLists), alClient)
	if err != nil {
		return trace.Wrap(err)
	}
	newMembers, err := accessListMembersFromUpstream(ctx, upstream)
	if err != nil {
		return trace.Wrap(err)
	}

	alReconciler, err := newAccessListReconciler(alClient, existingLists, newLists)
	if err != nil {
		return trace.Wrap(err)
	}
	memberReconciler, err := newAccessListMemberReconciler(alClient, existingMembers, newMembers)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.NewAggregate(
		alReconciler.Reconcile(ctx),
		memberReconciler.Reconcile(ctx),
	)
}
