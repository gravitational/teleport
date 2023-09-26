package pgbk

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
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

	rows, err := conn.Query(ctx, "EXPERIMENTAL CHANGEFEED FOR kv WITH diff, no_initial_scan;")
	if err != nil {
		return err
	}
	defer rows.Close()

	b.buf.SetInit()
	defer b.buf.Reset()

	cf := &changeFeed{}
	for rows.Next() && ctx.Err() == nil {
		err := rows.Scan(&cf.Table, &cf.Key, &cf.Value)
		if err != nil {
			return trace.Wrap(err)
		}
		events, err := cf.events()
		if err != nil {
			return trace.Wrap(err)
		}
		for _, event := range events {
			b.log.Debugf("event for: %s", event.Item.Key)
		}
		b.buf.Emit(events...)
	}
	return nil
}

type changeFeed struct {
	Table string `json:"table"`
	Key   []byte `json:"key"`
	Value []byte `json:"value"`
}

func (cf changeFeed) events() ([]backend.Event, error) {
	var keys []HexData
	if err := json.Unmarshal(cf.Key, &keys); err != nil {
		return nil, trace.Wrap(err, "unable to unmarshal key")
	}
	e := backend.Event{
		Item: backend.Item{
			Key: []byte(keys[0]),
		},
	}
	c := change{}
	if err := json.Unmarshal(cf.Value, &c); err != nil {
		return nil, trace.Wrap(err, "unable to unmarshal value")
	}

	if c.After == nil {
		e.Type = types.OpDelete
		return []backend.Event{e}, nil
	}
	e.Type = types.OpPut
	item, err := c.After.toItem()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	e.Item = item
	if c.Before != nil && !reflect.DeepEqual(c.After.Key, c.Before.Key) {
		item, err := c.Before.toItem()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return []backend.Event{{
			Type: types.OpDelete,
			Item: item,
		}, e}, nil
	}

	return []backend.Event{e}, nil
}

type change struct {
	Before *kv `json:"before"`
	After  *kv `json:"after"`
}

type kv struct {
	Key      HexData `json:"key"`
	Value    HexData `json:"value"`
	Expires  *string `json:"expires"`
	Revision string  `json:"revision"`
}

type HexData []byte

func (h *HexData) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	s = strings.TrimPrefix(s, "\\x")
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	*h = HexData(decoded)
	return nil
}

func (kv *kv) toItem() (backend.Item, error) {
	item := backend.Item{
		Key:   kv.Key,
		Value: kv.Value,
	}

	if kv.Expires != nil {
		t, err := time.Parse("2006-01-02T15:04:05Z", *kv.Expires)
		if err != nil {
			return item, trace.Wrap(err)
		}
		item.Expires = t
	}
	var err error
	item.ID, err = idFromRevision2(kv.Revision)
	if err != nil {
		return item, trace.Wrap(err)
	}
	return item, nil
}

func idFromRevision2(rev string) (int64, error) {
	uuid, err := uuid.Parse(rev)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	u := binary.LittleEndian.Uint64(uuid[:])
	u &= 0x7fff_ffff_ffff_ffff
	return int64(u), nil
}
