/*
Copyright 2024 Gravitational, Inc.

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
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/credentials"
	"github.com/gravitational/teleport/lib/events/export"
)

// TeleportSearchEventsClient is an interface for client.Client, required for testing
type TeleportSearchEventsClient interface {
	export.Client
	// SearchEvents searches for events in the audit log and returns them using their protobuf representation.
	SearchEvents(ctx context.Context, fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) ([]events.AuditEvent, string, error)
	// StreamSessionEvents returns session events stream for a given session ID using their protobuf representation.
	StreamSessionEvents(ctx context.Context, sessionID string, startIndex int64) (chan events.AuditEvent, chan error)
	// SearchUnstructuredEvents searches for events in the audit log and returns them using an unstructured representation (structpb.Struct).
	SearchUnstructuredEvents(ctx context.Context, fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) ([]*auditlogpb.EventUnstructured, string, error)
	// StreamUnstructuredSessionEvents returns session events stream for a given session ID using an unstructured representation (structpb.Struct).
	StreamUnstructuredSessionEvents(ctx context.Context, sessionID string, startIndex int64) (chan *auditlogpb.EventUnstructured, chan error)
	UpsertLock(ctx context.Context, lock types.Lock) error
	Ping(ctx context.Context) (proto.PingResponse, error)
	Close() error
}

// newClient performs teleport api client setup, including credentials loading, validation, and
// setup of credentials refresh if needed.
func newClient(ctx context.Context, log *slog.Logger, c *StartCmdConfig) (*client.Client, error) {
	var creds []client.Credentials
	switch {
	case c.TeleportIdentityFile != "" && !c.TeleportRefreshEnabled:
		creds = []client.Credentials{client.LoadIdentityFile(c.TeleportIdentityFile)}
	case c.TeleportIdentityFile != "" && c.TeleportRefreshEnabled:
		cred, err := lib.NewIdentityFileWatcher(ctx, c.TeleportIdentityFile, c.TeleportRefreshInterval)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		creds = []client.Credentials{cred}
	case c.TeleportCert != "" && c.TeleportKey != "" && c.TeleportCA != "":
		creds = []client.Credentials{client.LoadKeyPair(c.TeleportCert, c.TeleportKey, c.TeleportCA)}
	default:
		return nil, trace.BadParameter("no credentials configured")
	}

	if validCred, err := credentials.CheckIfExpired(creds); err != nil {
		log.WarnContext(ctx, "Encountered error when checking credentials", "error", err)
		if !validCred {
			return nil, trace.BadParameter(
				"No valid credentials found, this likely means credentials are expired. In this case, please sign new credentials and increase their TTL if needed.",
			)
		}
		log.InfoContext(ctx, "At least one non-expired credential has been found, continuing startup")
	}

	clientConfig := client.Config{
		Addrs:       []string{c.TeleportAddr},
		Credentials: creds,
	}

	teleportClient, err := client.New(ctx, clientConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return teleportClient, nil
}

// upsertLock is a helper used to create or update the event handler's auto lock.
func upsertLock(ctx context.Context, clt TeleportSearchEventsClient, user string, login string, period time.Duration) error {
	var expires *time.Time

	if period > 0 {
		t := time.Now()
		t = t.Add(period)
		expires = &t
	}

	lock := &types.LockV2{
		Metadata: types.Metadata{
			Name: fmt.Sprintf("event-handler-auto-lock-%v-%v", user, login),
		},
		Spec: types.LockSpecV2{
			Target: types.LockTarget{
				Login: login,
				User:  user,
			},
			Message: lockMessage,
			Expires: expires,
		},
	}

	return clt.UpsertLock(ctx, lock)
}

// normalizeDate normalizes a timestamp to the beginning of the day in UTC.
func normalizeDate(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
