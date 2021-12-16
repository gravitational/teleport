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

package watchers

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/gravitational/teleport/lib/services"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestAuroraMySQLVersion(t *testing.T) {
	require.Equal(t, "5.6.10a", auroraMySQLVersion(&rds.DBCluster{EngineVersion: aws.String("5.6.10a")}))
	require.Equal(t, "1.22.1", auroraMySQLVersion(&rds.DBCluster{EngineVersion: aws.String("5.6.mysql_aurora.1.22.1")}))
	require.Equal(t, "1.22.1.3", auroraMySQLVersion(&rds.DBCluster{EngineVersion: aws.String("5.6.mysql_aurora.1.22.1.3")}))
}

func TestIsDBClusterSupported(t *testing.T) {
	logger := logrus.WithField("test", t.Name())

	tests := []struct {
		name          string
		engineMode    string
		engineVersion string
		isSupported   bool
	}{
		{"provisioned", RDSEngineModeProvisioned, "5.6.mysql_aurora.1.22.0", true},
		{"serverless", RDSEngineModeServerless, "5.6.mysql_aurora.1.22.0", false},
		{"parallel query supported", RDSEngineModeParallelQuery, "5.6.mysql_aurora.1.22.0", true},
		{"parallel query unsupported", RDSEngineModeParallelQuery, "5.6.mysql_aurora.1.19.6", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cluster := &rds.DBCluster{
				DBClusterArn:        aws.String("arn:aws:rds:us-east-1:1234567890:cluster:test"),
				DBClusterIdentifier: aws.String(test.name),
				DbClusterResourceId: aws.String(uuid.New()),
				Engine:              aws.String(services.RDSEngineAuroraMySQL),
				EngineMode:          aws.String(test.engineMode),
				EngineVersion:       aws.String(test.engineVersion),
			}

			require.Equal(t, test.isSupported, isDBClusterSupported(cluster, logger))

		})
	}
}
