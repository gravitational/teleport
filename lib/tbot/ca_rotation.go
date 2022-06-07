/*
Copyright 2022 Gravitational, Inc.

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

package tbot

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

// See https://github.com/gravitational/teleport/blob/1aa38f4bc56997ba13b26a1ef1b4da7a3a078930/lib/auth/rotate.go#L135
// for server side details of how a CA rotation takes place.
//
// We can leverage the existing renewal system to fetch new certificates and
// CAs.
//
// We need to force a renewal for the following transitions:
// - Init -> Update Clients: So we can receive a set of certificates issued by
//   the new CA, and trust both the old and new CA.
// - Update Clients, Update Servers -> Rollback: So we can receive a set of
//   certificates issued by the old CA, and stop trusting the new CA.
// - Update Servers -> Standby: So we can stop trusting the old CA.

func (b *Bot) caRotationLoop(ctx context.Context) error {
	for ctx.Err() != nil {
		err := b.watchCARotations(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (b *Bot) watchCARotations(ctx context.Context) error {
	b.log.Debugf("Attempting to establish watch for CA events")
	watcher, err := b.client().NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{{
			Kind: types.KindCertAuthority,
		}},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()

	for {
		select {
		case event := <-watcher.Events():
			b.log.Debugf("Received event: %+v", event)
			if event.Type == types.OpInit {
				b.log.Infof("Started watching for CA rotations")
				continue
			}
			if event.Type != types.OpPut {
				b.log.Debugf("Ignoring CA event due to type: %s", event.Type)
				continue
			}
			ca, ok := event.Resource.(types.CertAuthority)
			if !ok {
				// TODO: Determine if we should take more drastic action here.
				// Realistically, if other event types are being delivered,
				// there's probably been some element of developer error. We
				// could hypothetically panic here.
				b.log.Debugf(
					"Skipping unexpected event type: %v for %v",
					event.Type, event.Resource.GetName(),
				)
				continue
			}

			phase := ca.GetRotation().Phase
			if phase == "init" || phase == "update_servers" {
				b.log.Debug("Skipping due to phase: %s", phase)
			}
			b.log.Debugf("CA: %+v", ca)

			// TODO: Fetch the CA so we can determine if we need to rotate.

			// We need to debounce here, as multiple events will be received if
			// the user is rotating multiple CAs at once.
			b.log.Infof("CA Rotation step detected; attempting to force renewal.")
			select {
			case b.reloadChan <- struct{}{}:
			default:
				// TODO: Come up with a significantly less janky debounce method
				b.log.Debugf("Renewal already in progress; ignoring event.")
			}
		case <-watcher.Done():
			if err := watcher.Error(); err != nil {
				b.log.WithError(err).Warnf("error watching for CA rotations")
				// return trace.Wrap(err)
			}
			return nil
		case <-ctx.Done():
			return nil
		}
	}
}
