// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package srv

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	userprovisioningv2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/label"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

const staticHostUserWatcherTimeout = 30 * time.Second

// InfoGetter is an interface for getting up-to-date server info.
type InfoGetter interface {
	// GetInfo gets a server, including dynamic labels.
	GetInfo() types.Server
}

// StaticHostUserHandler handles watching for static host user resources and
// applying them to the host.
type StaticHostUserHandler struct {
	events         types.Events
	staticHostUser services.StaticHostUser
	server         InfoGetter
	users          HostUsers
	sudoers        HostSudoers
	retry          *retryutils.Linear
	clock          clockwork.Clock
}

// StaticHostUserHandlerConfig configures a StaticHostUserHandler.
type StaticHostUserHandlerConfig struct {
	// Events is an events interface for creating a watcher.
	Events types.Events
	// StaticHostUser is a static host user client.
	StaticHostUser services.StaticHostUser
	// Server is a resource to fetch a types.Server for access checks. This is
	// here instead of a types.Server directly so we can get updated dynamic
	// labels.
	Server InfoGetter
	// Users is a host user backend.
	Users HostUsers
	// Sudoers is a host sudoers backend.
	Sudoers HostSudoers

	clock clockwork.Clock
}

// NewStaticHostUserHandler creates a new StaticHostUserHandler.
func NewStaticHostUserHandler(cfg StaticHostUserHandlerConfig) (*StaticHostUserHandler, error) {
	if cfg.Events == nil {
		return nil, trace.BadParameter("missing Events")
	}
	if cfg.StaticHostUser == nil {
		return nil, trace.BadParameter("missing StaticHostUser")
	}
	if cfg.Server == nil {
		return nil, trace.BadParameter("missing Server")
	}
	if cfg.clock == nil {
		cfg.clock = clockwork.NewRealClock()
	}
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  retryutils.FullJitter(defaults.MaxWatcherBackoff / 10),
		Step:   defaults.MaxWatcherBackoff / 5,
		Max:    defaults.MaxWatcherBackoff,
		Jitter: retryutils.HalfJitter,
		Clock:  cfg.clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &StaticHostUserHandler{
		events:         cfg.Events,
		staticHostUser: cfg.StaticHostUser,
		server:         cfg.Server,
		users:          cfg.Users,
		sudoers:        cfg.Sudoers,
		retry:          retry,
		clock:          cfg.clock,
	}, nil
}

// Run runs the static host user handler to completion.
func (s *StaticHostUserHandler) Run(ctx context.Context) error {
	if s.users == nil {
		return nil
	}

	for {
		err := s.run(ctx)
		if err == nil {
			return nil
		}
		slog.DebugContext(ctx, "Static host user handler encountered a network error, will restart.", "error", err)
		s.retry.Inc()

		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-s.retry.After():
		}
	}
}

func (s *StaticHostUserHandler) run(ctx context.Context) error {
	// Start the watcher.
	watcher, err := s.events.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			{
				Kind: types.KindStaticHostUser,
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()

	watcherTimer := s.clock.NewTimer(staticHostUserWatcherTimeout)
	defer watcherTimer.Stop()
	select {
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.Errorf("missing init event from watcher")
		}
		s.retry.Reset()
	case <-watcherTimer.Chan():
		return trace.LimitExceeded("timed out waiting for static host user watcher to initialize")
	case <-ctx.Done():
		return nil
	}

	// Fetch any host users that existed prior to creating the watcher.
	var startKey string
	for {
		users, nextKey, err := s.staticHostUser.ListStaticHostUsers(ctx, 0, startKey)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, hostUser := range users {
			if err := s.handleNewHostUser(ctx, hostUser); err != nil {
				// Log the error so we don't stop the handler.
				slog.WarnContext(ctx, "Error handling static host user.", "error", err, "login", hostUser.GetMetadata().Name)
			}
		}
		if nextKey == "" {
			break
		}
		startKey = nextKey
	}

	// Listen for new host users on the watcher.
	for {
		select {
		case event := <-watcher.Events():
			if event.Type != types.OpPut {
				continue
			}
			r, ok := event.Resource.(types.Resource153UnwrapperT[*userprovisioningv2.StaticHostUser])
			if !ok {
				slog.WarnContext(ctx, "Unexpected resource type.", "resource", event.Resource)
				continue
			}
			hostUser := r.UnwrapT()

			if err := s.handleNewHostUser(ctx, hostUser); err != nil {
				// Log the error so we don't stop the handler.
				slog.WarnContext(ctx, "Error handling static host user.", "error", err, "login", hostUser.GetMetadata().Name)
				continue
			}
		case <-watcher.Done():
			return trace.Wrap(watcher.Error())
		case <-ctx.Done():
			if !errors.Is(ctx.Err(), context.Canceled) {
				return trace.Wrap(ctx.Err())
			}
			return nil
		}
	}
}

func (s *StaticHostUserHandler) handleNewHostUser(ctx context.Context, hostUser *userprovisioningv2.StaticHostUser) error {
	var createUser *userprovisioningv2.Matcher
	login := hostUser.GetMetadata().Name
	server := s.server.GetInfo()
	for _, matcher := range hostUser.Spec.Matchers {
		// Check if this host user applies to this node.
		nodeLabels := make(types.Labels)
		for k, v := range label.ToMap(matcher.NodeLabels) {
			nodeLabels[k] = apiutils.Strings(v)
		}
		matched, _, err := services.CheckLabelsMatch(
			types.Allow,
			types.LabelMatchers{
				Labels:     nodeLabels,
				Expression: matcher.NodeLabelsExpression,
			},
			nil, // userTraits
			server,
			false, // debug
		)
		if err != nil {
			return trace.Wrap(err)
		}
		if !matched {
			continue
		}

		// Matching multiple times is an error.
		if createUser != nil {
			const msg = "Multiple matchers matched this node. Please update resource to ensure that each node is matched only once."
			slog.WarnContext(ctx, msg, slog.String("login", login),
				slog.Group("first_match", "labels", createUser.NodeLabels, "expression", createUser.NodeLabelsExpression),
				slog.Group("second_match", "labels", matcher.NodeLabels, "expression", matcher.NodeLabelsExpression),
			)
			return trace.BadParameter("%s", msg)
		}
		createUser = matcher
	}

	if createUser == nil {
		return nil
	}

	slog.DebugContext(ctx, "Attempt to update matched static host user.", "login", login)
	ui := decisionpb.HostUsersInfo{
		Groups: createUser.Groups,
		Mode:   decisionpb.HostUserMode_HOST_USER_MODE_STATIC,
		Shell:  createUser.DefaultShell,
	}
	if createUser.Uid != 0 {
		ui.Uid = strconv.Itoa(int(createUser.Uid))
	}
	if createUser.Gid != 0 {
		ui.Gid = strconv.Itoa(int(createUser.Gid))
	}
	if _, err := s.users.UpsertUser(login, &ui, TakeOwnershipIfUserExists(createUser.TakeOwnershipIfUserExists)); err != nil {
		return trace.Wrap(err)
	}
	if s.sudoers != nil && len(createUser.Sudoers) != 0 {
		if err := s.sudoers.WriteSudoers(login, createUser.Sudoers); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
