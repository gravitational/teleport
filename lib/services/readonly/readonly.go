/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package readonly

import (
	"time"

	protobuf "google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/constants"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
)

// NOTE: is best to avoid importing anything from lib other than lib/utils in this package in
// order to ensure that we can import it anywhere api/types is being used.

// AuthPreference is a read-only subset of types.AuthPreference used on certain hot paths
// to ensure that we do not modify the underlying AuthPreference as it may be shared across
// multiple goroutines.
type AuthPreference interface {
	GetDisconnectExpiredCert() bool
	GetLockingMode() constants.LockingMode
	GetDeviceTrust() *types.DeviceTrust
	GetPrivateKeyPolicy() keys.PrivateKeyPolicy
	GetSecondFactors() []types.SecondFactorType
	IsSecondFactorEnforced() bool
	IsSecondFactorTOTPAllowed() bool
	IsAdminActionMFAEnforced() bool
	GetRequireMFAType() types.RequireMFAType
	IsSAMLIdPEnabled() bool
	GetDefaultSessionTTL() types.Duration
	GetHardwareKeySerialNumberValidation() (*types.HardwareKeySerialNumberValidation, error)
	GetAllowPasswordless() bool
	GetStableUNIXUserConfig() *types.StableUNIXUserConfig

	GetRevision() string
	Clone() types.AuthPreference
}

type sealedAuthPreference struct {
	AuthPreference
}

// sealAuthPreference returns a read-only version of the AuthPreference.
func sealAuthPreference(p types.AuthPreference) AuthPreference {
	if p == nil {
		// preserving nils simplifies error flow-control
		return nil
	}
	return sealedAuthPreference{AuthPreference: p}
}

// ClusterNetworkingConfig is a read-only subset of types.ClusterNetworkingConfig used on certain hot paths
// to ensure that we do not modify the underlying ClusterNetworkingConfig as it may be shared across
// multiple goroutines.
type ClusterNetworkingConfig interface {
	GetCaseInsensitiveRouting() bool
	GetWebIdleTimeout() time.Duration
	GetRoutingStrategy() types.RoutingStrategy
	Clone() types.ClusterNetworkingConfig
}

type sealedClusterNetworkingConfig struct {
	ClusterNetworkingConfig
}

// sealClusterNetworkingConfig returns a read-only version of the ClusterNetworkingConfig.
func sealClusterNetworkingConfig(c ClusterNetworkingConfig) ClusterNetworkingConfig {
	if c == nil {
		// preserving nils simplifies error flow-control
		return nil
	}
	return sealedClusterNetworkingConfig{ClusterNetworkingConfig: c}
}

// SessionRecordingConfig is a read-only subset of types.SessionRecordingConfig used on certain hot paths
// to ensure that we do not modify the underlying SessionRecordingConfig as it may be shared across
// multiple goroutines.
type SessionRecordingConfig interface {
	GetMode() string
	GetProxyChecksHostKeys() bool
	Clone() types.SessionRecordingConfig
}

type sealedSessionRecordingConfig struct {
	SessionRecordingConfig
}

// sealSessionRecordingConfig returns a read-only version of the SessionRecordingConfig.
func sealSessionRecordingConfig(c SessionRecordingConfig) SessionRecordingConfig {
	if c == nil {
		// preserving nils simplifies error flow-control
		return nil
	}
	return sealedSessionRecordingConfig{SessionRecordingConfig: c}
}

// AccessGraphSettings is a read-only subset of clusterconfigpb.AccessGraphSettings used on certain hot paths
// to ensure that we do not modify the underlying AccessGraphSettings as it may be shared across
// multiple goroutines.
type AccessGraphSettings interface {
	SecretsScanConfig() clusterconfigpb.AccessGraphSecretsScanConfig
	Clone() *clusterconfigpb.AccessGraphSettings
}

type sealedAccessGraphSettings struct {
	*clusterconfigpb.AccessGraphSettings
}

// sealAccessGraphSettings returns a read-only version of the SessionRecordingConfig.
func sealAccessGraphSettings(c *clusterconfigpb.AccessGraphSettings) AccessGraphSettings {
	if c == nil {
		// preserving nils simplifies error flow-control
		return nil
	}
	return sealedAccessGraphSettings{c}
}

func (a sealedAccessGraphSettings) SecretsScanConfig() clusterconfigpb.AccessGraphSecretsScanConfig {
	return a.GetSpec().GetSecretsScanConfig()
}

