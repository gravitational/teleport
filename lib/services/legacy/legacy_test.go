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

package legacy

import (
	"crypto/rsa"
	"crypto/x509/pkix"
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

func TestLegacy(t *testing.T) { check.TestingT(t) }

var _ = fmt.Printf

type LegacySuite struct {
}

var _ = check.Suite(&LegacySuite{})

func (s *LegacySuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

// TestProtoServer tests that protobuf server is compatible with
// legacy proto server on the wire
func (s *LegacySuite) TestProtoServer(c *check.C) {
	in := &ServerV2{
		Kind:    services.KindNode,
		Version: services.V2,
		Metadata: Metadata{
			Name:      "id1",
			Labels:    map[string]string{"a": "b", "c": "d"},
			Namespace: defaults.Namespace,
		},
		Spec: ServerSpecV2{
			Addr:      "127.0.0.1:22",
			Hostname:  "localhost",
			CmdLabels: map[string]CommandLabelV2{"o": CommandLabelV2{Command: []string{"ls", "-l"}, Period: NewDuration(time.Second)}},
			Rotation: Rotation{
				State:       services.RotationStateInProgress,
				Phase:       services.RotationPhaseUpdateClients,
				CurrentID:   "1",
				Started:     time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
				GracePeriod: NewDuration(3 * time.Hour),
				LastRotated: time.Date(2017, 2, 3, 4, 5, 6, 7, time.UTC),
				Schedule: RotationSchedule{
					UpdateClients: time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
					UpdateServers: time.Date(2018, 3, 4, 7, 6, 7, 8, time.UTC),
					Standby:       time.Date(2018, 3, 4, 5, 6, 13, 8, time.UTC),
				},
			},
		},
	}
	data, err := utils.FastMarshal(in)
	c.Assert(err, check.IsNil)
	c.Assert(data, check.NotNil)

	out, err := services.GetServerMarshaler().UnmarshalServer(data, services.KindNode)
	c.Assert(err, check.IsNil)

	expected := &services.ServerV2{
		Kind:    services.KindNode,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      "id1",
			Labels:    map[string]string{"a": "b", "c": "d"},
			Namespace: defaults.Namespace,
		},
		Spec: services.ServerSpecV2{
			Addr:      "127.0.0.1:22",
			Hostname:  "localhost",
			CmdLabels: map[string]services.CommandLabelV2{"o": services.CommandLabelV2{Command: []string{"ls", "-l"}, Period: services.Duration(time.Second)}},
			Rotation: services.Rotation{
				State:       services.RotationStateInProgress,
				Phase:       services.RotationPhaseUpdateClients,
				CurrentID:   "1",
				Started:     time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
				GracePeriod: services.Duration(3 * time.Hour),
				LastRotated: time.Date(2017, 2, 3, 4, 5, 6, 7, time.UTC),
				Schedule: services.RotationSchedule{
					UpdateClients: time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
					UpdateServers: time.Date(2018, 3, 4, 7, 6, 7, 8, time.UTC),
					Standby:       time.Date(2018, 3, 4, 5, 6, 13, 8, time.UTC),
				},
			},
		},
	}

	fixtures.DeepCompare(c, out, expected)
}

// TestProtoCA tests compatibility with old certificate
// authority with a new one
func (s *LegacySuite) TestProtoCA(c *check.C) {
	keyBytes := fixtures.PEMBytes["rsa"]
	rsaKey, err := ssh.ParseRawPrivateKey(keyBytes)
	c.Assert(err, check.IsNil)

	signer, err := ssh.NewSignerFromKey(rsaKey)
	c.Assert(err, check.IsNil)

	clusterName := "example.com"
	key, cert, err := tlsca.GenerateSelfSignedCAWithPrivateKey(rsaKey.(*rsa.PrivateKey), pkix.Name{
		CommonName:   clusterName,
		Organization: []string{clusterName},
	}, nil, defaults.CATTL)
	c.Assert(err, check.IsNil)

	in := &CertAuthorityV2{
		Kind:    services.KindCertAuthority,
		Version: services.V2,
		Metadata: Metadata{
			Name:      "ca1",
			Labels:    map[string]string{"a": "b", "c": "d"},
			Namespace: defaults.Namespace,
		},
		Spec: CertAuthoritySpecV2{
			Type:         services.HostCA,
			ClusterName:  clusterName,
			CheckingKeys: [][]byte{ssh.MarshalAuthorizedKey(signer.PublicKey())},
			SigningKeys:  [][]byte{keyBytes},
			TLSKeyPairs:  []TLSKeyPair{{Cert: cert, Key: key}},
			Rotation: &Rotation{
				State:       services.RotationStateInProgress,
				Phase:       services.RotationPhaseUpdateClients,
				CurrentID:   "1",
				Started:     time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
				GracePeriod: NewDuration(3 * time.Hour),
				LastRotated: time.Date(2017, 2, 3, 4, 5, 6, 7, time.UTC),
				Schedule: RotationSchedule{
					UpdateClients: time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
					UpdateServers: time.Date(2018, 3, 4, 7, 6, 7, 8, time.UTC),
					Standby:       time.Date(2018, 3, 4, 5, 6, 13, 8, time.UTC),
				},
			},
		},
	}
	data, err := utils.FastMarshal(in)
	c.Assert(err, check.IsNil)
	c.Assert(data, check.NotNil)

	out, err := services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(data)
	c.Assert(err, check.IsNil)

	expected := &services.CertAuthorityV2{
		Kind:    services.KindCertAuthority,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      "ca1",
			Labels:    map[string]string{"a": "b", "c": "d"},
			Namespace: defaults.Namespace,
		},
		Spec: services.CertAuthoritySpecV2{
			Type:         services.HostCA,
			ClusterName:  clusterName,
			CheckingKeys: [][]byte{ssh.MarshalAuthorizedKey(signer.PublicKey())},
			SigningKeys:  [][]byte{keyBytes},
			TLSKeyPairs:  []services.TLSKeyPair{{Cert: cert, Key: key}},
			Rotation: &services.Rotation{
				State:       services.RotationStateInProgress,
				Phase:       services.RotationPhaseUpdateClients,
				CurrentID:   "1",
				Started:     time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
				GracePeriod: services.Duration(3 * time.Hour),
				LastRotated: time.Date(2017, 2, 3, 4, 5, 6, 7, time.UTC),
				Schedule: services.RotationSchedule{
					UpdateClients: time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
					UpdateServers: time.Date(2018, 3, 4, 7, 6, 7, 8, time.UTC),
					Standby:       time.Date(2018, 3, 4, 5, 6, 13, 8, time.UTC),
				},
			},
		},
	}
	fixtures.DeepCompare(c, out, expected)
}
