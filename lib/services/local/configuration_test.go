/*
Copyright 2017-2018 Gravitational, Inc.

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

package local

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"

	"gopkg.in/check.v1"
	"gopkg.in/yaml.v2"
)

type ClusterConfigurationSuite struct {
	bk backend.Backend
}

var _ = check.Suite(&ClusterConfigurationSuite{})

func (s *ClusterConfigurationSuite) SetUpTest(c *check.C) {
	var err error
	s.bk, err = lite.New(context.TODO(), backend.Params{"path": c.MkDir()})
	c.Assert(err, check.IsNil)
}

func (s *ClusterConfigurationSuite) TearDownTest(c *check.C) {
	c.Assert(s.bk.Close(), check.IsNil)
}

func (s *ClusterConfigurationSuite) TestAuthPreference(c *check.C) {
	clusterConfig, err := NewClusterConfigurationService(s.bk)
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		ConfigS: clusterConfig,
	}
	suite.AuthPreference(c)
}

func (s *ClusterConfigurationSuite) TestClusterConfig(c *check.C) {
	clusterConfig, err := NewClusterConfigurationService(s.bk)
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		ConfigS: clusterConfig,
	}
	suite.ClusterConfig(c)
}

func (s *ClusterConfigurationSuite) TestClusterNetworkingConfig(c *check.C) {
	clusterConfig, err := NewClusterConfigurationService(s.bk)
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		ConfigS: clusterConfig,
	}
	suite.ClusterNetworkingConfig(c)
}

func (s *ClusterConfigurationSuite) TestStaticTokens(c *check.C) {
	clusterConfig, err := NewClusterConfigurationService(s.bk)
	c.Assert(err, check.IsNil)

	suite := &suite.ServicesTestSuite{
		ConfigS: clusterConfig,
	}
	suite.StaticTokens(c)
}

func (s *ClusterConfigurationSuite) TestSessionRecording(c *check.C) {
	// don't allow invalid session recording values
	_, err := types.NewSessionRecordingConfig(types.SessionRecordingConfigSpecV2{
		Mode: "foo",
	})
	c.Assert(err, check.NotNil)

	// default is to record at the node
	recConfig, err := types.NewSessionRecordingConfig(types.SessionRecordingConfigSpecV2{})
	c.Assert(err, check.IsNil)
	c.Assert(recConfig.GetMode(), check.Equals, types.RecordAtNode)

	// update sessions to be recorded at the proxy and check again
	recConfig.SetMode(types.RecordAtProxy)
	c.Assert(recConfig.GetMode(), check.Equals, types.RecordAtProxy)
}

func (s *ClusterConfigurationSuite) TestAuditConfig(c *check.C) {
	// default is to record at the node
	clusterConfig, err := types.NewClusterConfig(types.ClusterConfigSpecV3{})
	c.Assert(err, check.IsNil)

	cfg := clusterConfig.GetAuditConfig()
	c.Assert(cfg, check.DeepEquals, types.AuditConfig{})

	// update sessions to be recorded at the proxy and check again
	in := types.AuditConfig{
		Region:           "us-west-1",
		Type:             "dynamodb",
		AuditSessionsURI: "file:///home/log",
		AuditTableName:   "audit_table_name",
		AuditEventsURI:   []string{"dynamodb://audit_table_name", "file:///home/log"},
	}
	clusterConfig.SetAuditConfig(in)
	out := clusterConfig.GetAuditConfig()
	fixtures.DeepCompare(c, out, in)

	config := `
region: 'us-west-1'
type: 'dynamodb'
audit_sessions_uri: file:///home/log
audit_table_name: audit_table_name
audit_events_uri: ['dynamodb://audit_table_name', 'file:///home/log']
`
	var data map[string]interface{}
	err = yaml.Unmarshal([]byte(config), &data)
	c.Assert(err, check.IsNil)

	out2, err := services.AuditConfigFromObject(data)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, *out2, in)

	config = `
region: 'us-west-1'
type: 'dir'
audit_sessions_uri: file:///home/log
audit_events_uri: 'dynamodb://audit_table_name'
`
	data = nil
	err = yaml.Unmarshal([]byte(config), &data)
	c.Assert(err, check.IsNil)

	out2, err = services.AuditConfigFromObject(data)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, *out2, types.AuditConfig{
		Region:           "us-west-1",
		Type:             "dir",
		AuditSessionsURI: "file:///home/log",
		AuditEventsURI:   []string{"dynamodb://audit_table_name"},
	})
}

func (s *ClusterConfigurationSuite) TestClusterConfigMarshal(c *check.C) {
	// signle audit_events uri value
	clusterConfig, err := types.NewClusterConfig(types.ClusterConfigSpecV3{
		DisconnectExpiredCert: types.NewBool(true),
		ClusterID:             "27",
		Audit: types.AuditConfig{
			Region:           "us-west-1",
			Type:             "dynamodb",
			AuditSessionsURI: "file:///home/log",
			AuditTableName:   "audit_table_name",
			AuditEventsURI:   []string{"dynamodb://audit_table_name"},
		},
	})
	c.Assert(err, check.IsNil)

	data, err := services.MarshalClusterConfig(clusterConfig)
	c.Assert(err, check.IsNil)

	out, err := services.UnmarshalClusterConfig(data)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, clusterConfig, out)

	// multiple events uri values
	clusterConfig, err = types.NewClusterConfig(types.ClusterConfigSpecV3{
		DisconnectExpiredCert: types.NewBool(true),
		ClusterID:             "27",
		Audit: types.AuditConfig{
			Region:           "us-west-1",
			Type:             "dynamodb",
			AuditSessionsURI: "file:///home/log",
			AuditTableName:   "audit_table_name",
			AuditEventsURI:   []string{"dynamodb://audit_table_name", "file:///home/test/log"},
		},
	})
	c.Assert(err, check.IsNil)

	data, err = services.MarshalClusterConfig(clusterConfig)
	c.Assert(err, check.IsNil)

	out, err = services.UnmarshalClusterConfig(data)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, clusterConfig, out)
}
