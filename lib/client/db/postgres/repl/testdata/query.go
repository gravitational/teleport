// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package testdata

import (
	"github.com/jackc/pgconn"
	"github.com/jackc/pgproto3/v2"
)

// Contains a query result with the most common fields in PostgreSQL.
// This can be used to understand how the REPL deals with different data types.
//
// Sampled from https://github.com/postgres/postgres/blob/b6612aedc53a6bf069eba5e356a8421ad6426486/src/include/catalog/pg_type.dat
// PostgreSQL version 17.2
var TestDataQueryResult = []pgproto3.BackendMessage{
	&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{
		// TableOID and TableAttributeNumber values omitted.
		{Name: []byte("serial_col"), DataTypeOID: 23, DataTypeSize: 4, TypeModifier: -1, Format: 0},
		{Name: []byte("int_col"), DataTypeOID: 23, DataTypeSize: 4, TypeModifier: -1, Format: 0},
		{Name: []byte("smallint_col"), DataTypeOID: 21, DataTypeSize: 2, TypeModifier: -1, Format: 0},
		{Name: []byte("bigint_col"), DataTypeOID: 20, DataTypeSize: 8, TypeModifier: -1, Format: 0},
		{Name: []byte("decimal_col"), DataTypeOID: 1700, DataTypeSize: -1, TypeModifier: 655366, Format: 0},
		{Name: []byte("numeric_col"), DataTypeOID: 1700, DataTypeSize: -1, TypeModifier: 983049, Format: 0},
		{Name: []byte("real_col"), DataTypeOID: 700, DataTypeSize: 4, TypeModifier: -1, Format: 0},
		{Name: []byte("double_col"), DataTypeOID: 701, DataTypeSize: 8, TypeModifier: -1, Format: 0},
		{Name: []byte("smallserial_col"), DataTypeOID: 21, DataTypeSize: 2, TypeModifier: -1, Format: 0},
		{Name: []byte("bigserial_col"), DataTypeOID: 20, DataTypeSize: 8, TypeModifier: -1, Format: 0},
		{Name: []byte("char_col"), DataTypeOID: 1042, DataTypeSize: -1, TypeModifier: 14, Format: 0},
		{Name: []byte("varchar_col"), DataTypeOID: 1043, DataTypeSize: -1, TypeModifier: 54, Format: 0},
		{Name: []byte("text_col"), DataTypeOID: 25, DataTypeSize: -1, TypeModifier: -1, Format: 0},
		{Name: []byte("boolean_col"), DataTypeOID: 16, DataTypeSize: 1, TypeModifier: -1, Format: 0},
		{Name: []byte("date_col"), DataTypeOID: 1082, DataTypeSize: 4, TypeModifier: -1, Format: 0},
		{Name: []byte("time_col"), DataTypeOID: 1083, DataTypeSize: 8, TypeModifier: -1, Format: 0},
		{Name: []byte("timetz_col"), DataTypeOID: 1266, DataTypeSize: 12, TypeModifier: -1, Format: 0},
		{Name: []byte("timestamp_col"), DataTypeOID: 1114, DataTypeSize: 8, TypeModifier: -1, Format: 0},
		{Name: []byte("timestamptz_col"), DataTypeOID: 1184, DataTypeSize: 8, TypeModifier: -1, Format: 0},
		{Name: []byte("interval_col"), DataTypeOID: 1186, DataTypeSize: 16, TypeModifier: -1, Format: 0},
		{Name: []byte("uuid_col"), DataTypeOID: 2950, DataTypeSize: 16, TypeModifier: -1, Format: 0},
		{Name: []byte("json_col"), DataTypeOID: 114, DataTypeSize: -1, TypeModifier: -1, Format: 0},
		{Name: []byte("jsonb_col"), DataTypeOID: 3802, DataTypeSize: -1, TypeModifier: -1, Format: 0},
		{Name: []byte("xml_col"), DataTypeOID: 142, DataTypeSize: -1, TypeModifier: -1, Format: 0},
		{Name: []byte("bytea_col"), DataTypeOID: 17, DataTypeSize: -1, TypeModifier: -1, Format: 0},
		{Name: []byte("inet_col"), DataTypeOID: 869, DataTypeSize: -1, TypeModifier: -1, Format: 0},
		{Name: []byte("cidr_col"), DataTypeOID: 650, DataTypeSize: -1, TypeModifier: -1, Format: 0},
		{Name: []byte("macaddr_col"), DataTypeOID: 829, DataTypeSize: 6, TypeModifier: -1, Format: 0},
		{Name: []byte("point_col"), DataTypeOID: 600, DataTypeSize: 16, TypeModifier: -1, Format: 0},
		{Name: []byte("line_col"), DataTypeOID: 628, DataTypeSize: 24, TypeModifier: -1, Format: 0},
		{Name: []byte("lseg_col"), DataTypeOID: 601, DataTypeSize: 32, TypeModifier: -1, Format: 0},
		{Name: []byte("box_col"), DataTypeOID: 603, DataTypeSize: 32, TypeModifier: -1, Format: 0},
		{Name: []byte("path_col"), DataTypeOID: 602, DataTypeSize: -1, TypeModifier: -1, Format: 0},
		{Name: []byte("polygon_col"), DataTypeOID: 604, DataTypeSize: -1, TypeModifier: -1, Format: 0},
		{Name: []byte("circle_col"), DataTypeOID: 718, DataTypeSize: 24, TypeModifier: -1, Format: 0},
		{Name: []byte("tsquery_col"), DataTypeOID: 3615, DataTypeSize: -1, TypeModifier: -1, Format: 0},
		{Name: []byte("tsvector_col"), DataTypeOID: 3614, DataTypeSize: -1, TypeModifier: -1, Format: 0},
	}},
	&pgproto3.DataRow{Values: [][]byte{
		[]byte("1"),
		[]byte("42"),
		[]byte("32767"),
		[]byte("9223372036854775807"),
		[]byte("12345.67"),
		[]byte("98765.43210"),
		[]byte("3.14"),
		[]byte("2.718281828459045"),
		[]byte("1"),
		[]byte("1"),
		[]byte("A         "),
		[]byte("Sample varchar text"),
		[]byte("Sample text data"),
		[]byte("t"),
		[]byte("2024-11-29"),
		[]byte("12:34:56"),
		[]byte("12:34:56+03"),
		[]byte("2024-11-29 12:34:56"),
		[]byte("2024-11-29 09:34:56+00"),
		[]byte("1 year 2 mons 3 days 04:05:06"),
		[]byte("550e8400-e29b-41d4-a716-446655440000"),
		[]byte("{\"key\": \"value\"}"),
		[]byte("{\"key\": \"value\"}"),
		[]byte("<root><child>XML content</child></root>"),
		[]byte("\\x48656c6c6f20576f726c64"),
		[]byte("192.168.1.1"),
		[]byte("192.168.1.0/24"),
		[]byte("08:00:2b:01:02:03"),
		[]byte("(1,2)"),
		[]byte("{1,-1,0}"),
		[]byte("[(0,0),(1,1)]"),
		[]byte("(1,1),(0,0)"),
		[]byte("((0,0),(1,1),(2,2))"),
		[]byte("((0,0),(1,1),(1,0))"),
		[]byte("<(0,0),1>"),
		[]byte("'fat' & 'rat'"),
		[]byte("'a' 'and' 'ate' 'cat' 'fat' 'mat' 'on' 'rat' 'sat'"),
	}},
	&pgproto3.CommandComplete{CommandTag: pgconn.CommandTag("SELECT")},
	&pgproto3.ReadyForQuery{},
}