func (a sealedAccessGraphSettings) Clone() *clusterconfigpb.AccessGraphSettings {
	return protobuf.Clone(a.AccessGraphSettings).(*clusterconfigpb.AccessGraphSettings)
}

// Resource is a read only variant of [types.Resource].
type Resource interface {
	// GetKind returns resource kind
	GetKind() string
	// GetSubKind returns resource subkind
	GetSubKind() string
	// GetVersion returns resource version
	GetVersion() string
	// GetName returns the name of the resource
	GetName() string
	// Expiry returns object expiry setting
	Expiry() time.Time
	// GetMetadata returns object metadata
	GetMetadata() types.Metadata
	// GetRevision returns the revision
	GetRevision() string
}

// ResourceWithOrigin is a read only variant of [types.ResourceWithOrigin].
type ResourceWithOrigin interface {
	Resource
	// Origin returns the origin value of the resource.
	Origin() string
}

// ResourceWithLabels is a read only variant of [types.ResourceWithLabels].
type ResourceWithLabels interface {
	ResourceWithOrigin
	// GetLabel retrieves the label with the provided key.
	GetLabel(key string) (value string, ok bool)
	// GetAllLabels returns all resource's labels.
	GetAllLabels() map[string]string
	// GetStaticLabels returns the resource's static labels.
	GetStaticLabels() map[string]string
	// MatchSearch goes through select field values of a resource
	// and tries to match against the list of search values.
	MatchSearch(searchValues []string) bool
}

// Application is a read only variant of [types.Application].
type Application interface {
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels
	// GetNamespace returns the app namespace.
	GetNamespace() string
	// GetStaticLabels returns the app static labels.
	GetStaticLabels() map[string]string
	// GetDynamicLabels returns the app dynamic labels.
	GetDynamicLabels() map[string]types.CommandLabel
	// String returns string representation of the app.
	String() string
	// GetDescription returns the app description.
	GetDescription() string
	// GetURI returns the app connection endpoint.
	GetURI() string
	// GetPublicAddr returns the app public address.
	GetPublicAddr() string
	// GetInsecureSkipVerify returns the app insecure setting.
	GetInsecureSkipVerify() bool
	// GetRewrite returns the app rewrite configuration.
	GetRewrite() *types.Rewrite
	// IsAWSConsole returns true if this app is AWS management console.
	IsAWSConsole() bool
	// IsAzureCloud returns true if this app represents Azure Cloud instance.
	IsAzureCloud() bool
	// IsGCP returns true if this app represents GCP instance.
	IsGCP() bool
	// IsTCP returns true if this app represents a TCP endpoint.
	IsTCP() bool
	// GetProtocol returns the application protocol.
	GetProtocol() string
	// GetAWSAccountID returns value of label containing AWS account ID on this app.
	GetAWSAccountID() string
	// GetAWSExternalID returns the AWS External ID configured for this app.
	GetAWSExternalID() string
	// GetAWSRolesAnywhereProfileARN returns the AWS IAM Roles Anywhere Profile ARN which originated this App.
	GetAWSRolesAnywhereProfileARN() string
	// GetAWSRolesAnywhereAcceptRoleSessionName returns whether the IAM Roles Anywhere Profile supports defining a custom AWS Session Name.
	GetAWSRolesAnywhereAcceptRoleSessionName() bool
	// GetUserGroups will get the list of user group IDs associated with the application.
	GetUserGroups() []string
	// Copy returns a copy of this app resource.
	Copy() *types.AppV3
	// GetIntegration will return the Integration.
	// If present, the Application must use the Integration's credentials instead of ambient credentials to access Cloud APIs.
	GetIntegration() string
	// GetRequiredAppNames will return a list of required apps names that should be authenticated during this apps authentication process.
	GetRequiredAppNames() []string
	// GetCORS returns the CORS configuration for the app.
	GetCORS() *types.CORSPolicy
}

// KubeServer is a read only variant of [types.KubeServer].
type KubeServer interface {
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels
	// GetNamespace returns server namespace.
	GetNamespace() string
	// GetTeleportVersion returns the teleport version the server is running on.
	GetTeleportVersion() string
	// GetHostname returns the server hostname.
	GetHostname() string
	// GetHostID returns ID of the host the server is running on.
	GetHostID() string
	// GetRotation gets the state of certificate authority rotation.
	GetRotation() types.Rotation
	// String returns string representation of the server.
	String() string
	// Copy returns a copy of this kube server object.
	Copy() types.KubeServer
	// CloneResource returns a copy of the KubeServer as a ResourceWithLabels
	CloneResource() types.ResourceWithLabels
	// GetCluster returns the Kubernetes Cluster this kube server proxies.
	GetCluster() types.KubeCluster
	// GetProxyIDs returns a list of proxy ids this service is connected to.
	GetProxyIDs() []string
}

