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

package envutils

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

// ReadEnvironmentFile will read environment variables from a passed in location.
// Lines that start with "#" or empty lines are ignored. Assignments are in the
// form name=value and no variable expansion occurs.
func ReadEnvironmentFile(filename string) ([]string, error) {
	// open the users environment file. if we don't find a file, move on as
	// having this file for the user is optional.
	file, err := utils.OpenFileNoUnsafeLinks(filename)
	if err != nil {
		slog.WarnContext(context.Background(), "Unable to open environment file, skipping",
			"file", filename,
			"error", err,
		)
		return []string{}, nil
	}
	defer file.Close()

	return readEnvironment(file)
}

func readEnvironment(r io.Reader) ([]string, error) {
	var lineno int
	env := &SafeEnv{}

	ctx := context.Background()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// follow the lead of OpenSSH and don't allow more than 1,000 environment variables
		// https://github.com/openssh/openssh-portable/blob/master/session.c#L873-L874
		lineno = lineno + 1
		if lineno > teleport.MaxEnvironmentFileLines {
			slog.WarnContext(ctx, "Too many lines in environment file, limiting how many are consumed",
				"lines_consumed", teleport.MaxEnvironmentFileLines,
			)
			return *env, nil
		}

		// empty lines or lines that start with # are ignored
		if line == "" || line[0] == '#' {
			continue
		}

		// split on first =, if not found, log it and continue
		idx := strings.Index(line, "=")
		if idx == -1 {
			slog.DebugContext(ctx, "Bad line while reading environment file: no = separator found", "line_number", lineno)
			continue
		}

		// split key and value and make sure that key has a name
		key := line[:idx]
		value := line[idx+1:]
		if strings.TrimSpace(key) == "" {
			slog.DebugContext(ctx, "Bad line while reading environment file: key without name", "line_number", lineno)
			continue
		}

		// key is added trusted within this context, but should be "AddFullUnique" when combined with any other values
		env.AddTrusted(key, value)
	}

	if err := scanner.Err(); err != nil {
		slog.WarnContext(ctx, "Unable to read environment file", "error", err)
		return []string{}, nil
	}

	return *env, nil
}

var unsafeEnvironmentPrefixes = []string{
	// Linux
	// Covering cases from LD (man ld.so) to prevent injection like LD_PRELOAD
	"LD_",
	// macOS
	// Covering cases from DYLD (man dyld) to prevent injection like DYLD_LIBRARY_PATH
	"DYLD_",
}

// SafeEnv allows you to build a system environment while avoiding potentially dangerous environment conditions.  In
// addition, SafeEnv will ignore any values added if the key already exists.  This allows earlier inserts to take
// priority and ensure there is no conflicting values.
type SafeEnv []string

// AddTrusted will add the key and value to the environment if it's a safe value to forward on for fork / exec.  This
// will not check for duplicates.
func (e *SafeEnv) AddTrusted(k, v string) {
	e.add(false, k, v)
}

// AddUnique will add the key and value to the environment if it's a safe value to forward on for fork / exec.  If the
// key already exists (case-insensitive) it will be ignored.
func (e *SafeEnv) AddUnique(k, v string) {
	e.add(true, k, v)
}

func (e *SafeEnv) add(preventDuplicates bool, k, v string) {
	k = strings.TrimSpace(k)
	v = strings.TrimSpace(v)
	if e.isUnsafeKey(preventDuplicates, k) {
		return
	}

	*e = append(*e, fmt.Sprintf("%s=%s", k, v))
}

// AddFullTrusted adds an exact value, in the KEY=VALUE format. This should only be used if they values are already
// combined.  When the values are separate the [Add] function is generally preferred.  This will not check for
// duplicates.
func (e *SafeEnv) AddFullTrusted(fullValues ...string) {
	e.addFull(false, fullValues)
}

// AddFullUnique adds an exact value, in the KEY=VALUE format. This should only be used if they values are already
// combined.  When the values are separate the [Add] function is generally preferred.  If any keys already exists
// (case-insensitive) they will be ignored.
func (e *SafeEnv) AddFullUnique(fullValues ...string) {
	e.addFull(true, fullValues)
}

func (e *SafeEnv) addFull(preventDuplicates bool, fullValues []string) {
	for _, kv := range fullValues {
		kv = strings.TrimSpace(kv)

		key := strings.SplitN(kv, "=", 2)[0]
		if e.isUnsafeKey(preventDuplicates, key) {
			continue
		}

		*e = append(*e, kv)
	}
}

func (e *SafeEnv) isUnsafeKey(preventDuplicates bool, key string) bool {
	if key == "" || key == "=" {
		return false
	}

	upperKey := strings.ToUpper(key)
	for _, prefix := range unsafeEnvironmentPrefixes {
		if strings.HasPrefix(upperKey, prefix) {
			return true
		}
	}

	if preventDuplicates {
		prefix := upperKey + "="
		for _, kv := range *e {
			if strings.HasPrefix(strings.ToUpper(kv), prefix) {
				return true
			}
		}
	}

	return false
}

// AddExecEnvironment will add safe values from [os.Environ], ignoring any duplicates that may have already been added.
func (e *SafeEnv) AddExecEnvironment() {
	e.addFull(true, os.Environ())
}
