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

package cassandra

import (
	"encoding/hex"

	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"

	"github.com/gravitational/teleport/api/types/events"
)

func eventTypesToString(ets []primitive.EventType) []string {
	out := make([]string, 0, len(ets))
	for _, v := range ets {
		out = append(out, string(v))
	}
	return out
}

func batchChildToProto(batches []*message.BatchChild) []*events.CassandraBatch_BatchChild {
	out := make([]*events.CassandraBatch_BatchChild, 0, len(batches))
	for _, v := range batches {
		out = append(out, &events.CassandraBatch_BatchChild{
			ID:     hex.EncodeToString(v.Id),
			Query:  v.Query,
			Values: convBatchChildValues(v.Values),
		})
	}
	return out
}

func convBatchChildValues(values []*primitive.Value) []*events.CassandraBatch_BatchChild_Value {
	out := make([]*events.CassandraBatch_BatchChild_Value, 0, len(values))
	for _, v := range values {
		out = append(out, &events.CassandraBatch_BatchChild_Value{
			Type:     uint32(v.Type),
			Contents: v.Contents,
		})
	}
	return out
}
