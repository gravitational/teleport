/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package local

import (
	"sync"

	"github.com/gravitational/teleport/api/types"
)

// GeneratedResourceParserFactory builds resource parsers for a watch kind.
type GeneratedResourceParserFactory func(kind types.WatchKind) (resourceParser, error)

var generatedResourceParsers struct {
	mu sync.RWMutex
	m  map[string]GeneratedResourceParserFactory
}

// RegisterGeneratedResourceParser registers a parser factory for a resource kind.
//
// This function is intended to be called from generated files using init().
func RegisterGeneratedResourceParser(kind string, factory GeneratedResourceParserFactory) {
	if kind == "" {
		panic("local: parser kind is required")
	}
	if factory == nil {
		panic("local: parser factory is nil")
	}

	generatedResourceParsers.mu.Lock()
	defer generatedResourceParsers.mu.Unlock()

	if generatedResourceParsers.m == nil {
		generatedResourceParsers.m = make(map[string]GeneratedResourceParserFactory)
	}
	if _, exists := generatedResourceParsers.m[kind]; exists {
		panic("local: duplicate generated parser for kind " + kind)
	}
	generatedResourceParsers.m[kind] = factory
}

func generatedResourceParserFactory(kind string) (GeneratedResourceParserFactory, bool) {
	generatedResourceParsers.mu.RLock()
	defer generatedResourceParsers.mu.RUnlock()

	f, ok := generatedResourceParsers.m[kind]
	return f, ok
}
