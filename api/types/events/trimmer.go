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

package events

import (
	"google.golang.org/protobuf/protoadapt"

	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils"
)

// fieldTrimer handles trimming a field from an event. fieldTrimmers usually
// holds both the field from the source event (for calculation) and the pointer
// to the field from the target copy (for trimming updates).
type fieldTrimmer interface {
	// nonEmptyStrs calculates non-empty strings from this field.
	nonEmptyStrs() int
	// trimToMaxFieldSize trims and updates the target field.
	trimToMaxFieldSize(maxFieldSize int)
}

type trimmableEvent interface {
	AuditEvent
	protoadapt.MessageV1
}

// trimEventToMaxSize handles trimming of an event.
// See MCPSessionInvalidHTTPRequest.TrimToMaxSize for example.
func trimEventToMaxSize[T trimmableEvent](m T, maxSize int, makeTrimmer func(m T, out T) fieldTrimmer) AuditEvent {
	size := m.Size()
	if size <= maxSize {
		return m
	}

	// Clone the message to "out". Call makeTrimmer to empty the fields in "out"
	// before adjusting for max size.
	out := utils.CloneProtoMsg(m)
	trimmer := makeTrimmer(m, out)
	maxSize = adjustedMaxSize(out, maxSize)
	maxFieldSize := maxSizePerField(maxSize, trimmer.nonEmptyStrs())
	trimmer.trimToMaxFieldSize(maxFieldSize)
	return out
}

// fieldTrimmers is a slice of fieldTrimmer that implements fieldTrimmer.
type fieldTrimmers []fieldTrimmer

func (t fieldTrimmers) nonEmptyStrs() (sum int) {
	for _, e := range t {
		sum += e.nonEmptyStrs()
	}
	return sum
}

func (t fieldTrimmers) trimToMaxFieldSize(maxFieldSize int) {
	for _, e := range t {
		e.trimToMaxFieldSize(maxFieldSize)
	}
}

type baseTrimmer[Source any, Target any] struct {
	nonEmptyStrsFunc       func() int
	trimToMaxFieldSizeFunc func(int)
}

func newBaseTrimmer[Source any, Target any](
	source Source,
	target *Target,
	nonEmptyStrs func(Source) int,
	trimToMaxFieldSize func(Source, int) Target,
) fieldTrimmer {
	// We must empty the target first.
	var empty Target
	*target = empty
	return &baseTrimmer[Source, Target]{
		nonEmptyStrsFunc: func() int {
			return nonEmptyStrs(source)
		},
		trimToMaxFieldSizeFunc: func(maxFieldSize int) {
			*target = trimToMaxFieldSize(source, maxFieldSize)
		},
	}
}

func (s *baseTrimmer[Source, Target]) nonEmptyStrs() int {
	return s.nonEmptyStrsFunc()
}
func (s *baseTrimmer[Source, Target]) trimToMaxFieldSize(maxFieldSize int) {
	s.trimToMaxFieldSizeFunc(maxFieldSize)
}

func newStrTrimmer(source string, target *string) fieldTrimmer {
	return newBaseTrimmer(source, target, nonEmptyStr, trimStr)
}

func newStrSliceTrimmer(source []string, target *[]string) fieldTrimmer {
	return newBaseTrimmer(
		source,
		target,
		func(source []string) int {
			return nonEmptyStrsInSlice(source)
		},
		func(source []string, maxFieldSize int) []string {
			return trimStrSlice(source, maxFieldSize)
		},
	)
}

func trimBytes(b []byte, maxFieldSize int) []byte {
	return []byte(trimStr(string(b), maxFieldSize))
}

func newBytesTrimmer(source []byte, target *[]byte) fieldTrimmer {
	return newBaseTrimmer(source, target, nonEmptyStr, trimBytes)
}

func newTraitsTrimmer(source wrappers.Traits, target *wrappers.Traits) fieldTrimmer {
	return newBaseTrimmer(source, target, nonEmptyTraits, trimTraits)
}

// trimmableField defines an interface for any struct (that is a field of an
// event) that can be trimmed.
type trimmableField[T any] interface {
	nonEmptyStrs() int
	trimToMaxFieldSize(maxFieldSize int) T
}

func newGenericTrimmer[T any, Trimmable trimmableField[T]](source Trimmable, target *T) fieldTrimmer {
	return newBaseTrimmer(
		source,
		target,
		func(source Trimmable) int {
			return source.nonEmptyStrs()
		},
		func(source Trimmable, maxFieldSize int) T {
			return source.trimToMaxFieldSize(maxFieldSize)
		},
	)
}
