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

package cloudsql

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

func ipMapping(ipType, addr string) *sqladmin.IpMapping {
	return &sqladmin.IpMapping{Type: ipType, IpAddress: addr}
}

func dnsMapping(name, connType, scope string) *sqladmin.DnsNameMapping {
	return &sqladmin.DnsNameMapping{Name: name, ConnectionType: connType, DnsScope: scope}
}

func TestCheckInstanceAvailable(t *testing.T) {
	t.Parallel()
	withPolicy := func(policy string) *sqladmin.Settings {
		return &sqladmin.Settings{ActivationPolicy: policy}
	}
	tests := []struct {
		name       string
		state      string
		settings   *sqladmin.Settings
		wantReason string
	}{
		{name: "runnable", state: "RUNNABLE"},
		{name: "runnable always", state: "RUNNABLE", settings: withPolicy("ALWAYS")},
		{name: "runnable on demand", state: "RUNNABLE", settings: withPolicy("ON_DEMAND")},
		{name: "runnable unspecified policy", state: "RUNNABLE", settings: withPolicy("")},
		{name: "stopped by owner", state: "RUNNABLE", settings: withPolicy("NEVER"), wantReason: `instance is stopped (activation policy "NEVER")`},
		{name: "suspended", state: "SUSPENDED", wantReason: `instance is not available (state "SUSPENDED")`},
		{name: "empty", state: "", wantReason: `instance is not available (state "")`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ok, reason := checkInstanceAvailable(&sqladmin.DatabaseInstance{State: tt.state, Settings: tt.settings})
			require.Equal(t, tt.wantReason == "", ok)
			require.Equal(t, tt.wantReason, reason)
		})
	}
}

func TestProtocolAndPort(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		version      string
		wantProtocol string
		wantPort     string
		wantOK       bool
	}{
		{name: "postgres", version: "POSTGRES_14", wantProtocol: defaults.ProtocolPostgres, wantPort: "5432", wantOK: true},
		{name: "mysql", version: "MYSQL_8_0", wantProtocol: defaults.ProtocolMySQL, wantPort: "3306", wantOK: true},
		{name: "sqlserver unsupported", version: "SQLSERVER_2019_STANDARD", wantOK: false},
		{name: "prefix without underscore", version: "POSTGRES", wantOK: false},
		{name: "case sensitive", version: "mysql_8_0", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			protocol, port, ok := protocolAndPort(tt.version)
			require.Equal(t, tt.wantOK, ok)
			require.Equal(t, tt.wantProtocol, protocol)
			require.Equal(t, tt.wantPort, port)
		})
	}
}

func TestPSCEnabled(t *testing.T) {
	t.Parallel()
	enabled := func(b bool) *sqladmin.Settings {
		return &sqladmin.Settings{
			IpConfiguration: &sqladmin.IpConfiguration{PscConfig: &sqladmin.PscConfig{PscEnabled: b}},
		}
	}
	tests := []struct {
		name     string
		settings *sqladmin.Settings
		want     bool
	}{
		{name: "nil settings", settings: nil, want: false},
		{name: "nil ip configuration", settings: &sqladmin.Settings{}, want: false},
		{name: "nil psc config", settings: &sqladmin.Settings{IpConfiguration: &sqladmin.IpConfiguration{}}, want: false},
		{name: "psc disabled", settings: enabled(false), want: false},
		{name: "psc enabled", settings: enabled(true), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, pscEnabled(&sqladmin.DatabaseInstance{Settings: tt.settings}))
		})
	}
}

