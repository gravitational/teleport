package pgbk

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v4"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

func (b *Backend) backgroundExpiry(ctx context.Context) {
	defer b.wg.Done()
	defer b.log.Info("Exited expiry loop.")

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}

		t0 := time.Now()
		var r int64
		if err := b.beginTxFunc(ctx, txReadWrite, func(tx pgx.Tx) error {
			tag, err := tx.Exec(ctx,
				"DELETE FROM kv WHERE expires IS NOT NULL AND expires <= $1",
				time.Now().UTC(),
			)
			if err != nil {
				return trace.Wrap(err)
			}
			r = tag.RowsAffected()
			return nil
		}); err != nil {
			b.log.WithError(err).Error("Failed to delete expired items.")
		} else {
			if r > 0 {
				b.log.WithFields(logrus.Fields{"deleted": r, "elapsed": time.Since(t0).String()}).Debug("Deleted expired items.")
			}
		}
	}
}

func (b *Backend) backgroundChangefeed(ctx context.Context) {
	defer b.wg.Done()
	defer b.log.Info("Exited change feed loop.")
	defer b.buf.Close()

	for {
		b.log.Info("Starting change feed stream.")
		err := b.streamChanges(ctx)
		if err == nil {
			break
		}
		b.log.WithError(err).Error("Change feed stream lost.")

		select {
		case <-ctx.Done():
			return
		case <-time.After(defaults.HighResPollingPeriod):
		}
	}
}

// streamChanges will connect to the database and start generating change
// events. Assumes that b.buf is not initialized but not closed, and will reset
// it before returning. It will not close b.buf .
func (b *Backend) streamChanges(ctx context.Context) error {
	poolConn, err := b.pool.Acquire(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	// we hijack the connection from the pool because the temporary replication
	// slot is tied to the connection, so we want it to be cleaned up no matter
	// what happens here
	conn := poolConn.Hijack()
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := conn.Close(ctx); err != nil {
			b.log.WithError(err).Warn("Error closing stream connection.")
		}
	}()

	slotUUID := uuid.New()
	slotName := hex.EncodeToString(slotUUID[:])

	b.log.WithField("slot_name", slotName).Info("Setting up change stream.")
	if _, err := conn.Exec(ctx,
		"SELECT * FROM pg_create_logical_replication_slot($1, 'wal2json', true)", slotName); err != nil {
		return trace.Wrap(err)
	}

	// prime the change stream with a message so we can initialize the buffer
	// even if there's no actual backend activity
	if _, err := conn.Exec(ctx,
		"SELECT * FROM pg_logical_emit_message(false, $1, 'init')",
		slotName,
	); err != nil {
		return trace.Wrap(err)
	}

	initialized := false
	defer func() {
		if initialized {
			b.buf.Reset()
		}
	}()

	for {
		var j string
		tag, err := conn.QueryFunc(ctx, `
			SELECT data FROM pg_logical_slot_get_changes($1::text, NULL, NULL,
				'format-version', '2', 'add-tables', 'public.kv', 'add-msg-prefixes', $1::text,
				'include-types', 'false', 'include-transaction', 'false')`,
			[]any{slotName}, []any{&j}, func(pgx.QueryFuncRow) error {
				if !initialized {
					b.buf.SetInit()
					initialized = true
				}

				if err := b.parseChange(j); err != nil {
					return trace.Wrap(err)
				}

				return nil
			})
		if err != nil {
			return trace.Wrap(err)
		}

		if tag.RowsAffected() > 0 {
			b.log.WithField("events", tag.RowsAffected()).Error("Fetched change feed events.")
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backend.DefaultPollStreamPeriod):
		}
	}
}

type Action string

const (
	Insert   Action = "I"
	Update   Action = "U"
	Delete   Action = "D"
	Message  Action = "M"
	Truncate Action = "T"
	Begin    Action = "B"
	Commit   Action = "C"
)

type Column struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Columns []Column

func (c Columns) ToEvent(ty types.OpType) backend.Event {
	ev := backend.Event{
		Type: ty,
	}
	for _, col := range c {
		switch col.Name {
		case "key":
			ev.Item.Key, _ = hex.DecodeString(col.Value)
		case "value":
			ev.Item.Value, _ = hex.DecodeString(col.Value)
		case "expires":
			if len(col.Value) != 0 {
				const pgTimestampFormat = "2006-01-02 15:04:05.999999999"
				ev.Item.Expires, _ = time.Parse(pgTimestampFormat, col.Value)
			}
		}
	}
	return ev
}

type Wal2jsonMessage struct {
	Action   Action  `json:"action"`
	Columns  Columns `json:"columns"`
	Identity Columns `json:"identity"`
}

func (b *Backend) parseChange(j string) error {
	var m Wal2jsonMessage
	if err := utils.FastUnmarshal([]byte(j), &m); err != nil {
		return trace.Wrap(err)
	}

	switch m.Action {
	case Insert, Update:
		ev := m.Columns.ToEvent(types.OpPut)
		b.log.WithFields(logrus.Fields{
			"key":     string(ev.Item.Key),
			"expires": ev.Item.Expires,
		}).Debug("Emitting Put event.")
		b.buf.Emit(ev)
	case Delete:
		ev := m.Identity.ToEvent(types.OpDelete)
		b.log.WithFields(logrus.Fields{
			"key":     string(ev.Item.Key),
			"expires": ev.Item.Expires,
		}).Debug("Emitting Delete event.")
		b.buf.Emit(ev)
	case Message:
		b.log.WithField("message", j).Debug("Got WAL message.")
	case Begin, Commit:
		b.log.WithField("message", j).Debug("Got BEGIN/COMMIT in change feed, this is a bug (but harmless).")
	case Truncate:
		b.log.WithField("message", j).Error("Witnessed truncate operation, this shouldn't happen.")
		return trace.BadParameter("received truncate WAL message")
	default:
		b.log.WithField("message", j).Error("Received unknown message, this shouldn't happen.")
		return trace.BadParameter("received unknown WAL message %q", m.Action)
	}

	return nil
}
