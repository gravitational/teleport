/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
package attrs

import (
	"fmt"
	"log/slog"

	"google.golang.org/protobuf/proto"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

// WorkloadAttrs wraps the underlying protobuf message to implement slog.LogValuer.
//
// We do this because the "raw" representation returned by String contains large
// unwieldy binary blobs (e.g. Sigstore bundles) which aren't useful in log output.
type WorkloadAttrs struct {
	*workloadidentityv1.WorkloadAttrs
}

// NewWorkloadAttrs creates a new empty WorkloadAttrs.
func NewWorkloadAttrs() *WorkloadAttrs {
	return FromWorkloadAttrs(new(workloadidentityv1.WorkloadAttrs))
}

// FromWorkloadAttrs wraps the given protobuf message.
func FromWorkloadAttrs(inner *workloadidentityv1.WorkloadAttrs) *WorkloadAttrs {
	return &WorkloadAttrs{WorkloadAttrs: inner}
}

// GetAttrs returns the underlying protobuf message.
func (a *WorkloadAttrs) GetAttrs() *workloadidentityv1.WorkloadAttrs {
	if a == nil {
		return nil
	}
	return a.WorkloadAttrs
}

// LogValue implements slog.LogValuer.
func (a *WorkloadAttrs) LogValue() slog.Value {
	// Strip the actual sigstore attributes out because they contain large binary
	// encoded bundles, and replace with a simple count.
	//
	// TODO: we should find a better way to encode workload attributes in logs
	// than prototext which, apart from anything, injects random whitespace.
	clone := proto.Clone(a.WorkloadAttrs).(*workloadidentityv1.WorkloadAttrs)
	clone.Sigstore = nil

	output := clone.String()
	if l := len(a.WorkloadAttrs.GetSigstore().GetPayloads()); l != 0 {
		output = fmt.Sprintf("%s  sigstore:{payloads:{count:%d}}", output, l)
	}
	return slog.StringValue(output)
}

var _ slog.LogValuer = (*WorkloadAttrs)(nil)
