//go:build windows

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

package main

import (
	"strconv"

	"golang.org/x/sys/windows/registry"
)

// getRegistryKey loads or creates a registry key handle and passes it back
func getRegistryKey(name string) (registry.Key, error) {
	// now check for and create the individual session key
	reg, err := registry.OpenKey(registry.CURRENT_USER, name, registry.QUERY_VALUE|registry.CREATE_SUB_KEY|registry.SET_VALUE)
	if err != nil {
		log.Debugf("Registry key %v doesn't exist, trying to create it", name)
		reg, _, err = registry.CreateKey(registry.CURRENT_USER, name, registry.QUERY_VALUE|registry.CREATE_SUB_KEY|registry.SET_VALUE)
		if err != nil {
			log.Debugf("Can't create registry key %v", name)
			return reg, err
		}
	}
	return reg, nil
}

// mustWriteDword writes DWORD value to the given registry key handle
func mustWriteDword(k registry.Key, name string, value string) {
	dwordValue, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		log.Debugf("Failed to convert value %v to uint32: %T", value, err)
		log.Fatal(err)
	}
	err = k.SetDWordValue(name, uint32(dwordValue))
	if err != nil {
		log.Debugf("Failed to write dword %v: %v to registry key %v: %T", name, value, k, err)
		log.Fatal(err)
	}
}

// mustWriteString writes a string value (SZ) to the given registry key handle
func mustWriteString(k registry.Key, name string, value string) {
	err := k.SetStringValue(name, value)
	if err != nil {
		log.Debugf("Failed to write string %v: %v to registry key %v: %T", name, value, k, err)
		log.Fatal(err)
	}
}

// mustWriteStrings writes a multi-string value (MULTI_SZ) to the given registry key handle
func mustWriteStrings(k registry.Key, name string, values []string) {
	err := k.SetStringsValue(name, values)
	if err != nil {
		log.Debugf("Failed to write strings %v: %v to registry key %v: %T", name, values, k, err)
		log.Fatal(err)
	}
}
