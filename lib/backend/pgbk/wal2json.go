// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pgbk

import (
	"bytes"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5/pgtype/zeronull"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

type wal2jsonColumn struct {
	Name  string  `json:"name"`
	Type  string  `json:"type"`
	Value *string `json:"value"`
}

func (c *wal2jsonColumn) Bytea() ([]byte, error) {
	if c == nil {
		return nil, trace.BadParameter("missing column")
	}

	if c.Type != "bytea" {
		return nil, trace.BadParameter("expected bytea, got %q", c.Type)
	}

	if c.Value == nil {
		return nil, trace.BadParameter("expected bytea, got NULL")
	}

	b, err := hex.DecodeString(*c.Value)
	if err != nil {
		return nil, trace.Wrap(err, "parsing bytea")
	}

	return b, nil
}

func (c *wal2jsonColumn) Timestamptz() (time.Time, error) {
	if c == nil {
		return time.Time{}, trace.BadParameter("missing column")
	}

	if c.Type != "timestamp with time zone" {
		return time.Time{}, trace.BadParameter("expected timestamptz, got %q", c.Type)
	}

	if c.Value == nil {
		return time.Time{}, nil
	}

	var t zeronull.Timestamptz
	if err := t.Scan(*c.Value); err != nil {
		return time.Time{}, trace.Wrap(err, "parsing timestamptz")
	}

	return time.Time(t), nil
}

func (c *wal2jsonColumn) UUID() (uuid.UUID, error) {
	if c == nil {
		return uuid.Nil, trace.BadParameter("missing column")
	}

	if c.Type != "uuid" {
		return uuid.Nil, trace.BadParameter("expected uuid, got %q", c.Type)
	}

	if c.Value == nil {
		return uuid.Nil, trace.BadParameter("expected uuid, got NULL")
	}

	u, err := uuid.Parse(*c.Value)
	if err != nil {
		return uuid.Nil, trace.Wrap(err, "parsing uuid")
	}

	return u, nil
}

type wal2jsonMessage struct {
	Action string `json:"action"`
	Schema string `json:"schema"`
	Table  string `json:"table"`

	Columns  []wal2jsonColumn `json:"columns"`
	Identity []wal2jsonColumn `json:"identity"`
}

func (w *wal2jsonMessage) Events() ([]backend.Event, error) {
	switch w.Action {
	case "B", "C", "M":
		return nil, nil
	default:
		return nil, trace.BadParameter("unexpected action %q", w.Action)

	case "T":
		if w.Schema != "public" || w.Table != "kv" {
			return nil, nil
		}
		return nil, trace.BadParameter("received truncate for table kv")

	case "I":
		if w.Schema != "public" || w.Table != "kv" {
			return nil, nil
		}

		key, err := w.newCol("key").Bytea()
		if err != nil {
			return nil, trace.Wrap(err, "parsing key on insert")
		}
		value, err := w.newCol("value").Bytea()
		if err != nil {
			return nil, trace.Wrap(err, "parsing value on insert")
		}
		expires, err := w.newCol("expires").Timestamptz()
		if err != nil {
			return nil, trace.Wrap(err, "parsing expires on insert")
		}
		revision, err := w.newCol("revision").UUID()
		if err != nil {
			return nil, trace.Wrap(err, "parsing revision on insert")
		}

		return []backend.Event{{
			Type: types.OpPut,
			Item: backend.Item{
				Key:     key,
				Value:   value,
				Expires: expires.UTC(),
				ID:      idFromRevision(revision),
			},
		}}, nil

	case "D":
		if w.Schema != "public" || w.Table != "kv" {
			return nil, nil
		}

		key, err := w.oldCol("key").Bytea()
		if err != nil {
			return nil, trace.Wrap(err, "parsing key on delete")
		}
		return []backend.Event{{
			Type: types.OpDelete,
			Item: backend.Item{
				Key: key,
			},
		}}, nil

	case "U":
		if w.Schema != "public" || w.Table != "kv" {
			return nil, nil
		}

		// on an UPDATE, an unmodified TOASTed column might be missing from
		// "columns", but it should be present in "identity" (and this also
		// applies to "key"), so we use the toastCol accessor function
		keyCol, oldKeyCol := w.toastCol("key"), w.oldCol("key")
		key, err := keyCol.Bytea()
		if err != nil {
			return nil, trace.Wrap(err, "parsing key on update")
		}
		var oldKey []byte
		// this check lets us skip a second hex parsing and a comparison (on a
		// big enough key to be TOASTed, so it's worth it)
		if oldKeyCol != keyCol {
			oldKey, err = oldKeyCol.Bytea()
			if err != nil {
				return nil, trace.Wrap(err, "parsing old key on update")
			}
			if bytes.Equal(oldKey, key) {
				oldKey = nil
			}
		}
		value, err := w.toastCol("value").Bytea()
		if err != nil {
			return nil, trace.Wrap(err, "parsing value on update")
		}
		expires, err := w.toastCol("expires").Timestamptz()
		if err != nil {
			return nil, trace.Wrap(err, "parsing expires on update")
		}
		revision, err := w.toastCol("revision").UUID()
		if err != nil {
			return nil, trace.Wrap(err, "parsing revision on update")
		}

		if oldKey != nil {
			return []backend.Event{{
				Type: types.OpDelete,
				Item: backend.Item{
					Key: oldKey,
				},
			}, {
				Type: types.OpPut,
				Item: backend.Item{
					Key:     key,
					Value:   value,
					Expires: expires.UTC(),
					ID:      idFromRevision(revision),
				},
			}}, nil
		}

		return []backend.Event{{
			Type: types.OpPut,
			Item: backend.Item{
				Key:     key,
				Value:   value,
				Expires: expires.UTC(),
				ID:      idFromRevision(revision),
			},
		}}, nil
	}
}

func (w *wal2jsonMessage) newCol(name string) *wal2jsonColumn {
	for i := range w.Columns {
		if w.Columns[i].Name == name {
			return &w.Columns[i]
		}
	}
	return nil
}

func (w *wal2jsonMessage) oldCol(name string) *wal2jsonColumn {
	for i := range w.Identity {
		if w.Identity[i].Name == name {
			return &w.Identity[i]
		}
	}
	return nil
}

func (w *wal2jsonMessage) toastCol(name string) *wal2jsonColumn {
	if c := w.newCol(name); c != nil {
		return c
	}
	return w.oldCol(name)
}
