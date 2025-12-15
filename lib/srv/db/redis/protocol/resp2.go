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

package protocol

import (
	"bufio"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
	"github.com/redis/go-redis/v9"
)

// ErrCmdNotSupported is returned when an unsupported Redis command is sent to Teleport proxy.
var ErrCmdNotSupported = trace.NotImplemented("command not supported")

// WriteCmd writes Redis commands passed as vals to Redis wire form.
// Most types are covered by go-redis implemented WriteArg() function. Types override by this function are:
// * Redis errors and Go error: go-redis returns a "human-readable" string instead of RESP compatible error message
// * integers: go-redis converts them to string, which is not always what we want.
// * slices: arrays are recursively converted to RESP responses.
func WriteCmd(wr *redis.Writer, vals any) error {
	switch val := vals.(type) {
	case nil:
		// Note: RESP3 has different sequence for nil, current nil is RESP2 compatible as the rest
		// of our implementation.
		if _, err := wr.WriteString("$-1\r\n"); err != nil {
			return trace.Wrap(err)
		}
	case redis.Error:
		if val == redis.Nil {
			// go-redis returns nil value as errors, but Redis Wire protocol decodes them differently.
			return trace.Wrap(WriteCmd(wr, nil))
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
	case any:
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

	// If the error message contains "\r" or "\n", redis-cli will have trouble
	// parsing the message and show "Bad simple string value" instead. So if
	// newlines are detected in the original error message, merge them to one
	// line.
	errString := val.Error()
	if strings.ContainsAny(errString, "\r\n") {
		scanner := bufio.NewScanner(strings.NewReader(errString))
		errString = ""

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			if errString != "" {
				errString += " "
			}
			errString += line
		}
	}

	if _, err := wr.WriteString(errString); err != nil {
		return trace.Wrap(err)
	}

	if _, err := wr.Write([]byte("\r\n")); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// writeSlice converts a slice to Redis wire form.
func writeSlice(wr *redis.Writer, vals any) error {
	v := reflect.ValueOf(vals)

	if v.Kind() != reflect.Slice {
		return trace.BadParameter("expected slice, passed %T", vals)
	}

	if err := wr.WriteByte(redis.RespArray); err != nil {
		return trace.Wrap(err)
	}

	n := v.Len()
	if err := wr.WriteLen(n); err != nil {
		return trace.Wrap(err)
	}

	for i := range n {
		if err := WriteCmd(wr, v.Index(i).Interface()); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// MakeUnknownCommandErrorForCmd creates an unknown command error for the
// provided command in the original Redis format.
func MakeUnknownCommandErrorForCmd(cmd *redis.Cmd) redis.RedisError {
	args := cmd.Args()

	if len(args) == 0 {
		// Should never happen.
		return redis.RedisError("ERR unknown command ''")
	}

	switch strings.ToLower(cmd.Name()) {
	case "cluster", "command":
		var subCmd string
		if len(args) > 1 {
			subCmd = fmt.Sprintf("%v", args[1])
		}
		// Example: ERR unknown subcommand 'aaa'. Try CLUSTER HELP.
		return redis.RedisError(fmt.Sprintf("ERR unknown subcommand '%s'. Try %s HELP.", subCmd, strings.ToUpper(cmd.Name())))

	default:
		// cmd.Name() may be lower cased. Use args[0] to get the original command.
		cmdName := fmt.Sprintf("%v", args[0])
		args = args[1:]
		argsInStrings := make([]string, 0, len(args))
		for _, arg := range args {
			argsInStrings = append(argsInStrings, fmt.Sprintf("'%v'", arg))
		}
		// Example: ERR unknown command 'cmd', with args beginning with: 'arg1' 'arg2' 'arg3' ...
		return redis.RedisError(fmt.Sprintf("ERR unknown command '%s', with args beginning with: %s", cmdName, strings.Join(argsInStrings, " ")))
	}
}

// writeInteger converts integer to Redis wire form.
func writeInteger(wr *redis.Writer, val int64) error {
	if err := wr.WriteByte(redis.RespInt); err != nil {
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
	if err := wr.WriteByte(redis.RespInt); err != nil {
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
