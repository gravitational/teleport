// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
)

// databaseConfigTemplateFunc list of template functions used on the database
// config template.
var databaseConfigTemplateFuncs = template.FuncMap{
	"quote": quote,
	"join":  strings.Join,
}

// databaseAgentConfigurationTemplate database configuration template.
var databaseAgentConfigurationTemplate = template.Must(template.New("").Funcs(databaseConfigTemplateFuncs).Parse(`#
# Teleport database agent configuration file.
# Configuration reference: https://goteleport.com/docs/database-access/reference/configuration/
#
version: v3
teleport:
  nodename: {{ .NodeName }}
  data_dir: {{ .DataDir }}
  proxy_server: {{ .ProxyServer }}
  auth_token: {{ .AuthToken }}
  {{- if .CAPins }}
  ca_pin:
  {{- range .CAPins }}
  - {{ . }}
  {{- end }}
  {{- end }}
db_service:
  enabled: "yes"
  # Matchers for database resources created with "tctl create" command.
  # For more information: https://goteleport.com/docs/database-access/guides/dynamic-registration/
  resources:
  - labels:
      "*": "*"
  {{- if or .RDSDiscoveryRegions .RDSProxyDiscoveryRegions .RedshiftDiscoveryRegions .ElastiCacheDiscoveryRegions}}
  # Matchers for registering AWS-hosted databases.
  aws:
  {{- end }}
  {{- if .RDSDiscoveryRegions }}
  # RDS/Aurora databases auto-discovery.
  # For more information about RDS/Aurora auto-discovery: https://goteleport.com/docs/database-access/guides/rds/
  - types: ["rds"]
    # AWS regions to register databases from.
    regions:
    {{- range .RDSDiscoveryRegions }}
    - {{ . }}
    {{- end }}
    # AWS resource tags to match when registering databases.
    tags:
      "*": "*"
  {{- end }}
  {{- if .RDSProxyDiscoveryRegions }}
  # RDS Proxies auto-discovery.
  # For more information about RDS Proxy auto-discovery: https://goteleport.com/docs/database-access/guides/rdsproxy/
  - types: ["rdsproxy"]
    # AWS regions to register databases from.
    regions:
    {{- range .RDSProxyDiscoveryRegions }}
    - {{ . }}
    {{- end }}
    # AWS resource tags to match when registering databases.
    tags:
      "*": "*"
  {{- end }}
  {{- if .RedshiftDiscoveryRegions }}
  # Redshift databases auto-discovery.
  # For more information about Redshift auto-discovery: https://goteleport.com/docs/database-access/guides/postgres-redshift/
  - types: ["redshift"]
    # AWS regions to register databases from.
    regions:
    {{- range .RedshiftDiscoveryRegions }}
    - {{ . }}
    {{- end }}
    # AWS resource tags to match when registering databases.
    tags:
      "*": "*"
  {{- end }}
  {{- if .ElastiCacheDiscoveryRegions }}
  # ElastiCache databases auto-discovery.
  # For more information about ElastiCache auto-discovery: https://goteleport.com/docs/database-access/guides/redis-aws/
  - types: ["elasticache"]
    # AWS regions to register databases from.
    regions:
    {{- range .ElastiCacheDiscoveryRegions }}
    - {{ . }}
    {{- end }}
    # AWS resource tags to match when registering databases.
    tags:
      "*": "*"
  {{- end }}
  {{- if .MemoryDBDiscoveryRegions }}
  # MemoryDB databases auto-discovery.
  # For more information about MemoryDB auto-discovery: https://goteleport.com/docs/database-access/guides/redis-aws/
  - types: ["memorydb"]
    # AWS regions to register databases from.
    regions:
    {{- range .MemoryDBDiscoveryRegions }}
    - {{ . }}
    {{- end }}
    # AWS resource tags to match when registering databases.
    tags:
      "*": "*"
  {{- end }}
  {{- if or .AzureMySQLDiscoveryRegions .AzurePostgresDiscoveryRegions .AzureRedisDiscoveryRegions}}
  # Matchers for registering Azure-hosted databases.
  azure:
  {{- end }}
  {{- if or .AzureMySQLDiscoveryRegions }}
  # Azure MySQL databases auto-discovery.
  # For more information about Azure MySQL auto-discovery: https://goteleport.com/docs/database-access/guides/azure-postgres-mysql/
  - types: ["mysql"]
    # Azure subscription IDs to match.
    subscriptions:
    {{- range .DatabaseAzureSubscriptions }}
    - "{{ . }}"
    {{- end }}
    # Azure resource groups to match.
    resource_groups:
    {{- range .DatabaseAzureResourceGroups }}
    - "{{ . }}"
    {{- end }}
    # Azure regions to register databases from.
    regions:
    {{- range .AzureMySQLDiscoveryRegions }}
    - "{{ . }}"
    {{- end }}
    # Azure resource tags to match when registering databases.
    tags:
      "*": "*"
  {{- end }}
  {{- if or .AzurePostgresDiscoveryRegions }}
  # Azure Postgres databases auto-discovery.
  # For more information about Azure Postgres auto-discovery: https://goteleport.com/docs/database-access/guides/azure-postgres-mysql/
  - types: ["postgres"]
    # Azure subscription IDs to match.
    subscriptions:
    {{- range .DatabaseAzureSubscriptions }}
    - "{{ . }}"
    {{- end }}
    # Azure resource groups to match.
    resource_groups:
    {{- range .DatabaseAzureResourceGroups }}
    - "{{ . }}"
    {{- end }}
    # Azure regions to register databases from.
    regions:
    {{- range .AzurePostgresDiscoveryRegions }}
    - "{{ . }}"
    {{- end }}
    # Azure resource tags to match when registering databases.
    tags:
      "*": "*"
  {{- end }}
  {{- if or .AzureRedisDiscoveryRegions }}
  # Azure Cache For Redis databases auto-discovery.
  # For more information about Azure Cache for Redis auto-discovery: https://goteleport.com/docs/database-access/guides/azure-redis/
  - types: ["redis"]
    # Azure subscription IDs to match.
    subscriptions:
    {{- range .DatabaseAzureSubscriptions }}
    - "{{ . }}"
    {{- end }}
    # Azure resource groups to match.
    resource_groups:
    {{- range .DatabaseAzureResourceGroups }}
    - "{{ . }}"
    {{- end }}
    # Azure regions to register databases from.
    regions:
    {{- range .AzureRedisDiscoveryRegions }}
    - "{{ . }}"
    {{- end }}
    # Azure resource tags to match when registering databases.
    tags:
      "*": "*"
  {{- end }}
  {{- if or .AzureSQLServerDiscoveryRegions }}
  # Azure SQL server and Managed instances auto-discovery.
  # For more information about SQL server and Managed instances auto-discovery: https://goteleport.com/docs/database-access/guides/azure-sql-server-ad/
  - types: ["sqlserver"]
    # Azure subscription IDs to match.
    subscriptions:
    {{- range .DatabaseAzureSubscriptions }}
    - "{{ . }}"
    {{- end }}
    # Azure resource groups to match.
    resource_groups:
    {{- range .DatabaseAzureResourceGroups }}
    - "{{ . }}"
    {{- end }}
    # Azure regions to register databases from.
    regions:
    {{- range .AzureSQLServerDiscoveryRegions }}
    - "{{ . }}"
    {{- end }}
    # Azure resource tags to match when registering databases.
    tags:
      "*": "*"
  {{- end }}
  # Lists statically registered databases proxied by this agent.
  {{- if .StaticDatabaseName }}
  databases:
  - name: {{ .StaticDatabaseName }}
    protocol: {{ .StaticDatabaseProtocol }}
    uri: {{ .StaticDatabaseURI }}
    {{- if .DatabaseCACertFile }}
    tls:
      ca_cert_file: {{ .DatabaseCACertFile }}
    {{- end }}
    {{- if or .DatabaseAWSRegion .DatabaseAWSRedshiftClusterID }}
    aws:
      {{- if .DatabaseAWSRegion }}
      region: {{ .DatabaseAWSRegion }}
      {{- end }}
      {{- if .DatabaseAWSRedshiftClusterID }}
      redshift:
        cluster_id: {{ .DatabaseAWSRedshiftClusterID }}
      {{- end }}
    {{- end }}
    {{- if or .DatabaseADDomain .DatabaseADSPN .DatabaseADKeytabFile }}
    ad:
      {{- if .DatabaseADKeytabFile }}
      keytab_file: {{ .DatabaseADKeytabFile }}
      {{- end }}
      {{- if .DatabaseADDomain }}
      domain: {{ .DatabaseADDomain }}
      {{- end }}
      {{- if .DatabaseADSPN }}
      spn: {{ .DatabaseADSPN }}
      {{- end }}
    {{- end }}
    {{- if or .DatabaseGCPProjectID .DatabaseGCPInstanceID }}
    gcp:
      {{- if .DatabaseGCPProjectID }}
      project_id: {{ .DatabaseGCPProjectID }}
      {{- end }}
      {{- if .DatabaseGCPInstanceID }}
      instance_id: {{ .DatabaseGCPInstanceID }}
      {{- end }}
    {{- end }}
    {{- if .StaticDatabaseStaticLabels }}
    static_labels:
    {{- range $name, $value := .StaticDatabaseStaticLabels }}
      "{{ $name }}": "{{ $value }}"
    {{- end }}
    {{- if .StaticDatabaseStaticLabels }}
    dynamic_labels:
    {{- range $name, $label := .StaticDatabaseDynamicLabels }}
    - name: {{ $name }}
      period: "{{ $label.Period.Duration }}"
      command:
      {{- range $command := $label.Command }}
      - {{ $command | quote }}
      {{- end }}
    {{- end }}
    {{- end }}
    {{- end }}
  {{- else }}
  # databases:
  # # RDS database static configuration.
  # # RDS/Aurora databases Auto-discovery reference: https://goteleport.com/docs/database-access/guides/rds/
  # - name: rds
  #   description: AWS RDS/Aurora instance configuration example.
  #   # Supported protocols for RDS/Aurora: "postgres" or "mysql"
  #   protocol: postgres
  #   # Database connection endpoint. Must be reachable from Database Service.
  #   uri: rds-instance-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432
  #   # AWS specific configuration.
  #   aws:
  #     # Region the database is deployed in.
  #     region: us-west-1
  #     # RDS/Aurora specific configuration.
  #     rds:
  #       # RDS Instance ID. Only present on RDS databases.
  #       instance_id: rds-instance-1
  # # Aurora database static configuration.
  # # RDS/Aurora databases Auto-discovery reference: https://goteleport.com/docs/database-access/guides/rds/
  # - name: aurora
  #   description: AWS Aurora cluster configuration example.
  #   # Supported protocols for RDS/Aurora: "postgres" or "mysql"
  #   protocol: postgres
  #   # Database connection endpoint. Must be reachable from Database Service.
  #   uri: aurora-cluster-1.abcdefghijklmnop.us-west-1.rds.amazonaws.com:5432
  #   # AWS specific configuration.
  #   aws:
  #     # Region the database is deployed in.
  #     region: us-west-1
  #     # RDS/Aurora specific configuration.
  #     rds:
  #       # Aurora Cluster ID. Only present on Aurora databases.
  #       cluster_id: aurora-cluster-1
  # # Redshift database static configuration.
  # # For more information: https://goteleport.com/docs/database-access/guides/postgres-redshift/
  # - name: redshift
  #   description: AWS Redshift cluster configuration example.
  #   # Supported protocols for Redshift: "postgres".
  #   protocol: postgres
  #   # Database connection endpoint. Must be reachable from Database service.
  #   uri: redshift-cluster-example-1.abcdefghijklmnop.us-west-1.redshift.amazonaws.com:5439
  #   # AWS specific configuration.
  #   aws:
  #     # Region the database is deployed in.
  #     region: us-west-1
  #     # Redshift specific configuration.
  #     redshift:
  #       # Redshift Cluster ID.
  #       cluster_id: redshift-cluster-example-1
  # # ElastiCache database static configuration.
  # - name: elasticache
  #   description: AWS ElastiCache cluster configuration example.
  #   protocol: redis
  #   # Database connection endpoint. Must be reachable from Database service.
  #   uri: master.redis-cluster-example.abcdef.usw1.cache.amazonaws.com:6379
  #   # AWS specific configuration.
  #   aws:
  #     # Region the database is deployed in.
  #     region: us-west-1
  #     # ElastiCache specific configuration.
  #     elasticache:
  #       # ElastiCache replication group ID.
  #       replication_group_id: redis-cluster-example
  # # MemoryDB database static configuration.
  # - name: memorydb
  #   description: AWS MemoryDB cluster configuration example.
  #   protocol: redis
  #   # Database connection endpoint. Must be reachable from Database service.
  #   uri: clustercfg.my-memorydb.xxxxxx.memorydb.us-east-1.amazonaws.com:6379
  #   # AWS specific configuration.
  #   aws:
  #     # Region the database is deployed in.
  #     region: us-west-1
  #     # MemoryDB specific configuration.
  #     memorydb:
  #       # MemoryDB cluster name.
  #       cluster_name: my-memorydb
  # # Self-hosted static configuration.
  # - name: self-hosted
  #   description: Self-hosted database configuration.
  #   # Supported protocols for self-hosted: {{ join .DatabaseProtocols ", " }}.
  #   protocol: postgres
  #   # Database connection endpoint. Must be reachable from Database service.
  #   uri: database.example.com:5432
  {{- end }}
auth_service:
  enabled: "no"
ssh_service:
  enabled: "no"
proxy_service:
  enabled: "no"`))

