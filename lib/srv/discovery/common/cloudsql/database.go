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
	"fmt"
	"maps"
	"net"
	"strings"

	"github.com/gravitational/trace"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

// The google.golang.org/api SDK types Cloud SQL enum-like fields as plain
// strings with no generated constants, so the values we depend on are named here.
const (
	// Accepted values for types.GCPDatabaseEndpointTypeOverrideLabel
	endpointTypePublic  = "public"
	endpointTypePrivate = "private"
	endpointTypePSC     = "psc"

	instanceTypePrimary     = "CLOUD_SQL_INSTANCE"
	instanceTypeReadReplica = "READ_REPLICA_INSTANCE"

	labelInstanceTypePrimary     = "primary"
	labelInstanceTypeReadReplica = "read-replica"
)

func checkInstanceAvailable(instance *sqladmin.DatabaseInstance) (ok bool, reason string) {
	const (
		stateRunnable         = "RUNNABLE"
		activationPolicyNever = "NEVER"
	)
	if instance.State != stateRunnable {
		return false, fmt.Sprintf("instance is not available (state %q)", instance.State)
	}
	// Instances stopped by their owner stay RUNNABLE; only the activation policy tells.
	if instance.Settings != nil && instance.Settings.ActivationPolicy == activationPolicyNever {
		return false, fmt.Sprintf("instance is stopped (activation policy %q)", instance.Settings.ActivationPolicy)
	}
	return true, ""
}

func protocolAndPort(databaseVersion string) (protocol, port string, ok bool) {
	const (
		versionPrefixMySQL    = "MYSQL_"
		versionPrefixPostgres = "POSTGRES_"

		defaultPortMySQL    = "3306"
		defaultPortPostgres = "5432"
	)

	switch {
	case strings.HasPrefix(databaseVersion, versionPrefixMySQL):
		return defaults.ProtocolMySQL, defaultPortMySQL, true
	case strings.HasPrefix(databaseVersion, versionPrefixPostgres):
		return defaults.ProtocolPostgres, defaultPortPostgres, true
	default:
		return "", "", false
	}
}

func pscEnabled(instance *sqladmin.DatabaseInstance) bool {
	return instance.Settings != nil &&
		instance.Settings.IpConfiguration != nil &&
		instance.Settings.IpConfiguration.PscConfig != nil &&
		instance.Settings.IpConfiguration.PscConfig.PscEnabled
}

type instanceEndpoints struct {
	public, private, psc string
}

func findInstanceEndpoints(instance *sqladmin.DatabaseInstance) instanceEndpoints {
	const (
		scopeInstance = "INSTANCE"

		connectionTypePublic = "PUBLIC"
		connectionTypePSA    = "PRIVATE_SERVICES_ACCESS"
		connectionTypePSC    = "PRIVATE_SERVICE_CONNECT"

		ipTypePrimary = "PRIMARY"
		ipTypePrivate = "PRIVATE"
	)

	endpoints := instanceEndpoints{}

	// find usable DNS names
	for _, dns := range instance.DnsNames {
		if dns.Name == "" || dns.DnsScope != scopeInstance {
			continue
		}
		switch dns.ConnectionType {
		case connectionTypePublic:
			endpoints.public = dns.Name
		case connectionTypePSA:
			endpoints.private = dns.Name
		case connectionTypePSC:
			if pscEnabled(instance) {
				endpoints.psc = dns.Name
			}
		}
	}

	// fallback to IP addresses
	for _, ipAddr := range instance.IpAddresses {
		if ipAddr.IpAddress == "" {
			continue
		}

		switch {
		case ipAddr.Type == ipTypePrimary && endpoints.public == "":
			endpoints.public = ipAddr.IpAddress
		case ipAddr.Type == ipTypePrivate && endpoints.private == "":
			endpoints.private = ipAddr.IpAddress
		}
	}

	return endpoints
}

func instanceUserLabel(instance *sqladmin.DatabaseInstance, key string) string {
	if instance.Settings == nil || instance.Settings.UserLabels == nil {
		return ""
	}
	return instance.Settings.UserLabels[key]
}