func TestFindInstanceEndpoints(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		instance *sqladmin.DatabaseInstance
		want     instanceEndpoints
	}{
		{
			name:     "public DNS name",
			instance: &sqladmin.DatabaseInstance{DnsNames: []*sqladmin.DnsNameMapping{dnsMapping("pub.example", "PUBLIC", "INSTANCE")}},
			want:     instanceEndpoints{public: "pub.example"},
		},
		{
			name:     "PSA DNS name maps to private",
			instance: &sqladmin.DatabaseInstance{DnsNames: []*sqladmin.DnsNameMapping{dnsMapping("priv.example", "PRIVATE_SERVICES_ACCESS", "INSTANCE")}},
			want:     instanceEndpoints{private: "priv.example"},
		},
		{
			name:     "PSC DNS name ignored when PSC disabled",
			instance: &sqladmin.DatabaseInstance{DnsNames: []*sqladmin.DnsNameMapping{dnsMapping("psc.example", "PRIVATE_SERVICE_CONNECT", "INSTANCE")}},
			want:     instanceEndpoints{},
		},
		{
			name: "PSC DNS name used when PSC enabled",
			instance: &sqladmin.DatabaseInstance{
				DnsNames: []*sqladmin.DnsNameMapping{dnsMapping("psc.example", "PRIVATE_SERVICE_CONNECT", "INSTANCE")},
				Settings: &sqladmin.Settings{IpConfiguration: &sqladmin.IpConfiguration{PscConfig: &sqladmin.PscConfig{PscEnabled: true}}},
			},
			want: instanceEndpoints{psc: "psc.example"},
		},
		{
			name:     "non-INSTANCE scope ignored",
			instance: &sqladmin.DatabaseInstance{DnsNames: []*sqladmin.DnsNameMapping{dnsMapping("pub.example", "PUBLIC", "REGIONAL")}},
			want:     instanceEndpoints{},
		},
		{
			name:     "empty DNS name ignored",
			instance: &sqladmin.DatabaseInstance{DnsNames: []*sqladmin.DnsNameMapping{dnsMapping("", "PUBLIC", "INSTANCE")}},
			want:     instanceEndpoints{},
		},
		{
			name: "IP fallback for primary and private",
			instance: &sqladmin.DatabaseInstance{IpAddresses: []*sqladmin.IpMapping{
				ipMapping("PRIMARY", "1.2.3.4"),
				ipMapping("PRIVATE", "10.0.0.1"),
			}},
			want: instanceEndpoints{public: "1.2.3.4", private: "10.0.0.1"},
		},
		{
			name: "DNS public not overridden by primary IP",
			instance: &sqladmin.DatabaseInstance{
				DnsNames:    []*sqladmin.DnsNameMapping{dnsMapping("pub.example", "PUBLIC", "INSTANCE")},
				IpAddresses: []*sqladmin.IpMapping{ipMapping("PRIMARY", "1.2.3.4")},
			},
			want: instanceEndpoints{public: "pub.example"},
		},
		{
			name:     "empty IP address ignored",
			instance: &sqladmin.DatabaseInstance{IpAddresses: []*sqladmin.IpMapping{ipMapping("PRIMARY", "")}},
			want:     instanceEndpoints{},
		},
		{
			name: "private IP does not override private DNS name",
			instance: &sqladmin.DatabaseInstance{
				DnsNames:    []*sqladmin.DnsNameMapping{dnsMapping("priv.example", "PRIVATE_SERVICES_ACCESS", "INSTANCE")},
				IpAddresses: []*sqladmin.IpMapping{ipMapping("PRIVATE", "10.0.0.1")},
			},
			want: instanceEndpoints{private: "priv.example"},
		},
		{
			name:     "unknown connection type ignored",
			instance: &sqladmin.DatabaseInstance{DnsNames: []*sqladmin.DnsNameMapping{dnsMapping("weird.example", "SOMETHING_ELSE", "INSTANCE")}},
			want:     instanceEndpoints{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, findInstanceEndpoints(tt.instance))
		})
	}
}

func TestInstanceUserLabel(t *testing.T) {
	t.Parallel()
	require.Empty(t, instanceUserLabel(&sqladmin.DatabaseInstance{}, "key"),
		"nil Settings yields empty string")
	require.Empty(t, instanceUserLabel(&sqladmin.DatabaseInstance{Settings: &sqladmin.Settings{}}, "key"),
		"nil UserLabels yields empty string")

	instance := &sqladmin.DatabaseInstance{
		Settings: &sqladmin.Settings{UserLabels: map[string]string{"env": "prod"}},
	}
	require.Equal(t, "prod", instanceUserLabel(instance, "env"))
	require.Empty(t, instanceUserLabel(instance, "missing"))
}

