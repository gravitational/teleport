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

package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestFetchDatabaseCreds makes sure fetching database credentials does not
// trigger an error when there's no logged in profile.
func TestFetchDatabaseCreds(t *testing.T) {
	var cf CLIConf
	cf.UserHost = "localhost"
	// Randomize proxy name to make sure there's no profile entry.
	cf.Proxy = uuid.New().String()

	tc, err := makeClient(&cf, true)
	require.NoError(t, err)

	err = fetchDatabaseCreds(&cf, tc)
	require.NoError(t, err)
}
