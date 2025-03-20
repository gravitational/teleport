/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package accessmonitoring

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestAccessMonitoringRule(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	events := newMockEventsClient()

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	monitor, err := NewAccessMonitor(Config{
		Backend: backend,
		Events:  events,
	})
	require.NoError(t, err)

	ruleHandler := newMockEventHandler()
	monitor.AddAccessMonitoringRuleHandler(ruleHandler.handleEvent)
	go func() { require.NoError(t, monitor.Run(ctx)) }()

	// Test rule handler initializaiton.
	events.watcher.ch <- types.Event{
		Type: types.OpInit,
		Resource: types.NewWatchStatus(
			[]types.WatchKind{
				{Kind: types.KindAccessMonitoringRule},
			},
		),
	}
	require.EventuallyWithT(t,
		ruleHandler.requireEvent(types.Event{
			Type: types.OpInit,
			Resource: types.NewWatchStatus(
				[]types.WatchKind{
					{Kind: types.KindAccessMonitoringRule},
				},
			),
		}), 10*time.Second, 100*time.Millisecond,
		"monitor passes init event")

	rule := newApprovalRule("test-rule", "condition")

	// Test new access monitoring rule event.
	events.watcher.ch <- types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToLegacy(rule),
	}
	require.EventuallyWithT(t,
		ruleHandler.requireEvent(types.Event{
			Type:     types.OpPut,
			Resource: types.Resource153ToLegacy(rule),
		}), 10*time.Second, 100*time.Millisecond,
		"handle create access monitoring rule event")

	// Test update access monitoring rule event.
	rule.Spec.Condition = "new-condition"
	events.watcher.ch <- types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToLegacy(rule),
	}
	require.EventuallyWithT(t,
		ruleHandler.requireEvent(types.Event{
			Type:     types.OpPut,
			Resource: types.Resource153ToLegacy(rule),
		}), 10*time.Second, 100*time.Millisecond,
		"handle update access monitoring rule event")

	// Test delete access monitoring rule event.
	events.watcher.ch <- types.Event{
		Type:     types.OpDelete,
		Resource: types.Resource153ToLegacy(rule),
	}
	require.EventuallyWithT(t,
		ruleHandler.requireEvent(types.Event{
			Type:     types.OpDelete,
			Resource: types.Resource153ToLegacy(rule),
		}), 10*time.Second, 100*time.Millisecond,
		"handle delete access monitoring rule event")

	// Test delete access monitoring rule event from resource header.
	// Delete events typically only include the resource kind and name.
	events.watcher.ch <- types.Event{
		Type: types.OpDelete,
		Resource: &types.ResourceHeader{
			Kind: types.KindAccessMonitoringRule,
			Metadata: types.Metadata{
				Name: rule.GetMetadata().GetName(),
			},
		},
	}
	require.EventuallyWithT(t,
		ruleHandler.requireEvent(types.Event{
			Type: types.OpDelete,
			Resource: &types.ResourceHeader{
				Kind: types.KindAccessMonitoringRule,
				Metadata: types.Metadata{
					Name: rule.GetMetadata().GetName(),
				},
			},
		}), 10*time.Second, 100*time.Millisecond,
		"handle delete access monitoring rule event from resource header")
}

func TestAccessRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	events := newMockEventsClient()

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	monitor, err := NewAccessMonitor(Config{
		Backend: backend,
		Events:  events,
	})
	require.NoError(t, err)

	requestHandler := newMockEventHandler()
	monitor.AddAccessRequestHandler(requestHandler.handleEvent)

	// Must wait for init event before handling access request events.
	initCh := make(chan types.Event)
	monitor.AddAccessMonitoringRuleHandler(func(ctx context.Context, event types.Event) error {
		initCh <- event
		return nil
	})

	go func() { require.NoError(t, monitor.Run(ctx)) }()

	// Test rule handler initializaiton.
	events.watcher.ch <- types.Event{
		Type: types.OpInit,
		Resource: types.NewWatchStatus(
			[]types.WatchKind{
				{Kind: types.KindAccessRequest},
			},
		),
	}
	require.EventuallyWithT(t,
		requireEvent(initCh, types.Event{
			Type: types.OpInit,
			Resource: types.NewWatchStatus(
				[]types.WatchKind{
					{Kind: types.KindAccessRequest},
				},
			),
		}), 10*time.Second, 100*time.Millisecond,
		"wait for initialize event")

	req, err := types.NewAccessRequest(uuid.New().String(), "test-requester", "test-role")
	require.NoError(t, err)

	// Test create access request event.
	events.watcher.ch <- types.Event{
		Type:     types.OpPut,
		Resource: req,
	}
	require.EventuallyWithT(t,
		requestHandler.requireEvent(types.Event{
			Type:     types.OpPut,
			Resource: req,
		}), 10*time.Second, 100*time.Millisecond,
		"handle create access request event")

	// Test review access request event.
	req.SetReviews([]types.AccessReview{{
		Author:        "test-reviewer",
		ProposedState: types.RequestState_APPROVED,
		Created:       time.Now(),
		Reason:        "okay",
	}})
	events.watcher.ch <- types.Event{
		Type:     types.OpPut,
		Resource: req,
	}
	require.EventuallyWithT(t,
		requestHandler.requireEvent(types.Event{
			Type:     types.OpPut,
			Resource: req,
		}), 10*time.Second, 100*time.Millisecond,
		"handle update access request event")

	// Test delete access request event.
	events.watcher.ch <- types.Event{
		Type:     types.OpDelete,
		Resource: req,
	}
	require.EventuallyWithT(t,
		requestHandler.requireEvent(types.Event{
			Type:     types.OpDelete,
			Resource: req,
		}), 10*time.Second, 100*time.Millisecond,
		"handle delete access request event")

	// Test delete access request event from resource header.
	// Delete events typically only include the resource kind and name.
	events.watcher.ch <- types.Event{
		Type: types.OpDelete,
		Resource: &types.ResourceHeader{
			Kind: types.KindAccessRequest,
			Metadata: types.Metadata{
				Name: req.GetName(),
			},
		},
	}
	require.EventuallyWithT(t,
		requestHandler.requireEvent(types.Event{
			Type: types.OpDelete,
			Resource: &types.ResourceHeader{
				Kind: types.KindAccessRequest,
				Metadata: types.Metadata{
					Name: req.GetName(),
				},
			},
		}), 10*time.Second, 100*time.Millisecond,
		"handle delete access request event from resource header")
}

// mockEventsClient is a mock implementation of the types.Events client.
type mockEventsClient struct {
	watcher *mockWatcher
}

// newMockEventsClient returns a new events client for testing.
func newMockEventsClient() *mockEventsClient {
	return &mockEventsClient{
		watcher: &mockWatcher{ch: make(chan types.Event, 1)},
	}
}

// NewWatcher returns a new watcher.
func (c *mockEventsClient) NewWatcher(_ context.Context, _ types.Watch) (types.Watcher, error) {
	return c.watcher, nil
}

type mockWatcher struct {
	ch chan types.Event
}

// Events returns a stream of events.
func (w mockWatcher) Events() <-chan types.Event {
	return w.ch
}

// Done returns a completion channel.
func (w mockWatcher) Done() <-chan struct{} {
	return nil
}

// Close sends a termination signal to watcher.
func (w mockWatcher) Close() error {
	return nil
}

// Error returns a watcher error.
func (w mockWatcher) Error() error {
	return nil
}

// newApprovalRule creates a new access monitoring rule for testing.
func newApprovalRule(name, condition string) *accessmonitoringrulesv1.AccessMonitoringRule {
	const stateApproved = "approved"
	const pluginName = "test"

	return &accessmonitoringrulesv1.AccessMonitoringRule{
		Kind:    types.KindAccessMonitoringRule,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
			Subjects:  []string{types.KindAccessRequest},
			States:    []string{stateApproved},
			Condition: condition,
			AutomaticApproval: &accessmonitoringrulesv1.AutomaticApproval{
				Name: pluginName,
			},
		},
	}
}

// mockEventHandler is a mock implementation of the EventHandler.
type mockEventHandler struct {
	eventCh chan types.Event
}

func newMockEventHandler() *mockEventHandler {
	return &mockEventHandler{
		eventCh: make(chan types.Event),
	}
}

func (m *mockEventHandler) handleEvent(ctx context.Context, event types.Event) error {
	m.eventCh <- event
	return nil
}

func (m *mockEventHandler) requireEvent(event types.Event) func(*assert.CollectT) {
	return requireEvent(m.eventCh, event)
}

// requireEvent asserts that the event received from the channel equals the
// the provided event.
func requireEvent(ch <-chan types.Event, event types.Event) func(*assert.CollectT) {
	return func(c *assert.CollectT) {
		select {
		case e := <-ch:
			assert.Equal(c, event.Type, e.Type)
			assert.Equal(c, event.Resource.GetKind(), e.Resource.GetKind())
			assert.Equal(c, event.Resource.GetName(), e.Resource.GetName())
		default:
			assert.Fail(c, "No event in queue")
		}
	}
}
