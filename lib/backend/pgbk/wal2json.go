/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package pgbk

import (
	"bytes"
	"encoding/hex"
	"strings"
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

	Columns  []wal2jsonColumn `json:"columns"`
	Identity []wal2jsonColumn `json:"identity"`
}

func (w *wal2jsonMessage) Events() ([]backend.Event, error) {
	switch w.Action {
	case "B", "C", "M":
		return nil, nil
	case "T":
		return nil, trace.BadParameter("received truncate for table kv")
	case "I":
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
				Key:      backend.KeyFromString(string(key)),
				Value:    value,
				Expires:  expires.UTC(),
				Revision: revisionToString(revision),
			},
		}}, nil
	case "D":
		key, err := w.oldCol("key").Bytea()
		if err != nil {
			return nil, trace.Wrap(err, "parsing key on delete")
		}
		return []backend.Event{{
			Type: types.OpDelete,
			Item: backend.Item{
				Key: backend.KeyFromString(string(key)),
			},
		}}, nil
	case "U":
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
					Key: backend.KeyFromString(string(oldKey)),
				},
			}, {
				Type: types.OpPut,
				Item: backend.Item{
					Key:      backend.KeyFromString(string(key)),
					Value:    value,
					Expires:  expires.UTC(),
					Revision: revisionToString(revision),
				},
			}}, nil
		}

		return []backend.Event{{
			Type: types.OpPut,
			Item: backend.Item{
				Key:      backend.KeyFromString(string(key)),
				Value:    value,
				Expires:  expires.UTC(),
				Revision: revisionToString(revision),
			},
		}}, nil
	default:
		return nil, trace.BadParameter("unexpected action %q", w.Action)
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

// wal2jsonEscape turns a schema or table name into a form suitable for use in
// wal2json's filter-tables or add-tables option, by prepending a backslash to
// each character.
func wal2jsonEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteRune('\\')
		b.WriteRune(r)
	}
	return b.String()
}
