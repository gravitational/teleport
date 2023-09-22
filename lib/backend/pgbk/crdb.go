package pgbk

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

func (b *Backend) runChangeFeedCRDB(ctx context.Context) (err error) {
	if ac := b.feedConfig.AfterConnect; ac != nil {
		return trace.BadParameter("this configuration is unsupported on CRDB")
	}
	connConfig := b.feedConfig.ConnConfig.Copy()
	if bc := b.feedConfig.BeforeConnect; bc != nil {
		if err := bc(ctx, connConfig); err != nil {
			return trace.Wrap(err)
		}
	}
	conn, err := pgconn.ConnectConfig(ctx, &connConfig.Config)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close(ctx)
	defer conn.Conn().Close()

	const watchdogTimeout = 10 * time.Second
	watchdog := time.AfterFunc(watchdogTimeout, func() {
		conn.Conn().Close()
	})
	defer func() {
		if !watchdog.Stop() {
			err = context.DeadlineExceeded
		}
	}()

	m := pgtype.NewMap()

	rr := conn.ExecParams(ctx,
		"CREATE CHANGEFEED FOR TABLE kv "+
			"WITH initial_scan = 'no', resolved, min_checkpoint_frequency = '5s'",
		nil, nil, nil,
		[]int16{pgtype.BinaryFormatCode, pgtype.BinaryFormatCode, pgtype.BinaryFormatCode})

	var init bool
	for rr.NextRow() {
		watchdog.Reset(watchdogTimeout)

		if !init {
			b.log.Info("Change feed started.")
			b.buf.SetInit()
			defer b.buf.Reset()
			init = true
		}

		vv := rr.Values()

		if string(vv[0]) != "kv" {
			continue
		}

		var row struct {
			After *struct {
				Key      string  `json:"key"`
				Value    string  `json:"value"`
				Expires  *string `json:"expires"`
				Revision string  `json:"revision"`
			} `json:"after"`
		}
		if err := json.Unmarshal(vv[2], &row); err != nil {
			return trace.Wrap(err)
		}

		if row.After == nil {
			var keys []string
			if err := json.Unmarshal(vv[1], &keys); err != nil {
				return trace.Wrap(err, "parsing keys on delete")
			}
			if len(keys) != 1 {
				return trace.BadParameter("missing key on delete")
			}

			var key []byte
			if err := m.Scan(pgtype.ByteaOID, pgtype.TextFormatCode, []byte(keys[0]), &key); err != nil {
				return trace.Wrap(err, "scanning key on delete")
			}

			b.buf.Emit(backend.Event{
				Type: types.OpDelete,
				Item: backend.Item{
					Key: key,
				},
			})
			continue
		}

		var key, value []byte
		var expires time.Time
		var revision revision

		if err := m.Scan(pgtype.ByteaOID, pgtype.TextFormatCode, []byte(row.After.Key), &key); err != nil {
			return trace.Wrap(err, "scanning key on put")
		}
		if err := m.Scan(pgtype.ByteaOID, pgtype.TextFormatCode, []byte(row.After.Value), &value); err != nil {
			return trace.Wrap(err, "scanning value on put")
		}
		if row.After.Expires != nil {
			expires, err = time.Parse(time.RFC3339Nano, *row.After.Expires)
			if err != nil {
				return trace.Wrap(err, "scanning expires on put")
			}
		}
		if err := m.Scan(pgtype.UUIDOID, pgtype.TextFormatCode, []byte(row.After.Revision), &revision); err != nil {
			return trace.Wrap(err, "scanning revision on put")
		}

		b.buf.Emit(backend.Event{
			Type: types.OpPut,
			Item: backend.Item{
				Key:      key,
				Value:    value,
				Expires:  expires,
				Revision: revisionToString(revision),
			},
		})
	}

	_, err = rr.Close()
	return trace.Wrap(err)
}
