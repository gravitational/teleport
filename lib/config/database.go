/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package config

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
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
# Configuration reference: https://goteleport.com/docs/reference/agent-services/database-access-reference/configuration/
#
version: v3
teleport:
  nodename: "{{ .NodeName }}"
  data_dir: "{{ .DataDir }}"
  proxy_server: "{{ .ProxyServer }}"
  auth_token: "{{ .AuthToken }}"
  {{- if .CAPins }}
  ca_pin:
  {{- range .CAPins }}
  - "{{ . }}"
  {{- end }}
  {{- end }}

db_service:
  enabled: true

  # Matchers for database resources created with "tctl create" command or by the discovery service.
  # For more information about dynamic registration: https://goteleport.com/docs/enroll-resources/database-access/guides/dynamic-registration/
  {{- if .DynamicResourcesLabels }}
  resources:
  {{- range $index, $resourceLabel := .DynamicResourcesLabels }}
  - labels:
	{{- range $name, $value := $resourceLabel }}
      "{{ $name }}": "{{ $value }}"
    {{- end }}
    {{- if $.DatabaseAWSAssumeRoleARN }}
    aws:
      {{- if $.DatabaseAWSAssumeRoleARN }}
      assume_role_arn: "{{ $.DatabaseAWSAssumeRoleARN }}"
      {{- end }}
      {{- if $.DatabaseAWSExternalID }}
      external_id: "{{ $.DatabaseAWSExternalID }}"
      {{- end }}
    {{- end }}
  {{- end }}
  {{- else }}
  #
  # resources:
  # - labels:
  #     "env": "dev"
  #   # Optional AWS role that the Database Service will assume to access the
  #   # databases.
  #   aws:
  #     assume_role_arn: "arn:aws:iam::123456789012:role/example-role-name"
  #     external_id: "example-external-id"
  {{- end }}

  # Matchers for registering AWS-hosted databases.
  {{- if or .RDSDiscoveryRegions .RDSProxyDiscoveryRegions .RedshiftDiscoveryRegions .RedshiftServerlessDiscoveryRegions .ElastiCacheDiscoveryRegions .MemoryDBDiscoveryRegions .OpenSearchDiscoveryRegions}}
  aws:
  {{- else }}
  # For more information about AWS auto-discovery:
  # https://goteleport.com/docs/enroll-resources/auto-discovery/databases/aws/
  #
  # aws:
  #   # Database types. Valid options are:
  #   # 'rds' - discovers and registers AWS RDS and Aurora databases
  #   # 'rdsproxy' - discovers and registers AWS RDS Proxy databases.
  #   # 'redshift' - discovers and registers AWS Redshift databases.
  #   # 'redshift-serverless' - discovers and registers AWS Redshift Serverless databases.
  #   # 'elasticache' - discovers and registers AWS ElastiCache Redis databases.
  #   # 'memorydb' - discovers and registers AWS MemoryDB Redis databases.
  #   # 'opensearch' - discovers and registers AWS OpenSearch domains.
  # - types: ["rds", "rdsproxy", "redshift", "redshift-serverless", "elasticache", "memorydb", "opensearch"]
  #   # AWS regions to register databases from.
  #   regions: ["us-west-1", "us-east-2"]
  #   # AWS resource tags to match when registering databases.
  #   tags:
  #     "*": "*"
  {{- end }}
  {{- if .RDSDiscoveryRegions }}
  # RDS/Aurora databases auto-discovery.
  # For more information about RDS/Aurora auto-discovery: https://goteleport.com/docs/enroll-resources/auto-discovery/databases/aws/
  - types: ["rds"]
    # AWS regions to register databases from.
    regions:
    {{- range .RDSDiscoveryRegions }}
    - "{{ . }}"
    {{- end }}
    # AWS resource tags to match when registering databases.
    tags:
    {{- range $name, $value := .AWSTags }}
      "{{ $name }}": "{{ $value }}"
    {{- end }}
  {{- end }}
  {{- if .RDSProxyDiscoveryRegions }}
  # RDS Proxies auto-discovery.
  # For more information about RDS Proxy auto-discovery: https://goteleport.com/docs/enroll-resources/auto-discovery/databases/aws/
  - types: ["rdsproxy"]
    # AWS regions to register databases from.
    regions:
    {{- range .RDSProxyDiscoveryRegions }}
    - "{{ . }}"
    {{- end }}
    # AWS resource tags to match when registering databases.
    tags:
    {{- range $name, $value := .AWSTags }}
      "{{ $name }}": "{{ $value }}"
    {{- end }}
  {{- end }}
  {{- if .RedshiftDiscoveryRegions }}
  # Redshift databases auto-discovery.
  # For more information about Redshift auto-discovery: https://goteleport.com/docs/enroll-resources/auto-discovery/databases/aws/
  - types: ["redshift"]
    # AWS regions to register databases from.
    regions:
    {{- range .RedshiftDiscoveryRegions }}
    - "{{ . }}"
    {{- end }}
    # AWS resource tags to match when registering databases.
    tags:
    {{- range $name, $value := .AWSTags }}
      "{{ $name }}": "{{ $value }}"
    {{- end }}
  {{- end }}
  {{- if .RedshiftServerlessDiscoveryRegions }}
  # Redshift Serverless databases auto-discovery.
  # For more information about Redshift Serverless auto-discovery: https://goteleport.com/docs/enroll-resources/auto-discovery/databases/aws/
  - types: ["redshift-serverless"]
    # AWS regions to register databases from.
    regions:
    {{- range .RedshiftServerlessDiscoveryRegions }}
    - "{{ . }}"
    {{- end }}
    # AWS resource tags to match when registering databases.
    tags:
    {{- range $name, $value := .AWSTags }}
      "{{ $name }}": "{{ $value }}"
    {{- end }}
  {{- end }}
  {{- if .ElastiCacheDiscoveryRegions }}
  # ElastiCache databases auto-discovery.
  # For more information about ElastiCache auto-discovery: https://goteleport.com/docs/enroll-resources/auto-discovery/databases/aws/
  - types: ["elasticache"]
    # AWS regions to register databases from.
    regions:
    {{- range .ElastiCacheDiscoveryRegions }}
    - "{{ . }}"
    {{- end }}
    # AWS resource tags to match when registering databases.
    tags:
    {{- range $name, $value := .AWSTags }}
      "{{ $name }}": "{{ $value }}"
    {{- end }}
  {{- end }}
  {{- if .MemoryDBDiscoveryRegions }}
  # MemoryDB databases auto-discovery.
  # For more information about MemoryDB auto-discovery: https://goteleport.com/docs/enroll-resources/auto-discovery/databases/aws/
  - types: ["memorydb"]
    # AWS regions to register databases from.
    regions:
    {{- range .MemoryDBDiscoveryRegions }}
    - "{{ . }}"
    {{- end }}
    # AWS resource tags to match when registering databases.
    tags:
    {{- range $name, $value := .AWSTags }}
      "{{ $name }}": "{{ $value }}"
    {{- end }}
  {{- end }}
  {{- if .OpenSearchDiscoveryRegions }}
  # OpenSearch databases auto-discovery.
  # For more information about OpenSearch auto-discovery: https://goteleport.com/docs/enroll-resources/auto-discovery/databases/aws/
  - types: ["opensearch"]
    # AWS regions to register databases from.
    regions:
    {{- range .OpenSearchDiscoveryRegions }}
    - "{{ . }}"
    {{- end }}
    # AWS resource tags to match when registering databases.
    tags:
    {{- range $name, $value := .AWSTags }}
      "{{ $name }}": "{{ $value }}"
    {{- end }}
  {{- end }}

  # Matchers for registering Azure-hosted databases.
  {{- if or .AzureMySQLDiscoveryRegions .AzurePostgresDiscoveryRegions .AzureRedisDiscoveryRegions .AzureSQLServerDiscoveryRegions }}
  azure:
  {{- else }}
  # For more information about Azure auto-discovery:
  # MySQL/PostgreSQL: https://goteleport.com/docs/enroll-resources/database-access/enroll-azure-databases/azure-postgres-mysql/
  # Redis: https://goteleport.com/docs/enroll-resources/database-access/enroll-azure-databases/azure-redis/
  # SQL Server: https://goteleport.com/docs/enroll-resources/database-access/enroll-azure-databases/azure-sql-server-ad/
  #
  # azure:
  #   # Database types. Valid options are:
  #   # 'mysql' - discovers and registers Azure MySQL databases.
  #   # 'postgres' - discovers and registers Azure PostgreSQL databases.
  #   # 'redis' - discovers and registers Azure Cache for Redis databases.
  #   # 'sqlserver' - discovers and registers Azure SQL Server databases.
  # - types: ["mysql", "postgres", "redis", "sqlserver"]
  #   # Azure regions to register databases from. Valid options are:
  #   # '*' - discovers databases in all regions (default).
  #   regions: ["eastus", "westus"]
  #   # Azure subscription IDs to register databases from. Valid options are:
  #   # '*' - discovers databases in all subscriptions (default).
  #   subscriptions: ["11111111-2222-3333-4444-555555555555"]
  #   # Azure resource groups to register databases from. Valid options are:
  #   # '*' - discovers databases in all resource groups within configured subscription(s) (default).
  #   resource_groups: ["group1", "group2"]
  #   # Azure resource tags to match when registering databases.
  #   tags:
  #     "*": "*"
  {{- end }}
  {{- if or .AzureMySQLDiscoveryRegions }}
  # Azure MySQL databases auto-discovery.
  # For more information about Azure MySQL auto-discovery: https://goteleport.com/docs/enroll-resources/database-access/enroll-azure-databases/azure-postgres-mysql/
  - types: ["mysql"]
    # Azure subscription IDs to match.
    subscriptions:
    {{- range .AzureSubscriptions }}
    - "{{ . }}"
    {{- end }}
    # Azure resource groups to match.
    resource_groups:
    {{- range .AzureResourceGroups }}
    - "{{ . }}"
    {{- end }}
    # Azure regions to register databases from.
    regions:
    {{- range .AzureMySQLDiscoveryRegions }}
    - "{{ . }}"
    {{- end }}
    # Azure resource tags to match when registering databases.
    tags:
    {{- range $name, $value := .AzureTags }}
      "{{ $name }}": "{{ $value }}"
    {{- end }}
  {{- end }}
  {{- if or .AzurePostgresDiscoveryRegions }}
  # Azure Postgres databases auto-discovery.
  # For more information about Azure Postgres auto-discovery: https://goteleport.com/docs/enroll-resources/database-access/enroll-azure-databases/azure-postgres-mysql/
  - types: ["postgres"]
    # Azure subscription IDs to match.
    subscriptions:
    {{- range .AzureSubscriptions }}
    - "{{ . }}"
    {{- end }}
    # Azure resource groups to match.
    resource_groups:
    {{- range .AzureResourceGroups }}
    - "{{ . }}"
    {{- end }}
    # Azure regions to register databases from.
    regions:
    {{- range .AzurePostgresDiscoveryRegions }}
    - "{{ . }}"
    {{- end }}
    # Azure resource tags to match when registering databases.
    tags:
    {{- range $name, $value := .AzureTags }}
      "{{ $name }}": "{{ $value }}"
    {{- end }}
  {{- end }}
  {{- if or .AzureRedisDiscoveryRegions }}
  # Azure Cache For Redis databases auto-discovery.
  # For more information about Azure Cache for Redis auto-discovery: https://goteleport.com/docs/enroll-resources/database-access/enroll-azure-databases/azure-redis/
  - types: ["redis"]
    # Azure subscription IDs to match.
    subscriptions:
    {{- range .AzureSubscriptions }}
    - "{{ . }}"
    {{- end }}
    # Azure resource groups to match.
    resource_groups:
    {{- range .AzureResourceGroups }}
    - "{{ . }}"
    {{- end }}
    # Azure regions to register databases from.
    regions:
    {{- range .AzureRedisDiscoveryRegions }}
    - "{{ . }}"
    {{- end }}
    # Azure resource tags to match when registering databases.
    tags:
    {{- range $name, $value := .AzureTags }}
      "{{ $name }}": "{{ $value }}"
    {{- end }}
  {{- end }}
  {{- if or .AzureSQLServerDiscoveryRegions }}
  # Azure SQL server and Managed instances auto-discovery.
  # For more information about SQL server and Managed instances auto-discovery: https://goteleport.com/docs/enroll-resources/database-access/enroll-azure-databases/azure-sql-server-ad/
  - types: ["sqlserver"]
    # Azure subscription IDs to match.
    subscriptions:
    {{- range .AzureSubscriptions }}
    - "{{ . }}"
    {{- end }}
    # Azure resource groups to match.
    resource_groups:
    {{- range .AzureResourceGroups }}
    - "{{ . }}"
    {{- end }}
    # Azure regions to register databases from.
    regions:
    {{- range .AzureSQLServerDiscoveryRegions }}
    - "{{ . }}"
    {{- end }}
    # Azure resource tags to match when registering databases.
    tags:
    {{- range $name, $value := .AzureTags }}
      "{{ $name }}": "{{ $value }}"
    {{- end }}
  {{- end }}

  # Lists statically registered databases proxied by this agent.
  {{- if or .StaticDatabaseName .StaticDatabaseProtocol .StaticDatabaseStaticLabels .StaticDatabaseDynamicLabels }}
  databases:
  - name: "{{ .StaticDatabaseName }}"
    protocol: "{{ .StaticDatabaseProtocol }}"
    {{- if .StaticDatabaseURI }}
    uri: "{{ .StaticDatabaseURI }}"
    {{- end}}
    {{- if or .DatabaseCACertFile .DatabaseTrustSystemCertPool}}
    tls:
      {{- if .DatabaseCACertFile }}
      ca_cert_file: "{{ .DatabaseCACertFile }}"
      {{- end }}
      {{- if .DatabaseTrustSystemCertPool }}
      trust_system_cert_pool: {{ .DatabaseTrustSystemCertPool }}
      {{- end }}
    {{- end }}
    {{- if or .DatabaseAWSRegion .DatabaseAWSAccountID .DatabaseAWSAssumeRoleARN .DatabaseAWSExternalID .DatabaseAWSRedshiftClusterID .DatabaseAWSRDSInstanceID .DatabaseAWSRDSClusterID .DatabaseAWSElastiCacheGroupID .DatabaseAWSMemoryDBClusterName }}
    aws:
      {{- if .DatabaseAWSRegion }}
      region: "{{ .DatabaseAWSRegion }}"
      {{- end }}
      {{- if .DatabaseAWSAccountID }}
      account_id: "{{ .DatabaseAWSAccountID }}"
      {{- end }}
      {{- if .DatabaseAWSAssumeRoleARN }}
      assume_role_arn: "{{ .DatabaseAWSAssumeRoleARN }}"
      {{- end }}
      {{- if .DatabaseAWSExternalID }}
      external_id: "{{ .DatabaseAWSExternalID }}"
      {{- end }}
      {{- if .DatabaseAWSRedshiftClusterID }}
      redshift:
        cluster_id: "{{ .DatabaseAWSRedshiftClusterID }}"
      {{- end }}
      {{- if or .DatabaseAWSRDSInstanceID .DatabaseAWSRDSClusterID }}
      rds:
        {{- if .DatabaseAWSRDSInstanceID }}
        instance_id: "{{ .DatabaseAWSRDSInstanceID }}"
        {{- end }}
        {{- if .DatabaseAWSRDSClusterID }}
        cluster_id: "{{ .DatabaseAWSRDSClusterID }}"
        {{- end }}
      {{- end }}
      {{- if .DatabaseAWSElastiCacheGroupID }}
      elasticache:
        replication_group_id: "{{ .DatabaseAWSElastiCacheGroupID }}"
      {{- end }}
      {{- if .DatabaseAWSMemoryDBClusterName }}
      memorydb:
        cluster_name: "{{ .DatabaseAWSMemoryDBClusterName }}"
      {{- end }}
    {{- end }}
    {{- if or .DatabaseADDomain .DatabaseADSPN .DatabaseADKeytabFile }}
    ad:
      {{- if .DatabaseADKeytabFile }}
      keytab_file: "{{ .DatabaseADKeytabFile }}"
      {{- end }}
      {{- if .DatabaseADDomain }}
      domain: "{{ .DatabaseADDomain }}"
      {{- end }}
      {{- if .DatabaseADSPN }}
      spn: "{{ .DatabaseADSPN }}"
      {{- end }}
      # Optional path to Kerberos configuration file. Defaults to /etc/krb5.conf.
      krb5_file: "/etc/krb5.conf"
    {{- end }}
    {{- if or .DatabaseGCPProjectID .DatabaseGCPInstanceID }}
    gcp:
      {{- if .DatabaseGCPProjectID }}
      project_id: "{{ .DatabaseGCPProjectID }}"
      {{- end }}
      {{- if .DatabaseGCPInstanceID }}
      instance_id: "{{ .DatabaseGCPInstanceID }}"
      {{- end }}
    {{- end }}
    {{- if .StaticDatabaseStaticLabels }}
    static_labels:
    {{- range $name, $value := .StaticDatabaseStaticLabels }}
      "{{ $name }}": "{{ $value }}"
    {{- end }}
    {{- end }}
    {{- if .StaticDatabaseDynamicLabels }}
    dynamic_labels:
    {{- range $name, $label := .StaticDatabaseDynamicLabels }}
    - name: "{{ $name }}"
      period: "{{ $label.Period.Duration }}"
      command:
      {{- range $command := $label.Command }}
      - {{ $command | quote }}
      {{- end }}
    {{- end }}
    {{- end }}
  {{- else }}
  #
  # databases:
  # # RDS database static configuration.
  # # RDS/Aurora databases Auto-discovery guide: https://goteleport.com/docs/enroll-resources/auto-discovery/databases/aws/
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
  # # RDS/Aurora databases Auto-discovery guide: https://goteleport.com/docs/enroll-resources/auto-discovery/databases/aws/
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
  # # For more information: https://goteleport.com/docs/enroll-resources/auto-discovery/databases/aws/
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
  # # OpenSearch database static configuration.
  # - name: opensearch
  #   description: AWS OpenSearch domain configuration example.
  #   protocol: opensearch
  #   # Database connection endpoint. Must be reachable from Database service.
  #   uri: search-my-domain-xxxxxx.us-east-1.es.amazonaws.com:443
  #   # AWS specific configuration.
  #   aws:
  #     # Region the database is deployed in.
  #     region: us-east-1
  #     account_id: "123456789000"
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
	// DynamicResourcesRawLabels is the "raw" list of labels for dynamic "resources".
	DynamicResourcesRawLabels []string
	// DynamicResourcesLabels is the list of labels for dynamic "resources".
	DynamicResourcesLabels []map[string]string
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
	// AzureSubscriptions is a list of Azure subscriptions.
	AzureSubscriptions []string
	// AzureResourceGroups is a list of Azure resource groups.
	AzureResourceGroups []string
	// AzureTags is the list of the Azure resource tags used for Azure discoveries.
	AzureTags map[string]string
	// AzureRawTags is the "raw" list of Azure resource tags used for Azure discoveries.
	AzureRawTags string
	// RDSDiscoveryRegions is a list of regions the RDS auto-discovery is
	// configured.
	RDSDiscoveryRegions []string
	// RDSProxyDiscoveryRegions is a list of regions the RDS Proxy
	// auto-discovery is configured.
	RDSProxyDiscoveryRegions []string
	// RedshiftDiscoveryRegions is a list of regions the Redshift
	// auto-discovery is configured.
	RedshiftDiscoveryRegions []string
	// RedshiftServerlessDiscoveryRegions is a list of regions the Redshift
	// Serverless auto-discovery is configured.
	RedshiftServerlessDiscoveryRegions []string
	// ElastiCacheDiscoveryRegions is a list of regions the ElastiCache
	// auto-discovery is configured.
	ElastiCacheDiscoveryRegions []string
	// MemoryDBDiscoveryRegions is a list of regions the MemoryDB
	// auto-discovery is configured.
	MemoryDBDiscoveryRegions []string
	// OpenSearchDiscoveryRegions is a list of regions the OpenSearch
	// auto-discovery is configured.
	OpenSearchDiscoveryRegions []string
	// AWSTags is the list of the AWS resource tags used for AWS discoveries.
	AWSTags map[string]string
	// AWSRawTags is the "raw" list of AWS resource tags used for AWS discoveries.
	AWSRawTags string
	// DatabaseProtocols is a list of database protocols supported.
	DatabaseProtocols []string
	// DatabaseAWSRegion is an optional database cloud region e.g. when using AWS RDS.
	DatabaseAWSRegion string
	// DatabaseAWSAccountID is an optional AWS account ID e.g. when using Keyspaces or DynamoDB.
	DatabaseAWSAccountID string
	// DatabaseAWSAssumeRoleARN is an optional AWS IAM role ARN to assume when accessing the database.
	DatabaseAWSAssumeRoleARN string
	// DatabaseAWSExternalID is an optional AWS database external ID, used when assuming roles.
	DatabaseAWSExternalID string
	// DatabaseAWSRedshiftClusterID is Redshift cluster identifier.
	DatabaseAWSRedshiftClusterID string
	// DatabaseAWSRDSClusterID is the RDS Aurora cluster identifier.
	DatabaseAWSRDSClusterID string
	// DatabaseAWSRDSInstanceID is the RDS instance identifier.
	DatabaseAWSRDSInstanceID string
	// DatabaseAWSElastiCacheGroupID is the ElastiCache replication group identifier.
	DatabaseAWSElastiCacheGroupID string
	// DatabaseAWSMemoryDBClusterName is the MemoryDB cluster name.
	DatabaseAWSMemoryDBClusterName string
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
	// DatabaseTrustSystemCertPool allows Teleport to trust certificate
	// authorities available on the host system.
	DatabaseTrustSystemCertPool bool
}