// KubeCluster is a read only variant of [types.KubeCluster].
type KubeCluster interface {
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels
	// GetNamespace returns the kube cluster namespace.
	GetNamespace() string
	// GetStaticLabels returns the kube cluster static labels.
	GetStaticLabels() map[string]string
	// GetDynamicLabels returns the kube cluster dynamic labels.
	GetDynamicLabels() map[string]types.CommandLabel
	// GetKubeconfig returns the kubeconfig payload.
	GetKubeconfig() []byte
	// String returns string representation of the kube cluster.
	String() string
	// GetDescription returns the kube cluster description.
	GetDescription() string
	// GetAzureConfig gets the Azure config.
	GetAzureConfig() types.KubeAzure
	// GetAWSConfig gets the AWS config.
	GetAWSConfig() types.KubeAWS
	// GetGCPConfig gets the GCP config.
	GetGCPConfig() types.KubeGCP
	// IsAzure indentifies if the KubeCluster contains Azure details.
	IsAzure() bool
	// IsAWS indentifies if the KubeCluster contains AWS details.
	IsAWS() bool
	// IsGCP indentifies if the KubeCluster contains GCP details.
	IsGCP() bool
	// IsKubeconfig identifies if the KubeCluster contains kubeconfig data.
	IsKubeconfig() bool
	// Copy returns a copy of this kube cluster resource.
	Copy() *types.KubernetesClusterV3
	// GetCloud gets the cloud this kube cluster is running on, or an empty string if it
	// isn't running on a cloud provider.
	GetCloud() string
}

// Database is a read only variant of [types.Database].
type Database interface {
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels
	// GetNamespace returns the database namespace.
	GetNamespace() string
	// GetStaticLabels returns the database static labels.
	GetStaticLabels() map[string]string
	// GetDynamicLabels returns the database dynamic labels.
	GetDynamicLabels() map[string]types.CommandLabel
	// String returns string representation of the database.
	String() string
	// GetDescription returns the database description.
	GetDescription() string
	// GetProtocol returns the database protocol.
	GetProtocol() string
	// GetURI returns the database connection endpoint.
	GetURI() string
	// GetCA returns the database CA certificate.
	GetCA() string
	// GetTLS returns the database TLS configuration.
	GetTLS() types.DatabaseTLS
	// GetStatusCA gets the database CA certificate in the status field.
	GetStatusCA() string
	// GetMySQL returns the database options from spec.
	GetMySQL() types.MySQLOptions
	// GetOracle returns the database options from spec.
	GetOracle() types.OracleOptions
	// GetMySQLServerVersion returns the MySQL server version either from configuration or
	// reported by the database.
	GetMySQLServerVersion() string
	// GetAWS returns the database AWS metadata.
	GetAWS() types.AWS
	// GetGCP returns GCP information for Cloud SQL databases.
	GetGCP() types.GCPCloudSQL
	// GetAzure returns Azure database server metadata.
	GetAzure() types.Azure
	// GetAD returns Active Directory database configuration.
	GetAD() types.AD
	// GetType returns the database authentication type: self-hosted, RDS, Redshift or Cloud SQL.
	GetType() string
	// GetSecretStore returns secret store configurations.
	GetSecretStore() types.SecretStore
	// GetManagedUsers returns a list of database users that are managed by Teleport.
	GetManagedUsers() []string
	// GetMongoAtlas returns Mongo Atlas database metadata.
	GetMongoAtlas() types.MongoAtlas
	// IsRDS returns true if this is an RDS/Aurora database.
	IsRDS() bool
	// IsRDSProxy returns true if this is an RDS Proxy database.
	IsRDSProxy() bool
	// IsRedshift returns true if this is a Redshift database.
	IsRedshift() bool
	// IsCloudSQL returns true if this is a Cloud SQL database.
	IsCloudSQL() bool
	// IsAzure returns true if this is an Azure database.
	IsAzure() bool
	// IsElastiCache returns true if this is an AWS ElastiCache database.
	IsElastiCache() bool
	// IsMemoryDB returns true if this is an AWS MemoryDB database.
	IsMemoryDB() bool
	// IsAWSHosted returns true if database is hosted by AWS.
	IsAWSHosted() bool
	// IsCloudHosted returns true if database is hosted in the cloud (AWS, Azure or Cloud SQL).
	IsCloudHosted() bool
	// RequireAWSIAMRolesAsUsers returns true for database types that require
	// AWS IAM roles as database users.
	RequireAWSIAMRolesAsUsers() bool
	// SupportAWSIAMRoleARNAsUsers returns true for database types that support
	// AWS IAM roles as database users.
	SupportAWSIAMRoleARNAsUsers() bool
	// Copy returns a copy of this database resource.
	Copy() *types.DatabaseV3
	// GetAdminUser returns database privileged user information.
	GetAdminUser() types.DatabaseAdminUser
	// SupportsAutoUsers returns true if this database supports automatic
	// user provisioning.
	SupportsAutoUsers() bool
	// GetEndpointType returns the endpoint type of the database, if available.
	GetEndpointType() string
	// GetCloud gets the cloud this database is running on, or an empty string if it
	// isn't running on a cloud provider.
	GetCloud() string
	// IsUsernameCaseInsensitive returns true if the database username is case
	// insensitive.
	IsUsernameCaseInsensitive() bool
}

