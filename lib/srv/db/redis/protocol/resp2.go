/*

 Copyright 2022 Gravitational, Inc.

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

package protocol

import (
	"reflect"
	"strconv"

	"github.com/go-redis/redis/v8"
	"github.com/gravitational/trace"
)

// ErrCmdNotSupported is returned when an unsupported Redis command is sent to Teleport proxy.
var ErrCmdNotSupported = trace.NotImplemented("command not supported")

// WriteCmd writes Redis commands passed as vals to Redis wire form.
// Most types are covered by go-redis implemented WriteArg() function. Types override by this function are:
// * Redis errors and Go error: go-redis returns a "human-readable" string instead of RESP compatible error message
// * integers: go-redis converts them to string, which is not always what we want.
// * slices: arrays are recursively converted to RESP responses.
func WriteCmd(wr *redis.Writer, vals interface{}) error {
	switch val := vals.(type) {
	case redis.Error:
		if val == redis.Nil {
			// go-redis returns nil value as errors, but Redis Wire protocol decodes them differently.
			// Note: RESP3 has different sequence for nil, current nil is RESP2 compatible as the rest
			// of our implementation.
			if _, err := wr.WriteString("$-1\r\n"); err != nil {
				return trace.Wrap(err)
			}
			return nil
		}

		if err := writeError(wr, "-", val); err != nil {
			return trace.Wrap(err)
		}
	case trace.Error:
		if err := writeError(wr, "-ERR Teleport: ", val); err != nil {
			return trace.Wrap(err)
		}
	case error:
		if err := writeError(wr, "-ERR ", val); err != nil {
			return trace.Wrap(err)
		}
	case int:
		if err := writeInteger(wr, int64(val)); err != nil {
			return trace.Wrap(err)
		}
	case int8:
		if err := writeInteger(wr, int64(val)); err != nil {
			return trace.Wrap(err)
		}
	case int16:
		if err := writeInteger(wr, int64(val)); err != nil {
			return trace.Wrap(err)
		}
	case int32:
		if err := writeInteger(wr, int64(val)); err != nil {
			return trace.Wrap(err)
		}
	case int64:
		if err := writeInteger(wr, val); err != nil {
			return trace.Wrap(err)
		}
	case uint:
		if err := writeUinteger(wr, uint64(val)); err != nil {
			return trace.Wrap(err)
		}
	case uint8:
		if err := writeUinteger(wr, uint64(val)); err != nil {
			return trace.Wrap(err)
		}
	case uint16:
		if err := writeUinteger(wr, uint64(val)); err != nil {
			return trace.Wrap(err)
		}
	case uint32:
		if err := writeUinteger(wr, uint64(val)); err != nil {
			return trace.Wrap(err)
		}
	case uint64:
		if err := writeUinteger(wr, val); err != nil {
			return trace.Wrap(err)
		}
	case interface{}:
		var err error
		v := reflect.ValueOf(val)

		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		if v.Kind() == reflect.Slice {
			err = writeSlice(wr, val)
		} else {
			err = wr.WriteArg(val)
		}

		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// writeError converts Go error to Redis wire form.
func writeError(wr *redis.Writer, prefix string, val error) error {
	if _, err := wr.WriteString(prefix); err != nil {
		// Add error header specified in https://redis.io/topics/protocol#resp-errors
		// to follow the convention.
		return trace.Wrap(err)
	}

	if _, err := wr.WriteString(val.Error()); err != nil {
		return trace.Wrap(err)
	}

	if _, err := wr.Write([]byte("\r\n")); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// writeSlice converts a slice to Redis wire form.
func writeSlice(wr *redis.Writer, vals interface{}) error {
	v := reflect.ValueOf(vals)

	if v.Kind() != reflect.Slice {
		return trace.BadParameter("expected slice, passed %T", vals)
	}

	if err := wr.WriteByte(redis.ArrayReply); err != nil {
		return trace.Wrap(err)
	}

	n := v.Len()
	if err := wr.WriteLen(n); err != nil {
		return trace.Wrap(err)
	}

	for i := 0; i < n; i++ {
		if err := WriteCmd(wr, v.Index(i).Interface()); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// writeInteger converts integer to Redis wire form.
func writeInteger(wr *redis.Writer, val int64) error {
	if err := wr.WriteByte(redis.IntReply); err != nil {
		return trace.Wrap(err)
	}

	v := strconv.FormatInt(val, 10)
	if _, err := wr.WriteString(v); err != nil {
		return trace.Wrap(err)
	}

	if _, err := wr.Write([]byte("\r\n")); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// writeUinteger converts unsigned integer to Redis wire form.
func writeUinteger(wr *redis.Writer, val uint64) error {
	if err := wr.WriteByte(redis.IntReply); err != nil {
		return trace.Wrap(err)
	}

	v := strconv.FormatUint(val, 10)
	if _, err := wr.WriteString(v); err != nil {
		return trace.Wrap(err)
	}

	if _, err := wr.Write([]byte("\r\n")); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