// CheckAndSetDefaults checks and sets default values for the flags.
func (f *DatabaseSampleFlags) CheckAndSetDefaults() error {
	conf := servicecfg.MakeDefaultConfig()
	f.DatabaseProtocols = defaults.DatabaseProtocols

	if f.NodeName == "" {
		f.NodeName = conf.Hostname
	}
	if f.DataDir == "" {
		f.DataDir = conf.DataDir
	}

	var err error
	if f.AWSTags, err = client.ParseLabelSpec(f.AWSRawTags); err != nil {
		return trace.Wrap(err)
	}
	if f.AzureTags, err = client.ParseLabelSpec(f.AzureRawTags); err != nil {
		return trace.Wrap(err)
	}

	if len(f.AWSTags) == 0 {
		f.AWSTags = map[string]string{types.Wildcard: types.Wildcard}
	}
	if len(f.AzureTags) == 0 {
		f.AzureTags = map[string]string{types.Wildcard: types.Wildcard}
	}

	if f.StaticDatabaseRawLabels != "" {
		f.StaticDatabaseStaticLabels, f.StaticDatabaseDynamicLabels, err = parseLabels(f.StaticDatabaseRawLabels)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Labels for "resources" section.
	for i := range f.DynamicResourcesRawLabels {
		labels, err := client.ParseLabelSpec(f.DynamicResourcesRawLabels[i])
		if err != nil {
			return trace.Wrap(err)
		}
		f.DynamicResourcesLabels = append(f.DynamicResourcesLabels, labels)
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

	// For consistent config checking, we parse the generated config and
	// run checks on it to ensure that generated config has no errors.
	fc, err := ReadConfig(bytes.NewBuffer(buf.Bytes()))
	if err != nil {
		return "", trace.Wrap(err)
	}
	cfg := servicecfg.MakeDefaultConfig()
	if err = ApplyFileConfig(fc, cfg); err != nil {
		return "", trace.Wrap(err)
	}
	return buf.String(), nil
}

// quote quotes a string, similar to the `quote` helper from Helm.
// Implementation reference: https://github.com/Masterminds/sprig/blob/3ac42c7bc5e4be6aa534e036fb19dde4a996da2e/strings.go#L83
func quote(str string) string {
	return fmt.Sprintf("%q", str)
}