// DatabaseSampleFlags specifies configuration parameters for a database agent.
type DatabaseSampleFlags struct {
	// StaticDatabaseName static database name provided by the user.
	StaticDatabaseName string
	// StaticDatabaseProtocol static databse protocol provided by the user.
	StaticDatabaseProtocol string
	// StaticDatabaseURI static database URI provided by the user.
	StaticDatabaseURI string
	// StaticDatabaseStaticLabels list of database static labels provided by
	// the user.
	StaticDatabaseStaticLabels map[string]string
	// StaticDatabaseDynamicLabels list of database dynamic labels provided by
	// the user.`
	StaticDatabaseDynamicLabels services.CommandLabels
	// StaticDatabaseRawLabels "raw" list of database labels provided by the
	// user.
	StaticDatabaseRawLabels string
	// NodeName `nodename` configuration.
	NodeName string
	// DataDir `data_dir` configuration.
	DataDir string
	// ProxyServer is the address of the proxy servers
	ProxyServer string
	// AuthToken auth server token.
	AuthToken string
	// CAPins are the SKPI hashes of the CAs used to verify the Auth Server.
	CAPins []string
	// AzureMySQLDiscoveryRegions is a list of regions Azure auto-discovery is
	// configured to discover MySQL servers in.
	AzureMySQLDiscoveryRegions []string
	// AzurePostgresDiscoveryRegions is a list of regions Azure auto-discovery is
	// configured to discover Postgres servers in.
	AzurePostgresDiscoveryRegions []string
	// AzureRedisDiscoveryRegions is a list of regions Azure auto-discovery is
	// configured to discover Azure Cache for Redis servers in.
	AzureRedisDiscoveryRegions []string
	// AzureSQLServerDiscoveryRegions is a list of regions Azure auto-discovery is
	// configured to discover Azure SQL servers and managed instances.
	AzureSQLServerDiscoveryRegions []string
	// RDSDiscoveryRegions is a list of regions the RDS auto-discovery is
	// configured.
	RDSDiscoveryRegions []string
	// RDSProxyDiscoveryRegions is a list of regions the RDS Proxy
	// auto-discovery is configured.
	RDSProxyDiscoveryRegions []string
	// RedshiftDiscoveryRegions is a list of regions the Redshift
	// auto-discovery is configured.
	RedshiftDiscoveryRegions []string
	// ElastiCacheDiscoveryRegions is a list of regions the ElastiCache
	// auto-discovery is configured.
	ElastiCacheDiscoveryRegions []string
	// MemoryDBDiscoveryRegions is a list of regions the MemoryDB
	// auto-discovery is configured.
	MemoryDBDiscoveryRegions []string
	// DatabaseProtocols is a list of database protocols supported.
	DatabaseProtocols []string
	// DatabaseAWSRegion is an optional database cloud region e.g. when using AWS RDS.
	DatabaseAWSRegion string
	// DatabaseAWSRedshiftClusterID is Redshift cluster identifier.
	DatabaseAWSRedshiftClusterID string
	// DatabaseADDomain is the Active Directory domain for authentication.
	DatabaseADDomain string
	// DatabaseADSPN is the database Service Principal Name.
	DatabaseADSPN string
	// DatabaseADKeytabFile is the path to Kerberos keytab file.
	DatabaseADKeytabFile string
	// DatabaseGCPProjectID is GCP Cloud SQL project identifier.
	DatabaseGCPProjectID string
	// DatabaseGCPInstanceID is GCP Cloud SQL instance identifier.
	DatabaseGCPInstanceID string
	// DatabaseCACertFile is the database CA cert path.
	DatabaseCACertFile string
	// DatabaseAzureSubscriptions is a list of Azure subscriptions.
	DatabaseAzureSubscriptions []string
	// DatabaseAzureResourceGroups is a list of Azure resource groups.
	DatabaseAzureResourceGroups []string
}

