/*
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package types

import (
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
)

func getTestVal(isTestField bool, testVal string) string {
	if isTestField {
		return testVal
	}

	return "foo"
}

func TestServerSorter(t *testing.T) {
	t.Parallel()

	testValsUnordered := []string{"d", "b", "a", "c"}

	makeServers := func(testVals []string, testField string) []Server {
		servers := make([]Server, len(testVals))
		for i := 0; i < len(testVals); i++ {
			testVal := testVals[i]
			var err error
			servers[i], err = NewServer(
				getTestVal(testField == ResourceMetadataName, testVal),
				KindNode,
				ServerSpecV2{
					Hostname: getTestVal(testField == ResourceSpecHostname, testVal),
					Addr:     getTestVal(testField == ResourceSpecAddr, testVal),
				})
			require.NoError(t, err)
		}
		return servers
	}

	cases := []struct {
		name      string
		wantErr   bool
		fieldName string
	}{
		{
			name:      "by name",
			fieldName: ResourceMetadataName,
		},
		{
			name:      "by hostname",
			fieldName: ResourceSpecHostname,
		},
		{
			name:      "by addr",
			fieldName: ResourceSpecAddr,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(fmt.Sprintf("%s desc", c.name), func(t *testing.T) {
			sortBy := SortBy{Field: c.fieldName, IsDesc: true}
			servers := Servers(makeServers(testValsUnordered, c.fieldName))
			require.NoError(t, servers.SortByCustom(sortBy))
			targetVals, err := servers.GetFieldVals(c.fieldName)
			require.NoError(t, err)
			require.IsDecreasing(t, targetVals)
		})

		t.Run(fmt.Sprintf("%s asc", c.name), func(t *testing.T) {
			sortBy := SortBy{Field: c.fieldName}
			servers := Servers(makeServers(testValsUnordered, c.fieldName))
			require.NoError(t, servers.SortByCustom(sortBy))
			targetVals, err := servers.GetFieldVals(c.fieldName)
			require.NoError(t, err)
			require.IsIncreasing(t, targetVals)
		})
	}

	// Test error.
	sortBy := SortBy{Field: "unsupported"}
	servers := makeServers(testValsUnordered, "does-not-matter")
	require.True(t, trace.IsNotImplemented(Servers(servers).SortByCustom(sortBy)))
}

func TestServerCheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	makeOpenSSHEC2InstanceConnectEndpointNode := func(fn func(s *ServerV2)) *ServerV2 {
		s := &ServerV2{
			Kind:    KindNode,
			SubKind: SubKindOpenSSHEICENode,
			Version: V2,
			Metadata: Metadata{
				Name:      "5da56852-2adb-4540-a37c-80790203f6a9",
				Namespace: defaults.Namespace,
			},
			Spec: ServerSpecV2{
				Addr:     "example:22",
				Hostname: "openssh-node",
				CloudMetadata: &CloudMetadata{
					AWS: &AWSInfo{
						AccountID:   "123456789012",
						InstanceID:  "i-123456789012",
						Region:      "us-east-1",
						VPCID:       "vpc-abcd",
						SubnetID:    "subnet-123",
						Integration: "teleportdev",
					},
				},
			},
		}
		if fn != nil {
			fn(s)
		}
		return s
	}

	tests := []struct {
		name      string
		server    *ServerV2
		assertion func(t *testing.T, s *ServerV2, err error)
	}{
		{
			name: "Teleport node",
			server: &ServerV2{
				Kind:    KindNode,
				SubKind: SubKindTeleportNode,
				Version: V2,
				Metadata: Metadata{
					Name:      "5da56852-2adb-4540-a37c-80790203f6a9",
					Namespace: defaults.Namespace,
				},
				Spec: ServerSpecV2{
					Addr:        "1.2.3.4:3022",
					Hostname:    "teleport-node",
					PublicAddrs: []string{"1.2.3.4:3080"},
				},
			},
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.NoError(t, err)
				expectedServer := &ServerV2{
					Kind:    KindNode,
					SubKind: SubKindTeleportNode,
					Version: V2,
					Metadata: Metadata{
						Name:      "5da56852-2adb-4540-a37c-80790203f6a9",
						Namespace: defaults.Namespace,
					},
					Spec: ServerSpecV2{
						Addr:        "1.2.3.4:3022",
						Hostname:    "teleport-node",
						PublicAddrs: []string{"1.2.3.4:3080"},
					},
				}
				require.Equal(t, expectedServer, s)
			},
		},
		{
			name: "Teleport node subkind unset",
			server: &ServerV2{
				Kind:    KindNode,
				Version: V2,
				Metadata: Metadata{
					Name:      "5da56852-2adb-4540-a37c-80790203f6a9",
					Namespace: defaults.Namespace,
				},
				Spec: ServerSpecV2{
					Addr:        "1.2.3.4:3022",
					Hostname:    "teleport-node",
					PublicAddrs: []string{"1.2.3.4:3080"},
				},
			},
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.NoError(t, err)
				expectedServer := &ServerV2{
					Kind:    KindNode,
					Version: V2,
					Metadata: Metadata{
						Name:      "5da56852-2adb-4540-a37c-80790203f6a9",
						Namespace: defaults.Namespace,
					},
					Spec: ServerSpecV2{
						Addr:        "1.2.3.4:3022",
						Hostname:    "teleport-node",
						PublicAddrs: []string{"1.2.3.4:3080"},
					},
				}
				require.Equal(t, expectedServer, s)
				require.False(t, s.IsOpenSSHNode(), "IsOpenSSHNode must be false for this node")
			},
		},
		{
			name: "OpenSSH node",
			server: &ServerV2{
				Kind:    KindNode,
				SubKind: SubKindOpenSSHNode,
				Version: V2,
				Metadata: Metadata{
					Name:      "5da56852-2adb-4540-a37c-80790203f6a9",
					Namespace: defaults.Namespace,
				},
				Spec: ServerSpecV2{
					Addr:     "1.2.3.4:3022",
					Hostname: "openssh-node",
				},
			},
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.NoError(t, err)
				expectedServer := &ServerV2{
					Kind:    KindNode,
					SubKind: SubKindOpenSSHNode,
					Version: V2,
					Metadata: Metadata{
						Name:      "5da56852-2adb-4540-a37c-80790203f6a9",
						Namespace: defaults.Namespace,
					},
					Spec: ServerSpecV2{
						Addr:     "1.2.3.4:3022",
						Hostname: "openssh-node",
					},
				}
				require.Equal(t, expectedServer, s)
			},
		},
		{
			name: "OpenSSH node with dns address",
			server: &ServerV2{
				Kind:    KindNode,
				SubKind: SubKindOpenSSHNode,
				Version: V2,
				Metadata: Metadata{
					Name:      "5da56852-2adb-4540-a37c-80790203f6a9",
					Namespace: defaults.Namespace,
				},
				Spec: ServerSpecV2{
					Addr:     "example:22",
					Hostname: "openssh-node",
				},
			},
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.NoError(t, err)
				expectedServer := &ServerV2{
					Kind:    KindNode,
					SubKind: SubKindOpenSSHNode,
					Version: V2,
					Metadata: Metadata{
						Name:      "5da56852-2adb-4540-a37c-80790203f6a9",
						Namespace: defaults.Namespace,
					},
					Spec: ServerSpecV2{
						Addr:     "example:22",
						Hostname: "openssh-node",
					},
				}
				require.Equal(t, expectedServer, s)
				require.True(t, s.IsOpenSSHNode(), "IsOpenSSHNode must be true for this node")
			},
		},
		{
			name: "OpenSSH node with unset name",
			server: &ServerV2{
				Kind:    KindNode,
				SubKind: SubKindOpenSSHNode,
				Version: V2,
				Spec: ServerSpecV2{
					Addr:     "1.2.3.4:22",
					Hostname: "openssh-node",
				},
			},
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, s.Metadata.Name)
				require.True(t, s.IsOpenSSHNode(), "IsOpenSSHNode must be true for this node")
			},
		},
		{
			name: "OpenSSH node with unset addr",
			server: &ServerV2{
				Kind:    KindNode,
				SubKind: SubKindOpenSSHNode,
				Version: V2,
				Metadata: Metadata{
					Name:      "5da56852-2adb-4540-a37c-80790203f6a9",
					Namespace: defaults.Namespace,
				},
				Spec: ServerSpecV2{
					Hostname: "openssh-node",
				},
			},
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.ErrorContains(t, err, "addr must be set")
			},
		},
		{
			name: "OpenSSH node with unset hostname",
			server: &ServerV2{
				Kind:    KindNode,
				SubKind: SubKindOpenSSHNode,
				Version: V2,
				Metadata: Metadata{
					Name:      "5da56852-2adb-4540-a37c-80790203f6a9",
					Namespace: defaults.Namespace,
				},
				Spec: ServerSpecV2{
					Addr: "1.2.3.4:3022",
				},
			},
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.ErrorContains(t, err, "hostname must be set")
			},
		},
		{
			name: "OpenSSH node with public addr",
			server: &ServerV2{
				Kind:    KindNode,
				SubKind: SubKindOpenSSHNode,
				Version: V2,
				Metadata: Metadata{
					Name:      "5da56852-2adb-4540-a37c-80790203f6a9",
					Namespace: defaults.Namespace,
				},
				Spec: ServerSpecV2{
					Addr:        "1.2.3.4:3022",
					Hostname:    "openssh-node",
					PublicAddrs: []string{"1.2.3.4:80"},
				},
			},
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.ErrorContains(t, err, "publicAddrs must not be set")
			},
		},
		{
			name: "OpenSSH node with invalid addr",
			server: &ServerV2{
				Kind:    KindNode,
				SubKind: SubKindOpenSSHNode,
				Version: V2,
				Metadata: Metadata{
					Name:      "5da56852-2adb-4540-a37c-80790203f6a9",
					Namespace: defaults.Namespace,
				},
				Spec: ServerSpecV2{
					Addr:     "invalid-addr",
					Hostname: "openssh-node",
				},
			},
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.ErrorContains(t, err, `invalid Addr "invalid-addr"`)
			},
		},
		{
			name: "node with invalid subkind",
			server: &ServerV2{
				Kind:    KindNode,
				SubKind: "invalid-subkind",
				Version: V2,
				Metadata: Metadata{
					Name:      "5da56852-2adb-4540-a37c-80790203f6a9",
					Namespace: defaults.Namespace,
				},
				Spec: ServerSpecV2{
					Addr:     "1.2.3.4:22",
					Hostname: "node",
				},
			},
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.EqualError(t, err, `invalid SubKind "invalid-subkind"`)
			},
		},
		{
			name: "OpenSSHEC2InstanceConnectEndpoint node without cloud metadata",
			server: makeOpenSSHEC2InstanceConnectEndpointNode(func(s *ServerV2) {
				s.Spec.CloudMetadata = nil
			}),
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.ErrorContains(t, err, "missing AWS CloudMetadata")
			},
		},
		{
			name: "OpenSSHEC2InstanceConnectEndpoint node with cloud metadata but missing aws info",
			server: makeOpenSSHEC2InstanceConnectEndpointNode(func(s *ServerV2) {
				s.Spec.CloudMetadata.AWS = nil
			}),
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.ErrorContains(t, err, "missing AWS CloudMetadata")
			},
		},
		{
			name: "OpenSSHEC2InstanceConnectEndpoint node with aws cloud metadata but missing accountid",
			server: makeOpenSSHEC2InstanceConnectEndpointNode(func(s *ServerV2) {
				s.Spec.CloudMetadata.AWS.AccountID = ""
			}),
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.ErrorContains(t, err, "missing AWS Account ID")
			},
		},
		{
			name: "OpenSSHEC2InstanceConnectEndpoint node with aws cloud metadata but missing instanceid",
			server: makeOpenSSHEC2InstanceConnectEndpointNode(func(s *ServerV2) {
				s.Spec.CloudMetadata.AWS.InstanceID = ""
			}),
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.ErrorContains(t, err, "missing AWS InstanceID")
			},
		},
		{
			name: "OpenSSHEC2InstanceConnectEndpoint node with aws cloud metadata but missing region",
			server: makeOpenSSHEC2InstanceConnectEndpointNode(func(s *ServerV2) {
				s.Spec.CloudMetadata.AWS.Region = ""
			}),
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.ErrorContains(t, err, "missing AWS Region")
			},
		},
		{
			name: "OpenSSHEC2InstanceConnectEndpoint node with aws cloud metadata but missing vpc id",
			server: &ServerV2{
				Kind:    KindNode,
				SubKind: SubKindOpenSSHEICENode,
				Version: V2,
				Metadata: Metadata{
					Name:      "5da56852-2adb-4540-a37c-80790203f6a9",
					Namespace: defaults.Namespace,
				},
				Spec: ServerSpecV2{
					Addr:     "example:22",
					Hostname: "openssh-node",
					CloudMetadata: &CloudMetadata{
						AWS: &AWSInfo{
							AccountID:   "123456789012",
							InstanceID:  "i-123456789012",
							Region:      "us-east-1",
							Integration: "teleportdev",
						},
					},
				},
			},
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.ErrorContains(t, err, "missing AWS VPC ID")
			},
		},
		{
			name: "OpenSSHEC2InstanceConnectEndpoint node with aws cloud metadata but missing integration",
			server: &ServerV2{
				Kind:    KindNode,
				SubKind: SubKindOpenSSHEICENode,
				Version: V2,
				Metadata: Metadata{
					Name:      "5da56852-2adb-4540-a37c-80790203f6a9",
					Namespace: defaults.Namespace,
				},
				Spec: ServerSpecV2{
					Addr:     "example:22",
					Hostname: "openssh-node",
					CloudMetadata: &CloudMetadata{
						AWS: &AWSInfo{
							AccountID:  "123456789012",
							InstanceID: "i-123456789012",
							Region:     "us-east-1",
							VPCID:      "vpc-abcd",
						},
					},
				},
			},
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.ErrorContains(t, err, "missing AWS OIDC Integration")
			},
		},
		{
			name:   "valid OpenSSHEC2InstanceConnectEndpoint node",
			server: makeOpenSSHEC2InstanceConnectEndpointNode(nil),
			assertion: func(t *testing.T, s *ServerV2, err error) {
				require.NoError(t, err)
				expectedServer := &ServerV2{
					Kind:    KindNode,
					SubKind: SubKindOpenSSHEICENode,
					Version: V2,
					Metadata: Metadata{
						Name:      "5da56852-2adb-4540-a37c-80790203f6a9",
						Namespace: defaults.Namespace,
					},
					Spec: ServerSpecV2{
						Addr:     "example:22",
						Hostname: "openssh-node",
						CloudMetadata: &CloudMetadata{
							AWS: &AWSInfo{
								AccountID:   "123456789012",
								InstanceID:  "i-123456789012",
								Region:      "us-east-1",
								VPCID:       "vpc-abcd",
								SubnetID:    "subnet-123",
								Integration: "teleportdev",
							},
						},
					},
				}
				assert.Equal(t, expectedServer, s)

				require.True(t, s.IsOpenSSHNode(), "IsOpenSSHNode must be true for this node")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.server.CheckAndSetDefaults()
			tt.assertion(t, tt.server, err)
		})
	}
}

func TestIsOpenSSHNodeSubKind(t *testing.T) {
	tests := []struct {
		name    string
		subkind string
		want    bool
	}{
		{
			name:    "openssh using EC2 Instance Connect Endpoint",
			subkind: SubKindOpenSSHEICENode,
			want:    true,
		},
		{
			name:    "openssh using raw sshd server",
			subkind: SubKindOpenSSHNode,
			want:    true,
		},
		{
			name:    "regular node",
			subkind: SubKindTeleportNode,
			want:    false,
		},
		{
			name:    "another value",
			subkind: "xyz",
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOpenSSHNodeSubKind(tt.subkind); got != tt.want {
				t.Errorf("IsOpenSSHNodeSubKind() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsEICE(t *testing.T) {
	tests := []struct {
		name   string
		server *ServerV2
		want   bool
	}{
		{
			name: "eice node with account and instance id labels is EICE",
			server: &ServerV2{
				SubKind: SubKindOpenSSHEICENode,
				Metadata: Metadata{
					Labels: map[string]string{
						AWSAccountIDLabel:  "123456789012",
						AWSInstanceIDLabel: "i-123",
					},
				},
			},
			want: true,
		},
		{
			name: "regular node not eice",
			server: &ServerV2{
				SubKind: SubKindTeleportNode,
			},
			want: false,
		},
		{
			name: "agentless openssh node is not eice",
			server: &ServerV2{
				SubKind: SubKindOpenSSHNode,
			},
			want: false,
		},
		{
			name: "eice node without account id label is not EICE",
			server: &ServerV2{
				SubKind: SubKindOpenSSHEICENode,
				Metadata: Metadata{
					Labels: map[string]string{
						AWSInstanceIDLabel: "i-123",
					},
				},
			},
			want: false,
		},
		{
			name: "eice node without instance id label is not EICE",
			server: &ServerV2{
				SubKind: SubKindOpenSSHEICENode,
				Metadata: Metadata{
					Labels: map[string]string{
						AWSAccountIDLabel: "123456789012",
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.server.IsEICE(); got != tt.want {
				t.Errorf("IsEICE() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCloudMetadataAWS(t *testing.T) {
	for _, tt := range []struct {
		name     string
		in       Server
		expected *AWSInfo
	}{
		{
			name: "no cloud metadata",
			in: &ServerV2{
				Spec: ServerSpecV2{},
			},
			expected: nil,
		},
		{
			name: "cloud metadata but no AWS Information",
			in: &ServerV2{
				Spec: ServerSpecV2{CloudMetadata: &CloudMetadata{}},
			},
			expected: nil,
		},
		{
			name: "cloud metadata with aws info",
			in: &ServerV2{
				Spec: ServerSpecV2{CloudMetadata: &CloudMetadata{
					AWS: &AWSInfo{
						AccountID: "abcd",
					},
				}},
			},
			expected: &AWSInfo{AccountID: "abcd"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out := tt.in.GetAWSInfo()
			require.Equal(t, tt.expected, out)
		})
	}
}
