// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// methodPaginatedPrefix is a list of RPC method name prefixes which should be paginated.
var methodPaginatedPrefix = []string{"List", "Search", "Get"}

// methodPrefixMustHaveRepeated is a prefix for methods that must have a repeated field without exception.
var methodPrefixMustHaveRepeated = "List"

const (
	// Naming based on rfd/0153-resource-guidelines.md
	paginationRequestSizeName       = "page_size"
	paginationRequestTokenName      = "page_token"
	paginationResponseNextTokenName = "next_page_token"
)

type fieldNames struct {
	size  string // Name of the max page size field in the request.
	token string // Name of the current page token field in the request.
	next  string // Name of the next page token field in the response.
}

type Config struct {
	// overwrites maps the full name of proto method to a struct of field names.
	overwrites map[protoreflect.FullName]fieldNames

	// skips contains entires of RPCs which should be skipped.
	skips map[protoreflect.FullName]struct{}
}

func (c *Config) getPageSizeFieldName(method protoreflect.MethodDescriptor) string {
	if overwrite, ok := c.overwrites[method.FullName()]; ok && overwrite.size != "" {
		return overwrite.size
	}
	return paginationRequestSizeName
}

func (c *Config) getPageFieldName(method protoreflect.MethodDescriptor) string {
	if overwrite, ok := c.overwrites[method.FullName()]; ok && overwrite.token != "" {
		return overwrite.token
	}
	return paginationRequestTokenName
}

func (c *Config) getNextPageFieldName(method protoreflect.MethodDescriptor) string {
	if overwrite, ok := c.overwrites[method.FullName()]; ok && overwrite.next != "" {
		return overwrite.next
	}
	return paginationResponseNextTokenName
}

func (c *Config) shouldSkip(method protoreflect.MethodDescriptor) bool {
	if _, skipped := c.skips[method.FullName()]; skipped {
		return true
	}

	if !hasAnyPrefix(string(method.Name()), methodPaginatedPrefix) {
		return true
	}

	return false
}

// hasAnyPrefix return true if given name starts with any of the given prefixes
func hasAnyPrefix(name string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