// CheckAndSetDefaults checks and sets default values for the flags.
func (f *DatabaseSampleFlags) CheckAndSetDefaults() error {
	conf := service.MakeDefaultConfig()
	f.DatabaseProtocols = defaults.DatabaseProtocols

	if f.NodeName == "" {
		f.NodeName = conf.Hostname
	}
	if f.DataDir == "" {
		f.DataDir = conf.DataDir
	}

	if f.StaticDatabaseName != "" || f.StaticDatabaseProtocol != "" || f.StaticDatabaseURI != "" {
		if f.StaticDatabaseName == "" {
			return trace.BadParameter("--name is required when configuring static database")
		}
		if f.StaticDatabaseProtocol == "" {
			return trace.BadParameter("--protocol is required when configuring static database")
		}
		if f.StaticDatabaseURI == "" {
			return trace.BadParameter("--uri is required when configuring static database")
		}

		if f.StaticDatabaseRawLabels != "" {
			var err error
			f.StaticDatabaseStaticLabels, f.StaticDatabaseDynamicLabels, err = parseLabels(f.StaticDatabaseRawLabels)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// MakeDatabaseAgentConfigString generates a simple database agent
// configuration based on the flags provided. Returns the configuration as a
// string.
func MakeDatabaseAgentConfigString(flags DatabaseSampleFlags) (string, error) {
	err := flags.CheckAndSetDefaults()
	if err != nil {
		return "", trace.Wrap(err)
	}

	buf := new(bytes.Buffer)
	err = databaseAgentConfigurationTemplate.Execute(buf, flags)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return buf.String(), nil
}

// quote quotes a string, similar to the `quote` helper from Helm.
// Implementation reference: https://github.com/Masterminds/sprig/blob/3ac42c7bc5e4be6aa534e036fb19dde4a996da2e/strings.go#L83
func quote(str string) string {
	return fmt.Sprintf("%q", str)
}
