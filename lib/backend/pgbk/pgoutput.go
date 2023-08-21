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

	"github.com/gravitational/trace"
	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgtype/zeronull"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

// PgoutputParser parses the output data from the pgoutput logical decoding
// plugin.
type PgoutputParser struct {
	// kvRelationID is the object ID of the kv table, as communicated via
	// relation messages. Zero is an invalid relation OID in Postgres, so the
	// value is zero iff the OID is unknown.
	kvRelationID uint32
}

// Parse parses the data as a single logical decoding message, emitting events
// through the emit function as they're parsed. If an error is returned, the
// replication stream should be restarted.
func (p *PgoutputParser) Parse(data []byte, m *pgtype.Map, emit func(...backend.Event) bool) error {
	if len(data) == 0 {
		return trace.BadParameter("got empty message")
	}

	// skip parsing messages that we don't care about or that we don't expect
	switch pglogrepl.MessageType(data[0]) {
	case pglogrepl.MessageTypeBegin,
		pglogrepl.MessageTypeCommit,
		pglogrepl.MessageTypeOrigin,
		pglogrepl.MessageTypeType,
		pglogrepl.MessageTypeMessage:
		return nil

	default:
		return trace.BadParameter("got unexpected message type %q", data[0])

	case pglogrepl.MessageTypeRelation,
		pglogrepl.MessageTypeInsert,
		pglogrepl.MessageTypeUpdate,
		pglogrepl.MessageTypeDelete,
		pglogrepl.MessageTypeTruncate:
	}

	msg, err := pglogrepl.Parse(data)
	if err != nil {
		return trace.Wrap(err)
	}

	switch msg := msg.(type) {
	default:
		return trace.BadParameter("got unexpected message type %T", msg)

	case *pglogrepl.RelationMessage:
		if msg.Namespace != "public" || msg.RelationName != "kv" {
			return nil
		}

		if msg.ReplicaIdentity != 'f' {
			return trace.BadParameter("expected replica identity full, got %q", msg.ReplicaIdentity)
		}
		if len(msg.Columns) != 4 {
			return trace.BadParameter("expected 4 columns, got %v", len(msg.Columns))
		}

		if c := msg.Columns[0]; c.Name != "key" || c.DataType != pgtype.ByteaOID ||
			c.Flags != 1 || c.TypeModifier != -1 {
			return trace.BadParameter("bad key column %+v", c)
		}
		if c := msg.Columns[1]; c.Name != "value" || c.DataType != pgtype.ByteaOID ||
			c.Flags != 1 || c.TypeModifier != -1 {
			return trace.BadParameter("bad value column %+v", c)
		}
		if c := msg.Columns[2]; c.Name != "expires" || c.DataType != pgtype.TimestamptzOID ||
			c.Flags != 1 || c.TypeModifier != -1 {
			return trace.BadParameter("bad expires column %+v", c)
		}
		if c := msg.Columns[3]; c.Name != "revision" || c.DataType != pgtype.UUIDOID ||
			c.Flags != 1 || c.TypeModifier != -1 {
			return trace.BadParameter("bad revision column %+v", c)
		}

		p.kvRelationID = msg.RelationID

	case *pglogrepl.InsertMessage:
		if msg.RelationID != p.kvRelationID {
			return nil
		}
		events, err := tuplesToEvents(m, msg.Tuple, nil)
		if err != nil {
			return trace.Wrap(err)
		}
		emit(events...)

	case *pglogrepl.UpdateMessage:
		if msg.RelationID != p.kvRelationID {
			return nil
		}
		if msg.OldTupleType != pglogrepl.UpdateMessageTupleTypeOld {
			return trace.BadParameter("expected old tuple, got %q", msg.OldTupleType)
		}
		events, err := tuplesToEvents(m, msg.NewTuple, msg.OldTuple)
		if err != nil {
			return trace.Wrap(err)
		}
		emit(events...)

	case *pglogrepl.DeleteMessage:
		if msg.RelationID != p.kvRelationID {
			return nil
		}
		if msg.OldTupleType != pglogrepl.UpdateMessageTupleTypeOld {
			return trace.BadParameter("expected old tuple, got %q", msg.OldTupleType)
		}
		events, err := tuplesToEvents(m, nil, msg.OldTuple)
		if err != nil {
			return trace.Wrap(err)
		}
		emit(events...)

	case *pglogrepl.TruncateMessage:
		if slices.Contains(msg.RelationIDs, p.kvRelationID) {
			return trace.BadParameter("received unexpected truncate on kv")
		}
	}

	return nil
}

