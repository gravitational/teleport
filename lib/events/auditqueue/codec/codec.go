/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package codec

import (
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
)

func Encode(events [][]byte) ([]byte, error) {
	payload, err := proto.Marshal(auditlogpb.AuditQueueBatch_builder{Events: events}.Build())
	return payload, trace.Wrap(err)
}

func Decode(payload []byte) ([]apievents.AuditEvent, error) {
	var batch auditlogpb.AuditQueueBatch
	if err := proto.Unmarshal(payload, &batch); err != nil {
		return nil, trace.Wrap(err)
	}
	if len(batch.ProtoReflect().GetUnknown()) > 0 {
		// reject the batch and let it be sent again to a more up-to-date auth server
		return nil, trace.BadParameter("audit event batch has unrecognized fields")
	}
	raws := batch.GetEvents()
	if len(raws) == 0 {
		return nil, trace.BadParameter("audit event batch is empty")
	}
	events := make([]apievents.AuditEvent, 0, len(raws))
	for _, raw := range raws {
		var oneOf apievents.OneOf
		if err := oneOf.Unmarshal(raw); err != nil {
			return nil, trace.Wrap(err)
		}
		event, err := apievents.FromOneOf(oneOf)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		events = append(events, event)
	}
	return events, nil
}

func ElementSize(n int) int {
	return protowire.SizeTag(1) + protowire.SizeBytes(n)
}
