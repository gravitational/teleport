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
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/backend"
)

func (b *Backend) runReplicationChangeFeed(ctx context.Context, config *pgconn.Config) error {
	conn, err := pgconn.ConnectConfig(ctx, config)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close(context.Background())
	defer func() { conn.Conn().Close() }()

	b.log.Info("Connected, starting change feed.")

	slotName, err := startPgoutputReplication(ctx, conn)
	if err != nil {
		return trace.Wrap(err)
	}

	b.log.WithField("slot_name", slotName).Info("Change feed started.")

	b.buf.SetInit()
	defer b.buf.Reset()

	const standbyInterval = 10 * time.Second
	sendDeadline := time.Now().Add(standbyInterval)
	recvDeadline := time.Now().Add(3 * standbyInterval)

	var walPosition pglogrepl.LSN
	var parser PgoutputParser
	m := pgtype.NewMap()

	var eventsEmitted int
	emit := func(events ...backend.Event) bool {
		eventsEmitted += len(events)
		return b.buf.Emit(events...)
	}

	for {
		if time.Now().After(recvDeadline) {
			return trace.ConnectionProblem(nil, "timed out waiting for message")
		}

		if time.Now().After(sendDeadline) {
			b.log.WithFields(logrus.Fields{
				"wal_position":   walPosition.String(),
				"events_emitted": eventsEmitted,
			}).Info("Sending status update.")
			eventsEmitted = 0

			if err := pglogrepl.SendStandbyStatusUpdate(ctx, conn, pglogrepl.StandbyStatusUpdate{
				WALWritePosition: walPosition,
				ReplyRequested:   time.Until(recvDeadline) < 2*standbyInterval,
			}); err != nil {
				return trace.Wrap(err, "sending status update")
			}
			sendDeadline = time.Now().Add(standbyInterval)
		}

		msg, err := receiveMessageDeadline(ctx, conn, sendDeadline.Add(time.Second))
		if _, isTimeout := err.(*receiveMessageDeadlineErr); isTimeout {
			sendDeadline = time.Time{}
			continue
		} else if err != nil {
			return trace.Wrap(err)
		}

		recvDeadline = time.Now().Add(3 * standbyInterval)

		switch msg := msg.(type) {
		default:
			return trace.BadParameter("waiting for CopyData: unexpected %T", msg)
		case *pgproto3.NoticeResponse,
			*pgproto3.ParameterStatus,
			*pgproto3.NotificationResponse:
			continue
		case *pgproto3.ErrorResponse:
			return trace.Wrap(pgconn.ErrorResponseToPgError(msg), "waiting for CopyData")
		case *pgproto3.CopyData:
			if len(msg.Data) == 0 {
				return trace.BadParameter("unexpected empty replication message")
			}
			switch byteID, payload := msg.Data[0], msg.Data[1:]; byteID {
			default:
				return trace.BadParameter("unexpected replication message %q", byteID)

			case pglogrepl.PrimaryKeepaliveMessageByteID:
				pkm, err := pglogrepl.ParsePrimaryKeepaliveMessage(payload)
				if err != nil {
					return trace.Wrap(err, "parsing primary keepalive message")
				}
				walPosition = max(walPosition, pkm.ServerWALEnd)
				if pkm.ReplyRequested {
					b.log.WithField("server_wal_position", pkm.ServerWALEnd.String()).
						Debug("Received keepalive requesting reply.")
					sendDeadline = time.Time{}
				}

			case pglogrepl.XLogDataByteID:
				xld, err := pglogrepl.ParseXLogData(payload)
				if err != nil {
					return trace.Wrap(err, "parsing xlogdata message")
				}
				walPosition = max(walPosition, xld.ServerWALEnd)
				if err := parser.Parse(xld.WALData, m, emit); err != nil {
					return trace.Wrap(err, "parsing pgoutput payload")
				}
			}
		}
	}
}

func receiveMessageDeadline(ctx context.Context, conn *pgconn.PgConn, deadline time.Time) (pgproto3.BackendMessage, error) {
	deadlineCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	msg, err := conn.ReceiveMessage(deadlineCtx)
	if err != nil {
		if pgconn.Timeout(err) && deadlineCtx.Err() != nil && ctx.Err() == nil {
			return nil, (*receiveMessageDeadlineErr)(nil)
		}
		return nil, trace.Wrap(err)
	}
	return msg, nil
}

type receiveMessageDeadlineErr struct{}

func (*receiveMessageDeadlineErr) Error() string { return "receiveMessageDeadlineErr" }

func startPgoutputReplication(ctx context.Context, conn *pgconn.PgConn) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_ = conn.Exec(ctx, "CREATE PUBLICATION kv_pub FOR TABLE kv;").Close()

	slotName := fmt.Sprintf("%x", [16]byte(uuid.New()))
	if err := conn.Exec(ctx, fmt.Sprintf(
		"CREATE_REPLICATION_SLOT %v TEMPORARY LOGICAL pgoutput NOEXPORT_SNAPSHOT",
		pgx.Identifier{slotName}.Sanitize(),
	)).Close(); err != nil {
		return "", trace.Wrap(err, "creating slot")
	}

	conn.Frontend().SendQuery(&pgproto3.Query{String: fmt.Sprintf(
		"START_REPLICATION SLOT %v LOGICAL 0/0 "+
			"(proto_version '1', publication_names 'kv_pub', binary 'true')",
		pgx.Identifier{slotName}.Sanitize(),
	)})
	if err := conn.Frontend().Flush(); err != nil {
		return "", trace.Wrap(err, "starting replication")
	}

	for {
		msg, err := conn.ReceiveMessage(ctx)
		if err != nil {
			return "", trace.Wrap(err, "receiving CopyBothResponse")
		}

		switch msg := msg.(type) {
		case *pgproto3.NoticeResponse,
			*pgproto3.ParameterStatus,
			*pgproto3.NotificationResponse:
		case *pgproto3.ErrorResponse:
			return "", trace.Wrap(pgconn.ErrorResponseToPgError(msg), "waiting for CopyBothResponse")
		default:
			return "", trace.BadParameter("waiting for CopyBothResponse: unexpected %T", msg)
		case *pgproto3.CopyBothResponse:
			return slotName, nil
		}
	}
}
