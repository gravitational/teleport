package crdb

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
)

var (
	// defaultChangeFeedTimeout is the max time between receiving events before
	// closing the changefeed. This is 3 times the resolve interval.
	defaultChangeFeedTimeout = time.Second * 15
	defaultCloseTimeout      = time.Second * 5
)

func (b *Backend) backgroundChangeFeed(ctx context.Context) {
	defer func() {
		b.log.InfoContext(ctx, "Exited change feed loop.")
		b.buf.Close()
	}()

	for ctx.Err() == nil {
		b.log.InfoContext(ctx, "Starting change feed stream.")
		err := b.runChangeFeed(ctx)
		b.log.ErrorContext(ctx, "Change feed stream lost.", "error", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(defaults.HighResPollingPeriod):
		}
	}
}

func (b *Backend) runChangeFeed(ctx context.Context) error {
	cr, err := newChangeReader(ctx, b.feedConfig, b.log)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		err := cr.close(ctx)
		if err != nil {
			b.log.WarnContext(ctx, "Error closing changefeed reader.")
		}
	}()

	if err := waitForHighWaterMark(ctx, cr); err != nil {
		return trace.Wrap(err, "waiting for high water mark")
	}

	b.buf.SetInit()
	defer b.buf.Reset()

	for {
		event, err := cr.readEvent(ctx)
		if err != nil {
			return trace.Wrap(err, "getting next changefeed event")
		}
		b.buf.Emit(event)
	}
}

// A high water mark is indicated by a change event with a resolve time. This
// ensures we are receiving the latest events for all ranges in the database.
// https://www.cockroachlabs.com/docs/stable/changefeed-messages#resolved-messages
func waitForHighWaterMark(ctx context.Context, cr *changeReader) error {
	for {
		c, err := cr.readChange(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		if c.Resolved == nil {
			continue
		}
		return nil
	}
}

// change represents a change from cockroachdb's change feed.
// https://www.cockroachlabs.com/docs/stable/changefeed-messages
type change struct {
	Keys     []string `json:"-"`
	After    *kv      `json:"after"`
	Resolved *string  `json:"resolved"`
}

// kv represents a [backend.Item] in the change feed.
type kv struct {
	Value    string     `json:"value"`
	Expires  *time.Time `json:"expires"`
	Revision uuid.UUID  `json:"revision"`
}

func newChangeReader(ctx context.Context, config *pgxpool.Config, log *slog.Logger) (_ *changeReader, err error) {
	ctx, cancel := context.WithCancel(ctx)

	// Give extra time for initial connect before going back to default timeout.
	watchdog := time.AfterFunc(defaultChangeFeedTimeout*2, sync.OnceFunc(func() {
		cancel()
		log.WarnContext(ctx, "Timeout waiting for changefeed progression.")
	}))
	context.AfterFunc(ctx, func() { watchdog.Stop() })

	defer func() {
		if err != nil {
			cancel()
		} else {
			watchdog.Reset(defaultChangeFeedTimeout)
		}
	}()

	connConfig := config.ConnConfig.Copy()
	if bc := config.BeforeConnect; bc != nil {
		if err = bc(ctx, connConfig); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defer func() {
		if err == nil {
			return
		}
		closeCtx, cancel := context.WithTimeout(ctx, defaultCloseTimeout)
		defer cancel()
		if err := conn.Close(closeCtx); err != nil {
			log.WarnContext(ctx, "Error closing changefeed connection.", "error", err)
		}
	}()

	if ac := config.AfterConnect; ac != nil {
		if err = ac(ctx, conn); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if _, err := conn.Exec(ctx, "SET CLUSTER SETTING kv.rangefeed.enabled = true", pgx.QueryExecModeExec); err != nil {
		log.WarnContext(ctx, "Failed to configure cluster settings kv.rangefeed.enabled = true;")
	}

	rr := conn.PgConn().ExecParams(
		ctx,
		"CREATE CHANGEFEED FOR kv WITH initial_scan='no', min_checkpoint_frequency='0s', resolved='5s'",
		nil /* paramValues */, nil /* paramOIDs */, nil, /* paramFormats */
		[]int16{pgtype.BinaryFormatCode, pgtype.BinaryFormatCode, pgtype.BinaryFormatCode},
	)

	return &changeReader{
		conn:     conn,
		rr:       rr,
		watchdog: watchdog,
		cancel:   cancel,
	}, nil
}

// changeReader handles reading changes and events from a crdb changefeed.
type changeReader struct {
	conn *pgx.Conn
	rr   *pgconn.ResultReader

	// watchdog calls cancel when the timer expires. Changes received from the
	// changefeed reset the timer. This ensures that the changefeed does not hang.
	watchdog *time.Timer
	// cancel cancels the context for the connection and rows.
	cancel context.CancelFunc
}

// readChange reads the next change from the changefeed.
func (r *changeReader) readChange(ctx context.Context) (change, error) {
	var c change
	if !r.rr.NextRow() {
		if _, err := r.rr.Close(); err != nil {
			return c, trace.Wrap(err)
		}
		return c, trace.Errorf("changefeed ended unexpectedly")
	}
	// Reset watchdog after successfully advancing to the next row.
	r.watchdog.Reset(defaultChangeFeedTimeout)
	vv := r.rr.Values()

	if err := json.Unmarshal(vv[2], &c); err != nil {
		return change{}, trace.Wrap(err)
	}
	if c.Resolved != nil {
		return c, nil
	}
	if len(vv[1]) == 0 {
		return change{}, trace.BadParameter("changefeed event missing key")
	}
	if err := json.Unmarshal(vv[1], &c.Keys); err != nil {
		return change{}, trace.Wrap(err)
	}
	return c, nil
}

// readEvent reads the next [backend.Event] from the changefeed.
func (r *changeReader) readEvent(ctx context.Context) (backend.Event, error) {
	var (
		event backend.Event
		c     change
		err   error
	)
	for {
		c, err = r.readChange(ctx)
		if err != nil {
			return event, trace.Wrap(err)
		}
		if c.Resolved == nil {
			break
		}
	}

	if len(c.Keys) != 1 {
		return event, trace.BadParameter("changefeed event missing key")
	}

	m := r.conn.TypeMap()
	if err := m.Scan(pgtype.ByteaOID, pgtype.TextFormatCode, []byte(c.Keys[0]), &event.Item.Key); err != nil {
		return backend.Event{}, trace.Wrap(err)
	}

	// A change with a nil after value indicates a delete.
	if c.After == nil {
		event.Type = types.OpDelete
		return event, nil
	}

	event.Type = types.OpPut
	if err := m.Scan(pgtype.ByteaOID, pgtype.TextFormatCode, []byte(c.After.Value), &event.Item.Value); err != nil {
		return backend.Event{}, trace.Wrap(err)
	}
	if c.After.Expires != nil {
		event.Item.Expires = *c.After.Expires
	}

	event.Item.Revision = c.After.Revision.String()
	return event, nil
}

func (r *changeReader) close(ctx context.Context) error {
	closeTimeout, cancel := context.WithTimeout(ctx, defaultCloseTimeout)
	defer cancel()
	r.watchdog.Stop()

	// Always close the connection before rows to prevent blocking.
	err := r.conn.Close(closeTimeout)
	_, rrErr := r.rr.Close()
	r.cancel()
	return trace.NewAggregate(err, rrErr)
}