// Server is a read only variant of [types.Server].
type Server interface {
	// ResourceWithLabels provides common resource headers
	ResourceWithLabels
	// GetTeleportVersion returns the teleport version the server is running on
	GetTeleportVersion() string
	// GetAddr return server address
	GetAddr() string
	// GetHostname returns server hostname
	GetHostname() string
	// GetNamespace returns server namespace
	GetNamespace() string
	// GetLabels returns server's static label key pairs
	GetLabels() map[string]string
	// GetCmdLabels gets command labels
	GetCmdLabels() map[string]types.CommandLabel
	// GetPublicAddr returns a public address where this server can be reached.
	GetPublicAddr() string
	// GetPublicAddrs returns a list of public addresses where this server can be reached.
	GetPublicAddrs() []string
	// GetRotation gets the state of certificate authority rotation.
	GetRotation() types.Rotation
	// GetUseTunnel gets if a reverse tunnel should be used to connect to this node.
	GetUseTunnel() bool
	// String returns string representation of the server
	String() string
	// GetPeerAddr returns the peer address of the server.
	GetPeerAddr() string
	// GetProxyIDs returns a list of proxy ids this service is connected to.
	GetProxyIDs() []string
	// DeepCopy creates a clone of this server value
	DeepCopy() types.Server

	// CloneResource is used to return a clone of the Server and match the CloneAny interface
	// This is helpful when interfacing with multiple types at the same time in unified resources
	CloneResource() types.ResourceWithLabels

	// GetCloudMetadata gets the cloud metadata for the server.
	GetCloudMetadata() *types.CloudMetadata
	// GetAWSInfo returns the AWSInfo for the server.
	GetAWSInfo() *types.AWSInfo

	// IsOpenSSHNode returns whether the connection to this Server must use OpenSSH.
	// This returns true for SubKindOpenSSHNode and SubKindOpenSSHEICENode.
	IsOpenSSHNode() bool

	// IsEICE returns whether the Node is an EICE instance.
	// Must be `openssh-ec2-ice` subkind and have the AccountID and InstanceID information (AWS Metadata or Labels).
	IsEICE() bool

	// GetAWSInstanceID returns the AWS Instance ID if this node comes from an EC2 instance.
	GetAWSInstanceID() string
	// GetAWSAccountID returns the AWS Account ID if this node comes from an EC2 instance.
	GetAWSAccountID() string

	// GetGitHub returns the GitHub server spec.
	GetGitHub() *types.GitHubServerMetadata
}

// DynamicWindowsDesktop represents a Windows desktop host that is automatically discovered by Windows Desktop Service.
type DynamicWindowsDesktop interface {
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels
	// GetAddr returns the network address of this host.
	GetAddr() string
	// GetDomain returns the ActiveDirectory domain of this host.
	GetDomain() string
	// NonAD checks whether this is a standalone host that
	// is not joined to an Active Directory domain.
	NonAD() bool
	// GetScreenSize returns the desired size of the screen to use for sessions
	// to this host. Returns (0, 0) if no screen size is set, which means to
	// use the size passed by the client over TDP.
	GetScreenSize() (width, height uint32)
	// Copy returns a copy of this dynamic Windows desktop
	Copy() *types.DynamicWindowsDesktopV1
}
