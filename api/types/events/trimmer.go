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

// fieldTrimer handles trimming a field from an event. fieldTrimmer usually
// holds both the field from the source event (for calculation) and the pointer
// to the field from the target copy (for trimming updates).
type fieldTrimmer interface {
	// emptyTarget empties the target field.
	emptyTarget()
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

	out := utils.CloneProtoMsg(m)
	trimmer := makeTrimmer(m, out)
	trimmer.emptyTarget() // Empty before adjusting max size
	maxSize = adjustedMaxSize(out, maxSize)
	maxFieldSize := maxSizePerField(maxSize, trimmer.nonEmptyStrs())
	trimmer.trimToMaxFieldSize(maxFieldSize)
	return out
}

// fieldTrimmers is a slice of fieldTrimmer that implements fieldTrimmer.
type fieldTrimmers []fieldTrimmer

func (t fieldTrimmers) emptyTarget() {
	for _, e := range t {
		e.emptyTarget()
	}
}
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

type baseTrimmer struct {
	emptyTargetFunc        func()
	nonEmptyStrsFunc       func() int
	trimToMaxFieldSizeFunc func(int)
}

func newBaseTrimmer[Source any, Target any](
	source Source,
	target *Target,
	nonEmptyStrs func(Source) int,
	trimToMaxFieldSize func(Source, int) Target,
) fieldTrimmer {
	return &baseTrimmer{
		emptyTargetFunc: func() {
			var empty Target
			*target = empty
		},
		nonEmptyStrsFunc: func() int {
			return nonEmptyStrs(source)
		},
		trimToMaxFieldSizeFunc: func(maxFieldSize int) {
			*target = trimToMaxFieldSize(source, maxFieldSize)
		},
	}
}

func (t *baseTrimmer) emptyTarget()                        { t.emptyTargetFunc() }
func (t *baseTrimmer) nonEmptyStrs() int                   { return t.nonEmptyStrsFunc() }
func (t *baseTrimmer) trimToMaxFieldSize(maxFieldSize int) { t.trimToMaxFieldSizeFunc(maxFieldSize) }

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
		trimStrSlice,
	)
}

func newBytesTrimmer(source []byte, target *[]byte) fieldTrimmer {
	return newBaseTrimmer(source, target, nonEmptyStr, trimStr)
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