func chooseEndpoint(instance *sqladmin.DatabaseInstance) (host, endpointType string, err error) {
	endpoints := findInstanceEndpoints(instance)

	candidates := map[string]string{
		endpointTypePublic:  endpoints.public,
		endpointTypePrivate: endpoints.private,
		endpointTypePSC:     endpoints.psc,
	}

	// the override label limits the choice to a single endpoint type,
	// with no fallback even if the chosen endpoint is absent.
	override := instanceUserLabel(instance, types.GCPDatabaseEndpointTypeOverrideLabel)
	if override != "" {
		value, found := candidates[override]
		if !found {
			return "", "", trace.BadParameter("unknown endpoint type %q in the %q label", override, types.GCPDatabaseEndpointTypeOverrideLabel)
		}
		return value, override, nil
	}

	// key order determines preference
	keys := []string{endpointTypePublic, endpointTypePrivate, endpointTypePSC}
	for _, key := range keys {
		c := candidates[key]
		if c != "" {
			return c, key, nil
		}
	}

	// all empty
	return "", "", nil
}

func checkSupportedInstanceType(instance *sqladmin.DatabaseInstance) (ok bool, reason string) {
	if mapInstanceTypeLabel(instance.InstanceType) == "" {
		return false, fmt.Sprintf("unsupported instance type %q", instance.InstanceType)
	}
	return true, ""
}

func mapInstanceTypeLabel(instanceType string) string {
	switch instanceType {
	case instanceTypePrimary:
		return labelInstanceTypePrimary
	case instanceTypeReadReplica:
		return labelInstanceTypeReadReplica
	}
	// Currently we don't support:
	// - READ_POOL_INSTANCE: need changes in DB agent TLS checks.
	// - ON_PREMISES_INSTANCE: not tested.
	return ""
}

// routing is the resolved protocol and connection endpoint for an instance.
type routing struct {
	protocol     string
	uri          string
	endpointType string
}

// resolveRouting determines how Teleport would route to an instance. On
// failure it returns a nil routing and the reason the instance is unroutable.
func resolveRouting(instance *sqladmin.DatabaseInstance) (_ *routing, reason string) {
	protocol, port, ok := protocolAndPort(instance.DatabaseVersion)
	if !ok {
		return nil, fmt.Sprintf("unsupported database version %q", instance.DatabaseVersion)
	}
	host, endpointType, err := chooseEndpoint(instance)
	if err != nil {
		return nil, fmt.Sprintf("unable to choose endpoint: %v", err)
	}
	if host == "" {
		return nil, "no reachable connection endpoint"
	}
	return &routing{
		protocol:     protocol,
		uri:          net.JoinHostPort(host, port),
		endpointType: endpointType,
	}, ""
}

// labelsFromInstance assembles the discovery labels for a Cloud SQL
// instance, including its user labels.
func labelsFromInstance(instance *sqladmin.DatabaseInstance, routing *routing) map[string]string {
	labels := make(map[string]string)

	if instance.Settings != nil {
		maps.Copy(labels, instance.Settings.UserLabels)
	}

	labels[types.CloudLabel] = types.CloudGCP
	labels[types.DiscoveryLabelGCPProjectID] = instance.Project
	labels[types.DiscoveryLabelRegion] = instance.Region
	labels[types.DiscoveryLabelEngine] = routing.protocol
	labels[types.DiscoveryLabelEngineVersion] = instance.DatabaseVersion
	labels[types.DiscoveryLabelStatus] = instance.State
	labels[types.DiscoveryLabelInstanceType] = mapInstanceTypeLabel(instance.InstanceType)
	labels[types.DiscoveryLabelEndpointType] = routing.endpointType
	return labels
}

// NewDatabaseFromInstance builds a types.Database from an instance.
// The metadata is passed through modifyMeta before construction.
// Ineligible instances are skipped: a non-empty skipReason is returned
// with a nil database and a nil error.
func NewDatabaseFromInstance(instance *sqladmin.DatabaseInstance, modifyMeta func(types.Metadata) types.Metadata) (db types.Database, skipReason string, err error) {
	if ok, reason := checkInstanceAvailable(instance); !ok {
		return nil, reason, nil
	}
	if ok, reason := checkSupportedInstanceType(instance); !ok {
		return nil, reason, nil
	}
	rt, skipReason := resolveRouting(instance)
	if rt == nil {
		return nil, skipReason, nil
	}

	labels := labelsFromInstance(instance, rt)
	db, err = types.NewDatabaseV3(
		modifyMeta(types.Metadata{
			Name:        instance.Name,
			Description: fmt.Sprintf("Cloud SQL instance in %v", instance.Region),
			Labels:      labels,
		}),
		types.DatabaseSpecV3{
			Protocol: rt.protocol,
			URI:      rt.uri,
			GCP: types.GCPCloudSQL{
				ProjectID:  instance.Project,
				InstanceID: instance.Name,
			},
		})
	return db, "", trace.Wrap(err)
}
