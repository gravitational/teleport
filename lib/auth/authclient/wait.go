package authclient

import (
	"context"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// SnowflakeSessionWatcher is watcher interface used by Snowflake web session watcher.
type snowflakeSessionWatcher interface {
	// NewWatcher returns a new event watcher.
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)
	// GetSnowflakeSession gets a Snowflake web session for a given request.
	GetSnowflakeSession(context.Context, types.GetSnowflakeSessionRequest) (types.WebSession, error)
}

type appSessionWatcher interface {
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)
	GetAppSession(context.Context, types.GetAppSessionRequest) (types.WebSession, error)
}

// WaitForAppSession will block until the requested application session shows up in the
// cache or a timeout occurs.
func WaitForAppSession(ctx context.Context, sessionID, user string, ap appSessionWatcher) error {
	req := waitForWebSessionReq{
		newWatcherFn: ap.NewWatcher,
		getSessionFn: func(ctx context.Context, sessionID string) (types.WebSession, error) {
			return ap.GetAppSession(ctx, types.GetAppSessionRequest{SessionID: sessionID})
		},
	}
	return trace.Wrap(waitForWebSession(ctx, sessionID, user, types.KindAppSession, req))
}

// WaitForSnowflakeSession waits until the requested Snowflake session shows up int the cache
// or a timeout occurs.
func WaitForSnowflakeSession(ctx context.Context, sessionID, user string, ap snowflakeSessionWatcher) error {
	req := waitForWebSessionReq{
		newWatcherFn: ap.NewWatcher,
		getSessionFn: func(ctx context.Context, sessionID string) (types.WebSession, error) {
			return ap.GetSnowflakeSession(ctx, types.GetSnowflakeSessionRequest{SessionID: sessionID})
		},
	}
	return trace.Wrap(waitForWebSession(ctx, sessionID, user, types.KindSnowflakeSession, req))
}

// waitForWebSessionReq is a request to wait for web session to be populated in the application cache.
type waitForWebSessionReq struct {
	// newWatcherFn is a function that returns new event watcher.
	newWatcherFn func(ctx context.Context, watch types.Watch) (types.Watcher, error)
	// getSessionFn is a function that returns web session by given ID.
	getSessionFn func(ctx context.Context, sessionID string) (types.WebSession, error)
}

// waitForWebSession is an implementation for web session wait functions.
func waitForWebSession(ctx context.Context, sessionID, user string, evenSubKind string, req waitForWebSessionReq) error {
	_, err := req.getSessionFn(ctx, sessionID)
	if err == nil {
		return nil
	}
	logger := log.WithField("session", sessionID)
	if !trace.IsNotFound(err) {
		logger.WithError(err).Debug("Failed to query web session.")
	}
	// Establish a watch on application session.
	watcher, err := req.newWatcherFn(ctx, types.Watch{
		Name: teleport.ComponentAppProxy,
		Kinds: []types.WatchKind{
			{
				Kind:    types.KindWebSession,
				SubKind: evenSubKind,
				Filter:  (&types.WebSessionFilter{User: user}).IntoMap(),
			},
		},
		MetricComponent: teleport.ComponentAppProxy,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()
	matchEvent := func(event types.Event) (types.Resource, error) {
		if event.Type == types.OpPut &&
			event.Resource.GetKind() == types.KindWebSession &&
			event.Resource.GetSubKind() == evenSubKind &&
			event.Resource.GetName() == sessionID {
			return event.Resource, nil
		}
		return nil, trace.CompareFailed("no match")
	}
	_, err = local.WaitForEvent(ctx, watcher, local.EventMatcherFunc(matchEvent), clockwork.NewRealClock())
	if err != nil {
		logger.WithError(err).Warn("Failed to wait for web session.")
		// See again if we maybe missed the event but the session was actually created.
		if _, err := req.getSessionFn(ctx, sessionID); err == nil {
			return nil
		}
	}
	return trace.Wrap(err)
}
