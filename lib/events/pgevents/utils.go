// Copyright 2022 Gravitational, Inc
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

package pgevents

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// queryBuilder is a dynamic SQL query builder.
type queryBuilder struct {
	builder strings.Builder
	args    []any
}

// Append adds a chunk of text to the query, replacing %v with consecutive
// placeholder strings. It's also possible to use positional format specifiers
// such as %[2]v to specify the same placeholder multiple times.
func (q *queryBuilder) Append(s string, args ...any) {
	fmtArgs := make([]any, 0, len(args))
	for _, a := range args {
		q.args = append(q.args, a)
		fmtArgs = append(fmtArgs, fmt.Sprintf("$%v", len(q.args)))
	}

	fmt.Fprintf(&q.builder, s, fmtArgs...)
}

// String returns the text of the query.
func (q *queryBuilder) String() string {
	return q.builder.String()
}

// Args returns the arguments representing the
func (q *queryBuilder) Args() []any {
	return q.args
}

// connectPostgres will open a single connection to the "postgres" database in
// the database cluster specified in poolConfig.
func connectPostgres(ctx context.Context, poolConfig *pgxpool.Config) (*pgx.Conn, error) {
	connConfig := poolConfig.ConnConfig.Copy()
	connConfig.Database = "postgres"

	if poolConfig.BeforeConnect != nil {
		if err := poolConfig.BeforeConnect(ctx, connConfig); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if poolConfig.AfterConnect != nil {
		if err := poolConfig.AfterConnect(ctx, conn); err != nil {
			conn.Close(ctx)
			return nil, trace.Wrap(err)
		}
	}

	return conn, nil
}

// keyset is a point at which the searchEvents pagination ended, and can be
// resumed from.
type keyset struct {
	t   time.Time
	sid uuid.UUID
	ei  int64
	id  uuid.UUID
}

// FromKey attempts to parse a keyset from a string. The string is a URL-safe
// base64 encoding of the time in microseconds as an int64, the session id, the
// event index as an int64, and the event UUID; numbers are encoded in
// little-endian.
func (ks *keyset) FromKey(key string) error {
	if key == "" {
		return nil
	}

	b, err := base64.URLEncoding.DecodeString(key)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(b) != 48 {
		return trace.BadParameter("malformed pagination key")
	}

	ks.t = time.UnixMicro(int64(binary.LittleEndian.Uint64(b[0:8]))).UTC()
	ks.sid, _ = uuid.FromBytes(b[8:24])
	ks.ei = int64(binary.LittleEndian.Uint64(b[24:32]))
	ks.id, _ = uuid.FromBytes(b[32:48])

	return nil
}

// ToKey converts the keyset into a URL-safe string.
func (ks *keyset) ToKey() string {
	var b [48]byte
	binary.LittleEndian.PutUint64(b[0:8], uint64(ks.t.UnixMicro()))
	copy(b[8:24], ks.sid[:])
	binary.LittleEndian.PutUint64(b[24:32], uint64(ks.ei))
	copy(b[32:48], ks.id[:])
	return base64.URLEncoding.EncodeToString(b[:])
}

// isCode checks if the passed error is a Postgres error with the given code.
func isCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}
