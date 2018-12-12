/*
Copyright 2015 Gravitational, Inc.

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

package services

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	. "gopkg.in/check.v1"
)

var _ = fmt.Printf

type MigrationsSuite struct {
}

var _ = Suite(&MigrationsSuite{})

func (s *MigrationsSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *MigrationsSuite) TestMigrateServers(c *C) {
	in := &ServerV1{
		Kind:      KindNode,
		ID:        "id1",
		Addr:      "127.0.0.1:22",
		Hostname:  "localhost",
		Labels:    map[string]string{"a": "b", "c": "d"},
		CmdLabels: map[string]CommandLabelV1{"o": CommandLabelV1{Command: []string{"ls", "-l"}, Period: time.Second}},
	}

	out := in.V2()
	expected := &ServerV2{
		Kind:    KindNode,
		Version: V2,
		Metadata: Metadata{
			Name:      in.ID,
			Labels:    in.Labels,
			Namespace: defaults.Namespace,
		},
		Spec: ServerSpecV2{
			Addr:      in.Addr,
			Hostname:  in.Hostname,
			CmdLabels: map[string]CommandLabelV2{"o": CommandLabelV2{Command: []string{"ls", "-l"}, Period: NewDuration(time.Second)}},
		},
	}
	c.Assert(out, DeepEquals, expected)

	data, err := json.Marshal(in)
	c.Assert(err, IsNil)
	out2, err := GetServerMarshaler().UnmarshalServer(data, KindNode)
	c.Assert(err, IsNil)
	c.Assert(out2, DeepEquals, out)

	// check V2 marshaling
	data, err = GetServerMarshaler().MarshalServer(expected, WithVersion(V2))
	c.Assert(err, IsNil)
	out3, err := GetServerMarshaler().UnmarshalServer(data, KindNode)
	c.Assert(err, IsNil)
	c.Assert(out3, DeepEquals, expected)

	// check V1 marshaling
	data, err = GetServerMarshaler().MarshalServer(expected, WithVersion(V1))
	c.Assert(err, IsNil)

	var out4 ServerV1
	err = json.Unmarshal(data, &out4)
	out4.Namespace = ""
	c.Assert(err, IsNil)
	c.Assert(out4, DeepEquals, *in)
}

func (s *MigrationsSuite) TestMigrateReverseTunnels(c *C) {
	in := &ReverseTunnelV1{
		DomainName: "example.com",
		DialAddrs:  []string{"127.0.0.1:3245", "127.0.0.1:3450"},
	}

	out := in.V2()
	expected := &ReverseTunnelV2{
		Kind:    KindReverseTunnel,
		Version: V2,
		Metadata: Metadata{
			Name:      in.DomainName,
			Namespace: defaults.Namespace,
		},
		Spec: ReverseTunnelSpecV2{
			DialAddrs:   in.DialAddrs,
			ClusterName: in.DomainName,
		},
	}
	c.Assert(out, DeepEquals, expected)

	data, err := json.Marshal(in)
	c.Assert(err, IsNil)
	out2, err := GetReverseTunnelMarshaler().UnmarshalReverseTunnel(data)
	c.Assert(err, IsNil)
	c.Assert(out2, DeepEquals, expected)

	data, err = json.Marshal(expected)
	c.Assert(err, IsNil)
	out3, err := GetReverseTunnelMarshaler().UnmarshalReverseTunnel(data)
	c.Assert(err, IsNil)
	c.Assert(out3, DeepEquals, expected)

	// test backwards compatibility
	data, err = GetReverseTunnelMarshaler().MarshalReverseTunnel(expected, WithVersion(V1))
	c.Assert(err, IsNil)

	var out4 ReverseTunnelV1
	err = json.Unmarshal(data, &out4)
	c.Assert(err, IsNil)

	c.Assert(out4, DeepEquals, *in)
}

func (s *MigrationsSuite) TestMigrateCertAuthorities(c *C) {
	in := &CertAuthorityV1{
		Type:          UserCA,
		DomainName:    "example.com",
		CheckingKeys:  [][]byte{[]byte("checking key")},
		SigningKeys:   [][]byte{[]byte("signing key")},
		AllowedLogins: []string{"root", "admin"},
	}

	out := in.V2()
	expected := &CertAuthorityV2{
		Kind:    KindCertAuthority,
		Version: V2,
		Metadata: Metadata{
			Name:      in.DomainName,
			Namespace: defaults.Namespace,
		},
		Spec: CertAuthoritySpecV2{
			ClusterName:  in.DomainName,
			Type:         in.Type,
			CheckingKeys: in.CheckingKeys,
			SigningKeys:  in.SigningKeys,
		},
	}
	c.Assert(out, DeepEquals, expected)

	data, err := json.Marshal(in)
	c.Assert(err, IsNil)
	out2, err := GetCertAuthorityMarshaler().UnmarshalCertAuthority(data)
	c.Assert(err, IsNil)
	c.Assert(out2, DeepEquals, expected)

	// test backwards compatibility
	data, err = GetCertAuthorityMarshaler().MarshalCertAuthority(expected, WithVersion(V1))
	c.Assert(err, IsNil)

	var out3 CertAuthorityV1
	err = json.Unmarshal(data, &out3)
	c.Assert(err, IsNil)
	in.AllowedLogins = nil
	c.Assert(out3, DeepEquals, *in)
}