func scanColNull(m *pgtype.Map, oid uint32, src *pglogrepl.TupleDataColumn, dst any) error {
	switch src.DataType {
	default:
		return trace.BadParameter("unexpected data type %q", src.DataType)
	case pglogrepl.TupleDataTypeBinary:
		return trace.Wrap(m.Scan(oid, pgtype.BinaryFormatCode, src.Data, dst))
	case pglogrepl.TupleDataTypeText:
		return trace.Wrap(m.Scan(oid, pgtype.TextFormatCode, src.Data, dst))
	case pglogrepl.TupleDataTypeNull:
		return trace.Wrap(m.Scan(oid, pgtype.TextFormatCode, nil, dst))
	}
}

func scanColNonNull(m *pgtype.Map, oid uint32, src *pglogrepl.TupleDataColumn, dst any) error {
	if src.DataType == pglogrepl.TupleDataTypeNull {
		return trace.BadParameter("unexpected null column")
	}
	return trace.Wrap(scanColNull(m, oid, src, dst))
}

func tuplesToEvents(m *pgtype.Map, tuple, oldTuple *pglogrepl.TupleData) ([]backend.Event, error) {
	// TODO(espadolini): handle key-only tuple for v13
	if tuple != nil && len(tuple.Columns) != 4 {
		return nil, trace.BadParameter("expected 4 columns, got %v", len(tuple.Columns))
	}
	if oldTuple != nil && len(oldTuple.Columns) != 4 {
		return nil, trace.BadParameter("expected 4 columns, got %v", len(tuple.Columns))
	}

	events := make([]backend.Event, 0, 2)

	if oldTuple != nil {
		events = append(events, backend.Event{Type: types.OpDelete})
		item := &events[len(events)-1].Item

		if err := scanColNonNull(m, pgtype.ByteaOID,
			oldTuple.Columns[0], (*pgtype.DriverBytes)(&item.Key),
		); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if tuple == nil {
		return events, nil
	}

	events = append(events, backend.Event{Type: types.OpPut})
	item := &events[len(events)-1].Item

	col := func(idx int) *pglogrepl.TupleDataColumn {
		c := tuple.Columns[idx]
		if c.DataType == pglogrepl.TupleDataTypeToast && oldTuple != nil {
			c = oldTuple.Columns[idx]
		}
		return c
	}

	if err := scanColNonNull(m, pgtype.ByteaOID,
		col(0), (*pgtype.DriverBytes)(&item.Key),
	); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := scanColNonNull(m, pgtype.ByteaOID,
		col(1), (*pgtype.DriverBytes)(&item.Value),
	); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := scanColNull(m, pgtype.TimestamptzOID,
		col(2), (*zeronull.Timestamptz)(&item.Expires),
	); err != nil {
		return nil, trace.Wrap(err)
	}
	var revision revision
	if err := scanColNonNull(m, pgtype.UUIDOID,
		col(3), &revision,
	); err != nil {
		return nil, trace.Wrap(err)
	}
	item.ID = idFromRevision(revision)
	item.Revision = revisionToString(revision)

	if oldTuple != nil && bytes.Equal(events[0].Item.Key, events[1].Item.Key) {
		events[0].Item.Key = nil
		events = events[1:]

	}
	return events, nil
}
