package pgbk

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (b *Backend) backgroundChangeFeed2(ctx context.Context) {
	defer b.log.Info("Exited change feed loop.")
	defer b.buf.Close()

	for ctx.Err() == nil {
		b.log.Info("Starting change feed stream.")
		err := b.runChangeFeed2(ctx)
		if ctx.Err() != nil {
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

func (b *Backend) runChangeFeed2(ctx context.Context) error {
	connConfig := b.feedConfig.ConnConfig.Copy()
	if bc := b.feedConfig.BeforeConnect; bc != nil {
		if err := bc(ctx, connConfig); err != nil {
			return trace.Wrap(err)
		}
	}
	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := conn.Close(closeCtx); err != nil && closeCtx.Err() != nil {
			b.log.WithError(err).Warn("Error closing change feed connection.")
		}
	}()
	if ac := b.feedConfig.AfterConnect; ac != nil {
		if err := ac(ctx, conn); err != nil {
			return trace.Wrap(err)
		}
	}
	_, err = conn.Exec(ctx, "SET CLUSTER SETTING kv.rangefeed.enabled = true;")
	if err != nil {
		return trace.Wrap(err)
	}

	rows, err := conn.Query(ctx, "EXPERIMENTAL CHANGEFEED FOR kv WITH diff;")
	if err != nil {
		return err
	}
	defer rows.Close()

	b.buf.SetInit()
	defer b.buf.Reset()

	m := conn.TypeMap()
	for rows.Next() && ctx.Err() == nil {
		events, err := scanEvents(m, rows)
		if err != nil {
			return trace.Wrap(err, "scan events")
		}
		for _, event := range events {
			// TODO(david): Remove debug logging here.
			b.log.Debugf("event: %s", event.Item.Key)
		}
		b.buf.Emit(events...)
	}
	return nil
}

func scanEvents(m *pgtype.Map, rows pgx.Rows) ([]backend.Event, error) {
	var key, value []byte
	err := rows.Scan(nil, &key, &value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var keys []string
	if err := json.Unmarshal(key, &keys); err != nil {
		return nil, trace.Wrap(err, "unmarshal keys")
	}
	if len(keys) != 1 {
		return nil, trace.Wrap(err, "missing key")
	}

	err = m.Scan(pgtype.ByteaOID, pgtype.TextFormatCode, []byte(keys[0]), &key)
	if err != nil {
		return nil, trace.Wrap(err, "parse key")
	}

	c := change{}
	if err := json.Unmarshal(value, &c); err != nil {
		return nil, trace.Wrap(err, "unable to unmarshal value")
	}
	event := backend.Event{
		Item: backend.Item{
			Key: key,
		},
	}
	if c.After == nil {
		event.Type = types.OpDelete
		return []backend.Event{event}, nil
	}

	event.Type = types.OpPut
	if err := m.Scan(pgtype.ByteaOID, pgtype.TextFormatCode, []byte(c.After.Key), &event.Item.Key); err != nil {
		return nil, trace.Wrap(err, "parse after key")
	}
	if err := m.Scan(pgtype.ByteaOID, pgtype.TextFormatCode, []byte(c.After.Value), &event.Item.Value); err != nil {
		return nil, trace.Wrap(err, "parse after value")
	}
	if c.After.Expires != nil {
		// TODO(david): Parse time with more precision.
		event.Item.Expires, err = time.Parse("2006-01-02T15:04:05Z", *c.After.Expires)
		if err != nil {
			return nil, trace.Wrap(err, "parse after expires")
		}
	}

	var revision string
	if err := m.Scan(pgtype.UUIDOID, pgtype.TextFormatCode, []byte(c.After.Revision), &revision); err != nil {
		return nil, trace.Wrap(err, "scanning revision on put")
	}
	event.Item.ID, err = idFromRevisionString(revision)
	if err != nil {
		return nil, trace.Wrap(err, "parse id from revision string")
	}

	// TODO(david): not sure if this is even possible.
	if !reflect.DeepEqual(event.Item.Key, key) {
		return []backend.Event{{
			Type: types.OpDelete,
			Item: backend.Item{
				Key: key,
			},
		}, event}, nil
	}
	return []backend.Event{event}, nil
}

type change struct {
	Before *kv `json:"before"`
	After  *kv `json:"after"`
}

type kv struct {
	Key      string  `json:"key"`
	Value    string  `json:"value"`
	Expires  *string `json:"expires"`
	Revision string  `json:"revision"`
}

func idFromRevisionString(rev string) (int64, error) {
	uuid, err := uuid.Parse(rev)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	u := binary.LittleEndian.Uint64(uuid[:])
	u &= 0x7fff_ffff_ffff_ffff
	return int64(u), nil
}