func TestChooseEndpoint(t *testing.T) {
	t.Parallel()

	type endpoints = int

	const (
		withPublic = endpoints(iota)
		withPrivate
		withPSC
	)

	const noOverride = ""

	instance := func(overrideSurface string, opts ...endpoints) *sqladmin.DatabaseInstance {
		db := &sqladmin.DatabaseInstance{
			IpAddresses: []*sqladmin.IpMapping{},
			Settings:    &sqladmin.Settings{},
		}

		for _, opt := range opts {
			switch opt {
			case withPublic:
				db.IpAddresses = append(db.IpAddresses, ipMapping("PRIMARY", "1.2.3.4"))
			case withPrivate:
				db.IpAddresses = append(db.IpAddresses, ipMapping("PRIVATE", "10.0.0.1"))
			case withPSC:
				db.DnsNames = []*sqladmin.DnsNameMapping{dnsMapping("psc.example", "PRIVATE_SERVICE_CONNECT", "INSTANCE")}
				db.Settings.IpConfiguration = &sqladmin.IpConfiguration{PscConfig: &sqladmin.PscConfig{PscEnabled: true}}
			default:
				t.Fatalf("unknown option: %v", opt)
			}
		}

		if overrideSurface != noOverride {
			db.Settings.UserLabels = map[string]string{types.GCPDatabaseEndpointTypeOverrideLabel: overrideSurface}
		}

		return db
	}

	instanceDefaultEndpoint := func(opts ...endpoints) *sqladmin.DatabaseInstance {
		return instance(noOverride, opts...)
	}

	tests := []struct {
		name     string
		instance *sqladmin.DatabaseInstance
		want     string
		wantType string
		wantErr  string
	}{
		// Default precedence (no override): public > private > psc.
		{
			name:     "public preferred over private",
			instance: instanceDefaultEndpoint(withPrivate, withPublic),
			want:     "1.2.3.4",
			wantType: endpointTypePublic,
		},
		{
			name:     "private when no public",
			instance: instanceDefaultEndpoint(withPrivate),
			want:     "10.0.0.1",
			wantType: endpointTypePrivate,
		},
		{
			name:     "psc when only psc",
			instance: instanceDefaultEndpoint(withPSC),
			want:     "psc.example",
			wantType: endpointTypePSC,
		},
		{
			name:     "no endpoint",
			instance: instanceDefaultEndpoint(),
			want:     "",
		},
		{
			name:     "override public with all surfaces",
			instance: instance(endpointTypePublic, withPublic, withPrivate, withPSC),
			want:     "1.2.3.4",
			wantType: endpointTypePublic,
		},
		{
			name:     "override private with all surfaces",
			instance: instance(endpointTypePrivate, withPublic, withPrivate, withPSC),
			want:     "10.0.0.1",
			wantType: endpointTypePrivate,
		},
		{
			name:     "override psc with all surfaces",
			instance: instance(endpointTypePSC, withPublic, withPrivate, withPSC),
			want:     "psc.example",
			wantType: endpointTypePSC,
		},
		{
			name:     "override public absent returns empty",
			instance: instance(endpointTypePublic, withPrivate),
			want:     "",
			wantType: endpointTypePublic,
		},
		{
			name:     "override private absent returns empty",
			instance: instance(endpointTypePrivate, withPublic),
			want:     "",
			wantType: endpointTypePrivate,
		},
		{
			name:     "override psc absent returns empty",
			instance: instance(endpointTypePSC, withPublic),
			want:     "",
			wantType: endpointTypePSC,
		},
		{
			name:     "unrecognized override fails closed",
			instance: instance("bogus"),
			wantErr:  `unknown endpoint type "bogus" in the "teleport-database-endpoint-type" label`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, endpointType, err := chooseEndpoint(tt.instance)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.want, got)
			require.Equal(t, tt.wantType, endpointType)
		})
	}
}

func TestMapInstanceTypeLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		instanceType string
		want         string
	}{
		{instanceType: instanceTypePrimary, want: labelInstanceTypePrimary},
		{instanceType: instanceTypeReadReplica, want: labelInstanceTypeReadReplica},
		{instanceType: "READ_POOL_INSTANCE", want: ""},
		{instanceType: "ON_PREMISES_INSTANCE", want: ""},
		{instanceType: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.instanceType, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, mapInstanceTypeLabel(tt.instanceType))
		})
	}
}

