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

package registry

import (
	"errors"
	"os"
	"strconv"

	"golang.org/x/sys/windows/registry"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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
