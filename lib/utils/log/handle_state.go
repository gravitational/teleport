// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"encoding"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

// handleState adapted from go/src/log/slog/handler.go
type handleState struct {
	h       *SlogTextHandler
	buf     *buffer
	freeBuf bool      // should buf be freed?
	prefix  *buffer   // for text: key prefix
	groups  *[]string // pool-allocated slice of active groups, for ReplaceAttr
}

var groupPool = sync.Pool{New: func() any {
	s := make([]string, 0, 10)
	return &s
}}

func (s *handleState) free() {
	if s.freeBuf {
		s.buf.Free()
	}
	if gs := s.groups; gs != nil {
		*gs = (*gs)[:0]
		groupPool.Put(gs)
	}
	s.prefix.Free()
}

func (s *handleState) openGroups() {
	for _, n := range s.h.groups[s.h.nOpenGroups:] {
		s.openGroup(n)
	}
}

// openGroup starts a new group of attributes
// with the given name.
func (s *handleState) openGroup(name string) {
	s.prefix.WriteString(name)
	s.prefix.WriteByte('.')

	// Collect group names for ReplaceAttr.
	if s.groups != nil {
		*s.groups = append(*s.groups, name)
	}
}

// closeGroup ends the group with the given name.
func (s *handleState) closeGroup(name string) {
	*s.prefix = (*s.prefix)[:len(*s.prefix)-len(name)-1 /* for keyComponentSep */]

	if s.groups != nil {
		*s.groups = (*s.groups)[:len(*s.groups)-1]
	}
}

// appendAttrs appends the slice of Attrs.
// It reports whether something was appended.
func (s *handleState) appendAttrs(as []slog.Attr) bool {
	nonEmpty := false
	for _, a := range as {
		if s.appendAttr(a) {
			nonEmpty = true
		}
	}
	return nonEmpty
}

// appendAttr appends the Attr's key and value.
// It handles replacement and checking for an empty key.
// It reports whether something was appended.
func (s *handleState) appendAttr(a slog.Attr) bool {
	a.Value = a.Value.Resolve()
	if rep := s.h.cfg.ReplaceAttr; rep != nil && a.Value.Kind() != slog.KindGroup {
		var gs []string
		if s.groups != nil {
			gs = *s.groups
		}
		// a.Value is resolved before calling ReplaceAttr, so the user doesn't have to.
		a = rep(gs, a)
		// The ReplaceAttr function may return an unresolved Attr.
		a.Value = a.Value.Resolve()
	}
	// Elide empty Attrs.
	if a.Equal(slog.Attr{}) {
		return false
	}

	// Handle nested attributes from within component fields.
	if a.Key == teleport.ComponentFields {
		nonEmpty := false
		switch fields := a.Value.Any().(type) {
		case map[string]any:
			for k, v := range fields {
				if s.appendAttr(slog.Any(k, v)) {
					nonEmpty = true
				}
			}
			return nonEmpty
		}
	}

	// Handle special cases before formatting.
	var traceError bool
	if a.Value.Kind() == slog.KindAny {
		switch v := a.Value.Any().(type) {
		case *slog.Source:
			a.Value = slog.StringValue(fmt.Sprintf(" %s:%d", v.File, v.Line))
		case trace.Error:
			traceError = true
			a.Value = slog.StringValue("[" + v.DebugReport() + "]")
		case error:
			a.Value = slog.StringValue(fmt.Sprintf("[%v]", v))
		}
	}

	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		// Output only non-empty groups.
		if len(attrs) > 0 {
			// The group may turn out to be empty even though it has attrs (for
			// example, ReplaceAttr may delete all the attrs).
			// So remember where we are in the buffer, to restore the position
			// later if necessary.
			pos := s.buf.Len()
			// Inline a group with an empty key.
			if a.Key != "" {
				s.openGroup(a.Key)
			}
			if !s.appendAttrs(attrs) {
				s.buf.SetLen(pos)
				return false
			}
			if a.Key != "" {
				s.closeGroup(a.Key)
			}
		}

		return true
	}

	s.appendKey(a.Key)

	// Write the level key to avoid quoting color formatting that exists or
	// [trace.Error]s so that the debug report is output in it's entirety.
	if traceError || a.Key == slog.LevelKey {
		s.buf.WriteString(a.Value.String())
	} else {
		s.appendValue(a.Value)
	}

	return true
}

func (s *handleState) appendError(err error) {
	s.appendString(fmt.Sprintf("!ERROR:%v", err))
}

func (s *handleState) appendKey(key string) {
	if s.buf.Len() > 0 {
		s.buf.WriteString(" ")
	}

	// These keys should not be included in the output to match
	// the behavior of the lorgus formatter.
	if key == slog.TimeKey ||
		key == teleport.ComponentKey ||
		key == slog.LevelKey ||
		key == CallerField ||
		key == slog.MessageKey ||
		key == slog.SourceKey {
		return
	}

	if s.prefix != nil && len(*s.prefix) > 0 {
		// TODO: optimize by avoiding allocation.
		s.appendString(string(*s.prefix) + key)
	} else {
		s.appendString(key)
	}

	s.buf.WriteByte(':')
}