func TestResolveRouting(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		instance         *sqladmin.DatabaseInstance
		wantNil          bool
		wantProtocol     string
		wantURI          string
		wantEndpointType string
	}{
		{
			name: "supported engine with reachable endpoint",
			instance: &sqladmin.DatabaseInstance{
				DatabaseVersion: "POSTGRES_14",
				IpAddresses:     []*sqladmin.IpMapping{ipMapping("PRIMARY", "1.2.3.4")},
			},
			wantProtocol:     defaults.ProtocolPostgres,
			wantURI:          "1.2.3.4:5432",
			wantEndpointType: endpointTypePublic,
		},
		{
			name: "hostname endpoint joined without brackets",
			instance: &sqladmin.DatabaseInstance{
				DatabaseVersion: "POSTGRES_14",
				DnsNames:        []*sqladmin.DnsNameMapping{dnsMapping("inst.psc.example", "PRIVATE_SERVICE_CONNECT", "INSTANCE")},
				Settings:        &sqladmin.Settings{IpConfiguration: &sqladmin.IpConfiguration{PscConfig: &sqladmin.PscConfig{PscEnabled: true}}},
			},
			wantProtocol:     defaults.ProtocolPostgres,
			wantURI:          "inst.psc.example:5432",
			wantEndpointType: endpointTypePSC,
		},
		{
			name: "unsupported engine",
			instance: &sqladmin.DatabaseInstance{
				DatabaseVersion: "SQLSERVER_2019_STANDARD",
				IpAddresses:     []*sqladmin.IpMapping{ipMapping("PRIMARY", "1.2.3.4")},
			},
			wantNil: true,
		},
		{
			name:     "no reachable endpoint",
			instance: &sqladmin.DatabaseInstance{DatabaseVersion: "POSTGRES_14"},
			wantNil:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, skipReason := resolveRouting(tt.instance)
			if tt.wantNil {
				require.Nil(t, r)
				require.NotEmpty(t, skipReason)
				return
			}
			require.NotNil(t, r)
			require.Empty(t, skipReason)
			require.Equal(t, tt.wantProtocol, r.protocol)
			require.Equal(t, tt.wantURI, r.uri)
			require.Equal(t, tt.wantEndpointType, r.endpointType)
		})
	}
}

func TestLabelsFromInstance(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		instance *sqladmin.DatabaseInstance
		routing  *routing
		want     map[string]string
	}{
		{
			name: "user labels merged with discovery labels",
			instance: &sqladmin.DatabaseInstance{
				Project:         "proj-1",
				Region:          "us-central1",
				State:           "RUNNABLE",
				DatabaseVersion: "POSTGRES_14",
				InstanceType:    instanceTypePrimary,
				Settings:        &sqladmin.Settings{UserLabels: map[string]string{"env": "prod", "team": "data"}},
			},
			routing: &routing{protocol: defaults.ProtocolPostgres, endpointType: endpointTypePublic},
			want: map[string]string{
				"env":                             "prod",
				"team":                            "data",
				types.CloudLabel:                  types.CloudGCP,
				types.DiscoveryLabelGCPProjectID:  "proj-1",
				types.DiscoveryLabelRegion:        "us-central1",
				types.DiscoveryLabelEngine:        defaults.ProtocolPostgres,
				types.DiscoveryLabelEngineVersion: "POSTGRES_14",
				types.DiscoveryLabelStatus:        "RUNNABLE",
				types.DiscoveryLabelInstanceType:  labelInstanceTypePrimary,
				types.DiscoveryLabelEndpointType:  endpointTypePublic,
			},
		},
		{
			name: "nil settings yields discovery labels only",
			instance: &sqladmin.DatabaseInstance{
				Project:         "proj-1",
				Region:          "us-central1",
				State:           "RUNNABLE",
				DatabaseVersion: "POSTGRES_14",
				InstanceType:    instanceTypePrimary,
			},
			routing: &routing{protocol: defaults.ProtocolPostgres, endpointType: endpointTypePrivate},
			want: map[string]string{
				types.CloudLabel:                  types.CloudGCP,
				types.DiscoveryLabelGCPProjectID:  "proj-1",
				types.DiscoveryLabelRegion:        "us-central1",
				types.DiscoveryLabelEngine:        defaults.ProtocolPostgres,
				types.DiscoveryLabelEngineVersion: "POSTGRES_14",
				types.DiscoveryLabelStatus:        "RUNNABLE",
				types.DiscoveryLabelInstanceType:  labelInstanceTypePrimary,
				types.DiscoveryLabelEndpointType:  endpointTypePrivate,
			},
		},
		{
			// raw engine-version enum; empty fields yield empty labels.
			name:     "mysql raw engine version, sparse instance",
			instance: &sqladmin.DatabaseInstance{DatabaseVersion: "MYSQL_8_0"},
			routing:  &routing{protocol: defaults.ProtocolMySQL},
			want: map[string]string{
				types.CloudLabel:                  types.CloudGCP,
				types.DiscoveryLabelGCPProjectID:  "",
				types.DiscoveryLabelRegion:        "",
				types.DiscoveryLabelEngine:        defaults.ProtocolMySQL,
				types.DiscoveryLabelEngineVersion: "MYSQL_8_0",
				types.DiscoveryLabelStatus:        "",
				types.DiscoveryLabelInstanceType:  "",
				types.DiscoveryLabelEndpointType:  "",
			},
		},
		{
			// computed keys override colliding user labels ("us-spoof"/"backdoor" lose).
			name: "computed discovery labels win over colliding user labels",
			instance: &sqladmin.DatabaseInstance{
				Project:         "proj-1",
				Region:          "us-central1",
				State:           "RUNNABLE",
				DatabaseVersion: "POSTGRES_14",
				InstanceType:    instanceTypePrimary,
				Settings: &sqladmin.Settings{UserLabels: map[string]string{
					types.DiscoveryLabelRegion:       "us-spoof", // collides with computed region
					types.DiscoveryLabelInstanceType: "backdoor", // collides with computed instance-type
					"env":                            "prod",     // non-colliding control
				}},
			},
			routing: &routing{protocol: defaults.ProtocolPostgres, endpointType: endpointTypePublic},
			want: map[string]string{
				"env":                             "prod",
				types.CloudLabel:                  types.CloudGCP,
				types.DiscoveryLabelGCPProjectID:  "proj-1",
				types.DiscoveryLabelRegion:        "us-central1",
				types.DiscoveryLabelEngine:        defaults.ProtocolPostgres,
				types.DiscoveryLabelEngineVersion: "POSTGRES_14",
				types.DiscoveryLabelStatus:        "RUNNABLE",
				types.DiscoveryLabelInstanceType:  labelInstanceTypePrimary,
				types.DiscoveryLabelEndpointType:  endpointTypePublic,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, labelsFromInstance(tt.instance, tt.routing))
		})
	}
}

