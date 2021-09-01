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
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// TestStatementsCache verifies functionality of the cache that holds per-session
// prepared statements and their parameters.
func TestStatementsCache(t *testing.T) {
	cache := NewStatementsCache()

	// Full parse/bind/execute flow with an unnamed statement/portal.
	cache.Save(testUnnamedStatement1.Name, testUnnamedStatement1.Query)
	cache.Bind(testUnnamedStatement1.Name, testUnnamedPortal1.Name, testUnnamedPortal1.Parameters...)

	statement, err := cache.Get(unnamedStatement)
	require.NoError(t, err)
	require.Equal(t, testUnnamedStatement1, statement)

	portal, err := cache.GetPortal(unnamedPortal)
	require.NoError(t, err)
	require.Equal(t, testUnnamedPortal1, portal)

	// Make sure another unnamed statement replaces the previous one.
	cache.Save(testUnnamedStatement2.Name, testUnnamedStatement2.Query)
	cache.Bind(testUnnamedStatement2.Name, testUnnamedPortal2.Name, testUnnamedPortal2.Parameters...)

	statement, err = cache.Get(unnamedStatement)
	require.NoError(t, err)
	require.Equal(t, testUnnamedStatement2, statement)

	portal, err = cache.GetPortal(unnamedPortal)
	require.NoError(t, err)
	require.Equal(t, testUnnamedPortal2, portal)

	// Create a named statement and a couple of destination portals.
	cache.Save(testStatement.Name, testStatement.Query)
	cache.Bind(testStatement.Name, testPortal1.Name, testPortal1.Parameters...)
	cache.Bind(testStatement.Name, testPortal2.Name, testPortal2.Parameters...)

	statement, err = cache.Get(testStatement.Name)
	require.NoError(t, err)
	require.Equal(t, testStatement, statement)

	portal1, err := cache.GetPortal(testPortal1.Name)
	require.NoError(t, err)
	require.Equal(t, testPortal1, portal1)

	portal2, err := cache.GetPortal(testPortal2.Name)
	require.NoError(t, err)
	require.Equal(t, testPortal2, portal2)

	// Try to get a couple non-existent statements/portals.
	_, err = cache.Get("unknown")
	require.IsType(t, trace.NotFound(""), err)

	_, err = cache.GetPortal("unknown")
	require.IsType(t, trace.NotFound(""), err)

	// Close a portal.
	cache.RemovePortal(testPortal1.Name)

	_, err = cache.GetPortal(testPortal1.Name)
	require.IsType(t, trace.NotFound(""), err)

	// Close a statement and make sure its portal is gone as well.
	cache.Remove(testStatement.Name)

	_, err = cache.Get(testStatement.Name)
	require.IsType(t, trace.NotFound(""), err)

	_, err = cache.GetPortal(testPortal2.Name)
	require.IsType(t, trace.NotFound(""), err)
}

const (
	unnamedStatement = ""
	unnamedPortal    = ""
)

var (
	testQuery1         = "select * from test"
	testUnnamedPortal1 = &Portal{
		Name:       unnamedPortal,
		Query:      testQuery1,
		Parameters: []string{},
	}
	testUnnamedStatement1 = &Statement{
		Name:  unnamedStatement,
		Query: testQuery1,
		Portals: map[string]*Portal{
			unnamedPortal: testUnnamedPortal1,
		},
	}

	testQuery2         = "select * from test where id = $1"
	testUnnamedPortal2 = &Portal{
		Name:       unnamedPortal,
		Query:      testQuery2,
		Parameters: []string{"123"},
	}
	testUnnamedStatement2 = &Statement{
		Name:  unnamedStatement,
		Query: testQuery2,
		Portals: map[string]*Portal{
			unnamedPortal: testUnnamedPortal2,
		},
	}

	testQuery3  = "update test set value = $1 where id = $2"
	testPortal1 = &Portal{
		Name:       "P_1",
		Query:      testQuery3,
		Parameters: []string{"abc", "123"},
	}
	testPortal2 = &Portal{
		Name:       "P_2",
		Query:      testQuery3,
		Parameters: []string{"def", "456"},
	}
	testStatement = &Statement{
		Name:  "S_1",
		Query: testQuery3,
		Portals: map[string]*Portal{
			"P_1": testPortal1,
			"P_2": testPortal2,
		},
	}
)
