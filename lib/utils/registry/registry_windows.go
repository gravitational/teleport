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
	"errors"
	"os"
	"strconv"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/registry"
)

// GetOrCreateRegistryKey loads or creates a registry key handle.
// The key handle must be released with Close() when it is no longer needed.
func GetOrCreateRegistryKey(name string) (registry.Key, error) {
	reg, err := registry.OpenKey(registry.CURRENT_USER, name, registry.QUERY_VALUE|registry.CREATE_SUB_KEY|registry.SET_VALUE)
	switch {
	case errors.Is(err, os.ErrNotExist):
		log.Debugf("Registry key %v doesn't exist, trying to create it", name)
		reg, _, err = registry.CreateKey(registry.CURRENT_USER, name, registry.QUERY_VALUE|registry.CREATE_SUB_KEY|registry.SET_VALUE)
		if err != nil {
			log.Debugf("Can't create registry key %v: %v", name, err)
			return reg, err
		}
	case err != nil:
		log.Errorf("registry.OpenKey returned error: %v", err)
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
		log.Debugf("Failed to convert value %v to uint32: %v", value, err)
		return trace.Wrap(err)
	}
	err = k.SetDWordValue(name, uint32(dwordValue))
	if err != nil {
		log.Debugf("Failed to write dword %v: %v to registry key %v: %v", name, value, k, err)
		return trace.Wrap(err)
	}
	return nil
}

// registryWriteString writes a string (SZ) value to the given registry key handle
func WriteString(k registry.Key, name string, value string) error {
	err := k.SetStringValue(name, value)
	if err != nil {
		log.Debugf("Failed to write string %v: %v to registry key %v: %v", name, value, k, err)
		return trace.Wrap(err)
	}
	return nil
}

// registryWriteMultiString writes a multi-string value (MULTI_SZ) to the given registry key handle
func WriteMultiString(k registry.Key, name string, values []string) error {
	err := k.SetStringsValue(name, values)
	if err != nil {
		log.Debugf("Failed to write strings %v: %v to registry key %v: %v", name, values, k, err)
		return trace.Wrap(err)
	}
	return nil
}
