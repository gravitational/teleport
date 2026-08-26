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

package registry

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strconv"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows/registry"
)

// GetOrCreateRegistryKey loads or creates a registry key handle.
// The key handle must be released with Close() when it is no longer needed.
func GetOrCreateRegistryKey(name string) (registry.Key, error) {
	reg, err := registry.OpenKey(registry.CURRENT_USER, name, registry.QUERY_VALUE|registry.CREATE_SUB_KEY|registry.SET_VALUE)
	switch {
	case errors.Is(err, os.ErrNotExist):
		slog.DebugContext(context.Background(), "Registry key doesn't exist, trying to create it", "key_name", name)
		reg, _, err = registry.CreateKey(registry.CURRENT_USER, name, registry.QUERY_VALUE|registry.CREATE_SUB_KEY|registry.SET_VALUE)
		if err != nil {
			slog.DebugContext(context.Background(), "Can't create registry key",
				"key_name", name,
				"error", err,
			)
			return reg, err
		}
	case err != nil:
		slog.ErrorContext(context.Background(), "registry.OpenKey returned error", "error", err)
		return reg, err
	default:
		return reg, nil
	}
	return reg, nil
}

// WriteDword writes a DWORD value to the given registry key handle
func WriteDword(k registry.Key, name string, value string) error {
	dwordValue, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		slog.DebugContext(context.Background(), "Failed to convert value to uint32",
			"value", value,
			"error", err,
		)
		return trace.Wrap(err)
	}
	err = k.SetDWordValue(name, uint32(dwordValue))
	if err != nil {
		slog.DebugContext(context.Background(), "Failed to write dword to registry key",
			"name", name,
			"value", value,
			"key_name", k,
			"error", err,
		)
		return trace.Wrap(err)
	}
	return nil
}

// registryWriteString writes a string (SZ) value to the given registry key handle
func WriteString(k registry.Key, name string, value string) error {
	err := k.SetStringValue(name, value)
	if err != nil {
		slog.DebugContext(context.Background(), "Failed to write string to registry key",
			"name", name,
			"value", value,
			"key_name", k,
			"error", err,
		)
		return trace.Wrap(err)
	}
	return nil
}

// registryWriteMultiString writes a multi-string value (MULTI_SZ) to the given registry key handle
func WriteMultiString(k registry.Key, name string, values []string) error {
	err := k.SetStringsValue(name, values)
	if err != nil {
		slog.DebugContext(context.Background(), "Failed to write strings to registry key",
			"name", name,
			"values", values,
			"key_name", k,
			"error", err,
		)
		return trace.Wrap(err)
	}
	return nil
}