func (s *handleState) appendString(str string) {
	if str == "" {
		return
	}

	if needsQuoting(str) {
		*s.buf = strconv.AppendQuote(*s.buf, str)
	} else {
		s.buf.WriteString(str)
	}
}

func (s *handleState) appendValue(v slog.Value) {
	defer func() {
		if r := recover(); r != nil {
			// If it panics with a nil pointer, the most likely cases are
			// an encoding.TextMarshaler or error fails to guard against nil,
			// in which case "<nil>" seems to be the feasible choice.
			//
			// Adapted from the code in fmt/print.go.
			if v := reflect.ValueOf(v.Any()); v.Kind() == reflect.Pointer && v.IsNil() {
				s.appendString("<nil>")
				return
			}

			// Otherwise just print the original panic message.
			s.appendString(fmt.Sprintf("!PANIC: %v", r))
		}
	}()

	if err := appendTextValue(s, v); err != nil {
		s.appendError(err)
	}
}

func (s *handleState) appendTime(t time.Time) {
	*s.buf = appendRFC3339Millis(*s.buf, t)
}

func (s *handleState) appendNonBuiltIns(r slog.Record) {
	// preformatted Attrs
	if pfa := s.h.preformatted; len(pfa) > 0 {
		s.buf.WriteString(" ")
		s.buf.Write(pfa)
	}
	// Attrs in Record -- unlike the built-in ones, they are in groups started
	// from WithGroup.
	// If the record has no Attrs, don't output any groups.
	if r.NumAttrs() > 0 {
		s.prefix.WriteString(s.h.groupPrefix)
		// The group may turn out to be empty even though it has attrs (for
		// example, ReplaceAttr may delete all the attrs).
		// So remember where we are in the buffer, to restore the position
		// later if necessary.
		pos := s.buf.Len()
		s.openGroups()
		empty := true
		r.Attrs(func(a slog.Attr) bool {
			// The component is handled by the top level handler.
			if a.Key == teleport.ComponentKey {
				return true
			}
			if s.appendAttr(a) {
				empty = false
			}
			return true
		})
		if empty {
			s.buf.SetLen(pos)
		}
	}
}

func byteSlice(a any) ([]byte, bool) {
	if bs, ok := a.([]byte); ok {
		return bs, true
	}
	// Like Printf's %s, we allow both the slice type and the byte element type to be named.
	t := reflect.TypeOf(a)
	if t != nil && t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
		return reflect.ValueOf(a).Bytes(), true
	}
	return nil, false
}

func appendTextValue(s *handleState, v slog.Value) error {
	switch v.Kind() {
	case slog.KindString:
		s.appendString(v.String())
	case slog.KindTime:
		s.appendTime(v.Time())
	case slog.KindAny:
		if tm, ok := v.Any().(encoding.TextMarshaler); ok {
			data, err := tm.MarshalText()
			if err != nil {
				return err
			}
			// TODO: avoid the conversion to string.
			s.appendString(string(data))
			return nil
		}
		if bs, ok := byteSlice(v.Any()); ok {
			// As of Go 1.19, this only allocates for strings longer than 32 bytes.
			s.buf.WriteString(strconv.Quote(string(bs)))
			return nil
		}
		s.appendString(fmt.Sprintf("%+v", v.Any()))
	case slog.KindInt64:
		*s.buf = strconv.AppendInt(*s.buf, v.Int64(), 10)
	case slog.KindUint64:
		*s.buf = strconv.AppendUint(*s.buf, v.Uint64(), 10)
	case slog.KindFloat64:
		*s.buf = strconv.AppendFloat(*s.buf, v.Float64(), 'g', -1, 64)
	case slog.KindBool:
		*s.buf = strconv.AppendBool(*s.buf, v.Bool())
	case slog.KindDuration:
		*s.buf = append(*s.buf, v.Duration().String()...)
	case slog.KindGroup:
		*s.buf = fmt.Append(*s.buf, v.Group())
	case slog.KindLogValuer:
		*s.buf = fmt.Append(*s.buf, v.Any())
	default:
		panic(fmt.Sprintf("bad kind: %s", v.Kind()))
	}
	return nil
}

func appendRFC3339Millis(b []byte, t time.Time) []byte {
	// Format according to time.RFC3339Nano since it is highly optimized,
	// but truncate it to use millisecond resolution.
	// Unfortunately, that format trims trailing 0s, so add 1/10 millisecond
	// to guarantee that there are exactly 4 digits after the period.
	const prefixLen = len("2006-01-02T15:04:05.000")
	n := len(b)
	t = t.Truncate(time.Millisecond).Add(time.Millisecond / 10)
	b = t.AppendFormat(b, time.RFC3339Nano)
	b = append(b[:n+prefixLen], b[n+prefixLen+1:]...) // drop the 4th digit
	return b
}
