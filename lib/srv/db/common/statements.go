/*
Copyright 2021 Gravitational, Inc.

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

package common

import (
	"sync"

	"github.com/gravitational/trace"
)

// StatementsCache contains prepared statements client executes during a
// database session.
//
// Currently only used to support Postgres extended protocol message flow
// to capture prepared statement queries in the audit log:
//
// https://www.postgresql.org/docs/10/protocol-flow.html#PROTOCOL-FLOW-EXT-QUERY
type StatementsCache struct {
	// cache maps prepared statement name to the statement itself.
	cache map[string]*Statement
	// mu is used to synchronize cache access.
	mu sync.RWMutex
}

// Statement represents a prepared statement.
type Statement struct {
	// Name is the prepared statement name.
	//
	// Can be empty (Postgres "unnamed statement").
	Name string
	// Query is the statement query string.
	Query string
	// Portals contains "destination portals" that bind prepared statement to
	// parameters.
	//
	// In Postgres extended query protocol, clients execute these "portals"
	// and not prepared statements directly.
	Portals map[string]*Portal
}

// Portal represents a destination portal that binds a prepared statement
// to parameters.
type Portal struct {
	// Name is the portal name.
	//
	// Can be empty (Postgres "unnamed portal").
	Name string
	// Query is the prepared statement query string.
	Query string
	// Parameters are the query parameters.
	Parameters []string
}

// NewStatementsCache returns a new instance of prepared statements cache.
func NewStatementsCache() *StatementsCache {
	return &StatementsCache{cache: make(map[string]*Statement)}
}

// Save adds the provided prepared statement information to the cache.
func (s *StatementsCache) Save(statementName, query string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[statementName] = &Statement{
		Name:    statementName,
		Query:   query,
		Portals: make(map[string]*Portal),
	}
}

// Get returns the specified prepared statement from the cache.
func (s *StatementsCache) Get(statementName string) (*Statement, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if statement, ok := s.cache[statementName]; ok {
		return statement, nil
	}
	return nil, trace.NotFound("prepared statement %q is not in cache", statementName)
}

// GetPortal returns the specified destination portal from the cache.
func (s *StatementsCache) GetPortal(portalName string) (*Portal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, statement := range s.cache {
		if portal, ok := statement.Portals[portalName]; ok {
			return portal, nil
		}
	}
	return nil, trace.NotFound("destination portal %q is not in cache", portalName)
}

// Bind adds the provided destination portal to the cache.
func (s *StatementsCache) Bind(statementName, portalName string, parameters ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, statement := range s.cache {
		if name == statementName {
			s.cache[name].Portals[portalName] = &Portal{
				Name:       portalName,
				Query:      statement.Query,
				Parameters: parameters,
			}
			return nil
		}
	}
	return trace.NotFound("prepared statement %q is not in cache", statementName)
}

// Remove removes the specified prepared statement from the cache, along with
// all its destination portals.
func (s *StatementsCache) Remove(statementName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, statementName)
}

// RemovePortal removes the specified destination portal from the cache.
func (s *StatementsCache) RemovePortal(portalName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for statementName, statement := range s.cache {
		if _, ok := statement.Portals[portalName]; ok {
			delete(s.cache[statementName].Portals, portalName)
			return
		}
	}
}
