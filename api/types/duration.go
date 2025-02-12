/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
)

// Duration is a wrapper around duration to set up custom marshal/unmarshal
type Duration time.Duration

// Duration returns time.Duration from Duration typex
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// Value returns time.Duration value of this wrapper
func (d Duration) Value() time.Duration {
	return time.Duration(d)
}

// MarshalJSON marshals Duration to string
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration().String())
}

// UnmarshalJSON interprets the given bytes as a Duration value
func (d *Duration) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	var stringVar string
	if err := json.Unmarshal(data, &stringVar); err != nil {
		return trace.Wrap(err)
	}
	if stringVar == constants.DurationNever {
		*d = Duration(0)
		return nil
	}
	out, err := parseDuration(stringVar)
	if err != nil {
		return trace.BadParameter("%s", err)
	}
	*d = out
	return nil
}

// MarshalYAML marshals duration into YAML value,
// encodes it as a string in format "1m"
func (d Duration) MarshalYAML() (interface{}, error) {
	return fmt.Sprintf("%v", d.Duration()), nil
}

// UnmarshalYAML unmarshals duration from YAML value.
func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var stringVar string
	if err := unmarshal(&stringVar); err != nil {
		return trace.Wrap(err)
	}
	if stringVar == constants.DurationNever {
		*d = Duration(0)
		return nil
	}
	out, err := parseDuration(stringVar)
	if err != nil {
		return trace.BadParameter("%s", err)
	}
	*d = out
	return nil
}

// MaxDuration returns the maximum duration value
func MaxDuration() Duration {
	return NewDuration(1<<63 - 1)
}

// NewDuration converts the given time.Duration value to a duration
func NewDuration(d time.Duration) Duration {
	return Duration(d)
}

// leadingInt consumes the leading [0-9]* from s.
func leadingInt(s string) (x int64, rem string, err error) {
	i := 0
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		if x > (1<<63-1)/10 {
			// overflow
			return 0, "", trace.BadParameter("time: bad [0-9]*")
		}
		x = x*10 + int64(c) - '0'
		if x < 0 {
			// overflow
			return 0, "", trace.BadParameter("time: bad [0-9]*")
		}
	}
	return x, s[i:], nil
}

// leadingFraction consumes the leading [0-9]* from s.
// It is used only for fractions, so does not return an error on overflow,
// it just stops accumulating precision.
func leadingFraction(s string) (x int64, scale float64, rem string) {
	i := 0
	scale = 1
	overflow := false
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		if overflow {
			continue
		}
		if x > (1<<63-1)/10 {
			// It's possible for overflow to give a positive number, so take care.
			overflow = true
			continue
		}
		y := x*10 + int64(c) - '0'
		if y < 0 {
			overflow = true
			continue
		}
		x = y
		scale *= 10
	}
	return x, scale, s[i:]
}

var unitMap = map[string]int64{
	"ns": int64(time.Nanosecond),
	"us": int64(time.Microsecond),
	"µs": int64(time.Microsecond), // U+00B5 = micro symbol
	"μs": int64(time.Microsecond), // U+03BC = Greek letter mu
	"ms": int64(time.Millisecond),
	"s":  int64(time.Second),
	"m":  int64(time.Minute),
	"h":  int64(time.Hour),
	"d":  int64(time.Hour * 24),
	"mo": int64(time.Hour * 24 * 30),
	"y":  int64(time.Hour * 24 * 365),
}

// parseDuration parses a duration string.
// A duration string is a possibly signed sequence of
// decimal numbers, each with optional fraction and a unit suffix,
// such as "300ms", "-1.5h" or "2h45m".
// Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
func parseDuration(s string) (Duration, error) {
	// [-+]?([0-9]*(\.[0-9]*)?[a-z]+)+
	orig := s
	var d int64
	neg := false

	// Consume [-+]?
	if s != "" {
		c := s[0]
		if c == '-' || c == '+' {
			neg = c == '-'
			s = s[1:]
		}
	}
	// Special case: if all that is left is "0", this is zero.
	if s == "0" {
		return 0, nil
	}
	if s == "" {
		return 0, trace.BadParameter("time: invalid duration %q", orig)
	}
	for s != "" {
		var (
			v, f  int64       // integers before, after decimal point
			scale float64 = 1 // value = v + f/scale
		)

		var err error

		// The next character must be [0-9.]
		if !(s[0] == '.' || '0' <= s[0] && s[0] <= '9') {
			return 0, trace.BadParameter("time: invalid duration %q", orig)
		}
		// Consume [0-9]*
		pl := len(s)
		v, s, err = leadingInt(s)
		if err != nil {
			return 0, trace.BadParameter("time: invalid duration %q", orig)
		}
		pre := pl != len(s) // whether we consumed anything before a period

		// Consume (\.[0-9]*)?
		post := false
		if s != "" && s[0] == '.' {
			s = s[1:]
			pl := len(s)
			f, scale, s = leadingFraction(s)
			post = pl != len(s)
		}
		if !pre && !post {
			// no digits (e.g. ".s" or "-.s")
			return 0, trace.BadParameter("time: invalid duration %q", orig)
		}

		// Consume unit.
		i := 0
		for ; i < len(s); i++ {
			c := s[i]
			if c == '.' || '0' <= c && c <= '9' {
				break
			}
		}
		if i == 0 {
			return 0, trace.BadParameter("time: missing unit in duration %q", orig)
		}
		u := s[:i]
		s = s[i:]
		unit, ok := unitMap[u]
		if !ok {
			return 0, trace.BadParameter("time: unknown unit in duration %q", orig)
		}
		if v > (1<<63-1)/unit {
			// overflow
			return 0, trace.BadParameter("time: invalid duration %q", orig)
		}
		v *= unit
		if f > 0 {
			// float64 is needed to be nanosecond accurate for fractions of hours.
			// v >= 0 && (f*unit/scale) <= 3.6e+12 (ns/h, h is the largest unit)
			v += int64(float64(f) * (float64(unit) / scale))
			if v < 0 {
				// overflow
				return 0, trace.BadParameter("time: invalid duration %q", orig)
			}
		}
		d += v
		if d < 0 {
			// overflow
			return 0, trace.BadParameter("time: invalid duration %q", orig)
		}
	}

	if neg {
		d = -d
	}
	return Duration(d), nil
}