func TestNewDatabaseFromInstance(t *testing.T) {
	t.Parallel()
	makeInstance := func(opts ...func(*sqladmin.DatabaseInstance)) *sqladmin.DatabaseInstance {
		instance := &sqladmin.DatabaseInstance{
			Name:            "pg-instance",
			Project:         "proj-1",
			Region:          "us-central1",
			State:           "RUNNABLE",
			DatabaseVersion: "POSTGRES_14",
			InstanceType:    instanceTypePrimary,
			IpAddresses:     []*sqladmin.IpMapping{ipMapping("PRIMARY", "1.2.3.4")},
			Settings:        &sqladmin.Settings{UserLabels: map[string]string{"env": "prod"}},
		}
		for _, opt := range opts {
			opt(instance)
		}
		return instance
	}

	wantDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name:        "pg-instance",
		Description: "Cloud SQL instance in us-central1",
		Labels: map[string]string{
			"env":                             "prod",
			types.CloudLabel:                  types.CloudGCP,
			types.DiscoveryLabelGCPProjectID:  "proj-1",
			types.DiscoveryLabelRegion:        "us-central1",
			types.DiscoveryLabelEngine:        defaults.ProtocolPostgres,
			types.DiscoveryLabelEngineVersion: "POSTGRES_14",
			types.DiscoveryLabelStatus:        "RUNNABLE",
			types.DiscoveryLabelInstanceType:  labelInstanceTypePrimary,
			types.DiscoveryLabelEndpointType:  endpointTypePublic,
		},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "1.2.3.4:5432",
		GCP: types.GCPCloudSQL{
			ProjectID:  "proj-1",
			InstanceID: "pg-instance",
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name           string
		instance       *sqladmin.DatabaseInstance
		want           types.Database
		wantSkipReason string
	}{
		{
			name:     "eligible instance",
			instance: makeInstance(),
			want:     wantDatabase,
		},
		{
			name: "stopped instance is skipped",
			instance: makeInstance(func(instance *sqladmin.DatabaseInstance) {
				instance.Settings.ActivationPolicy = "NEVER"
			}),
			wantSkipReason: `instance is stopped (activation policy "NEVER")`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			identity := func(meta types.Metadata) types.Metadata { return meta }
			got, skipReason, err := NewDatabaseFromInstance(tt.instance, identity)
			require.NoError(t, err)
			require.Equal(t, tt.wantSkipReason, skipReason)
			if tt.wantSkipReason != "" {
				require.Nil(t, got)
				return
			}
			require.Empty(t, cmp.Diff(tt.want, got))
		})
	}
}
