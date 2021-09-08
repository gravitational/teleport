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

package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestDatabaseServerGetDatabase verifies that older agents get adapted to
// the new database server interface for backward compatibility.
//
// DELETE IN 9.0.
func TestDatabaseServerGetDatabase(t *testing.T) {
	server, err := NewDatabaseServerV3(Metadata{
		Name:   "server-1",
		Labels: map[string]string{"a": "b"},
	}, DatabaseServerSpecV3{
		Description:   "description",
		Protocol:      "postgres",
		URI:           "localhost:5432",
		CACert:        []byte("cert"),
		AWS:           AWS{Region: "us-east-1", Redshift: Redshift{ClusterID: "cluster-1"}},
		Version:       "1.0.0",
		Hostname:      "host",
		HostID:        "host-1",
		DynamicLabels: map[string]CommandLabelV2{"c": {Period: Duration(time.Minute), Command: []string{"/bin/date"}}},
		GCP:           GCPCloudSQL{ProjectID: "project-1", InstanceID: "instance-1"},
	})
	require.NoError(t, err)
	database, err := NewDatabaseV3(Metadata{
		Name:        "server-1",
		Description: "description",
		Labels:      map[string]string{"a": "b"},
	}, DatabaseSpecV3{
		Protocol:      "postgres",
		URI:           "localhost:5432",
		CACert:        "cert",
		AWS:           AWS{Region: "us-east-1", Redshift: Redshift{ClusterID: "cluster-1"}},
		DynamicLabels: map[string]CommandLabelV2{"c": {Period: Duration(time.Minute), Command: []string{"/bin/date"}}},
		GCP:           GCPCloudSQL{ProjectID: "project-1", InstanceID: "instance-1"},
	})
	require.NoError(t, err)
	require.Equal(t, database, server.GetDatabase())
}
