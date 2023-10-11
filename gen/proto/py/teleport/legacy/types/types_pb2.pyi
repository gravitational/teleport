from gogoproto import gogo_pb2 as _gogo_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from teleport.attestation.v1 import attestation_pb2 as _attestation_pb2
from teleport.legacy.types.wrappers import wrappers_pb2 as _wrappers_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class IAMPolicyStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    IAM_POLICY_STATUS_UNSPECIFIED: _ClassVar[IAMPolicyStatus]
    IAM_POLICY_STATUS_PENDING: _ClassVar[IAMPolicyStatus]
    IAM_POLICY_STATUS_FAILED: _ClassVar[IAMPolicyStatus]
    IAM_POLICY_STATUS_SUCCESS: _ClassVar[IAMPolicyStatus]

class DatabaseTLSMode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    VERIFY_FULL: _ClassVar[DatabaseTLSMode]
    VERIFY_CA: _ClassVar[DatabaseTLSMode]
    INSECURE: _ClassVar[DatabaseTLSMode]

class PrivateKeyType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    RAW: _ClassVar[PrivateKeyType]
    PKCS11: _ClassVar[PrivateKeyType]
    GCP_KMS: _ClassVar[PrivateKeyType]

class ProxyListenerMode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    Separate: _ClassVar[ProxyListenerMode]
    Multiplex: _ClassVar[ProxyListenerMode]

class RoutingStrategy(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    UNAMBIGUOUS_MATCH: _ClassVar[RoutingStrategy]
    MOST_RECENT: _ClassVar[RoutingStrategy]

class UserTokenUsage(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    USER_TOKEN_USAGE_UNSPECIFIED: _ClassVar[UserTokenUsage]
    USER_TOKEN_RECOVER_PASSWORD: _ClassVar[UserTokenUsage]
    USER_TOKEN_RECOVER_MFA: _ClassVar[UserTokenUsage]
    USER_TOKEN_RENEWAL_BOT: _ClassVar[UserTokenUsage]

class RequestState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    NONE: _ClassVar[RequestState]
    PENDING: _ClassVar[RequestState]
    APPROVED: _ClassVar[RequestState]
    DENIED: _ClassVar[RequestState]
    PROMOTED: _ClassVar[RequestState]

class CreateHostUserMode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    HOST_USER_MODE_UNSPECIFIED: _ClassVar[CreateHostUserMode]
    HOST_USER_MODE_OFF: _ClassVar[CreateHostUserMode]
    HOST_USER_MODE_DROP: _ClassVar[CreateHostUserMode]
    HOST_USER_MODE_KEEP: _ClassVar[CreateHostUserMode]

class CertExtensionMode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    EXTENSION: _ClassVar[CertExtensionMode]

class CertExtensionType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    SSH: _ClassVar[CertExtensionType]

class SessionState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    SessionStatePending: _ClassVar[SessionState]
    SessionStateRunning: _ClassVar[SessionState]
    SessionStateTerminated: _ClassVar[SessionState]

class AlertSeverity(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    LOW: _ClassVar[AlertSeverity]
    MEDIUM: _ClassVar[AlertSeverity]
    HIGH: _ClassVar[AlertSeverity]

class RequireMFAType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    OFF: _ClassVar[RequireMFAType]
    SESSION: _ClassVar[RequireMFAType]
    SESSION_AND_HARDWARE_KEY: _ClassVar[RequireMFAType]
    HARDWARE_KEY_TOUCH: _ClassVar[RequireMFAType]

class PluginStatusCode(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    UNKNOWN: _ClassVar[PluginStatusCode]
    RUNNING: _ClassVar[PluginStatusCode]
    OTHER_ERROR: _ClassVar[PluginStatusCode]
    UNAUTHORIZED: _ClassVar[PluginStatusCode]
    SLACK_NOT_IN_CHANNEL: _ClassVar[PluginStatusCode]

class HeadlessAuthenticationState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    HEADLESS_AUTHENTICATION_STATE_UNSPECIFIED: _ClassVar[HeadlessAuthenticationState]
    HEADLESS_AUTHENTICATION_STATE_PENDING: _ClassVar[HeadlessAuthenticationState]
    HEADLESS_AUTHENTICATION_STATE_DENIED: _ClassVar[HeadlessAuthenticationState]
    HEADLESS_AUTHENTICATION_STATE_APPROVED: _ClassVar[HeadlessAuthenticationState]
IAM_POLICY_STATUS_UNSPECIFIED: IAMPolicyStatus
IAM_POLICY_STATUS_PENDING: IAMPolicyStatus
IAM_POLICY_STATUS_FAILED: IAMPolicyStatus
IAM_POLICY_STATUS_SUCCESS: IAMPolicyStatus
VERIFY_FULL: DatabaseTLSMode
VERIFY_CA: DatabaseTLSMode
INSECURE: DatabaseTLSMode
RAW: PrivateKeyType
PKCS11: PrivateKeyType
GCP_KMS: PrivateKeyType
Separate: ProxyListenerMode
Multiplex: ProxyListenerMode
UNAMBIGUOUS_MATCH: RoutingStrategy
MOST_RECENT: RoutingStrategy
USER_TOKEN_USAGE_UNSPECIFIED: UserTokenUsage
USER_TOKEN_RECOVER_PASSWORD: UserTokenUsage
USER_TOKEN_RECOVER_MFA: UserTokenUsage
USER_TOKEN_RENEWAL_BOT: UserTokenUsage
NONE: RequestState
PENDING: RequestState
APPROVED: RequestState
DENIED: RequestState
PROMOTED: RequestState
HOST_USER_MODE_UNSPECIFIED: CreateHostUserMode
HOST_USER_MODE_OFF: CreateHostUserMode
HOST_USER_MODE_DROP: CreateHostUserMode
HOST_USER_MODE_KEEP: CreateHostUserMode
EXTENSION: CertExtensionMode
SSH: CertExtensionType
SessionStatePending: SessionState
SessionStateRunning: SessionState
SessionStateTerminated: SessionState
LOW: AlertSeverity
MEDIUM: AlertSeverity
HIGH: AlertSeverity
OFF: RequireMFAType
SESSION: RequireMFAType
SESSION_AND_HARDWARE_KEY: RequireMFAType
HARDWARE_KEY_TOUCH: RequireMFAType
UNKNOWN: PluginStatusCode
RUNNING: PluginStatusCode
OTHER_ERROR: PluginStatusCode
UNAUTHORIZED: PluginStatusCode
SLACK_NOT_IN_CHANNEL: PluginStatusCode
HEADLESS_AUTHENTICATION_STATE_UNSPECIFIED: HeadlessAuthenticationState
HEADLESS_AUTHENTICATION_STATE_PENDING: HeadlessAuthenticationState
HEADLESS_AUTHENTICATION_STATE_DENIED: HeadlessAuthenticationState
HEADLESS_AUTHENTICATION_STATE_APPROVED: HeadlessAuthenticationState

class KeepAlive(_message.Message):
    __slots__ = ["Name", "Namespace", "LeaseID", "Expires", "Type", "HostID"]
    class KeepAliveType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = []
        UNKNOWN: _ClassVar[KeepAlive.KeepAliveType]
        NODE: _ClassVar[KeepAlive.KeepAliveType]
        APP: _ClassVar[KeepAlive.KeepAliveType]
        DATABASE: _ClassVar[KeepAlive.KeepAliveType]
        WINDOWS_DESKTOP: _ClassVar[KeepAlive.KeepAliveType]
        KUBERNETES: _ClassVar[KeepAlive.KeepAliveType]
        DATABASE_SERVICE: _ClassVar[KeepAlive.KeepAliveType]
    UNKNOWN: KeepAlive.KeepAliveType
    NODE: KeepAlive.KeepAliveType
    APP: KeepAlive.KeepAliveType
    DATABASE: KeepAlive.KeepAliveType
    WINDOWS_DESKTOP: KeepAlive.KeepAliveType
    KUBERNETES: KeepAlive.KeepAliveType
    DATABASE_SERVICE: KeepAlive.KeepAliveType
    NAME_FIELD_NUMBER: _ClassVar[int]
    NAMESPACE_FIELD_NUMBER: _ClassVar[int]
    LEASEID_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    HOSTID_FIELD_NUMBER: _ClassVar[int]
    Name: str
    Namespace: str
    LeaseID: int
    Expires: _timestamp_pb2.Timestamp
    Type: KeepAlive.KeepAliveType
    HostID: str
    def __init__(self, Name: _Optional[str] = ..., Namespace: _Optional[str] = ..., LeaseID: _Optional[int] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Type: _Optional[_Union[KeepAlive.KeepAliveType, str]] = ..., HostID: _Optional[str] = ...) -> None: ...

class Metadata(_message.Message):
    __slots__ = ["Name", "Namespace", "Description", "Labels", "Expires", "ID", "Revision"]
    class LabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    NAMESPACE_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    REVISION_FIELD_NUMBER: _ClassVar[int]
    Name: str
    Namespace: str
    Description: str
    Labels: _containers.ScalarMap[str, str]
    Expires: _timestamp_pb2.Timestamp
    ID: int
    Revision: str
    def __init__(self, Name: _Optional[str] = ..., Namespace: _Optional[str] = ..., Description: _Optional[str] = ..., Labels: _Optional[_Mapping[str, str]] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., ID: _Optional[int] = ..., Revision: _Optional[str] = ...) -> None: ...

class Rotation(_message.Message):
    __slots__ = ["State", "Phase", "Mode", "CurrentID", "Started", "GracePeriod", "LastRotated", "Schedule"]
    STATE_FIELD_NUMBER: _ClassVar[int]
    PHASE_FIELD_NUMBER: _ClassVar[int]
    MODE_FIELD_NUMBER: _ClassVar[int]
    CURRENTID_FIELD_NUMBER: _ClassVar[int]
    STARTED_FIELD_NUMBER: _ClassVar[int]
    GRACEPERIOD_FIELD_NUMBER: _ClassVar[int]
    LASTROTATED_FIELD_NUMBER: _ClassVar[int]
    SCHEDULE_FIELD_NUMBER: _ClassVar[int]
    State: str
    Phase: str
    Mode: str
    CurrentID: str
    Started: _timestamp_pb2.Timestamp
    GracePeriod: int
    LastRotated: _timestamp_pb2.Timestamp
    Schedule: RotationSchedule
    def __init__(self, State: _Optional[str] = ..., Phase: _Optional[str] = ..., Mode: _Optional[str] = ..., CurrentID: _Optional[str] = ..., Started: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., GracePeriod: _Optional[int] = ..., LastRotated: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Schedule: _Optional[_Union[RotationSchedule, _Mapping]] = ...) -> None: ...

class RotationSchedule(_message.Message):
    __slots__ = ["UpdateClients", "UpdateServers", "Standby"]
    UPDATECLIENTS_FIELD_NUMBER: _ClassVar[int]
    UPDATESERVERS_FIELD_NUMBER: _ClassVar[int]
    STANDBY_FIELD_NUMBER: _ClassVar[int]
    UpdateClients: _timestamp_pb2.Timestamp
    UpdateServers: _timestamp_pb2.Timestamp
    Standby: _timestamp_pb2.Timestamp
    def __init__(self, UpdateClients: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., UpdateServers: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Standby: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ResourceHeader(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ...) -> None: ...

class DatabaseServerV3(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: DatabaseServerSpecV3
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[DatabaseServerSpecV3, _Mapping]] = ...) -> None: ...

class DatabaseServerSpecV3(_message.Message):
    __slots__ = ["Version", "Hostname", "HostID", "Rotation", "Database", "ProxyIDs"]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    HOSTNAME_FIELD_NUMBER: _ClassVar[int]
    HOSTID_FIELD_NUMBER: _ClassVar[int]
    ROTATION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    PROXYIDS_FIELD_NUMBER: _ClassVar[int]
    Version: str
    Hostname: str
    HostID: str
    Rotation: Rotation
    Database: DatabaseV3
    ProxyIDs: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Version: _Optional[str] = ..., Hostname: _Optional[str] = ..., HostID: _Optional[str] = ..., Rotation: _Optional[_Union[Rotation, _Mapping]] = ..., Database: _Optional[_Union[DatabaseV3, _Mapping]] = ..., ProxyIDs: _Optional[_Iterable[str]] = ...) -> None: ...

class DatabaseV3List(_message.Message):
    __slots__ = ["Databases"]
    DATABASES_FIELD_NUMBER: _ClassVar[int]
    Databases: _containers.RepeatedCompositeFieldContainer[DatabaseV3]
    def __init__(self, Databases: _Optional[_Iterable[_Union[DatabaseV3, _Mapping]]] = ...) -> None: ...

class DatabaseV3(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec", "Status"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: DatabaseSpecV3
    Status: DatabaseStatusV3
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[DatabaseSpecV3, _Mapping]] = ..., Status: _Optional[_Union[DatabaseStatusV3, _Mapping]] = ...) -> None: ...

class DatabaseSpecV3(_message.Message):
    __slots__ = ["Protocol", "URI", "CACert", "DynamicLabels", "AWS", "GCP", "Azure", "TLS", "AD", "MySQL", "AdminUser", "MongoAtlas", "Oracle"]
    class DynamicLabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: CommandLabelV2
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[CommandLabelV2, _Mapping]] = ...) -> None: ...
    PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    CACERT_FIELD_NUMBER: _ClassVar[int]
    DYNAMICLABELS_FIELD_NUMBER: _ClassVar[int]
    AWS_FIELD_NUMBER: _ClassVar[int]
    GCP_FIELD_NUMBER: _ClassVar[int]
    AZURE_FIELD_NUMBER: _ClassVar[int]
    TLS_FIELD_NUMBER: _ClassVar[int]
    AD_FIELD_NUMBER: _ClassVar[int]
    MYSQL_FIELD_NUMBER: _ClassVar[int]
    ADMINUSER_FIELD_NUMBER: _ClassVar[int]
    MONGOATLAS_FIELD_NUMBER: _ClassVar[int]
    ORACLE_FIELD_NUMBER: _ClassVar[int]
    Protocol: str
    URI: str
    CACert: str
    DynamicLabels: _containers.MessageMap[str, CommandLabelV2]
    AWS: AWS
    GCP: GCPCloudSQL
    Azure: Azure
    TLS: DatabaseTLS
    AD: AD
    MySQL: MySQLOptions
    AdminUser: DatabaseAdminUser
    MongoAtlas: MongoAtlas
    Oracle: OracleOptions
    def __init__(self, Protocol: _Optional[str] = ..., URI: _Optional[str] = ..., CACert: _Optional[str] = ..., DynamicLabels: _Optional[_Mapping[str, CommandLabelV2]] = ..., AWS: _Optional[_Union[AWS, _Mapping]] = ..., GCP: _Optional[_Union[GCPCloudSQL, _Mapping]] = ..., Azure: _Optional[_Union[Azure, _Mapping]] = ..., TLS: _Optional[_Union[DatabaseTLS, _Mapping]] = ..., AD: _Optional[_Union[AD, _Mapping]] = ..., MySQL: _Optional[_Union[MySQLOptions, _Mapping]] = ..., AdminUser: _Optional[_Union[DatabaseAdminUser, _Mapping]] = ..., MongoAtlas: _Optional[_Union[MongoAtlas, _Mapping]] = ..., Oracle: _Optional[_Union[OracleOptions, _Mapping]] = ...) -> None: ...

class DatabaseAdminUser(_message.Message):
    __slots__ = ["Name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Name: str
    def __init__(self, Name: _Optional[str] = ...) -> None: ...

class OracleOptions(_message.Message):
    __slots__ = ["AuditUser"]
    AUDITUSER_FIELD_NUMBER: _ClassVar[int]
    AuditUser: str
    def __init__(self, AuditUser: _Optional[str] = ...) -> None: ...

class DatabaseStatusV3(_message.Message):
    __slots__ = ["CACert", "AWS", "MySQL", "ManagedUsers", "Azure"]
    CACERT_FIELD_NUMBER: _ClassVar[int]
    AWS_FIELD_NUMBER: _ClassVar[int]
    MYSQL_FIELD_NUMBER: _ClassVar[int]
    MANAGEDUSERS_FIELD_NUMBER: _ClassVar[int]
    AZURE_FIELD_NUMBER: _ClassVar[int]
    CACert: str
    AWS: AWS
    MySQL: MySQLOptions
    ManagedUsers: _containers.RepeatedScalarFieldContainer[str]
    Azure: Azure
    def __init__(self, CACert: _Optional[str] = ..., AWS: _Optional[_Union[AWS, _Mapping]] = ..., MySQL: _Optional[_Union[MySQLOptions, _Mapping]] = ..., ManagedUsers: _Optional[_Iterable[str]] = ..., Azure: _Optional[_Union[Azure, _Mapping]] = ...) -> None: ...

class AWS(_message.Message):
    __slots__ = ["Region", "Redshift", "RDS", "AccountID", "ElastiCache", "SecretStore", "MemoryDB", "RDSProxy", "RedshiftServerless", "ExternalID", "AssumeRoleARN", "OpenSearch", "IAMPolicyStatus"]
    REGION_FIELD_NUMBER: _ClassVar[int]
    REDSHIFT_FIELD_NUMBER: _ClassVar[int]
    RDS_FIELD_NUMBER: _ClassVar[int]
    ACCOUNTID_FIELD_NUMBER: _ClassVar[int]
    ELASTICACHE_FIELD_NUMBER: _ClassVar[int]
    SECRETSTORE_FIELD_NUMBER: _ClassVar[int]
    MEMORYDB_FIELD_NUMBER: _ClassVar[int]
    RDSPROXY_FIELD_NUMBER: _ClassVar[int]
    REDSHIFTSERVERLESS_FIELD_NUMBER: _ClassVar[int]
    EXTERNALID_FIELD_NUMBER: _ClassVar[int]
    ASSUMEROLEARN_FIELD_NUMBER: _ClassVar[int]
    OPENSEARCH_FIELD_NUMBER: _ClassVar[int]
    IAMPOLICYSTATUS_FIELD_NUMBER: _ClassVar[int]
    Region: str
    Redshift: Redshift
    RDS: RDS
    AccountID: str
    ElastiCache: ElastiCache
    SecretStore: SecretStore
    MemoryDB: MemoryDB
    RDSProxy: RDSProxy
    RedshiftServerless: RedshiftServerless
    ExternalID: str
    AssumeRoleARN: str
    OpenSearch: OpenSearch
    IAMPolicyStatus: IAMPolicyStatus
    def __init__(self, Region: _Optional[str] = ..., Redshift: _Optional[_Union[Redshift, _Mapping]] = ..., RDS: _Optional[_Union[RDS, _Mapping]] = ..., AccountID: _Optional[str] = ..., ElastiCache: _Optional[_Union[ElastiCache, _Mapping]] = ..., SecretStore: _Optional[_Union[SecretStore, _Mapping]] = ..., MemoryDB: _Optional[_Union[MemoryDB, _Mapping]] = ..., RDSProxy: _Optional[_Union[RDSProxy, _Mapping]] = ..., RedshiftServerless: _Optional[_Union[RedshiftServerless, _Mapping]] = ..., ExternalID: _Optional[str] = ..., AssumeRoleARN: _Optional[str] = ..., OpenSearch: _Optional[_Union[OpenSearch, _Mapping]] = ..., IAMPolicyStatus: _Optional[_Union[IAMPolicyStatus, str]] = ...) -> None: ...

class SecretStore(_message.Message):
    __slots__ = ["KeyPrefix", "KMSKeyID"]
    KEYPREFIX_FIELD_NUMBER: _ClassVar[int]
    KMSKEYID_FIELD_NUMBER: _ClassVar[int]
    KeyPrefix: str
    KMSKeyID: str
    def __init__(self, KeyPrefix: _Optional[str] = ..., KMSKeyID: _Optional[str] = ...) -> None: ...

class Redshift(_message.Message):
    __slots__ = ["ClusterID"]
    CLUSTERID_FIELD_NUMBER: _ClassVar[int]
    ClusterID: str
    def __init__(self, ClusterID: _Optional[str] = ...) -> None: ...

class RDS(_message.Message):
    __slots__ = ["InstanceID", "ClusterID", "ResourceID", "IAMAuth", "Subnets", "VPCID"]
    INSTANCEID_FIELD_NUMBER: _ClassVar[int]
    CLUSTERID_FIELD_NUMBER: _ClassVar[int]
    RESOURCEID_FIELD_NUMBER: _ClassVar[int]
    IAMAUTH_FIELD_NUMBER: _ClassVar[int]
    SUBNETS_FIELD_NUMBER: _ClassVar[int]
    VPCID_FIELD_NUMBER: _ClassVar[int]
    InstanceID: str
    ClusterID: str
    ResourceID: str
    IAMAuth: bool
    Subnets: _containers.RepeatedScalarFieldContainer[str]
    VPCID: str
    def __init__(self, InstanceID: _Optional[str] = ..., ClusterID: _Optional[str] = ..., ResourceID: _Optional[str] = ..., IAMAuth: bool = ..., Subnets: _Optional[_Iterable[str]] = ..., VPCID: _Optional[str] = ...) -> None: ...

class RDSProxy(_message.Message):
    __slots__ = ["Name", "CustomEndpointName", "ResourceID"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    CUSTOMENDPOINTNAME_FIELD_NUMBER: _ClassVar[int]
    RESOURCEID_FIELD_NUMBER: _ClassVar[int]
    Name: str
    CustomEndpointName: str
    ResourceID: str
    def __init__(self, Name: _Optional[str] = ..., CustomEndpointName: _Optional[str] = ..., ResourceID: _Optional[str] = ...) -> None: ...

class ElastiCache(_message.Message):
    __slots__ = ["ReplicationGroupID", "UserGroupIDs", "TransitEncryptionEnabled", "EndpointType"]
    REPLICATIONGROUPID_FIELD_NUMBER: _ClassVar[int]
    USERGROUPIDS_FIELD_NUMBER: _ClassVar[int]
    TRANSITENCRYPTIONENABLED_FIELD_NUMBER: _ClassVar[int]
    ENDPOINTTYPE_FIELD_NUMBER: _ClassVar[int]
    ReplicationGroupID: str
    UserGroupIDs: _containers.RepeatedScalarFieldContainer[str]
    TransitEncryptionEnabled: bool
    EndpointType: str
    def __init__(self, ReplicationGroupID: _Optional[str] = ..., UserGroupIDs: _Optional[_Iterable[str]] = ..., TransitEncryptionEnabled: bool = ..., EndpointType: _Optional[str] = ...) -> None: ...

class MemoryDB(_message.Message):
    __slots__ = ["ClusterName", "ACLName", "TLSEnabled", "EndpointType"]
    CLUSTERNAME_FIELD_NUMBER: _ClassVar[int]
    ACLNAME_FIELD_NUMBER: _ClassVar[int]
    TLSENABLED_FIELD_NUMBER: _ClassVar[int]
    ENDPOINTTYPE_FIELD_NUMBER: _ClassVar[int]
    ClusterName: str
    ACLName: str
    TLSEnabled: bool
    EndpointType: str
    def __init__(self, ClusterName: _Optional[str] = ..., ACLName: _Optional[str] = ..., TLSEnabled: bool = ..., EndpointType: _Optional[str] = ...) -> None: ...

class RedshiftServerless(_message.Message):
    __slots__ = ["WorkgroupName", "EndpointName", "WorkgroupID"]
    WORKGROUPNAME_FIELD_NUMBER: _ClassVar[int]
    ENDPOINTNAME_FIELD_NUMBER: _ClassVar[int]
    WORKGROUPID_FIELD_NUMBER: _ClassVar[int]
    WorkgroupName: str
    EndpointName: str
    WorkgroupID: str
    def __init__(self, WorkgroupName: _Optional[str] = ..., EndpointName: _Optional[str] = ..., WorkgroupID: _Optional[str] = ...) -> None: ...

class OpenSearch(_message.Message):
    __slots__ = ["DomainName", "DomainID", "EndpointType"]
    DOMAINNAME_FIELD_NUMBER: _ClassVar[int]
    DOMAINID_FIELD_NUMBER: _ClassVar[int]
    ENDPOINTTYPE_FIELD_NUMBER: _ClassVar[int]
    DomainName: str
    DomainID: str
    EndpointType: str
    def __init__(self, DomainName: _Optional[str] = ..., DomainID: _Optional[str] = ..., EndpointType: _Optional[str] = ...) -> None: ...

class GCPCloudSQL(_message.Message):
    __slots__ = ["ProjectID", "InstanceID"]
    PROJECTID_FIELD_NUMBER: _ClassVar[int]
    INSTANCEID_FIELD_NUMBER: _ClassVar[int]
    ProjectID: str
    InstanceID: str
    def __init__(self, ProjectID: _Optional[str] = ..., InstanceID: _Optional[str] = ...) -> None: ...

class Azure(_message.Message):
    __slots__ = ["Name", "ResourceID", "Redis", "IsFlexiServer"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    RESOURCEID_FIELD_NUMBER: _ClassVar[int]
    REDIS_FIELD_NUMBER: _ClassVar[int]
    ISFLEXISERVER_FIELD_NUMBER: _ClassVar[int]
    Name: str
    ResourceID: str
    Redis: AzureRedis
    IsFlexiServer: bool
    def __init__(self, Name: _Optional[str] = ..., ResourceID: _Optional[str] = ..., Redis: _Optional[_Union[AzureRedis, _Mapping]] = ..., IsFlexiServer: bool = ...) -> None: ...

class AzureRedis(_message.Message):
    __slots__ = ["ClusteringPolicy"]
    CLUSTERINGPOLICY_FIELD_NUMBER: _ClassVar[int]
    ClusteringPolicy: str
    def __init__(self, ClusteringPolicy: _Optional[str] = ...) -> None: ...

class AD(_message.Message):
    __slots__ = ["KeytabFile", "Krb5File", "Domain", "SPN", "LDAPCert", "KDCHostName"]
    KEYTABFILE_FIELD_NUMBER: _ClassVar[int]
    KRB5FILE_FIELD_NUMBER: _ClassVar[int]
    DOMAIN_FIELD_NUMBER: _ClassVar[int]
    SPN_FIELD_NUMBER: _ClassVar[int]
    LDAPCERT_FIELD_NUMBER: _ClassVar[int]
    KDCHOSTNAME_FIELD_NUMBER: _ClassVar[int]
    KeytabFile: str
    Krb5File: str
    Domain: str
    SPN: str
    LDAPCert: str
    KDCHostName: str
    def __init__(self, KeytabFile: _Optional[str] = ..., Krb5File: _Optional[str] = ..., Domain: _Optional[str] = ..., SPN: _Optional[str] = ..., LDAPCert: _Optional[str] = ..., KDCHostName: _Optional[str] = ...) -> None: ...

class DatabaseTLS(_message.Message):
    __slots__ = ["Mode", "CACert", "ServerName"]
    MODE_FIELD_NUMBER: _ClassVar[int]
    CACERT_FIELD_NUMBER: _ClassVar[int]
    SERVERNAME_FIELD_NUMBER: _ClassVar[int]
    Mode: DatabaseTLSMode
    CACert: str
    ServerName: str
    def __init__(self, Mode: _Optional[_Union[DatabaseTLSMode, str]] = ..., CACert: _Optional[str] = ..., ServerName: _Optional[str] = ...) -> None: ...

class MySQLOptions(_message.Message):
    __slots__ = ["ServerVersion"]
    SERVERVERSION_FIELD_NUMBER: _ClassVar[int]
    ServerVersion: str
    def __init__(self, ServerVersion: _Optional[str] = ...) -> None: ...

class MongoAtlas(_message.Message):
    __slots__ = ["Name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Name: str
    def __init__(self, Name: _Optional[str] = ...) -> None: ...

class InstanceV1(_message.Message):
    __slots__ = ["Header", "Spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Header: ResourceHeader
    Spec: InstanceSpecV1
    def __init__(self, Header: _Optional[_Union[ResourceHeader, _Mapping]] = ..., Spec: _Optional[_Union[InstanceSpecV1, _Mapping]] = ...) -> None: ...

class InstanceSpecV1(_message.Message):
    __slots__ = ["Version", "Services", "Hostname", "AuthID", "LastSeen", "ControlLog", "ExternalUpgrader"]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    SERVICES_FIELD_NUMBER: _ClassVar[int]
    HOSTNAME_FIELD_NUMBER: _ClassVar[int]
    AUTHID_FIELD_NUMBER: _ClassVar[int]
    LASTSEEN_FIELD_NUMBER: _ClassVar[int]
    CONTROLLOG_FIELD_NUMBER: _ClassVar[int]
    EXTERNALUPGRADER_FIELD_NUMBER: _ClassVar[int]
    Version: str
    Services: _containers.RepeatedScalarFieldContainer[str]
    Hostname: str
    AuthID: str
    LastSeen: _timestamp_pb2.Timestamp
    ControlLog: _containers.RepeatedCompositeFieldContainer[InstanceControlLogEntry]
    ExternalUpgrader: str
    def __init__(self, Version: _Optional[str] = ..., Services: _Optional[_Iterable[str]] = ..., Hostname: _Optional[str] = ..., AuthID: _Optional[str] = ..., LastSeen: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., ControlLog: _Optional[_Iterable[_Union[InstanceControlLogEntry, _Mapping]]] = ..., ExternalUpgrader: _Optional[str] = ...) -> None: ...

class InstanceControlLogEntry(_message.Message):
    __slots__ = ["Type", "ID", "Time", "TTL", "Labels"]
    class LabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    TYPE_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    TIME_FIELD_NUMBER: _ClassVar[int]
    TTL_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    Type: str
    ID: int
    Time: _timestamp_pb2.Timestamp
    TTL: int
    Labels: _containers.ScalarMap[str, str]
    def __init__(self, Type: _Optional[str] = ..., ID: _Optional[int] = ..., Time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., TTL: _Optional[int] = ..., Labels: _Optional[_Mapping[str, str]] = ...) -> None: ...

class InstanceFilter(_message.Message):
    __slots__ = ["ServerID", "Version", "Services", "ExternalUpgrader", "NoExtUpgrader", "OlderThanVersion", "NewerThanVersion"]
    SERVERID_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    SERVICES_FIELD_NUMBER: _ClassVar[int]
    EXTERNALUPGRADER_FIELD_NUMBER: _ClassVar[int]
    NOEXTUPGRADER_FIELD_NUMBER: _ClassVar[int]
    OLDERTHANVERSION_FIELD_NUMBER: _ClassVar[int]
    NEWERTHANVERSION_FIELD_NUMBER: _ClassVar[int]
    ServerID: str
    Version: str
    Services: _containers.RepeatedScalarFieldContainer[str]
    ExternalUpgrader: str
    NoExtUpgrader: bool
    OlderThanVersion: str
    NewerThanVersion: str
    def __init__(self, ServerID: _Optional[str] = ..., Version: _Optional[str] = ..., Services: _Optional[_Iterable[str]] = ..., ExternalUpgrader: _Optional[str] = ..., NoExtUpgrader: bool = ..., OlderThanVersion: _Optional[str] = ..., NewerThanVersion: _Optional[str] = ...) -> None: ...

class ServerV2(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: ServerSpecV2
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[ServerSpecV2, _Mapping]] = ...) -> None: ...

class ServerSpecV2(_message.Message):
    __slots__ = ["Addr", "PublicAddr", "Hostname", "CmdLabels", "Rotation", "UseTunnel", "Version", "PeerAddr", "ProxyIDs", "public_addrs", "CloudMetadata"]
    class CmdLabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: CommandLabelV2
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[CommandLabelV2, _Mapping]] = ...) -> None: ...
    ADDR_FIELD_NUMBER: _ClassVar[int]
    PUBLICADDR_FIELD_NUMBER: _ClassVar[int]
    HOSTNAME_FIELD_NUMBER: _ClassVar[int]
    CMDLABELS_FIELD_NUMBER: _ClassVar[int]
    ROTATION_FIELD_NUMBER: _ClassVar[int]
    USETUNNEL_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    PEERADDR_FIELD_NUMBER: _ClassVar[int]
    PROXYIDS_FIELD_NUMBER: _ClassVar[int]
    PUBLIC_ADDRS_FIELD_NUMBER: _ClassVar[int]
    CLOUDMETADATA_FIELD_NUMBER: _ClassVar[int]
    Addr: str
    PublicAddr: str
    Hostname: str
    CmdLabels: _containers.MessageMap[str, CommandLabelV2]
    Rotation: Rotation
    UseTunnel: bool
    Version: str
    PeerAddr: str
    ProxyIDs: _containers.RepeatedScalarFieldContainer[str]
    public_addrs: _containers.RepeatedScalarFieldContainer[str]
    CloudMetadata: CloudMetadata
    def __init__(self, Addr: _Optional[str] = ..., PublicAddr: _Optional[str] = ..., Hostname: _Optional[str] = ..., CmdLabels: _Optional[_Mapping[str, CommandLabelV2]] = ..., Rotation: _Optional[_Union[Rotation, _Mapping]] = ..., UseTunnel: bool = ..., Version: _Optional[str] = ..., PeerAddr: _Optional[str] = ..., ProxyIDs: _Optional[_Iterable[str]] = ..., public_addrs: _Optional[_Iterable[str]] = ..., CloudMetadata: _Optional[_Union[CloudMetadata, _Mapping]] = ...) -> None: ...

class AWSInfo(_message.Message):
    __slots__ = ["AccountID", "InstanceID", "Region", "VPCID", "Integration", "SubnetID"]
    ACCOUNTID_FIELD_NUMBER: _ClassVar[int]
    INSTANCEID_FIELD_NUMBER: _ClassVar[int]
    REGION_FIELD_NUMBER: _ClassVar[int]
    VPCID_FIELD_NUMBER: _ClassVar[int]
    INTEGRATION_FIELD_NUMBER: _ClassVar[int]
    SUBNETID_FIELD_NUMBER: _ClassVar[int]
    AccountID: str
    InstanceID: str
    Region: str
    VPCID: str
    Integration: str
    SubnetID: str
    def __init__(self, AccountID: _Optional[str] = ..., InstanceID: _Optional[str] = ..., Region: _Optional[str] = ..., VPCID: _Optional[str] = ..., Integration: _Optional[str] = ..., SubnetID: _Optional[str] = ...) -> None: ...

class CloudMetadata(_message.Message):
    __slots__ = ["AWS"]
    AWS_FIELD_NUMBER: _ClassVar[int]
    AWS: AWSInfo
    def __init__(self, AWS: _Optional[_Union[AWSInfo, _Mapping]] = ...) -> None: ...

class AppServerV3(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: AppServerSpecV3
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[AppServerSpecV3, _Mapping]] = ...) -> None: ...

class AppServerSpecV3(_message.Message):
    __slots__ = ["Version", "Hostname", "HostID", "Rotation", "App", "ProxyIDs"]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    HOSTNAME_FIELD_NUMBER: _ClassVar[int]
    HOSTID_FIELD_NUMBER: _ClassVar[int]
    ROTATION_FIELD_NUMBER: _ClassVar[int]
    APP_FIELD_NUMBER: _ClassVar[int]
    PROXYIDS_FIELD_NUMBER: _ClassVar[int]
    Version: str
    Hostname: str
    HostID: str
    Rotation: Rotation
    App: AppV3
    ProxyIDs: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Version: _Optional[str] = ..., Hostname: _Optional[str] = ..., HostID: _Optional[str] = ..., Rotation: _Optional[_Union[Rotation, _Mapping]] = ..., App: _Optional[_Union[AppV3, _Mapping]] = ..., ProxyIDs: _Optional[_Iterable[str]] = ...) -> None: ...

class AppV3List(_message.Message):
    __slots__ = ["Apps"]
    APPS_FIELD_NUMBER: _ClassVar[int]
    Apps: _containers.RepeatedCompositeFieldContainer[AppV3]
    def __init__(self, Apps: _Optional[_Iterable[_Union[AppV3, _Mapping]]] = ...) -> None: ...

class AppV3(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: AppSpecV3
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[AppSpecV3, _Mapping]] = ...) -> None: ...

class AppSpecV3(_message.Message):
    __slots__ = ["URI", "PublicAddr", "DynamicLabels", "InsecureSkipVerify", "Rewrite", "AWS", "Cloud", "UserGroups"]
    class DynamicLabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: CommandLabelV2
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[CommandLabelV2, _Mapping]] = ...) -> None: ...
    URI_FIELD_NUMBER: _ClassVar[int]
    PUBLICADDR_FIELD_NUMBER: _ClassVar[int]
    DYNAMICLABELS_FIELD_NUMBER: _ClassVar[int]
    INSECURESKIPVERIFY_FIELD_NUMBER: _ClassVar[int]
    REWRITE_FIELD_NUMBER: _ClassVar[int]
    AWS_FIELD_NUMBER: _ClassVar[int]
    CLOUD_FIELD_NUMBER: _ClassVar[int]
    USERGROUPS_FIELD_NUMBER: _ClassVar[int]
    URI: str
    PublicAddr: str
    DynamicLabels: _containers.MessageMap[str, CommandLabelV2]
    InsecureSkipVerify: bool
    Rewrite: Rewrite
    AWS: AppAWS
    Cloud: str
    UserGroups: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, URI: _Optional[str] = ..., PublicAddr: _Optional[str] = ..., DynamicLabels: _Optional[_Mapping[str, CommandLabelV2]] = ..., InsecureSkipVerify: bool = ..., Rewrite: _Optional[_Union[Rewrite, _Mapping]] = ..., AWS: _Optional[_Union[AppAWS, _Mapping]] = ..., Cloud: _Optional[str] = ..., UserGroups: _Optional[_Iterable[str]] = ...) -> None: ...

class AppServerOrSAMLIdPServiceProviderV1(_message.Message):
    __slots__ = ["Kind", "AppServer", "SAMLIdPServiceProvider"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    APPSERVER_FIELD_NUMBER: _ClassVar[int]
    SAMLIDPSERVICEPROVIDER_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    AppServer: AppServerV3
    SAMLIdPServiceProvider: SAMLIdPServiceProviderV1
    def __init__(self, Kind: _Optional[str] = ..., AppServer: _Optional[_Union[AppServerV3, _Mapping]] = ..., SAMLIdPServiceProvider: _Optional[_Union[SAMLIdPServiceProviderV1, _Mapping]] = ...) -> None: ...

class Rewrite(_message.Message):
    __slots__ = ["Redirect", "Headers", "JWTClaims"]
    REDIRECT_FIELD_NUMBER: _ClassVar[int]
    HEADERS_FIELD_NUMBER: _ClassVar[int]
    JWTCLAIMS_FIELD_NUMBER: _ClassVar[int]
    Redirect: _containers.RepeatedScalarFieldContainer[str]
    Headers: _containers.RepeatedCompositeFieldContainer[Header]
    JWTClaims: str
    def __init__(self, Redirect: _Optional[_Iterable[str]] = ..., Headers: _Optional[_Iterable[_Union[Header, _Mapping]]] = ..., JWTClaims: _Optional[str] = ...) -> None: ...

class Header(_message.Message):
    __slots__ = ["Name", "Value"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    Name: str
    Value: str
    def __init__(self, Name: _Optional[str] = ..., Value: _Optional[str] = ...) -> None: ...

class CommandLabelV2(_message.Message):
    __slots__ = ["Period", "Command", "Result"]
    PERIOD_FIELD_NUMBER: _ClassVar[int]
    COMMAND_FIELD_NUMBER: _ClassVar[int]
    RESULT_FIELD_NUMBER: _ClassVar[int]
    Period: int
    Command: _containers.RepeatedScalarFieldContainer[str]
    Result: str
    def __init__(self, Period: _Optional[int] = ..., Command: _Optional[_Iterable[str]] = ..., Result: _Optional[str] = ...) -> None: ...

class AppAWS(_message.Message):
    __slots__ = ["ExternalID"]
    EXTERNALID_FIELD_NUMBER: _ClassVar[int]
    ExternalID: str
    def __init__(self, ExternalID: _Optional[str] = ...) -> None: ...

class SSHKeyPair(_message.Message):
    __slots__ = ["PublicKey", "PrivateKey", "PrivateKeyType"]
    PUBLICKEY_FIELD_NUMBER: _ClassVar[int]
    PRIVATEKEY_FIELD_NUMBER: _ClassVar[int]
    PRIVATEKEYTYPE_FIELD_NUMBER: _ClassVar[int]
    PublicKey: bytes
    PrivateKey: bytes
    PrivateKeyType: PrivateKeyType
    def __init__(self, PublicKey: _Optional[bytes] = ..., PrivateKey: _Optional[bytes] = ..., PrivateKeyType: _Optional[_Union[PrivateKeyType, str]] = ...) -> None: ...

class TLSKeyPair(_message.Message):
    __slots__ = ["Cert", "Key", "KeyType"]
    CERT_FIELD_NUMBER: _ClassVar[int]
    KEY_FIELD_NUMBER: _ClassVar[int]
    KEYTYPE_FIELD_NUMBER: _ClassVar[int]
    Cert: bytes
    Key: bytes
    KeyType: PrivateKeyType
    def __init__(self, Cert: _Optional[bytes] = ..., Key: _Optional[bytes] = ..., KeyType: _Optional[_Union[PrivateKeyType, str]] = ...) -> None: ...

class JWTKeyPair(_message.Message):
    __slots__ = ["PublicKey", "PrivateKey", "PrivateKeyType"]
    PUBLICKEY_FIELD_NUMBER: _ClassVar[int]
    PRIVATEKEY_FIELD_NUMBER: _ClassVar[int]
    PRIVATEKEYTYPE_FIELD_NUMBER: _ClassVar[int]
    PublicKey: bytes
    PrivateKey: bytes
    PrivateKeyType: PrivateKeyType
    def __init__(self, PublicKey: _Optional[bytes] = ..., PrivateKey: _Optional[bytes] = ..., PrivateKeyType: _Optional[_Union[PrivateKeyType, str]] = ...) -> None: ...

class CertAuthorityV2(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: CertAuthoritySpecV2
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[CertAuthoritySpecV2, _Mapping]] = ...) -> None: ...

class CertAuthoritySpecV2(_message.Message):
    __slots__ = ["Type", "ClusterName", "Roles", "RoleMap", "Rotation", "SigningAlg", "ActiveKeys", "AdditionalTrustedKeys"]
    class SigningAlgType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = []
        UNKNOWN: _ClassVar[CertAuthoritySpecV2.SigningAlgType]
        RSA_SHA1: _ClassVar[CertAuthoritySpecV2.SigningAlgType]
        RSA_SHA2_256: _ClassVar[CertAuthoritySpecV2.SigningAlgType]
        RSA_SHA2_512: _ClassVar[CertAuthoritySpecV2.SigningAlgType]
    UNKNOWN: CertAuthoritySpecV2.SigningAlgType
    RSA_SHA1: CertAuthoritySpecV2.SigningAlgType
    RSA_SHA2_256: CertAuthoritySpecV2.SigningAlgType
    RSA_SHA2_512: CertAuthoritySpecV2.SigningAlgType
    TYPE_FIELD_NUMBER: _ClassVar[int]
    CLUSTERNAME_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    ROLEMAP_FIELD_NUMBER: _ClassVar[int]
    ROTATION_FIELD_NUMBER: _ClassVar[int]
    SIGNINGALG_FIELD_NUMBER: _ClassVar[int]
    ACTIVEKEYS_FIELD_NUMBER: _ClassVar[int]
    ADDITIONALTRUSTEDKEYS_FIELD_NUMBER: _ClassVar[int]
    Type: str
    ClusterName: str
    Roles: _containers.RepeatedScalarFieldContainer[str]
    RoleMap: _containers.RepeatedCompositeFieldContainer[RoleMapping]
    Rotation: Rotation
    SigningAlg: CertAuthoritySpecV2.SigningAlgType
    ActiveKeys: CAKeySet
    AdditionalTrustedKeys: CAKeySet
    def __init__(self, Type: _Optional[str] = ..., ClusterName: _Optional[str] = ..., Roles: _Optional[_Iterable[str]] = ..., RoleMap: _Optional[_Iterable[_Union[RoleMapping, _Mapping]]] = ..., Rotation: _Optional[_Union[Rotation, _Mapping]] = ..., SigningAlg: _Optional[_Union[CertAuthoritySpecV2.SigningAlgType, str]] = ..., ActiveKeys: _Optional[_Union[CAKeySet, _Mapping]] = ..., AdditionalTrustedKeys: _Optional[_Union[CAKeySet, _Mapping]] = ...) -> None: ...

class CAKeySet(_message.Message):
    __slots__ = ["SSH", "TLS", "JWT"]
    SSH_FIELD_NUMBER: _ClassVar[int]
    TLS_FIELD_NUMBER: _ClassVar[int]
    JWT_FIELD_NUMBER: _ClassVar[int]
    SSH: _containers.RepeatedCompositeFieldContainer[SSHKeyPair]
    TLS: _containers.RepeatedCompositeFieldContainer[TLSKeyPair]
    JWT: _containers.RepeatedCompositeFieldContainer[JWTKeyPair]
    def __init__(self, SSH: _Optional[_Iterable[_Union[SSHKeyPair, _Mapping]]] = ..., TLS: _Optional[_Iterable[_Union[TLSKeyPair, _Mapping]]] = ..., JWT: _Optional[_Iterable[_Union[JWTKeyPair, _Mapping]]] = ...) -> None: ...

class RoleMapping(_message.Message):
    __slots__ = ["Remote", "Local"]
    REMOTE_FIELD_NUMBER: _ClassVar[int]
    LOCAL_FIELD_NUMBER: _ClassVar[int]
    Remote: str
    Local: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Remote: _Optional[str] = ..., Local: _Optional[_Iterable[str]] = ...) -> None: ...

class ProvisionTokenV1(_message.Message):
    __slots__ = ["Roles", "Expires", "Token"]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    Roles: _containers.RepeatedScalarFieldContainer[str]
    Expires: _timestamp_pb2.Timestamp
    Token: str
    def __init__(self, Roles: _Optional[_Iterable[str]] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Token: _Optional[str] = ...) -> None: ...

class ProvisionTokenV2(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: ProvisionTokenSpecV2
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[ProvisionTokenSpecV2, _Mapping]] = ...) -> None: ...

class ProvisionTokenV2List(_message.Message):
    __slots__ = ["ProvisionTokens"]
    PROVISIONTOKENS_FIELD_NUMBER: _ClassVar[int]
    ProvisionTokens: _containers.RepeatedCompositeFieldContainer[ProvisionTokenV2]
    def __init__(self, ProvisionTokens: _Optional[_Iterable[_Union[ProvisionTokenV2, _Mapping]]] = ...) -> None: ...

class TokenRule(_message.Message):
    __slots__ = ["AWSAccount", "AWSRegions", "AWSRole", "AWSARN"]
    AWSACCOUNT_FIELD_NUMBER: _ClassVar[int]
    AWSREGIONS_FIELD_NUMBER: _ClassVar[int]
    AWSROLE_FIELD_NUMBER: _ClassVar[int]
    AWSARN_FIELD_NUMBER: _ClassVar[int]
    AWSAccount: str
    AWSRegions: _containers.RepeatedScalarFieldContainer[str]
    AWSRole: str
    AWSARN: str
    def __init__(self, AWSAccount: _Optional[str] = ..., AWSRegions: _Optional[_Iterable[str]] = ..., AWSRole: _Optional[str] = ..., AWSARN: _Optional[str] = ...) -> None: ...

class ProvisionTokenSpecV2(_message.Message):
    __slots__ = ["Roles", "Allow", "AWSIIDTTL", "JoinMethod", "BotName", "SuggestedLabels", "GitHub", "CircleCI", "SuggestedAgentMatcherLabels", "Kubernetes", "Azure", "GitLab", "GCP"]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    ALLOW_FIELD_NUMBER: _ClassVar[int]
    AWSIIDTTL_FIELD_NUMBER: _ClassVar[int]
    JOINMETHOD_FIELD_NUMBER: _ClassVar[int]
    BOTNAME_FIELD_NUMBER: _ClassVar[int]
    SUGGESTEDLABELS_FIELD_NUMBER: _ClassVar[int]
    GITHUB_FIELD_NUMBER: _ClassVar[int]
    CIRCLECI_FIELD_NUMBER: _ClassVar[int]
    SUGGESTEDAGENTMATCHERLABELS_FIELD_NUMBER: _ClassVar[int]
    KUBERNETES_FIELD_NUMBER: _ClassVar[int]
    AZURE_FIELD_NUMBER: _ClassVar[int]
    GITLAB_FIELD_NUMBER: _ClassVar[int]
    GCP_FIELD_NUMBER: _ClassVar[int]
    Roles: _containers.RepeatedScalarFieldContainer[str]
    Allow: _containers.RepeatedCompositeFieldContainer[TokenRule]
    AWSIIDTTL: int
    JoinMethod: str
    BotName: str
    SuggestedLabels: _wrappers_pb2.LabelValues
    GitHub: ProvisionTokenSpecV2GitHub
    CircleCI: ProvisionTokenSpecV2CircleCI
    SuggestedAgentMatcherLabels: _wrappers_pb2.LabelValues
    Kubernetes: ProvisionTokenSpecV2Kubernetes
    Azure: ProvisionTokenSpecV2Azure
    GitLab: ProvisionTokenSpecV2GitLab
    GCP: ProvisionTokenSpecV2GCP
    def __init__(self, Roles: _Optional[_Iterable[str]] = ..., Allow: _Optional[_Iterable[_Union[TokenRule, _Mapping]]] = ..., AWSIIDTTL: _Optional[int] = ..., JoinMethod: _Optional[str] = ..., BotName: _Optional[str] = ..., SuggestedLabels: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., GitHub: _Optional[_Union[ProvisionTokenSpecV2GitHub, _Mapping]] = ..., CircleCI: _Optional[_Union[ProvisionTokenSpecV2CircleCI, _Mapping]] = ..., SuggestedAgentMatcherLabels: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., Kubernetes: _Optional[_Union[ProvisionTokenSpecV2Kubernetes, _Mapping]] = ..., Azure: _Optional[_Union[ProvisionTokenSpecV2Azure, _Mapping]] = ..., GitLab: _Optional[_Union[ProvisionTokenSpecV2GitLab, _Mapping]] = ..., GCP: _Optional[_Union[ProvisionTokenSpecV2GCP, _Mapping]] = ...) -> None: ...

class ProvisionTokenSpecV2GitHub(_message.Message):
    __slots__ = ["Allow", "EnterpriseServerHost"]
    class Rule(_message.Message):
        __slots__ = ["Sub", "Repository", "RepositoryOwner", "Workflow", "Environment", "Actor", "Ref", "RefType"]
        SUB_FIELD_NUMBER: _ClassVar[int]
        REPOSITORY_FIELD_NUMBER: _ClassVar[int]
        REPOSITORYOWNER_FIELD_NUMBER: _ClassVar[int]
        WORKFLOW_FIELD_NUMBER: _ClassVar[int]
        ENVIRONMENT_FIELD_NUMBER: _ClassVar[int]
        ACTOR_FIELD_NUMBER: _ClassVar[int]
        REF_FIELD_NUMBER: _ClassVar[int]
        REFTYPE_FIELD_NUMBER: _ClassVar[int]
        Sub: str
        Repository: str
        RepositoryOwner: str
        Workflow: str
        Environment: str
        Actor: str
        Ref: str
        RefType: str
        def __init__(self, Sub: _Optional[str] = ..., Repository: _Optional[str] = ..., RepositoryOwner: _Optional[str] = ..., Workflow: _Optional[str] = ..., Environment: _Optional[str] = ..., Actor: _Optional[str] = ..., Ref: _Optional[str] = ..., RefType: _Optional[str] = ...) -> None: ...
    ALLOW_FIELD_NUMBER: _ClassVar[int]
    ENTERPRISESERVERHOST_FIELD_NUMBER: _ClassVar[int]
    Allow: _containers.RepeatedCompositeFieldContainer[ProvisionTokenSpecV2GitHub.Rule]
    EnterpriseServerHost: str
    def __init__(self, Allow: _Optional[_Iterable[_Union[ProvisionTokenSpecV2GitHub.Rule, _Mapping]]] = ..., EnterpriseServerHost: _Optional[str] = ...) -> None: ...

class ProvisionTokenSpecV2GitLab(_message.Message):
    __slots__ = ["Allow", "Domain"]
    class Rule(_message.Message):
        __slots__ = ["Sub", "Ref", "RefType", "NamespacePath", "ProjectPath", "PipelineSource", "Environment"]
        SUB_FIELD_NUMBER: _ClassVar[int]
        REF_FIELD_NUMBER: _ClassVar[int]
        REFTYPE_FIELD_NUMBER: _ClassVar[int]
        NAMESPACEPATH_FIELD_NUMBER: _ClassVar[int]
        PROJECTPATH_FIELD_NUMBER: _ClassVar[int]
        PIPELINESOURCE_FIELD_NUMBER: _ClassVar[int]
        ENVIRONMENT_FIELD_NUMBER: _ClassVar[int]
        Sub: str
        Ref: str
        RefType: str
        NamespacePath: str
        ProjectPath: str
        PipelineSource: str
        Environment: str
        def __init__(self, Sub: _Optional[str] = ..., Ref: _Optional[str] = ..., RefType: _Optional[str] = ..., NamespacePath: _Optional[str] = ..., ProjectPath: _Optional[str] = ..., PipelineSource: _Optional[str] = ..., Environment: _Optional[str] = ...) -> None: ...
    ALLOW_FIELD_NUMBER: _ClassVar[int]
    DOMAIN_FIELD_NUMBER: _ClassVar[int]
    Allow: _containers.RepeatedCompositeFieldContainer[ProvisionTokenSpecV2GitLab.Rule]
    Domain: str
    def __init__(self, Allow: _Optional[_Iterable[_Union[ProvisionTokenSpecV2GitLab.Rule, _Mapping]]] = ..., Domain: _Optional[str] = ...) -> None: ...

class ProvisionTokenSpecV2CircleCI(_message.Message):
    __slots__ = ["Allow", "OrganizationID"]
    class Rule(_message.Message):
        __slots__ = ["ProjectID", "ContextID"]
        PROJECTID_FIELD_NUMBER: _ClassVar[int]
        CONTEXTID_FIELD_NUMBER: _ClassVar[int]
        ProjectID: str
        ContextID: str
        def __init__(self, ProjectID: _Optional[str] = ..., ContextID: _Optional[str] = ...) -> None: ...
    ALLOW_FIELD_NUMBER: _ClassVar[int]
    ORGANIZATIONID_FIELD_NUMBER: _ClassVar[int]
    Allow: _containers.RepeatedCompositeFieldContainer[ProvisionTokenSpecV2CircleCI.Rule]
    OrganizationID: str
    def __init__(self, Allow: _Optional[_Iterable[_Union[ProvisionTokenSpecV2CircleCI.Rule, _Mapping]]] = ..., OrganizationID: _Optional[str] = ...) -> None: ...

class ProvisionTokenSpecV2Kubernetes(_message.Message):
    __slots__ = ["Allow", "Type", "StaticJWKS"]
    class StaticJWKSConfig(_message.Message):
        __slots__ = ["JWKS"]
        JWKS_FIELD_NUMBER: _ClassVar[int]
        JWKS: str
        def __init__(self, JWKS: _Optional[str] = ...) -> None: ...
    class Rule(_message.Message):
        __slots__ = ["ServiceAccount"]
        SERVICEACCOUNT_FIELD_NUMBER: _ClassVar[int]
        ServiceAccount: str
        def __init__(self, ServiceAccount: _Optional[str] = ...) -> None: ...
    ALLOW_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    STATICJWKS_FIELD_NUMBER: _ClassVar[int]
    Allow: _containers.RepeatedCompositeFieldContainer[ProvisionTokenSpecV2Kubernetes.Rule]
    Type: str
    StaticJWKS: ProvisionTokenSpecV2Kubernetes.StaticJWKSConfig
    def __init__(self, Allow: _Optional[_Iterable[_Union[ProvisionTokenSpecV2Kubernetes.Rule, _Mapping]]] = ..., Type: _Optional[str] = ..., StaticJWKS: _Optional[_Union[ProvisionTokenSpecV2Kubernetes.StaticJWKSConfig, _Mapping]] = ...) -> None: ...

class ProvisionTokenSpecV2Azure(_message.Message):
    __slots__ = ["Allow"]
    class Rule(_message.Message):
        __slots__ = ["Subscription", "ResourceGroups"]
        SUBSCRIPTION_FIELD_NUMBER: _ClassVar[int]
        RESOURCEGROUPS_FIELD_NUMBER: _ClassVar[int]
        Subscription: str
        ResourceGroups: _containers.RepeatedScalarFieldContainer[str]
        def __init__(self, Subscription: _Optional[str] = ..., ResourceGroups: _Optional[_Iterable[str]] = ...) -> None: ...
    ALLOW_FIELD_NUMBER: _ClassVar[int]
    Allow: _containers.RepeatedCompositeFieldContainer[ProvisionTokenSpecV2Azure.Rule]
    def __init__(self, Allow: _Optional[_Iterable[_Union[ProvisionTokenSpecV2Azure.Rule, _Mapping]]] = ...) -> None: ...

class ProvisionTokenSpecV2GCP(_message.Message):
    __slots__ = ["Allow"]
    class Rule(_message.Message):
        __slots__ = ["ProjectIDs", "Locations", "ServiceAccounts"]
        PROJECTIDS_FIELD_NUMBER: _ClassVar[int]
        LOCATIONS_FIELD_NUMBER: _ClassVar[int]
        SERVICEACCOUNTS_FIELD_NUMBER: _ClassVar[int]
        ProjectIDs: _containers.RepeatedScalarFieldContainer[str]
        Locations: _containers.RepeatedScalarFieldContainer[str]
        ServiceAccounts: _containers.RepeatedScalarFieldContainer[str]
        def __init__(self, ProjectIDs: _Optional[_Iterable[str]] = ..., Locations: _Optional[_Iterable[str]] = ..., ServiceAccounts: _Optional[_Iterable[str]] = ...) -> None: ...
    ALLOW_FIELD_NUMBER: _ClassVar[int]
    Allow: _containers.RepeatedCompositeFieldContainer[ProvisionTokenSpecV2GCP.Rule]
    def __init__(self, Allow: _Optional[_Iterable[_Union[ProvisionTokenSpecV2GCP.Rule, _Mapping]]] = ...) -> None: ...

class StaticTokensV2(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: StaticTokensSpecV2
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[StaticTokensSpecV2, _Mapping]] = ...) -> None: ...

class StaticTokensSpecV2(_message.Message):
    __slots__ = ["StaticTokens"]
    STATICTOKENS_FIELD_NUMBER: _ClassVar[int]
    StaticTokens: _containers.RepeatedCompositeFieldContainer[ProvisionTokenV1]
    def __init__(self, StaticTokens: _Optional[_Iterable[_Union[ProvisionTokenV1, _Mapping]]] = ...) -> None: ...

class ClusterNameV2(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: ClusterNameSpecV2
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[ClusterNameSpecV2, _Mapping]] = ...) -> None: ...

class ClusterNameSpecV2(_message.Message):
    __slots__ = ["ClusterName", "ClusterID"]
    CLUSTERNAME_FIELD_NUMBER: _ClassVar[int]
    CLUSTERID_FIELD_NUMBER: _ClassVar[int]
    ClusterName: str
    ClusterID: str
    def __init__(self, ClusterName: _Optional[str] = ..., ClusterID: _Optional[str] = ...) -> None: ...

class ClusterAuditConfigV2(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: ClusterAuditConfigSpecV2
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[ClusterAuditConfigSpecV2, _Mapping]] = ...) -> None: ...

class ClusterAuditConfigSpecV2(_message.Message):
    __slots__ = ["Type", "Region", "AuditSessionsURI", "AuditEventsURI", "EnableContinuousBackups", "EnableAutoScaling", "ReadMaxCapacity", "ReadMinCapacity", "ReadTargetValue", "WriteMaxCapacity", "WriteMinCapacity", "WriteTargetValue", "RetentionPeriod", "UseFIPSEndpoint"]
    class FIPSEndpointState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = []
        FIPS_UNSET: _ClassVar[ClusterAuditConfigSpecV2.FIPSEndpointState]
        FIPS_ENABLED: _ClassVar[ClusterAuditConfigSpecV2.FIPSEndpointState]
        FIPS_DISABLED: _ClassVar[ClusterAuditConfigSpecV2.FIPSEndpointState]
    FIPS_UNSET: ClusterAuditConfigSpecV2.FIPSEndpointState
    FIPS_ENABLED: ClusterAuditConfigSpecV2.FIPSEndpointState
    FIPS_DISABLED: ClusterAuditConfigSpecV2.FIPSEndpointState
    TYPE_FIELD_NUMBER: _ClassVar[int]
    REGION_FIELD_NUMBER: _ClassVar[int]
    AUDITSESSIONSURI_FIELD_NUMBER: _ClassVar[int]
    AUDITEVENTSURI_FIELD_NUMBER: _ClassVar[int]
    ENABLECONTINUOUSBACKUPS_FIELD_NUMBER: _ClassVar[int]
    ENABLEAUTOSCALING_FIELD_NUMBER: _ClassVar[int]
    READMAXCAPACITY_FIELD_NUMBER: _ClassVar[int]
    READMINCAPACITY_FIELD_NUMBER: _ClassVar[int]
    READTARGETVALUE_FIELD_NUMBER: _ClassVar[int]
    WRITEMAXCAPACITY_FIELD_NUMBER: _ClassVar[int]
    WRITEMINCAPACITY_FIELD_NUMBER: _ClassVar[int]
    WRITETARGETVALUE_FIELD_NUMBER: _ClassVar[int]
    RETENTIONPERIOD_FIELD_NUMBER: _ClassVar[int]
    USEFIPSENDPOINT_FIELD_NUMBER: _ClassVar[int]
    Type: str
    Region: str
    AuditSessionsURI: str
    AuditEventsURI: _wrappers_pb2.StringValues
    EnableContinuousBackups: bool
    EnableAutoScaling: bool
    ReadMaxCapacity: int
    ReadMinCapacity: int
    ReadTargetValue: float
    WriteMaxCapacity: int
    WriteMinCapacity: int
    WriteTargetValue: float
    RetentionPeriod: int
    UseFIPSEndpoint: ClusterAuditConfigSpecV2.FIPSEndpointState
    def __init__(self, Type: _Optional[str] = ..., Region: _Optional[str] = ..., AuditSessionsURI: _Optional[str] = ..., AuditEventsURI: _Optional[_Union[_wrappers_pb2.StringValues, _Mapping]] = ..., EnableContinuousBackups: bool = ..., EnableAutoScaling: bool = ..., ReadMaxCapacity: _Optional[int] = ..., ReadMinCapacity: _Optional[int] = ..., ReadTargetValue: _Optional[float] = ..., WriteMaxCapacity: _Optional[int] = ..., WriteMinCapacity: _Optional[int] = ..., WriteTargetValue: _Optional[float] = ..., RetentionPeriod: _Optional[int] = ..., UseFIPSEndpoint: _Optional[_Union[ClusterAuditConfigSpecV2.FIPSEndpointState, str]] = ...) -> None: ...

class ClusterNetworkingConfigV2(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: ClusterNetworkingConfigSpecV2
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[ClusterNetworkingConfigSpecV2, _Mapping]] = ...) -> None: ...

class ClusterNetworkingConfigSpecV2(_message.Message):
    __slots__ = ["ClientIdleTimeout", "KeepAliveInterval", "KeepAliveCountMax", "SessionControlTimeout", "ClientIdleTimeoutMessage", "WebIdleTimeout", "ProxyListenerMode", "RoutingStrategy", "TunnelStrategy", "ProxyPingInterval", "AssistCommandExecutionWorkers", "CaseInsensitiveRouting"]
    CLIENTIDLETIMEOUT_FIELD_NUMBER: _ClassVar[int]
    KEEPALIVEINTERVAL_FIELD_NUMBER: _ClassVar[int]
    KEEPALIVECOUNTMAX_FIELD_NUMBER: _ClassVar[int]
    SESSIONCONTROLTIMEOUT_FIELD_NUMBER: _ClassVar[int]
    CLIENTIDLETIMEOUTMESSAGE_FIELD_NUMBER: _ClassVar[int]
    WEBIDLETIMEOUT_FIELD_NUMBER: _ClassVar[int]
    PROXYLISTENERMODE_FIELD_NUMBER: _ClassVar[int]
    ROUTINGSTRATEGY_FIELD_NUMBER: _ClassVar[int]
    TUNNELSTRATEGY_FIELD_NUMBER: _ClassVar[int]
    PROXYPINGINTERVAL_FIELD_NUMBER: _ClassVar[int]
    ASSISTCOMMANDEXECUTIONWORKERS_FIELD_NUMBER: _ClassVar[int]
    CASEINSENSITIVEROUTING_FIELD_NUMBER: _ClassVar[int]
    ClientIdleTimeout: int
    KeepAliveInterval: int
    KeepAliveCountMax: int
    SessionControlTimeout: int
    ClientIdleTimeoutMessage: str
    WebIdleTimeout: int
    ProxyListenerMode: ProxyListenerMode
    RoutingStrategy: RoutingStrategy
    TunnelStrategy: TunnelStrategyV1
    ProxyPingInterval: int
    AssistCommandExecutionWorkers: int
    CaseInsensitiveRouting: bool
    def __init__(self, ClientIdleTimeout: _Optional[int] = ..., KeepAliveInterval: _Optional[int] = ..., KeepAliveCountMax: _Optional[int] = ..., SessionControlTimeout: _Optional[int] = ..., ClientIdleTimeoutMessage: _Optional[str] = ..., WebIdleTimeout: _Optional[int] = ..., ProxyListenerMode: _Optional[_Union[ProxyListenerMode, str]] = ..., RoutingStrategy: _Optional[_Union[RoutingStrategy, str]] = ..., TunnelStrategy: _Optional[_Union[TunnelStrategyV1, _Mapping]] = ..., ProxyPingInterval: _Optional[int] = ..., AssistCommandExecutionWorkers: _Optional[int] = ..., CaseInsensitiveRouting: bool = ...) -> None: ...

class TunnelStrategyV1(_message.Message):
    __slots__ = ["AgentMesh", "ProxyPeering"]
    AGENTMESH_FIELD_NUMBER: _ClassVar[int]
    PROXYPEERING_FIELD_NUMBER: _ClassVar[int]
    AgentMesh: AgentMeshTunnelStrategy
    ProxyPeering: ProxyPeeringTunnelStrategy
    def __init__(self, AgentMesh: _Optional[_Union[AgentMeshTunnelStrategy, _Mapping]] = ..., ProxyPeering: _Optional[_Union[ProxyPeeringTunnelStrategy, _Mapping]] = ...) -> None: ...

class AgentMeshTunnelStrategy(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class ProxyPeeringTunnelStrategy(_message.Message):
    __slots__ = ["AgentConnectionCount"]
    AGENTCONNECTIONCOUNT_FIELD_NUMBER: _ClassVar[int]
    AgentConnectionCount: int
    def __init__(self, AgentConnectionCount: _Optional[int] = ...) -> None: ...

class SessionRecordingConfigV2(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: SessionRecordingConfigSpecV2
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[SessionRecordingConfigSpecV2, _Mapping]] = ...) -> None: ...

class SessionRecordingConfigSpecV2(_message.Message):
    __slots__ = ["Mode", "ProxyChecksHostKeys"]
    MODE_FIELD_NUMBER: _ClassVar[int]
    PROXYCHECKSHOSTKEYS_FIELD_NUMBER: _ClassVar[int]
    Mode: str
    ProxyChecksHostKeys: BoolValue
    def __init__(self, Mode: _Optional[str] = ..., ProxyChecksHostKeys: _Optional[_Union[BoolValue, _Mapping]] = ...) -> None: ...

class AuthPreferenceV2(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: AuthPreferenceSpecV2
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[AuthPreferenceSpecV2, _Mapping]] = ...) -> None: ...

class AuthPreferenceSpecV2(_message.Message):
    __slots__ = ["Type", "SecondFactor", "ConnectorName", "U2F", "DisconnectExpiredCert", "AllowLocalAuth", "MessageOfTheDay", "LockingMode", "Webauthn", "AllowPasswordless", "RequireMFAType", "DeviceTrust", "IDP", "AllowHeadless", "DefaultSessionTTL", "Okta"]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    SECONDFACTOR_FIELD_NUMBER: _ClassVar[int]
    CONNECTORNAME_FIELD_NUMBER: _ClassVar[int]
    U2F_FIELD_NUMBER: _ClassVar[int]
    DISCONNECTEXPIREDCERT_FIELD_NUMBER: _ClassVar[int]
    ALLOWLOCALAUTH_FIELD_NUMBER: _ClassVar[int]
    MESSAGEOFTHEDAY_FIELD_NUMBER: _ClassVar[int]
    LOCKINGMODE_FIELD_NUMBER: _ClassVar[int]
    WEBAUTHN_FIELD_NUMBER: _ClassVar[int]
    ALLOWPASSWORDLESS_FIELD_NUMBER: _ClassVar[int]
    REQUIREMFATYPE_FIELD_NUMBER: _ClassVar[int]
    DEVICETRUST_FIELD_NUMBER: _ClassVar[int]
    IDP_FIELD_NUMBER: _ClassVar[int]
    ALLOWHEADLESS_FIELD_NUMBER: _ClassVar[int]
    DEFAULTSESSIONTTL_FIELD_NUMBER: _ClassVar[int]
    OKTA_FIELD_NUMBER: _ClassVar[int]
    Type: str
    SecondFactor: str
    ConnectorName: str
    U2F: U2F
    DisconnectExpiredCert: BoolValue
    AllowLocalAuth: BoolValue
    MessageOfTheDay: str
    LockingMode: str
    Webauthn: Webauthn
    AllowPasswordless: BoolValue
    RequireMFAType: RequireMFAType
    DeviceTrust: DeviceTrust
    IDP: IdPOptions
    AllowHeadless: BoolValue
    DefaultSessionTTL: int
    Okta: OktaOptions
    def __init__(self, Type: _Optional[str] = ..., SecondFactor: _Optional[str] = ..., ConnectorName: _Optional[str] = ..., U2F: _Optional[_Union[U2F, _Mapping]] = ..., DisconnectExpiredCert: _Optional[_Union[BoolValue, _Mapping]] = ..., AllowLocalAuth: _Optional[_Union[BoolValue, _Mapping]] = ..., MessageOfTheDay: _Optional[str] = ..., LockingMode: _Optional[str] = ..., Webauthn: _Optional[_Union[Webauthn, _Mapping]] = ..., AllowPasswordless: _Optional[_Union[BoolValue, _Mapping]] = ..., RequireMFAType: _Optional[_Union[RequireMFAType, str]] = ..., DeviceTrust: _Optional[_Union[DeviceTrust, _Mapping]] = ..., IDP: _Optional[_Union[IdPOptions, _Mapping]] = ..., AllowHeadless: _Optional[_Union[BoolValue, _Mapping]] = ..., DefaultSessionTTL: _Optional[int] = ..., Okta: _Optional[_Union[OktaOptions, _Mapping]] = ...) -> None: ...

class U2F(_message.Message):
    __slots__ = ["AppID", "Facets", "DeviceAttestationCAs"]
    APPID_FIELD_NUMBER: _ClassVar[int]
    FACETS_FIELD_NUMBER: _ClassVar[int]
    DEVICEATTESTATIONCAS_FIELD_NUMBER: _ClassVar[int]
    AppID: str
    Facets: _containers.RepeatedScalarFieldContainer[str]
    DeviceAttestationCAs: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, AppID: _Optional[str] = ..., Facets: _Optional[_Iterable[str]] = ..., DeviceAttestationCAs: _Optional[_Iterable[str]] = ...) -> None: ...

class Webauthn(_message.Message):
    __slots__ = ["RPID", "AttestationAllowedCAs", "AttestationDeniedCAs"]
    RPID_FIELD_NUMBER: _ClassVar[int]
    ATTESTATIONALLOWEDCAS_FIELD_NUMBER: _ClassVar[int]
    ATTESTATIONDENIEDCAS_FIELD_NUMBER: _ClassVar[int]
    RPID: str
    AttestationAllowedCAs: _containers.RepeatedScalarFieldContainer[str]
    AttestationDeniedCAs: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, RPID: _Optional[str] = ..., AttestationAllowedCAs: _Optional[_Iterable[str]] = ..., AttestationDeniedCAs: _Optional[_Iterable[str]] = ...) -> None: ...

class DeviceTrust(_message.Message):
    __slots__ = ["Mode", "AutoEnroll", "EKCertAllowedCAs"]
    MODE_FIELD_NUMBER: _ClassVar[int]
    AUTOENROLL_FIELD_NUMBER: _ClassVar[int]
    EKCERTALLOWEDCAS_FIELD_NUMBER: _ClassVar[int]
    Mode: str
    AutoEnroll: bool
    EKCertAllowedCAs: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Mode: _Optional[str] = ..., AutoEnroll: bool = ..., EKCertAllowedCAs: _Optional[_Iterable[str]] = ...) -> None: ...

class Namespace(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: NamespaceSpec
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[NamespaceSpec, _Mapping]] = ...) -> None: ...

class NamespaceSpec(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class UserTokenV3(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: UserTokenSpecV3
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[UserTokenSpecV3, _Mapping]] = ...) -> None: ...

class UserTokenSpecV3(_message.Message):
    __slots__ = ["User", "URL", "Usage", "Created"]
    USER_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    CREATED_FIELD_NUMBER: _ClassVar[int]
    User: str
    URL: str
    Usage: UserTokenUsage
    Created: _timestamp_pb2.Timestamp
    def __init__(self, User: _Optional[str] = ..., URL: _Optional[str] = ..., Usage: _Optional[_Union[UserTokenUsage, str]] = ..., Created: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class UserTokenSecretsV3(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: UserTokenSecretsSpecV3
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[UserTokenSecretsSpecV3, _Mapping]] = ...) -> None: ...

class UserTokenSecretsSpecV3(_message.Message):
    __slots__ = ["OTPKey", "QRCode", "Created"]
    OTPKEY_FIELD_NUMBER: _ClassVar[int]
    QRCODE_FIELD_NUMBER: _ClassVar[int]
    CREATED_FIELD_NUMBER: _ClassVar[int]
    OTPKey: str
    QRCode: str
    Created: _timestamp_pb2.Timestamp
    def __init__(self, OTPKey: _Optional[str] = ..., QRCode: _Optional[str] = ..., Created: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class AccessRequestV3(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: AccessRequestSpecV3
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[AccessRequestSpecV3, _Mapping]] = ...) -> None: ...

class AccessReviewThreshold(_message.Message):
    __slots__ = ["Name", "Filter", "Approve", "Deny"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    FILTER_FIELD_NUMBER: _ClassVar[int]
    APPROVE_FIELD_NUMBER: _ClassVar[int]
    DENY_FIELD_NUMBER: _ClassVar[int]
    Name: str
    Filter: str
    Approve: int
    Deny: int
    def __init__(self, Name: _Optional[str] = ..., Filter: _Optional[str] = ..., Approve: _Optional[int] = ..., Deny: _Optional[int] = ...) -> None: ...

class PromotedAccessList(_message.Message):
    __slots__ = ["Name", "Title"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    Name: str
    Title: str
    def __init__(self, Name: _Optional[str] = ..., Title: _Optional[str] = ...) -> None: ...

class AccessReview(_message.Message):
    __slots__ = ["Author", "Roles", "ProposedState", "Reason", "Created", "Annotations", "ThresholdIndexes", "accessList"]
    AUTHOR_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    PROPOSEDSTATE_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    CREATED_FIELD_NUMBER: _ClassVar[int]
    ANNOTATIONS_FIELD_NUMBER: _ClassVar[int]
    THRESHOLDINDEXES_FIELD_NUMBER: _ClassVar[int]
    ACCESSLIST_FIELD_NUMBER: _ClassVar[int]
    Author: str
    Roles: _containers.RepeatedScalarFieldContainer[str]
    ProposedState: RequestState
    Reason: str
    Created: _timestamp_pb2.Timestamp
    Annotations: _wrappers_pb2.LabelValues
    ThresholdIndexes: _containers.RepeatedScalarFieldContainer[int]
    accessList: PromotedAccessList
    def __init__(self, Author: _Optional[str] = ..., Roles: _Optional[_Iterable[str]] = ..., ProposedState: _Optional[_Union[RequestState, str]] = ..., Reason: _Optional[str] = ..., Created: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Annotations: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., ThresholdIndexes: _Optional[_Iterable[int]] = ..., accessList: _Optional[_Union[PromotedAccessList, _Mapping]] = ...) -> None: ...

class AccessReviewSubmission(_message.Message):
    __slots__ = ["RequestID", "Review"]
    REQUESTID_FIELD_NUMBER: _ClassVar[int]
    REVIEW_FIELD_NUMBER: _ClassVar[int]
    RequestID: str
    Review: AccessReview
    def __init__(self, RequestID: _Optional[str] = ..., Review: _Optional[_Union[AccessReview, _Mapping]] = ...) -> None: ...

class ThresholdIndexSet(_message.Message):
    __slots__ = ["Indexes"]
    INDEXES_FIELD_NUMBER: _ClassVar[int]
    Indexes: _containers.RepeatedScalarFieldContainer[int]
    def __init__(self, Indexes: _Optional[_Iterable[int]] = ...) -> None: ...

class ThresholdIndexSets(_message.Message):
    __slots__ = ["Sets"]
    SETS_FIELD_NUMBER: _ClassVar[int]
    Sets: _containers.RepeatedCompositeFieldContainer[ThresholdIndexSet]
    def __init__(self, Sets: _Optional[_Iterable[_Union[ThresholdIndexSet, _Mapping]]] = ...) -> None: ...

class AccessRequestSpecV3(_message.Message):
    __slots__ = ["User", "Roles", "State", "Created", "Expires", "RequestReason", "ResolveReason", "ResolveAnnotations", "SystemAnnotations", "Thresholds", "RoleThresholdMapping", "Reviews", "SuggestedReviewers", "RequestedResourceIDs", "LoginHint", "DryRun", "MaxDuration", "SessionTTL", "accessList"]
    class RoleThresholdMappingEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: ThresholdIndexSets
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[ThresholdIndexSets, _Mapping]] = ...) -> None: ...
    USER_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    CREATED_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    REQUESTREASON_FIELD_NUMBER: _ClassVar[int]
    RESOLVEREASON_FIELD_NUMBER: _ClassVar[int]
    RESOLVEANNOTATIONS_FIELD_NUMBER: _ClassVar[int]
    SYSTEMANNOTATIONS_FIELD_NUMBER: _ClassVar[int]
    THRESHOLDS_FIELD_NUMBER: _ClassVar[int]
    ROLETHRESHOLDMAPPING_FIELD_NUMBER: _ClassVar[int]
    REVIEWS_FIELD_NUMBER: _ClassVar[int]
    SUGGESTEDREVIEWERS_FIELD_NUMBER: _ClassVar[int]
    REQUESTEDRESOURCEIDS_FIELD_NUMBER: _ClassVar[int]
    LOGINHINT_FIELD_NUMBER: _ClassVar[int]
    DRYRUN_FIELD_NUMBER: _ClassVar[int]
    MAXDURATION_FIELD_NUMBER: _ClassVar[int]
    SESSIONTTL_FIELD_NUMBER: _ClassVar[int]
    ACCESSLIST_FIELD_NUMBER: _ClassVar[int]
    User: str
    Roles: _containers.RepeatedScalarFieldContainer[str]
    State: RequestState
    Created: _timestamp_pb2.Timestamp
    Expires: _timestamp_pb2.Timestamp
    RequestReason: str
    ResolveReason: str
    ResolveAnnotations: _wrappers_pb2.LabelValues
    SystemAnnotations: _wrappers_pb2.LabelValues
    Thresholds: _containers.RepeatedCompositeFieldContainer[AccessReviewThreshold]
    RoleThresholdMapping: _containers.MessageMap[str, ThresholdIndexSets]
    Reviews: _containers.RepeatedCompositeFieldContainer[AccessReview]
    SuggestedReviewers: _containers.RepeatedScalarFieldContainer[str]
    RequestedResourceIDs: _containers.RepeatedCompositeFieldContainer[ResourceID]
    LoginHint: str
    DryRun: bool
    MaxDuration: _timestamp_pb2.Timestamp
    SessionTTL: _timestamp_pb2.Timestamp
    accessList: PromotedAccessList
    def __init__(self, User: _Optional[str] = ..., Roles: _Optional[_Iterable[str]] = ..., State: _Optional[_Union[RequestState, str]] = ..., Created: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., RequestReason: _Optional[str] = ..., ResolveReason: _Optional[str] = ..., ResolveAnnotations: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., SystemAnnotations: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., Thresholds: _Optional[_Iterable[_Union[AccessReviewThreshold, _Mapping]]] = ..., RoleThresholdMapping: _Optional[_Mapping[str, ThresholdIndexSets]] = ..., Reviews: _Optional[_Iterable[_Union[AccessReview, _Mapping]]] = ..., SuggestedReviewers: _Optional[_Iterable[str]] = ..., RequestedResourceIDs: _Optional[_Iterable[_Union[ResourceID, _Mapping]]] = ..., LoginHint: _Optional[str] = ..., DryRun: bool = ..., MaxDuration: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., SessionTTL: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., accessList: _Optional[_Union[PromotedAccessList, _Mapping]] = ...) -> None: ...

class AccessRequestFilter(_message.Message):
    __slots__ = ["ID", "User", "State"]
    ID_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    ID: str
    User: str
    State: RequestState
    def __init__(self, ID: _Optional[str] = ..., User: _Optional[str] = ..., State: _Optional[_Union[RequestState, str]] = ...) -> None: ...

class AccessCapabilities(_message.Message):
    __slots__ = ["RequestableRoles", "SuggestedReviewers", "ApplicableRolesForResources", "RequestPrompt", "RequireReason", "AutoRequest"]
    REQUESTABLEROLES_FIELD_NUMBER: _ClassVar[int]
    SUGGESTEDREVIEWERS_FIELD_NUMBER: _ClassVar[int]
    APPLICABLEROLESFORRESOURCES_FIELD_NUMBER: _ClassVar[int]
    REQUESTPROMPT_FIELD_NUMBER: _ClassVar[int]
    REQUIREREASON_FIELD_NUMBER: _ClassVar[int]
    AUTOREQUEST_FIELD_NUMBER: _ClassVar[int]
    RequestableRoles: _containers.RepeatedScalarFieldContainer[str]
    SuggestedReviewers: _containers.RepeatedScalarFieldContainer[str]
    ApplicableRolesForResources: _containers.RepeatedScalarFieldContainer[str]
    RequestPrompt: str
    RequireReason: bool
    AutoRequest: bool
    def __init__(self, RequestableRoles: _Optional[_Iterable[str]] = ..., SuggestedReviewers: _Optional[_Iterable[str]] = ..., ApplicableRolesForResources: _Optional[_Iterable[str]] = ..., RequestPrompt: _Optional[str] = ..., RequireReason: bool = ..., AutoRequest: bool = ...) -> None: ...

class AccessCapabilitiesRequest(_message.Message):
    __slots__ = ["User", "RequestableRoles", "SuggestedReviewers", "ResourceIDs"]
    USER_FIELD_NUMBER: _ClassVar[int]
    REQUESTABLEROLES_FIELD_NUMBER: _ClassVar[int]
    SUGGESTEDREVIEWERS_FIELD_NUMBER: _ClassVar[int]
    RESOURCEIDS_FIELD_NUMBER: _ClassVar[int]
    User: str
    RequestableRoles: bool
    SuggestedReviewers: bool
    ResourceIDs: _containers.RepeatedCompositeFieldContainer[ResourceID]
    def __init__(self, User: _Optional[str] = ..., RequestableRoles: bool = ..., SuggestedReviewers: bool = ..., ResourceIDs: _Optional[_Iterable[_Union[ResourceID, _Mapping]]] = ...) -> None: ...

class ResourceID(_message.Message):
    __slots__ = ["ClusterName", "Kind", "Name", "SubResourceName"]
    CLUSTERNAME_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    SUBRESOURCENAME_FIELD_NUMBER: _ClassVar[int]
    ClusterName: str
    Kind: str
    Name: str
    SubResourceName: str
    def __init__(self, ClusterName: _Optional[str] = ..., Kind: _Optional[str] = ..., Name: _Optional[str] = ..., SubResourceName: _Optional[str] = ...) -> None: ...

class PluginDataV3(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: PluginDataSpecV3
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[PluginDataSpecV3, _Mapping]] = ...) -> None: ...

class PluginDataEntry(_message.Message):
    __slots__ = ["Data"]
    class DataEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    DATA_FIELD_NUMBER: _ClassVar[int]
    Data: _containers.ScalarMap[str, str]
    def __init__(self, Data: _Optional[_Mapping[str, str]] = ...) -> None: ...

class PluginDataSpecV3(_message.Message):
    __slots__ = ["Entries"]
    class EntriesEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: PluginDataEntry
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[PluginDataEntry, _Mapping]] = ...) -> None: ...
    ENTRIES_FIELD_NUMBER: _ClassVar[int]
    Entries: _containers.MessageMap[str, PluginDataEntry]
    def __init__(self, Entries: _Optional[_Mapping[str, PluginDataEntry]] = ...) -> None: ...

class PluginDataFilter(_message.Message):
    __slots__ = ["Kind", "Resource", "Plugin"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    PLUGIN_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    Resource: str
    Plugin: str
    def __init__(self, Kind: _Optional[str] = ..., Resource: _Optional[str] = ..., Plugin: _Optional[str] = ...) -> None: ...

class PluginDataUpdateParams(_message.Message):
    __slots__ = ["Kind", "Resource", "Plugin", "Set", "Expect"]
    class SetEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    class ExpectEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    KIND_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    PLUGIN_FIELD_NUMBER: _ClassVar[int]
    SET_FIELD_NUMBER: _ClassVar[int]
    EXPECT_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    Resource: str
    Plugin: str
    Set: _containers.ScalarMap[str, str]
    Expect: _containers.ScalarMap[str, str]
    def __init__(self, Kind: _Optional[str] = ..., Resource: _Optional[str] = ..., Plugin: _Optional[str] = ..., Set: _Optional[_Mapping[str, str]] = ..., Expect: _Optional[_Mapping[str, str]] = ...) -> None: ...

class RoleV6(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: RoleSpecV6
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[RoleSpecV6, _Mapping]] = ...) -> None: ...

class RoleSpecV6(_message.Message):
    __slots__ = ["Options", "Allow", "Deny"]
    OPTIONS_FIELD_NUMBER: _ClassVar[int]
    ALLOW_FIELD_NUMBER: _ClassVar[int]
    DENY_FIELD_NUMBER: _ClassVar[int]
    Options: RoleOptions
    Allow: RoleConditions
    Deny: RoleConditions
    def __init__(self, Options: _Optional[_Union[RoleOptions, _Mapping]] = ..., Allow: _Optional[_Union[RoleConditions, _Mapping]] = ..., Deny: _Optional[_Union[RoleConditions, _Mapping]] = ...) -> None: ...

class RoleOptions(_message.Message):
    __slots__ = ["ForwardAgent", "MaxSessionTTL", "PortForwarding", "CertificateFormat", "ClientIdleTimeout", "DisconnectExpiredCert", "BPF", "PermitX11Forwarding", "MaxConnections", "MaxSessions", "RequestAccess", "RequestPrompt", "Lock", "RecordSession", "DesktopClipboard", "CertExtensions", "MaxKubernetesConnections", "DesktopDirectorySharing", "CreateHostUser", "PinSourceIP", "SSHFileCopy", "RequireMFAType", "DeviceTrustMode", "IDP", "CreateDesktopUser", "CreateDatabaseUser", "CreateHostUserMode"]
    FORWARDAGENT_FIELD_NUMBER: _ClassVar[int]
    MAXSESSIONTTL_FIELD_NUMBER: _ClassVar[int]
    PORTFORWARDING_FIELD_NUMBER: _ClassVar[int]
    CERTIFICATEFORMAT_FIELD_NUMBER: _ClassVar[int]
    CLIENTIDLETIMEOUT_FIELD_NUMBER: _ClassVar[int]
    DISCONNECTEXPIREDCERT_FIELD_NUMBER: _ClassVar[int]
    BPF_FIELD_NUMBER: _ClassVar[int]
    PERMITX11FORWARDING_FIELD_NUMBER: _ClassVar[int]
    MAXCONNECTIONS_FIELD_NUMBER: _ClassVar[int]
    MAXSESSIONS_FIELD_NUMBER: _ClassVar[int]
    REQUESTACCESS_FIELD_NUMBER: _ClassVar[int]
    REQUESTPROMPT_FIELD_NUMBER: _ClassVar[int]
    LOCK_FIELD_NUMBER: _ClassVar[int]
    RECORDSESSION_FIELD_NUMBER: _ClassVar[int]
    DESKTOPCLIPBOARD_FIELD_NUMBER: _ClassVar[int]
    CERTEXTENSIONS_FIELD_NUMBER: _ClassVar[int]
    MAXKUBERNETESCONNECTIONS_FIELD_NUMBER: _ClassVar[int]
    DESKTOPDIRECTORYSHARING_FIELD_NUMBER: _ClassVar[int]
    CREATEHOSTUSER_FIELD_NUMBER: _ClassVar[int]
    PINSOURCEIP_FIELD_NUMBER: _ClassVar[int]
    SSHFILECOPY_FIELD_NUMBER: _ClassVar[int]
    REQUIREMFATYPE_FIELD_NUMBER: _ClassVar[int]
    DEVICETRUSTMODE_FIELD_NUMBER: _ClassVar[int]
    IDP_FIELD_NUMBER: _ClassVar[int]
    CREATEDESKTOPUSER_FIELD_NUMBER: _ClassVar[int]
    CREATEDATABASEUSER_FIELD_NUMBER: _ClassVar[int]
    CREATEHOSTUSERMODE_FIELD_NUMBER: _ClassVar[int]
    ForwardAgent: bool
    MaxSessionTTL: int
    PortForwarding: BoolValue
    CertificateFormat: str
    ClientIdleTimeout: int
    DisconnectExpiredCert: bool
    BPF: _containers.RepeatedScalarFieldContainer[str]
    PermitX11Forwarding: bool
    MaxConnections: int
    MaxSessions: int
    RequestAccess: str
    RequestPrompt: str
    Lock: str
    RecordSession: RecordSession
    DesktopClipboard: BoolValue
    CertExtensions: _containers.RepeatedCompositeFieldContainer[CertExtension]
    MaxKubernetesConnections: int
    DesktopDirectorySharing: BoolValue
    CreateHostUser: BoolValue
    PinSourceIP: bool
    SSHFileCopy: BoolValue
    RequireMFAType: RequireMFAType
    DeviceTrustMode: str
    IDP: IdPOptions
    CreateDesktopUser: BoolValue
    CreateDatabaseUser: BoolValue
    CreateHostUserMode: CreateHostUserMode
    def __init__(self, ForwardAgent: bool = ..., MaxSessionTTL: _Optional[int] = ..., PortForwarding: _Optional[_Union[BoolValue, _Mapping]] = ..., CertificateFormat: _Optional[str] = ..., ClientIdleTimeout: _Optional[int] = ..., DisconnectExpiredCert: bool = ..., BPF: _Optional[_Iterable[str]] = ..., PermitX11Forwarding: bool = ..., MaxConnections: _Optional[int] = ..., MaxSessions: _Optional[int] = ..., RequestAccess: _Optional[str] = ..., RequestPrompt: _Optional[str] = ..., Lock: _Optional[str] = ..., RecordSession: _Optional[_Union[RecordSession, _Mapping]] = ..., DesktopClipboard: _Optional[_Union[BoolValue, _Mapping]] = ..., CertExtensions: _Optional[_Iterable[_Union[CertExtension, _Mapping]]] = ..., MaxKubernetesConnections: _Optional[int] = ..., DesktopDirectorySharing: _Optional[_Union[BoolValue, _Mapping]] = ..., CreateHostUser: _Optional[_Union[BoolValue, _Mapping]] = ..., PinSourceIP: bool = ..., SSHFileCopy: _Optional[_Union[BoolValue, _Mapping]] = ..., RequireMFAType: _Optional[_Union[RequireMFAType, str]] = ..., DeviceTrustMode: _Optional[str] = ..., IDP: _Optional[_Union[IdPOptions, _Mapping]] = ..., CreateDesktopUser: _Optional[_Union[BoolValue, _Mapping]] = ..., CreateDatabaseUser: _Optional[_Union[BoolValue, _Mapping]] = ..., CreateHostUserMode: _Optional[_Union[CreateHostUserMode, str]] = ...) -> None: ...

class RecordSession(_message.Message):
    __slots__ = ["Desktop", "Default", "SSH"]
    DESKTOP_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_FIELD_NUMBER: _ClassVar[int]
    SSH_FIELD_NUMBER: _ClassVar[int]
    Desktop: BoolValue
    Default: str
    SSH: str
    def __init__(self, Desktop: _Optional[_Union[BoolValue, _Mapping]] = ..., Default: _Optional[str] = ..., SSH: _Optional[str] = ...) -> None: ...

class CertExtension(_message.Message):
    __slots__ = ["Type", "Mode", "Name", "Value"]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    MODE_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    Type: CertExtensionType
    Mode: CertExtensionMode
    Name: str
    Value: str
    def __init__(self, Type: _Optional[_Union[CertExtensionType, str]] = ..., Mode: _Optional[_Union[CertExtensionMode, str]] = ..., Name: _Optional[str] = ..., Value: _Optional[str] = ...) -> None: ...

class RoleConditions(_message.Message):
    __slots__ = ["Logins", "Namespaces", "NodeLabels", "Rules", "KubeGroups", "Request", "KubeUsers", "AppLabels", "ClusterLabels", "KubernetesLabels", "DatabaseLabels", "DatabaseNames", "DatabaseUsers", "Impersonate", "ReviewRequests", "AWSRoleARNs", "WindowsDesktopLogins", "WindowsDesktopLabels", "RequireSessionJoin", "JoinSessions", "HostGroups", "HostSudoers", "AzureIdentities", "KubernetesResources", "GCPServiceAccounts", "DatabaseServiceLabels", "GroupLabels", "DesktopGroups", "DatabaseRoles", "NodeLabelsExpression", "AppLabelsExpression", "ClusterLabelsExpression", "KubernetesLabelsExpression", "DatabaseLabelsExpression", "DatabaseServiceLabelsExpression", "WindowsDesktopLabelsExpression", "GroupLabelsExpression"]
    LOGINS_FIELD_NUMBER: _ClassVar[int]
    NAMESPACES_FIELD_NUMBER: _ClassVar[int]
    NODELABELS_FIELD_NUMBER: _ClassVar[int]
    RULES_FIELD_NUMBER: _ClassVar[int]
    KUBEGROUPS_FIELD_NUMBER: _ClassVar[int]
    REQUEST_FIELD_NUMBER: _ClassVar[int]
    KUBEUSERS_FIELD_NUMBER: _ClassVar[int]
    APPLABELS_FIELD_NUMBER: _ClassVar[int]
    CLUSTERLABELS_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESLABELS_FIELD_NUMBER: _ClassVar[int]
    DATABASELABELS_FIELD_NUMBER: _ClassVar[int]
    DATABASENAMES_FIELD_NUMBER: _ClassVar[int]
    DATABASEUSERS_FIELD_NUMBER: _ClassVar[int]
    IMPERSONATE_FIELD_NUMBER: _ClassVar[int]
    REVIEWREQUESTS_FIELD_NUMBER: _ClassVar[int]
    AWSROLEARNS_FIELD_NUMBER: _ClassVar[int]
    WINDOWSDESKTOPLOGINS_FIELD_NUMBER: _ClassVar[int]
    WINDOWSDESKTOPLABELS_FIELD_NUMBER: _ClassVar[int]
    REQUIRESESSIONJOIN_FIELD_NUMBER: _ClassVar[int]
    JOINSESSIONS_FIELD_NUMBER: _ClassVar[int]
    HOSTGROUPS_FIELD_NUMBER: _ClassVar[int]
    HOSTSUDOERS_FIELD_NUMBER: _ClassVar[int]
    AZUREIDENTITIES_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESRESOURCES_FIELD_NUMBER: _ClassVar[int]
    GCPSERVICEACCOUNTS_FIELD_NUMBER: _ClassVar[int]
    DATABASESERVICELABELS_FIELD_NUMBER: _ClassVar[int]
    GROUPLABELS_FIELD_NUMBER: _ClassVar[int]
    DESKTOPGROUPS_FIELD_NUMBER: _ClassVar[int]
    DATABASEROLES_FIELD_NUMBER: _ClassVar[int]
    NODELABELSEXPRESSION_FIELD_NUMBER: _ClassVar[int]
    APPLABELSEXPRESSION_FIELD_NUMBER: _ClassVar[int]
    CLUSTERLABELSEXPRESSION_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESLABELSEXPRESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASELABELSEXPRESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASESERVICELABELSEXPRESSION_FIELD_NUMBER: _ClassVar[int]
    WINDOWSDESKTOPLABELSEXPRESSION_FIELD_NUMBER: _ClassVar[int]
    GROUPLABELSEXPRESSION_FIELD_NUMBER: _ClassVar[int]
    Logins: _containers.RepeatedScalarFieldContainer[str]
    Namespaces: _containers.RepeatedScalarFieldContainer[str]
    NodeLabels: _wrappers_pb2.LabelValues
    Rules: _containers.RepeatedCompositeFieldContainer[Rule]
    KubeGroups: _containers.RepeatedScalarFieldContainer[str]
    Request: AccessRequestConditions
    KubeUsers: _containers.RepeatedScalarFieldContainer[str]
    AppLabels: _wrappers_pb2.LabelValues
    ClusterLabels: _wrappers_pb2.LabelValues
    KubernetesLabels: _wrappers_pb2.LabelValues
    DatabaseLabels: _wrappers_pb2.LabelValues
    DatabaseNames: _containers.RepeatedScalarFieldContainer[str]
    DatabaseUsers: _containers.RepeatedScalarFieldContainer[str]
    Impersonate: ImpersonateConditions
    ReviewRequests: AccessReviewConditions
    AWSRoleARNs: _containers.RepeatedScalarFieldContainer[str]
    WindowsDesktopLogins: _containers.RepeatedScalarFieldContainer[str]
    WindowsDesktopLabels: _wrappers_pb2.LabelValues
    RequireSessionJoin: _containers.RepeatedCompositeFieldContainer[SessionRequirePolicy]
    JoinSessions: _containers.RepeatedCompositeFieldContainer[SessionJoinPolicy]
    HostGroups: _containers.RepeatedScalarFieldContainer[str]
    HostSudoers: _containers.RepeatedScalarFieldContainer[str]
    AzureIdentities: _containers.RepeatedScalarFieldContainer[str]
    KubernetesResources: _containers.RepeatedCompositeFieldContainer[KubernetesResource]
    GCPServiceAccounts: _containers.RepeatedScalarFieldContainer[str]
    DatabaseServiceLabels: _wrappers_pb2.LabelValues
    GroupLabels: _wrappers_pb2.LabelValues
    DesktopGroups: _containers.RepeatedScalarFieldContainer[str]
    DatabaseRoles: _containers.RepeatedScalarFieldContainer[str]
    NodeLabelsExpression: str
    AppLabelsExpression: str
    ClusterLabelsExpression: str
    KubernetesLabelsExpression: str
    DatabaseLabelsExpression: str
    DatabaseServiceLabelsExpression: str
    WindowsDesktopLabelsExpression: str
    GroupLabelsExpression: str
    def __init__(self, Logins: _Optional[_Iterable[str]] = ..., Namespaces: _Optional[_Iterable[str]] = ..., NodeLabels: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., Rules: _Optional[_Iterable[_Union[Rule, _Mapping]]] = ..., KubeGroups: _Optional[_Iterable[str]] = ..., Request: _Optional[_Union[AccessRequestConditions, _Mapping]] = ..., KubeUsers: _Optional[_Iterable[str]] = ..., AppLabels: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., ClusterLabels: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., KubernetesLabels: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., DatabaseLabels: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., DatabaseNames: _Optional[_Iterable[str]] = ..., DatabaseUsers: _Optional[_Iterable[str]] = ..., Impersonate: _Optional[_Union[ImpersonateConditions, _Mapping]] = ..., ReviewRequests: _Optional[_Union[AccessReviewConditions, _Mapping]] = ..., AWSRoleARNs: _Optional[_Iterable[str]] = ..., WindowsDesktopLogins: _Optional[_Iterable[str]] = ..., WindowsDesktopLabels: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., RequireSessionJoin: _Optional[_Iterable[_Union[SessionRequirePolicy, _Mapping]]] = ..., JoinSessions: _Optional[_Iterable[_Union[SessionJoinPolicy, _Mapping]]] = ..., HostGroups: _Optional[_Iterable[str]] = ..., HostSudoers: _Optional[_Iterable[str]] = ..., AzureIdentities: _Optional[_Iterable[str]] = ..., KubernetesResources: _Optional[_Iterable[_Union[KubernetesResource, _Mapping]]] = ..., GCPServiceAccounts: _Optional[_Iterable[str]] = ..., DatabaseServiceLabels: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., GroupLabels: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., DesktopGroups: _Optional[_Iterable[str]] = ..., DatabaseRoles: _Optional[_Iterable[str]] = ..., NodeLabelsExpression: _Optional[str] = ..., AppLabelsExpression: _Optional[str] = ..., ClusterLabelsExpression: _Optional[str] = ..., KubernetesLabelsExpression: _Optional[str] = ..., DatabaseLabelsExpression: _Optional[str] = ..., DatabaseServiceLabelsExpression: _Optional[str] = ..., WindowsDesktopLabelsExpression: _Optional[str] = ..., GroupLabelsExpression: _Optional[str] = ...) -> None: ...

class KubernetesResource(_message.Message):
    __slots__ = ["Kind", "Namespace", "Name", "Verbs"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    NAMESPACE_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    VERBS_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    Namespace: str
    Name: str
    Verbs: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Kind: _Optional[str] = ..., Namespace: _Optional[str] = ..., Name: _Optional[str] = ..., Verbs: _Optional[_Iterable[str]] = ...) -> None: ...

class SessionRequirePolicy(_message.Message):
    __slots__ = ["Name", "Filter", "Kinds", "Count", "Modes", "OnLeave"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    FILTER_FIELD_NUMBER: _ClassVar[int]
    KINDS_FIELD_NUMBER: _ClassVar[int]
    COUNT_FIELD_NUMBER: _ClassVar[int]
    MODES_FIELD_NUMBER: _ClassVar[int]
    ONLEAVE_FIELD_NUMBER: _ClassVar[int]
    Name: str
    Filter: str
    Kinds: _containers.RepeatedScalarFieldContainer[str]
    Count: int
    Modes: _containers.RepeatedScalarFieldContainer[str]
    OnLeave: str
    def __init__(self, Name: _Optional[str] = ..., Filter: _Optional[str] = ..., Kinds: _Optional[_Iterable[str]] = ..., Count: _Optional[int] = ..., Modes: _Optional[_Iterable[str]] = ..., OnLeave: _Optional[str] = ...) -> None: ...

class SessionJoinPolicy(_message.Message):
    __slots__ = ["Name", "Roles", "Kinds", "Modes"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    KINDS_FIELD_NUMBER: _ClassVar[int]
    MODES_FIELD_NUMBER: _ClassVar[int]
    Name: str
    Roles: _containers.RepeatedScalarFieldContainer[str]
    Kinds: _containers.RepeatedScalarFieldContainer[str]
    Modes: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Name: _Optional[str] = ..., Roles: _Optional[_Iterable[str]] = ..., Kinds: _Optional[_Iterable[str]] = ..., Modes: _Optional[_Iterable[str]] = ...) -> None: ...

class AccessRequestConditions(_message.Message):
    __slots__ = ["Roles", "ClaimsToRoles", "Annotations", "Thresholds", "SuggestedReviewers", "SearchAsRoles", "MaxDuration"]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    CLAIMSTOROLES_FIELD_NUMBER: _ClassVar[int]
    ANNOTATIONS_FIELD_NUMBER: _ClassVar[int]
    THRESHOLDS_FIELD_NUMBER: _ClassVar[int]
    SUGGESTEDREVIEWERS_FIELD_NUMBER: _ClassVar[int]
    SEARCHASROLES_FIELD_NUMBER: _ClassVar[int]
    MAXDURATION_FIELD_NUMBER: _ClassVar[int]
    Roles: _containers.RepeatedScalarFieldContainer[str]
    ClaimsToRoles: _containers.RepeatedCompositeFieldContainer[ClaimMapping]
    Annotations: _wrappers_pb2.LabelValues
    Thresholds: _containers.RepeatedCompositeFieldContainer[AccessReviewThreshold]
    SuggestedReviewers: _containers.RepeatedScalarFieldContainer[str]
    SearchAsRoles: _containers.RepeatedScalarFieldContainer[str]
    MaxDuration: int
    def __init__(self, Roles: _Optional[_Iterable[str]] = ..., ClaimsToRoles: _Optional[_Iterable[_Union[ClaimMapping, _Mapping]]] = ..., Annotations: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., Thresholds: _Optional[_Iterable[_Union[AccessReviewThreshold, _Mapping]]] = ..., SuggestedReviewers: _Optional[_Iterable[str]] = ..., SearchAsRoles: _Optional[_Iterable[str]] = ..., MaxDuration: _Optional[int] = ...) -> None: ...

class AccessReviewConditions(_message.Message):
    __slots__ = ["Roles", "ClaimsToRoles", "Where", "PreviewAsRoles"]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    CLAIMSTOROLES_FIELD_NUMBER: _ClassVar[int]
    WHERE_FIELD_NUMBER: _ClassVar[int]
    PREVIEWASROLES_FIELD_NUMBER: _ClassVar[int]
    Roles: _containers.RepeatedScalarFieldContainer[str]
    ClaimsToRoles: _containers.RepeatedCompositeFieldContainer[ClaimMapping]
    Where: str
    PreviewAsRoles: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Roles: _Optional[_Iterable[str]] = ..., ClaimsToRoles: _Optional[_Iterable[_Union[ClaimMapping, _Mapping]]] = ..., Where: _Optional[str] = ..., PreviewAsRoles: _Optional[_Iterable[str]] = ...) -> None: ...

class AccessRequestAllowedPromotion(_message.Message):
    __slots__ = ["accessListName"]
    ACCESSLISTNAME_FIELD_NUMBER: _ClassVar[int]
    accessListName: str
    def __init__(self, accessListName: _Optional[str] = ...) -> None: ...

class AccessRequestAllowedPromotions(_message.Message):
    __slots__ = ["promotions"]
    PROMOTIONS_FIELD_NUMBER: _ClassVar[int]
    promotions: _containers.RepeatedCompositeFieldContainer[AccessRequestAllowedPromotion]
    def __init__(self, promotions: _Optional[_Iterable[_Union[AccessRequestAllowedPromotion, _Mapping]]] = ...) -> None: ...

class ClaimMapping(_message.Message):
    __slots__ = ["Claim", "Value", "Roles"]
    CLAIM_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    Claim: str
    Value: str
    Roles: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Claim: _Optional[str] = ..., Value: _Optional[str] = ..., Roles: _Optional[_Iterable[str]] = ...) -> None: ...

class TraitMapping(_message.Message):
    __slots__ = ["Trait", "Value", "Roles"]
    TRAIT_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    Trait: str
    Value: str
    Roles: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Trait: _Optional[str] = ..., Value: _Optional[str] = ..., Roles: _Optional[_Iterable[str]] = ...) -> None: ...

class Rule(_message.Message):
    __slots__ = ["Resources", "Verbs", "Where", "Actions"]
    RESOURCES_FIELD_NUMBER: _ClassVar[int]
    VERBS_FIELD_NUMBER: _ClassVar[int]
    WHERE_FIELD_NUMBER: _ClassVar[int]
    ACTIONS_FIELD_NUMBER: _ClassVar[int]
    Resources: _containers.RepeatedScalarFieldContainer[str]
    Verbs: _containers.RepeatedScalarFieldContainer[str]
    Where: str
    Actions: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Resources: _Optional[_Iterable[str]] = ..., Verbs: _Optional[_Iterable[str]] = ..., Where: _Optional[str] = ..., Actions: _Optional[_Iterable[str]] = ...) -> None: ...

class ImpersonateConditions(_message.Message):
    __slots__ = ["Users", "Roles", "Where"]
    USERS_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    WHERE_FIELD_NUMBER: _ClassVar[int]
    Users: _containers.RepeatedScalarFieldContainer[str]
    Roles: _containers.RepeatedScalarFieldContainer[str]
    Where: str
    def __init__(self, Users: _Optional[_Iterable[str]] = ..., Roles: _Optional[_Iterable[str]] = ..., Where: _Optional[str] = ...) -> None: ...

class BoolValue(_message.Message):
    __slots__ = ["Value"]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    Value: bool
    def __init__(self, Value: bool = ...) -> None: ...

class UserV2(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: UserSpecV2
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[UserSpecV2, _Mapping]] = ...) -> None: ...

class UserSpecV2(_message.Message):
    __slots__ = ["OIDCIdentities", "SAMLIdentities", "GithubIdentities", "Roles", "Traits", "Status", "Expires", "CreatedBy", "LocalAuth", "TrustedDeviceIDs"]
    OIDCIDENTITIES_FIELD_NUMBER: _ClassVar[int]
    SAMLIDENTITIES_FIELD_NUMBER: _ClassVar[int]
    GITHUBIDENTITIES_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    TRAITS_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    CREATEDBY_FIELD_NUMBER: _ClassVar[int]
    LOCALAUTH_FIELD_NUMBER: _ClassVar[int]
    TRUSTEDDEVICEIDS_FIELD_NUMBER: _ClassVar[int]
    OIDCIdentities: _containers.RepeatedCompositeFieldContainer[ExternalIdentity]
    SAMLIdentities: _containers.RepeatedCompositeFieldContainer[ExternalIdentity]
    GithubIdentities: _containers.RepeatedCompositeFieldContainer[ExternalIdentity]
    Roles: _containers.RepeatedScalarFieldContainer[str]
    Traits: _wrappers_pb2.LabelValues
    Status: LoginStatus
    Expires: _timestamp_pb2.Timestamp
    CreatedBy: CreatedBy
    LocalAuth: LocalAuthSecrets
    TrustedDeviceIDs: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, OIDCIdentities: _Optional[_Iterable[_Union[ExternalIdentity, _Mapping]]] = ..., SAMLIdentities: _Optional[_Iterable[_Union[ExternalIdentity, _Mapping]]] = ..., GithubIdentities: _Optional[_Iterable[_Union[ExternalIdentity, _Mapping]]] = ..., Roles: _Optional[_Iterable[str]] = ..., Traits: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., Status: _Optional[_Union[LoginStatus, _Mapping]] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., CreatedBy: _Optional[_Union[CreatedBy, _Mapping]] = ..., LocalAuth: _Optional[_Union[LocalAuthSecrets, _Mapping]] = ..., TrustedDeviceIDs: _Optional[_Iterable[str]] = ...) -> None: ...

class ExternalIdentity(_message.Message):
    __slots__ = ["ConnectorID", "Username"]
    CONNECTORID_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    ConnectorID: str
    Username: str
    def __init__(self, ConnectorID: _Optional[str] = ..., Username: _Optional[str] = ...) -> None: ...

class LoginStatus(_message.Message):
    __slots__ = ["IsLocked", "LockedMessage", "LockedTime", "LockExpires", "RecoveryAttemptLockExpires"]
    ISLOCKED_FIELD_NUMBER: _ClassVar[int]
    LOCKEDMESSAGE_FIELD_NUMBER: _ClassVar[int]
    LOCKEDTIME_FIELD_NUMBER: _ClassVar[int]
    LOCKEXPIRES_FIELD_NUMBER: _ClassVar[int]
    RECOVERYATTEMPTLOCKEXPIRES_FIELD_NUMBER: _ClassVar[int]
    IsLocked: bool
    LockedMessage: str
    LockedTime: _timestamp_pb2.Timestamp
    LockExpires: _timestamp_pb2.Timestamp
    RecoveryAttemptLockExpires: _timestamp_pb2.Timestamp
    def __init__(self, IsLocked: bool = ..., LockedMessage: _Optional[str] = ..., LockedTime: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., LockExpires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., RecoveryAttemptLockExpires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class CreatedBy(_message.Message):
    __slots__ = ["Connector", "Time", "User"]
    CONNECTOR_FIELD_NUMBER: _ClassVar[int]
    TIME_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Connector: ConnectorRef
    Time: _timestamp_pb2.Timestamp
    User: UserRef
    def __init__(self, Connector: _Optional[_Union[ConnectorRef, _Mapping]] = ..., Time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., User: _Optional[_Union[UserRef, _Mapping]] = ...) -> None: ...

class LocalAuthSecrets(_message.Message):
    __slots__ = ["PasswordHash", "TOTPKey", "MFA", "Webauthn"]
    PASSWORDHASH_FIELD_NUMBER: _ClassVar[int]
    TOTPKEY_FIELD_NUMBER: _ClassVar[int]
    MFA_FIELD_NUMBER: _ClassVar[int]
    WEBAUTHN_FIELD_NUMBER: _ClassVar[int]
    PasswordHash: bytes
    TOTPKey: str
    MFA: _containers.RepeatedCompositeFieldContainer[MFADevice]
    Webauthn: WebauthnLocalAuth
    def __init__(self, PasswordHash: _Optional[bytes] = ..., TOTPKey: _Optional[str] = ..., MFA: _Optional[_Iterable[_Union[MFADevice, _Mapping]]] = ..., Webauthn: _Optional[_Union[WebauthnLocalAuth, _Mapping]] = ...) -> None: ...

class MFADevice(_message.Message):
    __slots__ = ["kind", "sub_kind", "version", "metadata", "id", "added_at", "last_used", "totp", "u2f", "webauthn"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUB_KIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    ADDED_AT_FIELD_NUMBER: _ClassVar[int]
    LAST_USED_FIELD_NUMBER: _ClassVar[int]
    TOTP_FIELD_NUMBER: _ClassVar[int]
    U2F_FIELD_NUMBER: _ClassVar[int]
    WEBAUTHN_FIELD_NUMBER: _ClassVar[int]
    kind: str
    sub_kind: str
    version: str
    metadata: Metadata
    id: str
    added_at: _timestamp_pb2.Timestamp
    last_used: _timestamp_pb2.Timestamp
    totp: TOTPDevice
    u2f: U2FDevice
    webauthn: WebauthnDevice
    def __init__(self, kind: _Optional[str] = ..., sub_kind: _Optional[str] = ..., version: _Optional[str] = ..., metadata: _Optional[_Union[Metadata, _Mapping]] = ..., id: _Optional[str] = ..., added_at: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., last_used: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., totp: _Optional[_Union[TOTPDevice, _Mapping]] = ..., u2f: _Optional[_Union[U2FDevice, _Mapping]] = ..., webauthn: _Optional[_Union[WebauthnDevice, _Mapping]] = ...) -> None: ...

class TOTPDevice(_message.Message):
    __slots__ = ["key"]
    KEY_FIELD_NUMBER: _ClassVar[int]
    key: str
    def __init__(self, key: _Optional[str] = ...) -> None: ...

class U2FDevice(_message.Message):
    __slots__ = ["key_handle", "pub_key", "counter"]
    KEY_HANDLE_FIELD_NUMBER: _ClassVar[int]
    PUB_KEY_FIELD_NUMBER: _ClassVar[int]
    COUNTER_FIELD_NUMBER: _ClassVar[int]
    key_handle: bytes
    pub_key: bytes
    counter: int
    def __init__(self, key_handle: _Optional[bytes] = ..., pub_key: _Optional[bytes] = ..., counter: _Optional[int] = ...) -> None: ...

class WebauthnDevice(_message.Message):
    __slots__ = ["credential_id", "public_key_cbor", "attestation_type", "aaguid", "signature_counter", "attestation_object", "resident_key", "credential_rp_id"]
    CREDENTIAL_ID_FIELD_NUMBER: _ClassVar[int]
    PUBLIC_KEY_CBOR_FIELD_NUMBER: _ClassVar[int]
    ATTESTATION_TYPE_FIELD_NUMBER: _ClassVar[int]
    AAGUID_FIELD_NUMBER: _ClassVar[int]
    SIGNATURE_COUNTER_FIELD_NUMBER: _ClassVar[int]
    ATTESTATION_OBJECT_FIELD_NUMBER: _ClassVar[int]
    RESIDENT_KEY_FIELD_NUMBER: _ClassVar[int]
    CREDENTIAL_RP_ID_FIELD_NUMBER: _ClassVar[int]
    credential_id: bytes
    public_key_cbor: bytes
    attestation_type: str
    aaguid: bytes
    signature_counter: int
    attestation_object: bytes
    resident_key: bool
    credential_rp_id: str
    def __init__(self, credential_id: _Optional[bytes] = ..., public_key_cbor: _Optional[bytes] = ..., attestation_type: _Optional[str] = ..., aaguid: _Optional[bytes] = ..., signature_counter: _Optional[int] = ..., attestation_object: _Optional[bytes] = ..., resident_key: bool = ..., credential_rp_id: _Optional[str] = ...) -> None: ...

class WebauthnLocalAuth(_message.Message):
    __slots__ = ["UserID"]
    USERID_FIELD_NUMBER: _ClassVar[int]
    UserID: bytes
    def __init__(self, UserID: _Optional[bytes] = ...) -> None: ...

class ConnectorRef(_message.Message):
    __slots__ = ["Type", "ID", "Identity"]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    IDENTITY_FIELD_NUMBER: _ClassVar[int]
    Type: str
    ID: str
    Identity: str
    def __init__(self, Type: _Optional[str] = ..., ID: _Optional[str] = ..., Identity: _Optional[str] = ...) -> None: ...

class UserRef(_message.Message):
    __slots__ = ["Name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Name: str
    def __init__(self, Name: _Optional[str] = ...) -> None: ...

class ReverseTunnelV2(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: ReverseTunnelSpecV2
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[ReverseTunnelSpecV2, _Mapping]] = ...) -> None: ...

class ReverseTunnelSpecV2(_message.Message):
    __slots__ = ["ClusterName", "DialAddrs", "Type"]
    CLUSTERNAME_FIELD_NUMBER: _ClassVar[int]
    DIALADDRS_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    ClusterName: str
    DialAddrs: _containers.RepeatedScalarFieldContainer[str]
    Type: str
    def __init__(self, ClusterName: _Optional[str] = ..., DialAddrs: _Optional[_Iterable[str]] = ..., Type: _Optional[str] = ...) -> None: ...

class TunnelConnectionV2(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: TunnelConnectionSpecV2
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[TunnelConnectionSpecV2, _Mapping]] = ...) -> None: ...

class TunnelConnectionSpecV2(_message.Message):
    __slots__ = ["ClusterName", "ProxyName", "LastHeartbeat", "Type"]
    CLUSTERNAME_FIELD_NUMBER: _ClassVar[int]
    PROXYNAME_FIELD_NUMBER: _ClassVar[int]
    LASTHEARTBEAT_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    ClusterName: str
    ProxyName: str
    LastHeartbeat: _timestamp_pb2.Timestamp
    Type: str
    def __init__(self, ClusterName: _Optional[str] = ..., ProxyName: _Optional[str] = ..., LastHeartbeat: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Type: _Optional[str] = ...) -> None: ...

class SemaphoreFilter(_message.Message):
    __slots__ = ["SemaphoreKind", "SemaphoreName"]
    SEMAPHOREKIND_FIELD_NUMBER: _ClassVar[int]
    SEMAPHORENAME_FIELD_NUMBER: _ClassVar[int]
    SemaphoreKind: str
    SemaphoreName: str
    def __init__(self, SemaphoreKind: _Optional[str] = ..., SemaphoreName: _Optional[str] = ...) -> None: ...

class AcquireSemaphoreRequest(_message.Message):
    __slots__ = ["SemaphoreKind", "SemaphoreName", "MaxLeases", "Expires", "Holder"]
    SEMAPHOREKIND_FIELD_NUMBER: _ClassVar[int]
    SEMAPHORENAME_FIELD_NUMBER: _ClassVar[int]
    MAXLEASES_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    HOLDER_FIELD_NUMBER: _ClassVar[int]
    SemaphoreKind: str
    SemaphoreName: str
    MaxLeases: int
    Expires: _timestamp_pb2.Timestamp
    Holder: str
    def __init__(self, SemaphoreKind: _Optional[str] = ..., SemaphoreName: _Optional[str] = ..., MaxLeases: _Optional[int] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Holder: _Optional[str] = ...) -> None: ...

class SemaphoreLease(_message.Message):
    __slots__ = ["SemaphoreKind", "SemaphoreName", "LeaseID", "Expires"]
    SEMAPHOREKIND_FIELD_NUMBER: _ClassVar[int]
    SEMAPHORENAME_FIELD_NUMBER: _ClassVar[int]
    LEASEID_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    SemaphoreKind: str
    SemaphoreName: str
    LeaseID: str
    Expires: _timestamp_pb2.Timestamp
    def __init__(self, SemaphoreKind: _Optional[str] = ..., SemaphoreName: _Optional[str] = ..., LeaseID: _Optional[str] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class SemaphoreLeaseRef(_message.Message):
    __slots__ = ["LeaseID", "Expires", "Holder"]
    LEASEID_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    HOLDER_FIELD_NUMBER: _ClassVar[int]
    LeaseID: str
    Expires: _timestamp_pb2.Timestamp
    Holder: str
    def __init__(self, LeaseID: _Optional[str] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Holder: _Optional[str] = ...) -> None: ...

class SemaphoreV3(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: SemaphoreSpecV3
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[SemaphoreSpecV3, _Mapping]] = ...) -> None: ...

class SemaphoreSpecV3(_message.Message):
    __slots__ = ["Leases"]
    LEASES_FIELD_NUMBER: _ClassVar[int]
    Leases: _containers.RepeatedCompositeFieldContainer[SemaphoreLeaseRef]
    def __init__(self, Leases: _Optional[_Iterable[_Union[SemaphoreLeaseRef, _Mapping]]] = ...) -> None: ...

class WebSessionV2(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: WebSessionSpecV2
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[WebSessionSpecV2, _Mapping]] = ...) -> None: ...

class WebSessionSpecV2(_message.Message):
    __slots__ = ["User", "Pub", "Priv", "TLSCert", "BearerToken", "BearerTokenExpires", "Expires", "LoginTime", "IdleTimeout", "ConsumedAccessRequestID", "SAMLSession"]
    USER_FIELD_NUMBER: _ClassVar[int]
    PUB_FIELD_NUMBER: _ClassVar[int]
    PRIV_FIELD_NUMBER: _ClassVar[int]
    TLSCERT_FIELD_NUMBER: _ClassVar[int]
    BEARERTOKEN_FIELD_NUMBER: _ClassVar[int]
    BEARERTOKENEXPIRES_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    LOGINTIME_FIELD_NUMBER: _ClassVar[int]
    IDLETIMEOUT_FIELD_NUMBER: _ClassVar[int]
    CONSUMEDACCESSREQUESTID_FIELD_NUMBER: _ClassVar[int]
    SAMLSESSION_FIELD_NUMBER: _ClassVar[int]
    User: str
    Pub: bytes
    Priv: bytes
    TLSCert: bytes
    BearerToken: str
    BearerTokenExpires: _timestamp_pb2.Timestamp
    Expires: _timestamp_pb2.Timestamp
    LoginTime: _timestamp_pb2.Timestamp
    IdleTimeout: int
    ConsumedAccessRequestID: str
    SAMLSession: SAMLSessionData
    def __init__(self, User: _Optional[str] = ..., Pub: _Optional[bytes] = ..., Priv: _Optional[bytes] = ..., TLSCert: _Optional[bytes] = ..., BearerToken: _Optional[str] = ..., BearerTokenExpires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., LoginTime: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., IdleTimeout: _Optional[int] = ..., ConsumedAccessRequestID: _Optional[str] = ..., SAMLSession: _Optional[_Union[SAMLSessionData, _Mapping]] = ...) -> None: ...

class WebSessionFilter(_message.Message):
    __slots__ = ["User"]
    USER_FIELD_NUMBER: _ClassVar[int]
    User: str
    def __init__(self, User: _Optional[str] = ...) -> None: ...

class SAMLSessionData(_message.Message):
    __slots__ = ["ID", "CreateTime", "ExpireTime", "Index", "NameID", "NameIDFormat", "SubjectID", "Groups", "UserName", "UserEmail", "UserCommonName", "UserSurname", "UserGivenName", "UserScopedAffiliation", "CustomAttributes"]
    ID_FIELD_NUMBER: _ClassVar[int]
    CREATETIME_FIELD_NUMBER: _ClassVar[int]
    EXPIRETIME_FIELD_NUMBER: _ClassVar[int]
    INDEX_FIELD_NUMBER: _ClassVar[int]
    NAMEID_FIELD_NUMBER: _ClassVar[int]
    NAMEIDFORMAT_FIELD_NUMBER: _ClassVar[int]
    SUBJECTID_FIELD_NUMBER: _ClassVar[int]
    GROUPS_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    USEREMAIL_FIELD_NUMBER: _ClassVar[int]
    USERCOMMONNAME_FIELD_NUMBER: _ClassVar[int]
    USERSURNAME_FIELD_NUMBER: _ClassVar[int]
    USERGIVENNAME_FIELD_NUMBER: _ClassVar[int]
    USERSCOPEDAFFILIATION_FIELD_NUMBER: _ClassVar[int]
    CUSTOMATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    ID: str
    CreateTime: _timestamp_pb2.Timestamp
    ExpireTime: _timestamp_pb2.Timestamp
    Index: str
    NameID: str
    NameIDFormat: str
    SubjectID: str
    Groups: _containers.RepeatedScalarFieldContainer[str]
    UserName: str
    UserEmail: str
    UserCommonName: str
    UserSurname: str
    UserGivenName: str
    UserScopedAffiliation: str
    CustomAttributes: _containers.RepeatedCompositeFieldContainer[SAMLAttribute]
    def __init__(self, ID: _Optional[str] = ..., CreateTime: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., ExpireTime: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Index: _Optional[str] = ..., NameID: _Optional[str] = ..., NameIDFormat: _Optional[str] = ..., SubjectID: _Optional[str] = ..., Groups: _Optional[_Iterable[str]] = ..., UserName: _Optional[str] = ..., UserEmail: _Optional[str] = ..., UserCommonName: _Optional[str] = ..., UserSurname: _Optional[str] = ..., UserGivenName: _Optional[str] = ..., UserScopedAffiliation: _Optional[str] = ..., CustomAttributes: _Optional[_Iterable[_Union[SAMLAttribute, _Mapping]]] = ...) -> None: ...

class SAMLAttribute(_message.Message):
    __slots__ = ["FriendlyName", "Name", "NameFormat", "Values"]
    FRIENDLYNAME_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    NAMEFORMAT_FIELD_NUMBER: _ClassVar[int]
    VALUES_FIELD_NUMBER: _ClassVar[int]
    FriendlyName: str
    Name: str
    NameFormat: str
    Values: _containers.RepeatedCompositeFieldContainer[SAMLAttributeValue]
    def __init__(self, FriendlyName: _Optional[str] = ..., Name: _Optional[str] = ..., NameFormat: _Optional[str] = ..., Values: _Optional[_Iterable[_Union[SAMLAttributeValue, _Mapping]]] = ...) -> None: ...

class SAMLAttributeValue(_message.Message):
    __slots__ = ["Type", "Value", "NameID"]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    NAMEID_FIELD_NUMBER: _ClassVar[int]
    Type: str
    Value: str
    NameID: SAMLNameID
    def __init__(self, Type: _Optional[str] = ..., Value: _Optional[str] = ..., NameID: _Optional[_Union[SAMLNameID, _Mapping]] = ...) -> None: ...

class SAMLNameID(_message.Message):
    __slots__ = ["NameQualifier", "SPNameQualifier", "Format", "SPProvidedID", "Value"]
    NAMEQUALIFIER_FIELD_NUMBER: _ClassVar[int]
    SPNAMEQUALIFIER_FIELD_NUMBER: _ClassVar[int]
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    SPPROVIDEDID_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    NameQualifier: str
    SPNameQualifier: str
    Format: str
    SPProvidedID: str
    Value: str
    def __init__(self, NameQualifier: _Optional[str] = ..., SPNameQualifier: _Optional[str] = ..., Format: _Optional[str] = ..., SPProvidedID: _Optional[str] = ..., Value: _Optional[str] = ...) -> None: ...

class RemoteClusterV3(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Status"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Status: RemoteClusterStatusV3
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Status: _Optional[_Union[RemoteClusterStatusV3, _Mapping]] = ...) -> None: ...

class RemoteClusterStatusV3(_message.Message):
    __slots__ = ["Connection", "LastHeartbeat"]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    LASTHEARTBEAT_FIELD_NUMBER: _ClassVar[int]
    Connection: str
    LastHeartbeat: _timestamp_pb2.Timestamp
    def __init__(self, Connection: _Optional[str] = ..., LastHeartbeat: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class KubernetesCluster(_message.Message):
    __slots__ = ["Name", "StaticLabels", "DynamicLabels"]
    class StaticLabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    class DynamicLabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: CommandLabelV2
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[CommandLabelV2, _Mapping]] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    STATICLABELS_FIELD_NUMBER: _ClassVar[int]
    DYNAMICLABELS_FIELD_NUMBER: _ClassVar[int]
    Name: str
    StaticLabels: _containers.ScalarMap[str, str]
    DynamicLabels: _containers.MessageMap[str, CommandLabelV2]
    def __init__(self, Name: _Optional[str] = ..., StaticLabels: _Optional[_Mapping[str, str]] = ..., DynamicLabels: _Optional[_Mapping[str, CommandLabelV2]] = ...) -> None: ...

class KubernetesClusterV3(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: KubernetesClusterSpecV3
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[KubernetesClusterSpecV3, _Mapping]] = ...) -> None: ...

class KubernetesClusterSpecV3(_message.Message):
    __slots__ = ["DynamicLabels", "Kubeconfig", "Azure", "AWS", "GCP"]
    class DynamicLabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: CommandLabelV2
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[CommandLabelV2, _Mapping]] = ...) -> None: ...
    DYNAMICLABELS_FIELD_NUMBER: _ClassVar[int]
    KUBECONFIG_FIELD_NUMBER: _ClassVar[int]
    AZURE_FIELD_NUMBER: _ClassVar[int]
    AWS_FIELD_NUMBER: _ClassVar[int]
    GCP_FIELD_NUMBER: _ClassVar[int]
    DynamicLabels: _containers.MessageMap[str, CommandLabelV2]
    Kubeconfig: bytes
    Azure: KubeAzure
    AWS: KubeAWS
    GCP: KubeGCP
    def __init__(self, DynamicLabels: _Optional[_Mapping[str, CommandLabelV2]] = ..., Kubeconfig: _Optional[bytes] = ..., Azure: _Optional[_Union[KubeAzure, _Mapping]] = ..., AWS: _Optional[_Union[KubeAWS, _Mapping]] = ..., GCP: _Optional[_Union[KubeGCP, _Mapping]] = ...) -> None: ...

class KubeAzure(_message.Message):
    __slots__ = ["ResourceName", "ResourceGroup", "TenantID", "SubscriptionID"]
    RESOURCENAME_FIELD_NUMBER: _ClassVar[int]
    RESOURCEGROUP_FIELD_NUMBER: _ClassVar[int]
    TENANTID_FIELD_NUMBER: _ClassVar[int]
    SUBSCRIPTIONID_FIELD_NUMBER: _ClassVar[int]
    ResourceName: str
    ResourceGroup: str
    TenantID: str
    SubscriptionID: str
    def __init__(self, ResourceName: _Optional[str] = ..., ResourceGroup: _Optional[str] = ..., TenantID: _Optional[str] = ..., SubscriptionID: _Optional[str] = ...) -> None: ...

class KubeAWS(_message.Message):
    __slots__ = ["Region", "AccountID", "Name"]
    REGION_FIELD_NUMBER: _ClassVar[int]
    ACCOUNTID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Region: str
    AccountID: str
    Name: str
    def __init__(self, Region: _Optional[str] = ..., AccountID: _Optional[str] = ..., Name: _Optional[str] = ...) -> None: ...

class KubeGCP(_message.Message):
    __slots__ = ["Location", "ProjectID", "Name"]
    LOCATION_FIELD_NUMBER: _ClassVar[int]
    PROJECTID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Location: str
    ProjectID: str
    Name: str
    def __init__(self, Location: _Optional[str] = ..., ProjectID: _Optional[str] = ..., Name: _Optional[str] = ...) -> None: ...

class KubernetesClusterV3List(_message.Message):
    __slots__ = ["KubernetesClusters"]
    KUBERNETESCLUSTERS_FIELD_NUMBER: _ClassVar[int]
    KubernetesClusters: _containers.RepeatedCompositeFieldContainer[KubernetesClusterV3]
    def __init__(self, KubernetesClusters: _Optional[_Iterable[_Union[KubernetesClusterV3, _Mapping]]] = ...) -> None: ...

class KubernetesServerV3(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: KubernetesServerSpecV3
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[KubernetesServerSpecV3, _Mapping]] = ...) -> None: ...

class KubernetesServerSpecV3(_message.Message):
    __slots__ = ["Version", "Hostname", "HostID", "Rotation", "Cluster", "ProxyIDs"]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    HOSTNAME_FIELD_NUMBER: _ClassVar[int]
    HOSTID_FIELD_NUMBER: _ClassVar[int]
    ROTATION_FIELD_NUMBER: _ClassVar[int]
    CLUSTER_FIELD_NUMBER: _ClassVar[int]
    PROXYIDS_FIELD_NUMBER: _ClassVar[int]
    Version: str
    Hostname: str
    HostID: str
    Rotation: Rotation
    Cluster: KubernetesClusterV3
    ProxyIDs: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Version: _Optional[str] = ..., Hostname: _Optional[str] = ..., HostID: _Optional[str] = ..., Rotation: _Optional[_Union[Rotation, _Mapping]] = ..., Cluster: _Optional[_Union[KubernetesClusterV3, _Mapping]] = ..., ProxyIDs: _Optional[_Iterable[str]] = ...) -> None: ...

class WebTokenV3(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: WebTokenSpecV3
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[WebTokenSpecV3, _Mapping]] = ...) -> None: ...

class WebTokenSpecV3(_message.Message):
    __slots__ = ["User", "Token"]
    USER_FIELD_NUMBER: _ClassVar[int]
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    User: str
    Token: str
    def __init__(self, User: _Optional[str] = ..., Token: _Optional[str] = ...) -> None: ...

class GetWebSessionRequest(_message.Message):
    __slots__ = ["User", "SessionID"]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    User: str
    SessionID: str
    def __init__(self, User: _Optional[str] = ..., SessionID: _Optional[str] = ...) -> None: ...

class DeleteWebSessionRequest(_message.Message):
    __slots__ = ["User", "SessionID"]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    User: str
    SessionID: str
    def __init__(self, User: _Optional[str] = ..., SessionID: _Optional[str] = ...) -> None: ...

class GetWebTokenRequest(_message.Message):
    __slots__ = ["User", "Token"]
    USER_FIELD_NUMBER: _ClassVar[int]
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    User: str
    Token: str
    def __init__(self, User: _Optional[str] = ..., Token: _Optional[str] = ...) -> None: ...

class DeleteWebTokenRequest(_message.Message):
    __slots__ = ["User", "Token"]
    USER_FIELD_NUMBER: _ClassVar[int]
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    User: str
    Token: str
    def __init__(self, User: _Optional[str] = ..., Token: _Optional[str] = ...) -> None: ...

class ResourceRequest(_message.Message):
    __slots__ = ["Name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Name: str
    def __init__(self, Name: _Optional[str] = ...) -> None: ...

class ResourceWithSecretsRequest(_message.Message):
    __slots__ = ["Name", "WithSecrets"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    WITHSECRETS_FIELD_NUMBER: _ClassVar[int]
    Name: str
    WithSecrets: bool
    def __init__(self, Name: _Optional[str] = ..., WithSecrets: bool = ...) -> None: ...

class ResourcesWithSecretsRequest(_message.Message):
    __slots__ = ["WithSecrets"]
    WITHSECRETS_FIELD_NUMBER: _ClassVar[int]
    WithSecrets: bool
    def __init__(self, WithSecrets: bool = ...) -> None: ...

class ResourceInNamespaceRequest(_message.Message):
    __slots__ = ["Name", "Namespace"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    NAMESPACE_FIELD_NUMBER: _ClassVar[int]
    Name: str
    Namespace: str
    def __init__(self, Name: _Optional[str] = ..., Namespace: _Optional[str] = ...) -> None: ...

class ResourcesInNamespaceRequest(_message.Message):
    __slots__ = ["Namespace"]
    NAMESPACE_FIELD_NUMBER: _ClassVar[int]
    Namespace: str
    def __init__(self, Namespace: _Optional[str] = ...) -> None: ...

class OIDCConnectorV3(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: OIDCConnectorSpecV3
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[OIDCConnectorSpecV3, _Mapping]] = ...) -> None: ...

class OIDCConnectorV3List(_message.Message):
    __slots__ = ["OIDCConnectors"]
    OIDCCONNECTORS_FIELD_NUMBER: _ClassVar[int]
    OIDCConnectors: _containers.RepeatedCompositeFieldContainer[OIDCConnectorV3]
    def __init__(self, OIDCConnectors: _Optional[_Iterable[_Union[OIDCConnectorV3, _Mapping]]] = ...) -> None: ...

class OIDCConnectorSpecV3(_message.Message):
    __slots__ = ["IssuerURL", "ClientID", "ClientSecret", "ACR", "Provider", "Display", "Scope", "Prompt", "ClaimsToRoles", "GoogleServiceAccountURI", "GoogleServiceAccount", "GoogleAdminEmail", "RedirectURLs", "AllowUnverifiedEmail", "UsernameClaim", "MaxAge"]
    ISSUERURL_FIELD_NUMBER: _ClassVar[int]
    CLIENTID_FIELD_NUMBER: _ClassVar[int]
    CLIENTSECRET_FIELD_NUMBER: _ClassVar[int]
    ACR_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_FIELD_NUMBER: _ClassVar[int]
    SCOPE_FIELD_NUMBER: _ClassVar[int]
    PROMPT_FIELD_NUMBER: _ClassVar[int]
    CLAIMSTOROLES_FIELD_NUMBER: _ClassVar[int]
    GOOGLESERVICEACCOUNTURI_FIELD_NUMBER: _ClassVar[int]
    GOOGLESERVICEACCOUNT_FIELD_NUMBER: _ClassVar[int]
    GOOGLEADMINEMAIL_FIELD_NUMBER: _ClassVar[int]
    REDIRECTURLS_FIELD_NUMBER: _ClassVar[int]
    ALLOWUNVERIFIEDEMAIL_FIELD_NUMBER: _ClassVar[int]
    USERNAMECLAIM_FIELD_NUMBER: _ClassVar[int]
    MAXAGE_FIELD_NUMBER: _ClassVar[int]
    IssuerURL: str
    ClientID: str
    ClientSecret: str
    ACR: str
    Provider: str
    Display: str
    Scope: _containers.RepeatedScalarFieldContainer[str]
    Prompt: str
    ClaimsToRoles: _containers.RepeatedCompositeFieldContainer[ClaimMapping]
    GoogleServiceAccountURI: str
    GoogleServiceAccount: str
    GoogleAdminEmail: str
    RedirectURLs: _wrappers_pb2.StringValues
    AllowUnverifiedEmail: bool
    UsernameClaim: str
    MaxAge: MaxAge
    def __init__(self, IssuerURL: _Optional[str] = ..., ClientID: _Optional[str] = ..., ClientSecret: _Optional[str] = ..., ACR: _Optional[str] = ..., Provider: _Optional[str] = ..., Display: _Optional[str] = ..., Scope: _Optional[_Iterable[str]] = ..., Prompt: _Optional[str] = ..., ClaimsToRoles: _Optional[_Iterable[_Union[ClaimMapping, _Mapping]]] = ..., GoogleServiceAccountURI: _Optional[str] = ..., GoogleServiceAccount: _Optional[str] = ..., GoogleAdminEmail: _Optional[str] = ..., RedirectURLs: _Optional[_Union[_wrappers_pb2.StringValues, _Mapping]] = ..., AllowUnverifiedEmail: bool = ..., UsernameClaim: _Optional[str] = ..., MaxAge: _Optional[_Union[MaxAge, _Mapping]] = ...) -> None: ...

class MaxAge(_message.Message):
    __slots__ = ["Value"]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    Value: int
    def __init__(self, Value: _Optional[int] = ...) -> None: ...

class OIDCAuthRequest(_message.Message):
    __slots__ = ["ConnectorID", "Type", "CheckUser", "StateToken", "CSRFToken", "RedirectURL", "PublicKey", "CertTTL", "CreateWebSession", "ClientRedirectURL", "Compatibility", "RouteToCluster", "KubernetesCluster", "SSOTestFlow", "ConnectorSpec", "ProxyAddress", "attestation_statement", "ClientLoginIP"]
    CONNECTORID_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    CHECKUSER_FIELD_NUMBER: _ClassVar[int]
    STATETOKEN_FIELD_NUMBER: _ClassVar[int]
    CSRFTOKEN_FIELD_NUMBER: _ClassVar[int]
    REDIRECTURL_FIELD_NUMBER: _ClassVar[int]
    PUBLICKEY_FIELD_NUMBER: _ClassVar[int]
    CERTTTL_FIELD_NUMBER: _ClassVar[int]
    CREATEWEBSESSION_FIELD_NUMBER: _ClassVar[int]
    CLIENTREDIRECTURL_FIELD_NUMBER: _ClassVar[int]
    COMPATIBILITY_FIELD_NUMBER: _ClassVar[int]
    ROUTETOCLUSTER_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESCLUSTER_FIELD_NUMBER: _ClassVar[int]
    SSOTESTFLOW_FIELD_NUMBER: _ClassVar[int]
    CONNECTORSPEC_FIELD_NUMBER: _ClassVar[int]
    PROXYADDRESS_FIELD_NUMBER: _ClassVar[int]
    ATTESTATION_STATEMENT_FIELD_NUMBER: _ClassVar[int]
    CLIENTLOGINIP_FIELD_NUMBER: _ClassVar[int]
    ConnectorID: str
    Type: str
    CheckUser: bool
    StateToken: str
    CSRFToken: str
    RedirectURL: str
    PublicKey: bytes
    CertTTL: int
    CreateWebSession: bool
    ClientRedirectURL: str
    Compatibility: str
    RouteToCluster: str
    KubernetesCluster: str
    SSOTestFlow: bool
    ConnectorSpec: OIDCConnectorSpecV3
    ProxyAddress: str
    attestation_statement: _attestation_pb2.AttestationStatement
    ClientLoginIP: str
    def __init__(self, ConnectorID: _Optional[str] = ..., Type: _Optional[str] = ..., CheckUser: bool = ..., StateToken: _Optional[str] = ..., CSRFToken: _Optional[str] = ..., RedirectURL: _Optional[str] = ..., PublicKey: _Optional[bytes] = ..., CertTTL: _Optional[int] = ..., CreateWebSession: bool = ..., ClientRedirectURL: _Optional[str] = ..., Compatibility: _Optional[str] = ..., RouteToCluster: _Optional[str] = ..., KubernetesCluster: _Optional[str] = ..., SSOTestFlow: bool = ..., ConnectorSpec: _Optional[_Union[OIDCConnectorSpecV3, _Mapping]] = ..., ProxyAddress: _Optional[str] = ..., attestation_statement: _Optional[_Union[_attestation_pb2.AttestationStatement, _Mapping]] = ..., ClientLoginIP: _Optional[str] = ...) -> None: ...

class SAMLConnectorV2(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: SAMLConnectorSpecV2
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[SAMLConnectorSpecV2, _Mapping]] = ...) -> None: ...

class SAMLConnectorV2List(_message.Message):
    __slots__ = ["SAMLConnectors"]
    SAMLCONNECTORS_FIELD_NUMBER: _ClassVar[int]
    SAMLConnectors: _containers.RepeatedCompositeFieldContainer[SAMLConnectorV2]
    def __init__(self, SAMLConnectors: _Optional[_Iterable[_Union[SAMLConnectorV2, _Mapping]]] = ...) -> None: ...

class SAMLConnectorSpecV2(_message.Message):
    __slots__ = ["Issuer", "SSO", "Cert", "Display", "AssertionConsumerService", "Audience", "ServiceProviderIssuer", "EntityDescriptor", "EntityDescriptorURL", "AttributesToRoles", "SigningKeyPair", "Provider", "EncryptionKeyPair", "AllowIDPInitiated"]
    ISSUER_FIELD_NUMBER: _ClassVar[int]
    SSO_FIELD_NUMBER: _ClassVar[int]
    CERT_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_FIELD_NUMBER: _ClassVar[int]
    ASSERTIONCONSUMERSERVICE_FIELD_NUMBER: _ClassVar[int]
    AUDIENCE_FIELD_NUMBER: _ClassVar[int]
    SERVICEPROVIDERISSUER_FIELD_NUMBER: _ClassVar[int]
    ENTITYDESCRIPTOR_FIELD_NUMBER: _ClassVar[int]
    ENTITYDESCRIPTORURL_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTESTOROLES_FIELD_NUMBER: _ClassVar[int]
    SIGNINGKEYPAIR_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    ENCRYPTIONKEYPAIR_FIELD_NUMBER: _ClassVar[int]
    ALLOWIDPINITIATED_FIELD_NUMBER: _ClassVar[int]
    Issuer: str
    SSO: str
    Cert: str
    Display: str
    AssertionConsumerService: str
    Audience: str
    ServiceProviderIssuer: str
    EntityDescriptor: str
    EntityDescriptorURL: str
    AttributesToRoles: _containers.RepeatedCompositeFieldContainer[AttributeMapping]
    SigningKeyPair: AsymmetricKeyPair
    Provider: str
    EncryptionKeyPair: AsymmetricKeyPair
    AllowIDPInitiated: bool
    def __init__(self, Issuer: _Optional[str] = ..., SSO: _Optional[str] = ..., Cert: _Optional[str] = ..., Display: _Optional[str] = ..., AssertionConsumerService: _Optional[str] = ..., Audience: _Optional[str] = ..., ServiceProviderIssuer: _Optional[str] = ..., EntityDescriptor: _Optional[str] = ..., EntityDescriptorURL: _Optional[str] = ..., AttributesToRoles: _Optional[_Iterable[_Union[AttributeMapping, _Mapping]]] = ..., SigningKeyPair: _Optional[_Union[AsymmetricKeyPair, _Mapping]] = ..., Provider: _Optional[str] = ..., EncryptionKeyPair: _Optional[_Union[AsymmetricKeyPair, _Mapping]] = ..., AllowIDPInitiated: bool = ...) -> None: ...

class SAMLAuthRequest(_message.Message):
    __slots__ = ["ID", "ConnectorID", "Type", "CheckUser", "RedirectURL", "PublicKey", "CertTTL", "CSRFToken", "CreateWebSession", "ClientRedirectURL", "Compatibility", "RouteToCluster", "KubernetesCluster", "SSOTestFlow", "ConnectorSpec", "attestation_statement", "ClientLoginIP"]
    ID_FIELD_NUMBER: _ClassVar[int]
    CONNECTORID_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    CHECKUSER_FIELD_NUMBER: _ClassVar[int]
    REDIRECTURL_FIELD_NUMBER: _ClassVar[int]
    PUBLICKEY_FIELD_NUMBER: _ClassVar[int]
    CERTTTL_FIELD_NUMBER: _ClassVar[int]
    CSRFTOKEN_FIELD_NUMBER: _ClassVar[int]
    CREATEWEBSESSION_FIELD_NUMBER: _ClassVar[int]
    CLIENTREDIRECTURL_FIELD_NUMBER: _ClassVar[int]
    COMPATIBILITY_FIELD_NUMBER: _ClassVar[int]
    ROUTETOCLUSTER_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESCLUSTER_FIELD_NUMBER: _ClassVar[int]
    SSOTESTFLOW_FIELD_NUMBER: _ClassVar[int]
    CONNECTORSPEC_FIELD_NUMBER: _ClassVar[int]
    ATTESTATION_STATEMENT_FIELD_NUMBER: _ClassVar[int]
    CLIENTLOGINIP_FIELD_NUMBER: _ClassVar[int]
    ID: str
    ConnectorID: str
    Type: str
    CheckUser: bool
    RedirectURL: str
    PublicKey: bytes
    CertTTL: int
    CSRFToken: str
    CreateWebSession: bool
    ClientRedirectURL: str
    Compatibility: str
    RouteToCluster: str
    KubernetesCluster: str
    SSOTestFlow: bool
    ConnectorSpec: SAMLConnectorSpecV2
    attestation_statement: _attestation_pb2.AttestationStatement
    ClientLoginIP: str
    def __init__(self, ID: _Optional[str] = ..., ConnectorID: _Optional[str] = ..., Type: _Optional[str] = ..., CheckUser: bool = ..., RedirectURL: _Optional[str] = ..., PublicKey: _Optional[bytes] = ..., CertTTL: _Optional[int] = ..., CSRFToken: _Optional[str] = ..., CreateWebSession: bool = ..., ClientRedirectURL: _Optional[str] = ..., Compatibility: _Optional[str] = ..., RouteToCluster: _Optional[str] = ..., KubernetesCluster: _Optional[str] = ..., SSOTestFlow: bool = ..., ConnectorSpec: _Optional[_Union[SAMLConnectorSpecV2, _Mapping]] = ..., attestation_statement: _Optional[_Union[_attestation_pb2.AttestationStatement, _Mapping]] = ..., ClientLoginIP: _Optional[str] = ...) -> None: ...

class AttributeMapping(_message.Message):
    __slots__ = ["Name", "Value", "Roles"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    Name: str
    Value: str
    Roles: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Name: _Optional[str] = ..., Value: _Optional[str] = ..., Roles: _Optional[_Iterable[str]] = ...) -> None: ...

class AsymmetricKeyPair(_message.Message):
    __slots__ = ["PrivateKey", "Cert"]
    PRIVATEKEY_FIELD_NUMBER: _ClassVar[int]
    CERT_FIELD_NUMBER: _ClassVar[int]
    PrivateKey: str
    Cert: str
    def __init__(self, PrivateKey: _Optional[str] = ..., Cert: _Optional[str] = ...) -> None: ...

class GithubConnectorV3(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: GithubConnectorSpecV3
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[GithubConnectorSpecV3, _Mapping]] = ...) -> None: ...

class GithubConnectorV3List(_message.Message):
    __slots__ = ["GithubConnectors"]
    GITHUBCONNECTORS_FIELD_NUMBER: _ClassVar[int]
    GithubConnectors: _containers.RepeatedCompositeFieldContainer[GithubConnectorV3]
    def __init__(self, GithubConnectors: _Optional[_Iterable[_Union[GithubConnectorV3, _Mapping]]] = ...) -> None: ...

class GithubConnectorSpecV3(_message.Message):
    __slots__ = ["ClientID", "ClientSecret", "RedirectURL", "TeamsToLogins", "Display", "TeamsToRoles", "EndpointURL", "APIEndpointURL"]
    CLIENTID_FIELD_NUMBER: _ClassVar[int]
    CLIENTSECRET_FIELD_NUMBER: _ClassVar[int]
    REDIRECTURL_FIELD_NUMBER: _ClassVar[int]
    TEAMSTOLOGINS_FIELD_NUMBER: _ClassVar[int]
    DISPLAY_FIELD_NUMBER: _ClassVar[int]
    TEAMSTOROLES_FIELD_NUMBER: _ClassVar[int]
    ENDPOINTURL_FIELD_NUMBER: _ClassVar[int]
    APIENDPOINTURL_FIELD_NUMBER: _ClassVar[int]
    ClientID: str
    ClientSecret: str
    RedirectURL: str
    TeamsToLogins: _containers.RepeatedCompositeFieldContainer[TeamMapping]
    Display: str
    TeamsToRoles: _containers.RepeatedCompositeFieldContainer[TeamRolesMapping]
    EndpointURL: str
    APIEndpointURL: str
    def __init__(self, ClientID: _Optional[str] = ..., ClientSecret: _Optional[str] = ..., RedirectURL: _Optional[str] = ..., TeamsToLogins: _Optional[_Iterable[_Union[TeamMapping, _Mapping]]] = ..., Display: _Optional[str] = ..., TeamsToRoles: _Optional[_Iterable[_Union[TeamRolesMapping, _Mapping]]] = ..., EndpointURL: _Optional[str] = ..., APIEndpointURL: _Optional[str] = ...) -> None: ...

class GithubAuthRequest(_message.Message):
    __slots__ = ["ConnectorID", "Type", "StateToken", "CSRFToken", "PublicKey", "CertTTL", "CreateWebSession", "RedirectURL", "ClientRedirectURL", "Compatibility", "Expires", "RouteToCluster", "KubernetesCluster", "SSOTestFlow", "ConnectorSpec", "attestation_statement", "ClientLoginIP"]
    CONNECTORID_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    STATETOKEN_FIELD_NUMBER: _ClassVar[int]
    CSRFTOKEN_FIELD_NUMBER: _ClassVar[int]
    PUBLICKEY_FIELD_NUMBER: _ClassVar[int]
    CERTTTL_FIELD_NUMBER: _ClassVar[int]
    CREATEWEBSESSION_FIELD_NUMBER: _ClassVar[int]
    REDIRECTURL_FIELD_NUMBER: _ClassVar[int]
    CLIENTREDIRECTURL_FIELD_NUMBER: _ClassVar[int]
    COMPATIBILITY_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    ROUTETOCLUSTER_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESCLUSTER_FIELD_NUMBER: _ClassVar[int]
    SSOTESTFLOW_FIELD_NUMBER: _ClassVar[int]
    CONNECTORSPEC_FIELD_NUMBER: _ClassVar[int]
    ATTESTATION_STATEMENT_FIELD_NUMBER: _ClassVar[int]
    CLIENTLOGINIP_FIELD_NUMBER: _ClassVar[int]
    ConnectorID: str
    Type: str
    StateToken: str
    CSRFToken: str
    PublicKey: bytes
    CertTTL: int
    CreateWebSession: bool
    RedirectURL: str
    ClientRedirectURL: str
    Compatibility: str
    Expires: _timestamp_pb2.Timestamp
    RouteToCluster: str
    KubernetesCluster: str
    SSOTestFlow: bool
    ConnectorSpec: GithubConnectorSpecV3
    attestation_statement: _attestation_pb2.AttestationStatement
    ClientLoginIP: str
    def __init__(self, ConnectorID: _Optional[str] = ..., Type: _Optional[str] = ..., StateToken: _Optional[str] = ..., CSRFToken: _Optional[str] = ..., PublicKey: _Optional[bytes] = ..., CertTTL: _Optional[int] = ..., CreateWebSession: bool = ..., RedirectURL: _Optional[str] = ..., ClientRedirectURL: _Optional[str] = ..., Compatibility: _Optional[str] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., RouteToCluster: _Optional[str] = ..., KubernetesCluster: _Optional[str] = ..., SSOTestFlow: bool = ..., ConnectorSpec: _Optional[_Union[GithubConnectorSpecV3, _Mapping]] = ..., attestation_statement: _Optional[_Union[_attestation_pb2.AttestationStatement, _Mapping]] = ..., ClientLoginIP: _Optional[str] = ...) -> None: ...

class SSOWarnings(_message.Message):
    __slots__ = ["Message", "Warnings"]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    WARNINGS_FIELD_NUMBER: _ClassVar[int]
    Message: str
    Warnings: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Message: _Optional[str] = ..., Warnings: _Optional[_Iterable[str]] = ...) -> None: ...

class CreateUserParams(_message.Message):
    __slots__ = ["ConnectorName", "Username", "Logins", "KubeGroups", "KubeUsers", "Roles", "Traits", "SessionTTL"]
    CONNECTORNAME_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    LOGINS_FIELD_NUMBER: _ClassVar[int]
    KUBEGROUPS_FIELD_NUMBER: _ClassVar[int]
    KUBEUSERS_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    TRAITS_FIELD_NUMBER: _ClassVar[int]
    SESSIONTTL_FIELD_NUMBER: _ClassVar[int]
    ConnectorName: str
    Username: str
    Logins: _containers.RepeatedScalarFieldContainer[str]
    KubeGroups: _containers.RepeatedScalarFieldContainer[str]
    KubeUsers: _containers.RepeatedScalarFieldContainer[str]
    Roles: _containers.RepeatedScalarFieldContainer[str]
    Traits: _wrappers_pb2.LabelValues
    SessionTTL: int
    def __init__(self, ConnectorName: _Optional[str] = ..., Username: _Optional[str] = ..., Logins: _Optional[_Iterable[str]] = ..., KubeGroups: _Optional[_Iterable[str]] = ..., KubeUsers: _Optional[_Iterable[str]] = ..., Roles: _Optional[_Iterable[str]] = ..., Traits: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., SessionTTL: _Optional[int] = ...) -> None: ...

class SSODiagnosticInfo(_message.Message):
    __slots__ = ["TestFlow", "Error", "Success", "CreateUserParams", "SAMLAttributesToRoles", "SAMLAttributesToRolesWarnings", "SAMLAttributeStatements", "SAMLAssertionInfo", "SAMLTraitsFromAssertions", "SAMLConnectorTraitMapping", "OIDCClaimsToRoles", "OIDCClaimsToRolesWarnings", "OIDCClaims", "OIDCIdentity", "OIDCTraitsFromClaims", "OIDCConnectorTraitMapping", "GithubClaims", "GithubTeamsToLogins", "GithubTeamsToRoles", "GithubTokenInfo", "AppliedLoginRules"]
    TESTFLOW_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    CREATEUSERPARAMS_FIELD_NUMBER: _ClassVar[int]
    SAMLATTRIBUTESTOROLES_FIELD_NUMBER: _ClassVar[int]
    SAMLATTRIBUTESTOROLESWARNINGS_FIELD_NUMBER: _ClassVar[int]
    SAMLATTRIBUTESTATEMENTS_FIELD_NUMBER: _ClassVar[int]
    SAMLASSERTIONINFO_FIELD_NUMBER: _ClassVar[int]
    SAMLTRAITSFROMASSERTIONS_FIELD_NUMBER: _ClassVar[int]
    SAMLCONNECTORTRAITMAPPING_FIELD_NUMBER: _ClassVar[int]
    OIDCCLAIMSTOROLES_FIELD_NUMBER: _ClassVar[int]
    OIDCCLAIMSTOROLESWARNINGS_FIELD_NUMBER: _ClassVar[int]
    OIDCCLAIMS_FIELD_NUMBER: _ClassVar[int]
    OIDCIDENTITY_FIELD_NUMBER: _ClassVar[int]
    OIDCTRAITSFROMCLAIMS_FIELD_NUMBER: _ClassVar[int]
    OIDCCONNECTORTRAITMAPPING_FIELD_NUMBER: _ClassVar[int]
    GITHUBCLAIMS_FIELD_NUMBER: _ClassVar[int]
    GITHUBTEAMSTOLOGINS_FIELD_NUMBER: _ClassVar[int]
    GITHUBTEAMSTOROLES_FIELD_NUMBER: _ClassVar[int]
    GITHUBTOKENINFO_FIELD_NUMBER: _ClassVar[int]
    APPLIEDLOGINRULES_FIELD_NUMBER: _ClassVar[int]
    TestFlow: bool
    Error: str
    Success: bool
    CreateUserParams: CreateUserParams
    SAMLAttributesToRoles: _containers.RepeatedCompositeFieldContainer[AttributeMapping]
    SAMLAttributesToRolesWarnings: SSOWarnings
    SAMLAttributeStatements: _wrappers_pb2.LabelValues
    SAMLAssertionInfo: _wrappers_pb2.CustomType
    SAMLTraitsFromAssertions: _wrappers_pb2.LabelValues
    SAMLConnectorTraitMapping: _containers.RepeatedCompositeFieldContainer[TraitMapping]
    OIDCClaimsToRoles: _containers.RepeatedCompositeFieldContainer[ClaimMapping]
    OIDCClaimsToRolesWarnings: SSOWarnings
    OIDCClaims: _wrappers_pb2.CustomType
    OIDCIdentity: _wrappers_pb2.CustomType
    OIDCTraitsFromClaims: _wrappers_pb2.LabelValues
    OIDCConnectorTraitMapping: _containers.RepeatedCompositeFieldContainer[TraitMapping]
    GithubClaims: GithubClaims
    GithubTeamsToLogins: _containers.RepeatedCompositeFieldContainer[TeamMapping]
    GithubTeamsToRoles: _containers.RepeatedCompositeFieldContainer[TeamRolesMapping]
    GithubTokenInfo: GithubTokenInfo
    AppliedLoginRules: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, TestFlow: bool = ..., Error: _Optional[str] = ..., Success: bool = ..., CreateUserParams: _Optional[_Union[CreateUserParams, _Mapping]] = ..., SAMLAttributesToRoles: _Optional[_Iterable[_Union[AttributeMapping, _Mapping]]] = ..., SAMLAttributesToRolesWarnings: _Optional[_Union[SSOWarnings, _Mapping]] = ..., SAMLAttributeStatements: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., SAMLAssertionInfo: _Optional[_Union[_wrappers_pb2.CustomType, _Mapping]] = ..., SAMLTraitsFromAssertions: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., SAMLConnectorTraitMapping: _Optional[_Iterable[_Union[TraitMapping, _Mapping]]] = ..., OIDCClaimsToRoles: _Optional[_Iterable[_Union[ClaimMapping, _Mapping]]] = ..., OIDCClaimsToRolesWarnings: _Optional[_Union[SSOWarnings, _Mapping]] = ..., OIDCClaims: _Optional[_Union[_wrappers_pb2.CustomType, _Mapping]] = ..., OIDCIdentity: _Optional[_Union[_wrappers_pb2.CustomType, _Mapping]] = ..., OIDCTraitsFromClaims: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., OIDCConnectorTraitMapping: _Optional[_Iterable[_Union[TraitMapping, _Mapping]]] = ..., GithubClaims: _Optional[_Union[GithubClaims, _Mapping]] = ..., GithubTeamsToLogins: _Optional[_Iterable[_Union[TeamMapping, _Mapping]]] = ..., GithubTeamsToRoles: _Optional[_Iterable[_Union[TeamRolesMapping, _Mapping]]] = ..., GithubTokenInfo: _Optional[_Union[GithubTokenInfo, _Mapping]] = ..., AppliedLoginRules: _Optional[_Iterable[str]] = ...) -> None: ...

class GithubTokenInfo(_message.Message):
    __slots__ = ["TokenType", "Expires", "Scope"]
    TOKENTYPE_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    SCOPE_FIELD_NUMBER: _ClassVar[int]
    TokenType: str
    Expires: int
    Scope: str
    def __init__(self, TokenType: _Optional[str] = ..., Expires: _Optional[int] = ..., Scope: _Optional[str] = ...) -> None: ...

class GithubClaims(_message.Message):
    __slots__ = ["Username", "OrganizationToTeams", "Teams"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    ORGANIZATIONTOTEAMS_FIELD_NUMBER: _ClassVar[int]
    TEAMS_FIELD_NUMBER: _ClassVar[int]
    Username: str
    OrganizationToTeams: _wrappers_pb2.LabelValues
    Teams: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Username: _Optional[str] = ..., OrganizationToTeams: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., Teams: _Optional[_Iterable[str]] = ...) -> None: ...

class TeamMapping(_message.Message):
    __slots__ = ["Organization", "Team", "Logins", "KubeGroups", "KubeUsers"]
    ORGANIZATION_FIELD_NUMBER: _ClassVar[int]
    TEAM_FIELD_NUMBER: _ClassVar[int]
    LOGINS_FIELD_NUMBER: _ClassVar[int]
    KUBEGROUPS_FIELD_NUMBER: _ClassVar[int]
    KUBEUSERS_FIELD_NUMBER: _ClassVar[int]
    Organization: str
    Team: str
    Logins: _containers.RepeatedScalarFieldContainer[str]
    KubeGroups: _containers.RepeatedScalarFieldContainer[str]
    KubeUsers: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Organization: _Optional[str] = ..., Team: _Optional[str] = ..., Logins: _Optional[_Iterable[str]] = ..., KubeGroups: _Optional[_Iterable[str]] = ..., KubeUsers: _Optional[_Iterable[str]] = ...) -> None: ...

class TeamRolesMapping(_message.Message):
    __slots__ = ["Organization", "Team", "Roles"]
    ORGANIZATION_FIELD_NUMBER: _ClassVar[int]
    TEAM_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    Organization: str
    Team: str
    Roles: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Organization: _Optional[str] = ..., Team: _Optional[str] = ..., Roles: _Optional[_Iterable[str]] = ...) -> None: ...

class TrustedClusterV2(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: TrustedClusterSpecV2
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[TrustedClusterSpecV2, _Mapping]] = ...) -> None: ...

class TrustedClusterV2List(_message.Message):
    __slots__ = ["TrustedClusters"]
    TRUSTEDCLUSTERS_FIELD_NUMBER: _ClassVar[int]
    TrustedClusters: _containers.RepeatedCompositeFieldContainer[TrustedClusterV2]
    def __init__(self, TrustedClusters: _Optional[_Iterable[_Union[TrustedClusterV2, _Mapping]]] = ...) -> None: ...

class TrustedClusterSpecV2(_message.Message):
    __slots__ = ["Enabled", "Roles", "Token", "ProxyAddress", "ReverseTunnelAddress", "RoleMap"]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    PROXYADDRESS_FIELD_NUMBER: _ClassVar[int]
    REVERSETUNNELADDRESS_FIELD_NUMBER: _ClassVar[int]
    ROLEMAP_FIELD_NUMBER: _ClassVar[int]
    Enabled: bool
    Roles: _containers.RepeatedScalarFieldContainer[str]
    Token: str
    ProxyAddress: str
    ReverseTunnelAddress: str
    RoleMap: _containers.RepeatedCompositeFieldContainer[RoleMapping]
    def __init__(self, Enabled: bool = ..., Roles: _Optional[_Iterable[str]] = ..., Token: _Optional[str] = ..., ProxyAddress: _Optional[str] = ..., ReverseTunnelAddress: _Optional[str] = ..., RoleMap: _Optional[_Iterable[_Union[RoleMapping, _Mapping]]] = ...) -> None: ...

class LockV2(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: LockSpecV2
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[LockSpecV2, _Mapping]] = ...) -> None: ...

class LockSpecV2(_message.Message):
    __slots__ = ["Target", "Message", "Expires", "CreatedAt", "CreatedBy"]
    TARGET_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    CREATEDAT_FIELD_NUMBER: _ClassVar[int]
    CREATEDBY_FIELD_NUMBER: _ClassVar[int]
    Target: LockTarget
    Message: str
    Expires: _timestamp_pb2.Timestamp
    CreatedAt: _timestamp_pb2.Timestamp
    CreatedBy: str
    def __init__(self, Target: _Optional[_Union[LockTarget, _Mapping]] = ..., Message: _Optional[str] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., CreatedAt: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., CreatedBy: _Optional[str] = ...) -> None: ...

class LockTarget(_message.Message):
    __slots__ = ["User", "Role", "Login", "Node", "MFADevice", "WindowsDesktop", "AccessRequest", "Device", "ServerID"]
    USER_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    LOGIN_FIELD_NUMBER: _ClassVar[int]
    NODE_FIELD_NUMBER: _ClassVar[int]
    MFADEVICE_FIELD_NUMBER: _ClassVar[int]
    WINDOWSDESKTOP_FIELD_NUMBER: _ClassVar[int]
    ACCESSREQUEST_FIELD_NUMBER: _ClassVar[int]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    SERVERID_FIELD_NUMBER: _ClassVar[int]
    User: str
    Role: str
    Login: str
    Node: str
    MFADevice: str
    WindowsDesktop: str
    AccessRequest: str
    Device: str
    ServerID: str
    def __init__(self, User: _Optional[str] = ..., Role: _Optional[str] = ..., Login: _Optional[str] = ..., Node: _Optional[str] = ..., MFADevice: _Optional[str] = ..., WindowsDesktop: _Optional[str] = ..., AccessRequest: _Optional[str] = ..., Device: _Optional[str] = ..., ServerID: _Optional[str] = ...) -> None: ...

class AddressCondition(_message.Message):
    __slots__ = ["CIDR"]
    CIDR_FIELD_NUMBER: _ClassVar[int]
    CIDR: str
    def __init__(self, CIDR: _Optional[str] = ...) -> None: ...

class NetworkRestrictionsSpecV4(_message.Message):
    __slots__ = ["Allow", "Deny"]
    ALLOW_FIELD_NUMBER: _ClassVar[int]
    DENY_FIELD_NUMBER: _ClassVar[int]
    Allow: _containers.RepeatedCompositeFieldContainer[AddressCondition]
    Deny: _containers.RepeatedCompositeFieldContainer[AddressCondition]
    def __init__(self, Allow: _Optional[_Iterable[_Union[AddressCondition, _Mapping]]] = ..., Deny: _Optional[_Iterable[_Union[AddressCondition, _Mapping]]] = ...) -> None: ...

class NetworkRestrictionsV4(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: NetworkRestrictionsSpecV4
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[NetworkRestrictionsSpecV4, _Mapping]] = ...) -> None: ...

class WindowsDesktopServiceV3(_message.Message):
    __slots__ = ["Header", "Spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Header: ResourceHeader
    Spec: WindowsDesktopServiceSpecV3
    def __init__(self, Header: _Optional[_Union[ResourceHeader, _Mapping]] = ..., Spec: _Optional[_Union[WindowsDesktopServiceSpecV3, _Mapping]] = ...) -> None: ...

class WindowsDesktopServiceSpecV3(_message.Message):
    __slots__ = ["Addr", "TeleportVersion", "Hostname", "ProxyIDs"]
    ADDR_FIELD_NUMBER: _ClassVar[int]
    TELEPORTVERSION_FIELD_NUMBER: _ClassVar[int]
    HOSTNAME_FIELD_NUMBER: _ClassVar[int]
    PROXYIDS_FIELD_NUMBER: _ClassVar[int]
    Addr: str
    TeleportVersion: str
    Hostname: str
    ProxyIDs: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Addr: _Optional[str] = ..., TeleportVersion: _Optional[str] = ..., Hostname: _Optional[str] = ..., ProxyIDs: _Optional[_Iterable[str]] = ...) -> None: ...

class WindowsDesktopFilter(_message.Message):
    __slots__ = ["HostID", "Name"]
    HOSTID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    HostID: str
    Name: str
    def __init__(self, HostID: _Optional[str] = ..., Name: _Optional[str] = ...) -> None: ...

class WindowsDesktopV3(_message.Message):
    __slots__ = ["Header", "Spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Header: ResourceHeader
    Spec: WindowsDesktopSpecV3
    def __init__(self, Header: _Optional[_Union[ResourceHeader, _Mapping]] = ..., Spec: _Optional[_Union[WindowsDesktopSpecV3, _Mapping]] = ...) -> None: ...

class WindowsDesktopSpecV3(_message.Message):
    __slots__ = ["Addr", "Domain", "HostID", "NonAD"]
    ADDR_FIELD_NUMBER: _ClassVar[int]
    DOMAIN_FIELD_NUMBER: _ClassVar[int]
    HOSTID_FIELD_NUMBER: _ClassVar[int]
    NONAD_FIELD_NUMBER: _ClassVar[int]
    Addr: str
    Domain: str
    HostID: str
    NonAD: bool
    def __init__(self, Addr: _Optional[str] = ..., Domain: _Optional[str] = ..., HostID: _Optional[str] = ..., NonAD: bool = ...) -> None: ...

class RegisterUsingTokenRequest(_message.Message):
    __slots__ = ["HostID", "NodeName", "Role", "Token", "AdditionalPrincipals", "DNSNames", "PublicTLSKey", "PublicSSHKey", "RemoteAddr", "EC2IdentityDocument", "IDToken", "Expires"]
    HOSTID_FIELD_NUMBER: _ClassVar[int]
    NODENAME_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    ADDITIONALPRINCIPALS_FIELD_NUMBER: _ClassVar[int]
    DNSNAMES_FIELD_NUMBER: _ClassVar[int]
    PUBLICTLSKEY_FIELD_NUMBER: _ClassVar[int]
    PUBLICSSHKEY_FIELD_NUMBER: _ClassVar[int]
    REMOTEADDR_FIELD_NUMBER: _ClassVar[int]
    EC2IDENTITYDOCUMENT_FIELD_NUMBER: _ClassVar[int]
    IDTOKEN_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    HostID: str
    NodeName: str
    Role: str
    Token: str
    AdditionalPrincipals: _containers.RepeatedScalarFieldContainer[str]
    DNSNames: _containers.RepeatedScalarFieldContainer[str]
    PublicTLSKey: bytes
    PublicSSHKey: bytes
    RemoteAddr: str
    EC2IdentityDocument: bytes
    IDToken: str
    Expires: _timestamp_pb2.Timestamp
    def __init__(self, HostID: _Optional[str] = ..., NodeName: _Optional[str] = ..., Role: _Optional[str] = ..., Token: _Optional[str] = ..., AdditionalPrincipals: _Optional[_Iterable[str]] = ..., DNSNames: _Optional[_Iterable[str]] = ..., PublicTLSKey: _Optional[bytes] = ..., PublicSSHKey: _Optional[bytes] = ..., RemoteAddr: _Optional[str] = ..., EC2IdentityDocument: _Optional[bytes] = ..., IDToken: _Optional[str] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class RecoveryCodesV1(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: RecoveryCodesSpecV1
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[RecoveryCodesSpecV1, _Mapping]] = ...) -> None: ...

class RecoveryCodesSpecV1(_message.Message):
    __slots__ = ["Codes", "Created"]
    CODES_FIELD_NUMBER: _ClassVar[int]
    CREATED_FIELD_NUMBER: _ClassVar[int]
    Codes: _containers.RepeatedCompositeFieldContainer[RecoveryCode]
    Created: _timestamp_pb2.Timestamp
    def __init__(self, Codes: _Optional[_Iterable[_Union[RecoveryCode, _Mapping]]] = ..., Created: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class RecoveryCode(_message.Message):
    __slots__ = ["HashedCode", "IsUsed"]
    HASHEDCODE_FIELD_NUMBER: _ClassVar[int]
    ISUSED_FIELD_NUMBER: _ClassVar[int]
    HashedCode: bytes
    IsUsed: bool
    def __init__(self, HashedCode: _Optional[bytes] = ..., IsUsed: bool = ...) -> None: ...

class NullableSessionState(_message.Message):
    __slots__ = ["State"]
    STATE_FIELD_NUMBER: _ClassVar[int]
    State: SessionState
    def __init__(self, State: _Optional[_Union[SessionState, str]] = ...) -> None: ...

class SessionTrackerFilter(_message.Message):
    __slots__ = ["Kind", "State", "DesktopName"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    DESKTOPNAME_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    State: NullableSessionState
    DesktopName: str
    def __init__(self, Kind: _Optional[str] = ..., State: _Optional[_Union[NullableSessionState, _Mapping]] = ..., DesktopName: _Optional[str] = ...) -> None: ...

class SessionTrackerV1(_message.Message):
    __slots__ = ["Header", "Spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Header: ResourceHeader
    Spec: SessionTrackerSpecV1
    def __init__(self, Header: _Optional[_Union[ResourceHeader, _Mapping]] = ..., Spec: _Optional[_Union[SessionTrackerSpecV1, _Mapping]] = ...) -> None: ...

class SessionTrackerSpecV1(_message.Message):
    __slots__ = ["SessionID", "Kind", "State", "Created", "Expires", "AttachedData", "Reason", "Invited", "Hostname", "Address", "ClusterName", "Login", "Participants", "KubernetesCluster", "HostUser", "HostPolicies", "DatabaseName", "AppName", "AppSessionID", "DesktopName", "HostID", "TargetSubKind"]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    CREATED_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    ATTACHEDDATA_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    INVITED_FIELD_NUMBER: _ClassVar[int]
    HOSTNAME_FIELD_NUMBER: _ClassVar[int]
    ADDRESS_FIELD_NUMBER: _ClassVar[int]
    CLUSTERNAME_FIELD_NUMBER: _ClassVar[int]
    LOGIN_FIELD_NUMBER: _ClassVar[int]
    PARTICIPANTS_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESCLUSTER_FIELD_NUMBER: _ClassVar[int]
    HOSTUSER_FIELD_NUMBER: _ClassVar[int]
    HOSTPOLICIES_FIELD_NUMBER: _ClassVar[int]
    DATABASENAME_FIELD_NUMBER: _ClassVar[int]
    APPNAME_FIELD_NUMBER: _ClassVar[int]
    APPSESSIONID_FIELD_NUMBER: _ClassVar[int]
    DESKTOPNAME_FIELD_NUMBER: _ClassVar[int]
    HOSTID_FIELD_NUMBER: _ClassVar[int]
    TARGETSUBKIND_FIELD_NUMBER: _ClassVar[int]
    SessionID: str
    Kind: str
    State: SessionState
    Created: _timestamp_pb2.Timestamp
    Expires: _timestamp_pb2.Timestamp
    AttachedData: str
    Reason: str
    Invited: _containers.RepeatedScalarFieldContainer[str]
    Hostname: str
    Address: str
    ClusterName: str
    Login: str
    Participants: _containers.RepeatedCompositeFieldContainer[Participant]
    KubernetesCluster: str
    HostUser: str
    HostPolicies: _containers.RepeatedCompositeFieldContainer[SessionTrackerPolicySet]
    DatabaseName: str
    AppName: str
    AppSessionID: str
    DesktopName: str
    HostID: str
    TargetSubKind: str
    def __init__(self, SessionID: _Optional[str] = ..., Kind: _Optional[str] = ..., State: _Optional[_Union[SessionState, str]] = ..., Created: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., AttachedData: _Optional[str] = ..., Reason: _Optional[str] = ..., Invited: _Optional[_Iterable[str]] = ..., Hostname: _Optional[str] = ..., Address: _Optional[str] = ..., ClusterName: _Optional[str] = ..., Login: _Optional[str] = ..., Participants: _Optional[_Iterable[_Union[Participant, _Mapping]]] = ..., KubernetesCluster: _Optional[str] = ..., HostUser: _Optional[str] = ..., HostPolicies: _Optional[_Iterable[_Union[SessionTrackerPolicySet, _Mapping]]] = ..., DatabaseName: _Optional[str] = ..., AppName: _Optional[str] = ..., AppSessionID: _Optional[str] = ..., DesktopName: _Optional[str] = ..., HostID: _Optional[str] = ..., TargetSubKind: _Optional[str] = ...) -> None: ...

class SessionTrackerPolicySet(_message.Message):
    __slots__ = ["Name", "Version", "RequireSessionJoin"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    REQUIRESESSIONJOIN_FIELD_NUMBER: _ClassVar[int]
    Name: str
    Version: str
    RequireSessionJoin: _containers.RepeatedCompositeFieldContainer[SessionRequirePolicy]
    def __init__(self, Name: _Optional[str] = ..., Version: _Optional[str] = ..., RequireSessionJoin: _Optional[_Iterable[_Union[SessionRequirePolicy, _Mapping]]] = ...) -> None: ...

class Participant(_message.Message):
    __slots__ = ["ID", "User", "Mode", "LastActive"]
    ID_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    MODE_FIELD_NUMBER: _ClassVar[int]
    LASTACTIVE_FIELD_NUMBER: _ClassVar[int]
    ID: str
    User: str
    Mode: str
    LastActive: _timestamp_pb2.Timestamp
    def __init__(self, ID: _Optional[str] = ..., User: _Optional[str] = ..., Mode: _Optional[str] = ..., LastActive: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class UIConfigV1(_message.Message):
    __slots__ = ["Header", "Spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Header: ResourceHeader
    Spec: UIConfigSpecV1
    def __init__(self, Header: _Optional[_Union[ResourceHeader, _Mapping]] = ..., Spec: _Optional[_Union[UIConfigSpecV1, _Mapping]] = ...) -> None: ...

class UIConfigSpecV1(_message.Message):
    __slots__ = ["ScrollbackLines"]
    SCROLLBACKLINES_FIELD_NUMBER: _ClassVar[int]
    ScrollbackLines: int
    def __init__(self, ScrollbackLines: _Optional[int] = ...) -> None: ...

class InstallerV1(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: InstallerSpecV1
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[InstallerSpecV1, _Mapping]] = ...) -> None: ...

class InstallerSpecV1(_message.Message):
    __slots__ = ["Script"]
    SCRIPT_FIELD_NUMBER: _ClassVar[int]
    Script: str
    def __init__(self, Script: _Optional[str] = ...) -> None: ...

class InstallerV1List(_message.Message):
    __slots__ = ["installers"]
    INSTALLERS_FIELD_NUMBER: _ClassVar[int]
    installers: _containers.RepeatedCompositeFieldContainer[InstallerV1]
    def __init__(self, installers: _Optional[_Iterable[_Union[InstallerV1, _Mapping]]] = ...) -> None: ...

class SortBy(_message.Message):
    __slots__ = ["IsDesc", "Field"]
    ISDESC_FIELD_NUMBER: _ClassVar[int]
    FIELD_FIELD_NUMBER: _ClassVar[int]
    IsDesc: bool
    Field: str
    def __init__(self, IsDesc: bool = ..., Field: _Optional[str] = ...) -> None: ...

class ConnectionDiagnosticV1(_message.Message):
    __slots__ = ["Header", "Spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Header: ResourceHeader
    Spec: ConnectionDiagnosticSpecV1
    def __init__(self, Header: _Optional[_Union[ResourceHeader, _Mapping]] = ..., Spec: _Optional[_Union[ConnectionDiagnosticSpecV1, _Mapping]] = ...) -> None: ...

class ConnectionDiagnosticSpecV1(_message.Message):
    __slots__ = ["Success", "Message", "Traces"]
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    TRACES_FIELD_NUMBER: _ClassVar[int]
    Success: bool
    Message: str
    Traces: _containers.RepeatedCompositeFieldContainer[ConnectionDiagnosticTrace]
    def __init__(self, Success: bool = ..., Message: _Optional[str] = ..., Traces: _Optional[_Iterable[_Union[ConnectionDiagnosticTrace, _Mapping]]] = ...) -> None: ...

class ConnectionDiagnosticTrace(_message.Message):
    __slots__ = ["Type", "Status", "Details", "Error"]
    class TraceType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = []
        TRACE_TYPE_UNSPECIFIED: _ClassVar[ConnectionDiagnosticTrace.TraceType]
        UNKNOWN_ERROR: _ClassVar[ConnectionDiagnosticTrace.TraceType]
        RBAC_NODE: _ClassVar[ConnectionDiagnosticTrace.TraceType]
        CONNECTIVITY: _ClassVar[ConnectionDiagnosticTrace.TraceType]
        RBAC_PRINCIPAL: _ClassVar[ConnectionDiagnosticTrace.TraceType]
        NODE_PRINCIPAL: _ClassVar[ConnectionDiagnosticTrace.TraceType]
        RBAC_KUBE: _ClassVar[ConnectionDiagnosticTrace.TraceType]
        KUBE_PRINCIPAL: _ClassVar[ConnectionDiagnosticTrace.TraceType]
        RBAC_DATABASE: _ClassVar[ConnectionDiagnosticTrace.TraceType]
        RBAC_DATABASE_LOGIN: _ClassVar[ConnectionDiagnosticTrace.TraceType]
        DATABASE_DB_USER: _ClassVar[ConnectionDiagnosticTrace.TraceType]
        DATABASE_DB_NAME: _ClassVar[ConnectionDiagnosticTrace.TraceType]
    TRACE_TYPE_UNSPECIFIED: ConnectionDiagnosticTrace.TraceType
    UNKNOWN_ERROR: ConnectionDiagnosticTrace.TraceType
    RBAC_NODE: ConnectionDiagnosticTrace.TraceType
    CONNECTIVITY: ConnectionDiagnosticTrace.TraceType
    RBAC_PRINCIPAL: ConnectionDiagnosticTrace.TraceType
    NODE_PRINCIPAL: ConnectionDiagnosticTrace.TraceType
    RBAC_KUBE: ConnectionDiagnosticTrace.TraceType
    KUBE_PRINCIPAL: ConnectionDiagnosticTrace.TraceType
    RBAC_DATABASE: ConnectionDiagnosticTrace.TraceType
    RBAC_DATABASE_LOGIN: ConnectionDiagnosticTrace.TraceType
    DATABASE_DB_USER: ConnectionDiagnosticTrace.TraceType
    DATABASE_DB_NAME: ConnectionDiagnosticTrace.TraceType
    class StatusType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = []
        STATUS_UNSPECIFIED: _ClassVar[ConnectionDiagnosticTrace.StatusType]
        SUCCESS: _ClassVar[ConnectionDiagnosticTrace.StatusType]
        FAILED: _ClassVar[ConnectionDiagnosticTrace.StatusType]
    STATUS_UNSPECIFIED: ConnectionDiagnosticTrace.StatusType
    SUCCESS: ConnectionDiagnosticTrace.StatusType
    FAILED: ConnectionDiagnosticTrace.StatusType
    TYPE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    DETAILS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    Type: ConnectionDiagnosticTrace.TraceType
    Status: ConnectionDiagnosticTrace.StatusType
    Details: str
    Error: str
    def __init__(self, Type: _Optional[_Union[ConnectionDiagnosticTrace.TraceType, str]] = ..., Status: _Optional[_Union[ConnectionDiagnosticTrace.StatusType, str]] = ..., Details: _Optional[str] = ..., Error: _Optional[str] = ...) -> None: ...

class DatabaseServiceV1(_message.Message):
    __slots__ = ["Header", "Spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Header: ResourceHeader
    Spec: DatabaseServiceSpecV1
    def __init__(self, Header: _Optional[_Union[ResourceHeader, _Mapping]] = ..., Spec: _Optional[_Union[DatabaseServiceSpecV1, _Mapping]] = ...) -> None: ...

class DatabaseServiceSpecV1(_message.Message):
    __slots__ = ["ResourceMatchers"]
    RESOURCEMATCHERS_FIELD_NUMBER: _ClassVar[int]
    ResourceMatchers: _containers.RepeatedCompositeFieldContainer[DatabaseResourceMatcher]
    def __init__(self, ResourceMatchers: _Optional[_Iterable[_Union[DatabaseResourceMatcher, _Mapping]]] = ...) -> None: ...

class DatabaseResourceMatcher(_message.Message):
    __slots__ = ["Labels", "AWS"]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    AWS_FIELD_NUMBER: _ClassVar[int]
    Labels: _wrappers_pb2.LabelValues
    AWS: ResourceMatcherAWS
    def __init__(self, Labels: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., AWS: _Optional[_Union[ResourceMatcherAWS, _Mapping]] = ...) -> None: ...

class ResourceMatcherAWS(_message.Message):
    __slots__ = ["AssumeRoleARN", "ExternalID"]
    ASSUMEROLEARN_FIELD_NUMBER: _ClassVar[int]
    EXTERNALID_FIELD_NUMBER: _ClassVar[int]
    AssumeRoleARN: str
    ExternalID: str
    def __init__(self, AssumeRoleARN: _Optional[str] = ..., ExternalID: _Optional[str] = ...) -> None: ...

class ClusterAlert(_message.Message):
    __slots__ = ["Header", "Spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Header: ResourceHeader
    Spec: ClusterAlertSpec
    def __init__(self, Header: _Optional[_Union[ResourceHeader, _Mapping]] = ..., Spec: _Optional[_Union[ClusterAlertSpec, _Mapping]] = ...) -> None: ...

class ClusterAlertSpec(_message.Message):
    __slots__ = ["Severity", "Message", "Created"]
    SEVERITY_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    CREATED_FIELD_NUMBER: _ClassVar[int]
    Severity: AlertSeverity
    Message: str
    Created: _timestamp_pb2.Timestamp
    def __init__(self, Severity: _Optional[_Union[AlertSeverity, str]] = ..., Message: _Optional[str] = ..., Created: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetClusterAlertsRequest(_message.Message):
    __slots__ = ["Severity", "AlertID", "Labels", "WithSuperseded", "WithAcknowledged", "WithUntargeted"]
    class LabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    SEVERITY_FIELD_NUMBER: _ClassVar[int]
    ALERTID_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    WITHSUPERSEDED_FIELD_NUMBER: _ClassVar[int]
    WITHACKNOWLEDGED_FIELD_NUMBER: _ClassVar[int]
    WITHUNTARGETED_FIELD_NUMBER: _ClassVar[int]
    Severity: AlertSeverity
    AlertID: str
    Labels: _containers.ScalarMap[str, str]
    WithSuperseded: bool
    WithAcknowledged: bool
    WithUntargeted: bool
    def __init__(self, Severity: _Optional[_Union[AlertSeverity, str]] = ..., AlertID: _Optional[str] = ..., Labels: _Optional[_Mapping[str, str]] = ..., WithSuperseded: bool = ..., WithAcknowledged: bool = ..., WithUntargeted: bool = ...) -> None: ...

class AlertAcknowledgement(_message.Message):
    __slots__ = ["AlertID", "Reason", "Expires"]
    ALERTID_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    AlertID: str
    Reason: str
    Expires: _timestamp_pb2.Timestamp
    def __init__(self, AlertID: _Optional[str] = ..., Reason: _Optional[str] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class Release(_message.Message):
    __slots__ = ["NotesMD", "Product", "ReleaseID", "Status", "Version", "Assets"]
    NOTESMD_FIELD_NUMBER: _ClassVar[int]
    PRODUCT_FIELD_NUMBER: _ClassVar[int]
    RELEASEID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    ASSETS_FIELD_NUMBER: _ClassVar[int]
    NotesMD: str
    Product: str
    ReleaseID: str
    Status: str
    Version: str
    Assets: _containers.RepeatedCompositeFieldContainer[Asset]
    def __init__(self, NotesMD: _Optional[str] = ..., Product: _Optional[str] = ..., ReleaseID: _Optional[str] = ..., Status: _Optional[str] = ..., Version: _Optional[str] = ..., Assets: _Optional[_Iterable[_Union[Asset, _Mapping]]] = ...) -> None: ...

class Asset(_message.Message):
    __slots__ = ["Arch", "Description", "Name", "OS", "SHA256", "AssetSize", "DisplaySize", "ReleaseIDs", "PublicURL"]
    ARCH_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    OS_FIELD_NUMBER: _ClassVar[int]
    SHA256_FIELD_NUMBER: _ClassVar[int]
    ASSETSIZE_FIELD_NUMBER: _ClassVar[int]
    DISPLAYSIZE_FIELD_NUMBER: _ClassVar[int]
    RELEASEIDS_FIELD_NUMBER: _ClassVar[int]
    PUBLICURL_FIELD_NUMBER: _ClassVar[int]
    Arch: str
    Description: str
    Name: str
    OS: str
    SHA256: str
    AssetSize: int
    DisplaySize: str
    ReleaseIDs: _containers.RepeatedScalarFieldContainer[str]
    PublicURL: str
    def __init__(self, Arch: _Optional[str] = ..., Description: _Optional[str] = ..., Name: _Optional[str] = ..., OS: _Optional[str] = ..., SHA256: _Optional[str] = ..., AssetSize: _Optional[int] = ..., DisplaySize: _Optional[str] = ..., ReleaseIDs: _Optional[_Iterable[str]] = ..., PublicURL: _Optional[str] = ...) -> None: ...

class PluginV1(_message.Message):
    __slots__ = ["kind", "sub_kind", "version", "metadata", "spec", "status", "credentials"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUB_KIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    CREDENTIALS_FIELD_NUMBER: _ClassVar[int]
    kind: str
    sub_kind: str
    version: str
    metadata: Metadata
    spec: PluginSpecV1
    status: PluginStatusV1
    credentials: PluginCredentialsV1
    def __init__(self, kind: _Optional[str] = ..., sub_kind: _Optional[str] = ..., version: _Optional[str] = ..., metadata: _Optional[_Union[Metadata, _Mapping]] = ..., spec: _Optional[_Union[PluginSpecV1, _Mapping]] = ..., status: _Optional[_Union[PluginStatusV1, _Mapping]] = ..., credentials: _Optional[_Union[PluginCredentialsV1, _Mapping]] = ...) -> None: ...

class PluginSpecV1(_message.Message):
    __slots__ = ["slack_access_plugin", "opsgenie", "openai", "okta", "jamf", "pager_duty", "mattermost", "jira", "discord", "serviceNow"]
    SLACK_ACCESS_PLUGIN_FIELD_NUMBER: _ClassVar[int]
    OPSGENIE_FIELD_NUMBER: _ClassVar[int]
    OPENAI_FIELD_NUMBER: _ClassVar[int]
    OKTA_FIELD_NUMBER: _ClassVar[int]
    JAMF_FIELD_NUMBER: _ClassVar[int]
    PAGER_DUTY_FIELD_NUMBER: _ClassVar[int]
    MATTERMOST_FIELD_NUMBER: _ClassVar[int]
    JIRA_FIELD_NUMBER: _ClassVar[int]
    DISCORD_FIELD_NUMBER: _ClassVar[int]
    SERVICENOW_FIELD_NUMBER: _ClassVar[int]
    slack_access_plugin: PluginSlackAccessSettings
    opsgenie: PluginOpsgenieAccessSettings
    openai: PluginOpenAISettings
    okta: PluginOktaSettings
    jamf: PluginJamfSettings
    pager_duty: PluginPagerDutySettings
    mattermost: PluginMattermostSettings
    jira: PluginJiraSettings
    discord: PluginDiscordSettings
    serviceNow: PluginServiceNowSettings
    def __init__(self, slack_access_plugin: _Optional[_Union[PluginSlackAccessSettings, _Mapping]] = ..., opsgenie: _Optional[_Union[PluginOpsgenieAccessSettings, _Mapping]] = ..., openai: _Optional[_Union[PluginOpenAISettings, _Mapping]] = ..., okta: _Optional[_Union[PluginOktaSettings, _Mapping]] = ..., jamf: _Optional[_Union[PluginJamfSettings, _Mapping]] = ..., pager_duty: _Optional[_Union[PluginPagerDutySettings, _Mapping]] = ..., mattermost: _Optional[_Union[PluginMattermostSettings, _Mapping]] = ..., jira: _Optional[_Union[PluginJiraSettings, _Mapping]] = ..., discord: _Optional[_Union[PluginDiscordSettings, _Mapping]] = ..., serviceNow: _Optional[_Union[PluginServiceNowSettings, _Mapping]] = ...) -> None: ...

class PluginSlackAccessSettings(_message.Message):
    __slots__ = ["fallback_channel"]
    FALLBACK_CHANNEL_FIELD_NUMBER: _ClassVar[int]
    fallback_channel: str
    def __init__(self, fallback_channel: _Optional[str] = ...) -> None: ...

class PluginOpsgenieAccessSettings(_message.Message):
    __slots__ = ["priority", "alert_tags", "default_schedules", "api_endpoint"]
    PRIORITY_FIELD_NUMBER: _ClassVar[int]
    ALERT_TAGS_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_SCHEDULES_FIELD_NUMBER: _ClassVar[int]
    API_ENDPOINT_FIELD_NUMBER: _ClassVar[int]
    priority: str
    alert_tags: _containers.RepeatedScalarFieldContainer[str]
    default_schedules: _containers.RepeatedScalarFieldContainer[str]
    api_endpoint: str
    def __init__(self, priority: _Optional[str] = ..., alert_tags: _Optional[_Iterable[str]] = ..., default_schedules: _Optional[_Iterable[str]] = ..., api_endpoint: _Optional[str] = ...) -> None: ...

class PluginServiceNowSettings(_message.Message):
    __slots__ = ["api_endpoint", "username", "password", "close_code"]
    API_ENDPOINT_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    CLOSE_CODE_FIELD_NUMBER: _ClassVar[int]
    api_endpoint: str
    username: str
    password: str
    close_code: str
    def __init__(self, api_endpoint: _Optional[str] = ..., username: _Optional[str] = ..., password: _Optional[str] = ..., close_code: _Optional[str] = ...) -> None: ...

class PluginPagerDutySettings(_message.Message):
    __slots__ = ["user_email", "api_endpoint"]
    USER_EMAIL_FIELD_NUMBER: _ClassVar[int]
    API_ENDPOINT_FIELD_NUMBER: _ClassVar[int]
    user_email: str
    api_endpoint: str
    def __init__(self, user_email: _Optional[str] = ..., api_endpoint: _Optional[str] = ...) -> None: ...

class PluginJiraSettings(_message.Message):
    __slots__ = ["server_url", "project_key", "issue_type"]
    SERVER_URL_FIELD_NUMBER: _ClassVar[int]
    PROJECT_KEY_FIELD_NUMBER: _ClassVar[int]
    ISSUE_TYPE_FIELD_NUMBER: _ClassVar[int]
    server_url: str
    project_key: str
    issue_type: str
    def __init__(self, server_url: _Optional[str] = ..., project_key: _Optional[str] = ..., issue_type: _Optional[str] = ...) -> None: ...

class PluginOpenAISettings(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class PluginMattermostSettings(_message.Message):
    __slots__ = ["server_url", "team", "channel", "report_to_email"]
    SERVER_URL_FIELD_NUMBER: _ClassVar[int]
    TEAM_FIELD_NUMBER: _ClassVar[int]
    CHANNEL_FIELD_NUMBER: _ClassVar[int]
    REPORT_TO_EMAIL_FIELD_NUMBER: _ClassVar[int]
    server_url: str
    team: str
    channel: str
    report_to_email: str
    def __init__(self, server_url: _Optional[str] = ..., team: _Optional[str] = ..., channel: _Optional[str] = ..., report_to_email: _Optional[str] = ...) -> None: ...

class PluginJamfSettings(_message.Message):
    __slots__ = ["jamf_spec"]
    JAMF_SPEC_FIELD_NUMBER: _ClassVar[int]
    jamf_spec: JamfSpecV1
    def __init__(self, jamf_spec: _Optional[_Union[JamfSpecV1, _Mapping]] = ...) -> None: ...

class PluginOktaSettings(_message.Message):
    __slots__ = ["org_url"]
    ORG_URL_FIELD_NUMBER: _ClassVar[int]
    org_url: str
    def __init__(self, org_url: _Optional[str] = ...) -> None: ...

class DiscordChannels(_message.Message):
    __slots__ = ["channel_ids"]
    CHANNEL_IDS_FIELD_NUMBER: _ClassVar[int]
    channel_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, channel_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class PluginDiscordSettings(_message.Message):
    __slots__ = ["role_to_recipients"]
    class RoleToRecipientsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: DiscordChannels
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[DiscordChannels, _Mapping]] = ...) -> None: ...
    ROLE_TO_RECIPIENTS_FIELD_NUMBER: _ClassVar[int]
    role_to_recipients: _containers.MessageMap[str, DiscordChannels]
    def __init__(self, role_to_recipients: _Optional[_Mapping[str, DiscordChannels]] = ...) -> None: ...

class PluginBootstrapCredentialsV1(_message.Message):
    __slots__ = ["oauth2_authorization_code", "bearer_token", "id_secret"]
    OAUTH2_AUTHORIZATION_CODE_FIELD_NUMBER: _ClassVar[int]
    BEARER_TOKEN_FIELD_NUMBER: _ClassVar[int]
    ID_SECRET_FIELD_NUMBER: _ClassVar[int]
    oauth2_authorization_code: PluginOAuth2AuthorizationCodeCredentials
    bearer_token: PluginBearerTokenCredentials
    id_secret: PluginIdSecretCredential
    def __init__(self, oauth2_authorization_code: _Optional[_Union[PluginOAuth2AuthorizationCodeCredentials, _Mapping]] = ..., bearer_token: _Optional[_Union[PluginBearerTokenCredentials, _Mapping]] = ..., id_secret: _Optional[_Union[PluginIdSecretCredential, _Mapping]] = ...) -> None: ...

class PluginIdSecretCredential(_message.Message):
    __slots__ = ["id", "secret"]
    ID_FIELD_NUMBER: _ClassVar[int]
    SECRET_FIELD_NUMBER: _ClassVar[int]
    id: str
    secret: str
    def __init__(self, id: _Optional[str] = ..., secret: _Optional[str] = ...) -> None: ...

class PluginOAuth2AuthorizationCodeCredentials(_message.Message):
    __slots__ = ["authorization_code", "redirect_uri"]
    AUTHORIZATION_CODE_FIELD_NUMBER: _ClassVar[int]
    REDIRECT_URI_FIELD_NUMBER: _ClassVar[int]
    authorization_code: str
    redirect_uri: str
    def __init__(self, authorization_code: _Optional[str] = ..., redirect_uri: _Optional[str] = ...) -> None: ...

class PluginStatusV1(_message.Message):
    __slots__ = ["code"]
    CODE_FIELD_NUMBER: _ClassVar[int]
    code: PluginStatusCode
    def __init__(self, code: _Optional[_Union[PluginStatusCode, str]] = ...) -> None: ...

class PluginCredentialsV1(_message.Message):
    __slots__ = ["oauth2_access_token", "bearer_token", "id_secret", "static_credentials_ref"]
    OAUTH2_ACCESS_TOKEN_FIELD_NUMBER: _ClassVar[int]
    BEARER_TOKEN_FIELD_NUMBER: _ClassVar[int]
    ID_SECRET_FIELD_NUMBER: _ClassVar[int]
    STATIC_CREDENTIALS_REF_FIELD_NUMBER: _ClassVar[int]
    oauth2_access_token: PluginOAuth2AccessTokenCredentials
    bearer_token: PluginBearerTokenCredentials
    id_secret: PluginIdSecretCredential
    static_credentials_ref: PluginStaticCredentialsRef
    def __init__(self, oauth2_access_token: _Optional[_Union[PluginOAuth2AccessTokenCredentials, _Mapping]] = ..., bearer_token: _Optional[_Union[PluginBearerTokenCredentials, _Mapping]] = ..., id_secret: _Optional[_Union[PluginIdSecretCredential, _Mapping]] = ..., static_credentials_ref: _Optional[_Union[PluginStaticCredentialsRef, _Mapping]] = ...) -> None: ...

class PluginOAuth2AccessTokenCredentials(_message.Message):
    __slots__ = ["access_token", "refresh_token", "expires"]
    ACCESS_TOKEN_FIELD_NUMBER: _ClassVar[int]
    REFRESH_TOKEN_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    access_token: str
    refresh_token: str
    expires: _timestamp_pb2.Timestamp
    def __init__(self, access_token: _Optional[str] = ..., refresh_token: _Optional[str] = ..., expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class PluginBearerTokenCredentials(_message.Message):
    __slots__ = ["token"]
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    token: str
    def __init__(self, token: _Optional[str] = ...) -> None: ...

class PluginStaticCredentialsRef(_message.Message):
    __slots__ = ["Labels"]
    class LabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    LABELS_FIELD_NUMBER: _ClassVar[int]
    Labels: _containers.ScalarMap[str, str]
    def __init__(self, Labels: _Optional[_Mapping[str, str]] = ...) -> None: ...

class PluginListV1(_message.Message):
    __slots__ = ["plugins"]
    PLUGINS_FIELD_NUMBER: _ClassVar[int]
    plugins: _containers.RepeatedCompositeFieldContainer[PluginV1]
    def __init__(self, plugins: _Optional[_Iterable[_Union[PluginV1, _Mapping]]] = ...) -> None: ...

class PluginStaticCredentialsV1(_message.Message):
    __slots__ = ["Header", "Spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Header: ResourceHeader
    Spec: PluginStaticCredentialsSpecV1
    def __init__(self, Header: _Optional[_Union[ResourceHeader, _Mapping]] = ..., Spec: _Optional[_Union[PluginStaticCredentialsSpecV1, _Mapping]] = ...) -> None: ...

class PluginStaticCredentialsSpecV1(_message.Message):
    __slots__ = ["APIToken", "BasicAuth", "OAuthClientSecret"]
    APITOKEN_FIELD_NUMBER: _ClassVar[int]
    BASICAUTH_FIELD_NUMBER: _ClassVar[int]
    OAUTHCLIENTSECRET_FIELD_NUMBER: _ClassVar[int]
    APIToken: str
    BasicAuth: PluginStaticCredentialsBasicAuth
    OAuthClientSecret: PluginStaticCredentialsOAuthClientSecret
    def __init__(self, APIToken: _Optional[str] = ..., BasicAuth: _Optional[_Union[PluginStaticCredentialsBasicAuth, _Mapping]] = ..., OAuthClientSecret: _Optional[_Union[PluginStaticCredentialsOAuthClientSecret, _Mapping]] = ...) -> None: ...

class PluginStaticCredentialsBasicAuth(_message.Message):
    __slots__ = ["Username", "Password"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    Username: str
    Password: str
    def __init__(self, Username: _Optional[str] = ..., Password: _Optional[str] = ...) -> None: ...

class PluginStaticCredentialsOAuthClientSecret(_message.Message):
    __slots__ = ["ClientId", "ClientSecret"]
    CLIENTID_FIELD_NUMBER: _ClassVar[int]
    CLIENTSECRET_FIELD_NUMBER: _ClassVar[int]
    ClientId: str
    ClientSecret: str
    def __init__(self, ClientId: _Optional[str] = ..., ClientSecret: _Optional[str] = ...) -> None: ...

class SAMLIdPServiceProviderV1(_message.Message):
    __slots__ = ["Header", "Spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Header: ResourceHeader
    Spec: SAMLIdPServiceProviderSpecV1
    def __init__(self, Header: _Optional[_Union[ResourceHeader, _Mapping]] = ..., Spec: _Optional[_Union[SAMLIdPServiceProviderSpecV1, _Mapping]] = ...) -> None: ...

class SAMLIdPServiceProviderSpecV1(_message.Message):
    __slots__ = ["EntityDescriptor", "EntityID"]
    ENTITYDESCRIPTOR_FIELD_NUMBER: _ClassVar[int]
    ENTITYID_FIELD_NUMBER: _ClassVar[int]
    EntityDescriptor: str
    EntityID: str
    def __init__(self, EntityDescriptor: _Optional[str] = ..., EntityID: _Optional[str] = ...) -> None: ...

class IdPOptions(_message.Message):
    __slots__ = ["SAML"]
    SAML_FIELD_NUMBER: _ClassVar[int]
    SAML: IdPSAMLOptions
    def __init__(self, SAML: _Optional[_Union[IdPSAMLOptions, _Mapping]] = ...) -> None: ...

class IdPSAMLOptions(_message.Message):
    __slots__ = ["Enabled"]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    Enabled: BoolValue
    def __init__(self, Enabled: _Optional[_Union[BoolValue, _Mapping]] = ...) -> None: ...

class KubernetesResourceV1(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: KubernetesResourceSpecV1
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[KubernetesResourceSpecV1, _Mapping]] = ...) -> None: ...

class KubernetesResourceSpecV1(_message.Message):
    __slots__ = ["Namespace"]
    NAMESPACE_FIELD_NUMBER: _ClassVar[int]
    Namespace: str
    def __init__(self, Namespace: _Optional[str] = ...) -> None: ...

class ClusterMaintenanceConfigV1(_message.Message):
    __slots__ = ["Header", "Spec", "Nonce"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    NONCE_FIELD_NUMBER: _ClassVar[int]
    Header: ResourceHeader
    Spec: ClusterMaintenanceConfigSpecV1
    Nonce: int
    def __init__(self, Header: _Optional[_Union[ResourceHeader, _Mapping]] = ..., Spec: _Optional[_Union[ClusterMaintenanceConfigSpecV1, _Mapping]] = ..., Nonce: _Optional[int] = ...) -> None: ...

class ClusterMaintenanceConfigSpecV1(_message.Message):
    __slots__ = ["AgentUpgrades"]
    AGENTUPGRADES_FIELD_NUMBER: _ClassVar[int]
    AgentUpgrades: AgentUpgradeWindow
    def __init__(self, AgentUpgrades: _Optional[_Union[AgentUpgradeWindow, _Mapping]] = ...) -> None: ...

class AgentUpgradeWindow(_message.Message):
    __slots__ = ["UTCStartHour", "Weekdays"]
    UTCSTARTHOUR_FIELD_NUMBER: _ClassVar[int]
    WEEKDAYS_FIELD_NUMBER: _ClassVar[int]
    UTCStartHour: int
    Weekdays: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, UTCStartHour: _Optional[int] = ..., Weekdays: _Optional[_Iterable[str]] = ...) -> None: ...

class ScheduledAgentUpgradeWindow(_message.Message):
    __slots__ = ["Start", "Stop"]
    START_FIELD_NUMBER: _ClassVar[int]
    STOP_FIELD_NUMBER: _ClassVar[int]
    Start: _timestamp_pb2.Timestamp
    Stop: _timestamp_pb2.Timestamp
    def __init__(self, Start: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Stop: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class AgentUpgradeSchedule(_message.Message):
    __slots__ = ["Windows"]
    WINDOWS_FIELD_NUMBER: _ClassVar[int]
    Windows: _containers.RepeatedCompositeFieldContainer[ScheduledAgentUpgradeWindow]
    def __init__(self, Windows: _Optional[_Iterable[_Union[ScheduledAgentUpgradeWindow, _Mapping]]] = ...) -> None: ...

class UserGroupV1(_message.Message):
    __slots__ = ["Header", "Spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Header: ResourceHeader
    Spec: UserGroupSpecV1
    def __init__(self, Header: _Optional[_Union[ResourceHeader, _Mapping]] = ..., Spec: _Optional[_Union[UserGroupSpecV1, _Mapping]] = ...) -> None: ...

class UserGroupSpecV1(_message.Message):
    __slots__ = ["Applications"]
    APPLICATIONS_FIELD_NUMBER: _ClassVar[int]
    Applications: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Applications: _Optional[_Iterable[str]] = ...) -> None: ...

class OktaImportRuleSpecV1(_message.Message):
    __slots__ = ["Priority", "Mappings"]
    PRIORITY_FIELD_NUMBER: _ClassVar[int]
    MAPPINGS_FIELD_NUMBER: _ClassVar[int]
    Priority: int
    Mappings: _containers.RepeatedCompositeFieldContainer[OktaImportRuleMappingV1]
    def __init__(self, Priority: _Optional[int] = ..., Mappings: _Optional[_Iterable[_Union[OktaImportRuleMappingV1, _Mapping]]] = ...) -> None: ...

class OktaImportRuleMappingV1(_message.Message):
    __slots__ = ["Match", "AddLabels"]
    class AddLabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    MATCH_FIELD_NUMBER: _ClassVar[int]
    ADDLABELS_FIELD_NUMBER: _ClassVar[int]
    Match: _containers.RepeatedCompositeFieldContainer[OktaImportRuleMatchV1]
    AddLabels: _containers.ScalarMap[str, str]
    def __init__(self, Match: _Optional[_Iterable[_Union[OktaImportRuleMatchV1, _Mapping]]] = ..., AddLabels: _Optional[_Mapping[str, str]] = ...) -> None: ...

class OktaImportRuleV1(_message.Message):
    __slots__ = ["Header", "Spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Header: ResourceHeader
    Spec: OktaImportRuleSpecV1
    def __init__(self, Header: _Optional[_Union[ResourceHeader, _Mapping]] = ..., Spec: _Optional[_Union[OktaImportRuleSpecV1, _Mapping]] = ...) -> None: ...

class OktaImportRuleMatchV1(_message.Message):
    __slots__ = ["AppIDs", "GroupIDs", "AppNameRegexes", "GroupNameRegexes"]
    APPIDS_FIELD_NUMBER: _ClassVar[int]
    GROUPIDS_FIELD_NUMBER: _ClassVar[int]
    APPNAMEREGEXES_FIELD_NUMBER: _ClassVar[int]
    GROUPNAMEREGEXES_FIELD_NUMBER: _ClassVar[int]
    AppIDs: _containers.RepeatedScalarFieldContainer[str]
    GroupIDs: _containers.RepeatedScalarFieldContainer[str]
    AppNameRegexes: _containers.RepeatedScalarFieldContainer[str]
    GroupNameRegexes: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, AppIDs: _Optional[_Iterable[str]] = ..., GroupIDs: _Optional[_Iterable[str]] = ..., AppNameRegexes: _Optional[_Iterable[str]] = ..., GroupNameRegexes: _Optional[_Iterable[str]] = ...) -> None: ...

class OktaAssignmentV1(_message.Message):
    __slots__ = ["Header", "Spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Header: ResourceHeader
    Spec: OktaAssignmentSpecV1
    def __init__(self, Header: _Optional[_Union[ResourceHeader, _Mapping]] = ..., Spec: _Optional[_Union[OktaAssignmentSpecV1, _Mapping]] = ...) -> None: ...

class OktaAssignmentSpecV1(_message.Message):
    __slots__ = ["User", "Targets", "CleanupTime", "status", "LastTransition", "Finalized"]
    class OktaAssignmentStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = []
        UNKNOWN: _ClassVar[OktaAssignmentSpecV1.OktaAssignmentStatus]
        PENDING: _ClassVar[OktaAssignmentSpecV1.OktaAssignmentStatus]
        PROCESSING: _ClassVar[OktaAssignmentSpecV1.OktaAssignmentStatus]
        SUCCESSFUL: _ClassVar[OktaAssignmentSpecV1.OktaAssignmentStatus]
        FAILED: _ClassVar[OktaAssignmentSpecV1.OktaAssignmentStatus]
    UNKNOWN: OktaAssignmentSpecV1.OktaAssignmentStatus
    PENDING: OktaAssignmentSpecV1.OktaAssignmentStatus
    PROCESSING: OktaAssignmentSpecV1.OktaAssignmentStatus
    SUCCESSFUL: OktaAssignmentSpecV1.OktaAssignmentStatus
    FAILED: OktaAssignmentSpecV1.OktaAssignmentStatus
    USER_FIELD_NUMBER: _ClassVar[int]
    TARGETS_FIELD_NUMBER: _ClassVar[int]
    CLEANUPTIME_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    LASTTRANSITION_FIELD_NUMBER: _ClassVar[int]
    FINALIZED_FIELD_NUMBER: _ClassVar[int]
    User: str
    Targets: _containers.RepeatedCompositeFieldContainer[OktaAssignmentTargetV1]
    CleanupTime: _timestamp_pb2.Timestamp
    status: OktaAssignmentSpecV1.OktaAssignmentStatus
    LastTransition: _timestamp_pb2.Timestamp
    Finalized: bool
    def __init__(self, User: _Optional[str] = ..., Targets: _Optional[_Iterable[_Union[OktaAssignmentTargetV1, _Mapping]]] = ..., CleanupTime: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., status: _Optional[_Union[OktaAssignmentSpecV1.OktaAssignmentStatus, str]] = ..., LastTransition: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Finalized: bool = ...) -> None: ...

class OktaAssignmentTargetV1(_message.Message):
    __slots__ = ["type", "id"]
    class OktaAssignmentTargetType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = []
        UNKNOWN: _ClassVar[OktaAssignmentTargetV1.OktaAssignmentTargetType]
        APPLICATION: _ClassVar[OktaAssignmentTargetV1.OktaAssignmentTargetType]
        GROUP: _ClassVar[OktaAssignmentTargetV1.OktaAssignmentTargetType]
    UNKNOWN: OktaAssignmentTargetV1.OktaAssignmentTargetType
    APPLICATION: OktaAssignmentTargetV1.OktaAssignmentTargetType
    GROUP: OktaAssignmentTargetV1.OktaAssignmentTargetType
    TYPE_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    type: OktaAssignmentTargetV1.OktaAssignmentTargetType
    id: str
    def __init__(self, type: _Optional[_Union[OktaAssignmentTargetV1.OktaAssignmentTargetType, str]] = ..., id: _Optional[str] = ...) -> None: ...

class IntegrationV1(_message.Message):
    __slots__ = ["Header", "Spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Header: ResourceHeader
    Spec: IntegrationSpecV1
    def __init__(self, Header: _Optional[_Union[ResourceHeader, _Mapping]] = ..., Spec: _Optional[_Union[IntegrationSpecV1, _Mapping]] = ...) -> None: ...

class IntegrationSpecV1(_message.Message):
    __slots__ = ["AWSOIDC"]
    AWSOIDC_FIELD_NUMBER: _ClassVar[int]
    AWSOIDC: AWSOIDCIntegrationSpecV1
    def __init__(self, AWSOIDC: _Optional[_Union[AWSOIDCIntegrationSpecV1, _Mapping]] = ...) -> None: ...

class AWSOIDCIntegrationSpecV1(_message.Message):
    __slots__ = ["RoleARN"]
    ROLEARN_FIELD_NUMBER: _ClassVar[int]
    RoleARN: str
    def __init__(self, RoleARN: _Optional[str] = ...) -> None: ...

class HeadlessAuthentication(_message.Message):
    __slots__ = ["header", "user", "public_key", "state", "mfa_device", "client_ip_address"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    PUBLIC_KEY_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    MFA_DEVICE_FIELD_NUMBER: _ClassVar[int]
    CLIENT_IP_ADDRESS_FIELD_NUMBER: _ClassVar[int]
    header: ResourceHeader
    user: str
    public_key: bytes
    state: HeadlessAuthenticationState
    mfa_device: MFADevice
    client_ip_address: str
    def __init__(self, header: _Optional[_Union[ResourceHeader, _Mapping]] = ..., user: _Optional[str] = ..., public_key: _Optional[bytes] = ..., state: _Optional[_Union[HeadlessAuthenticationState, str]] = ..., mfa_device: _Optional[_Union[MFADevice, _Mapping]] = ..., client_ip_address: _Optional[str] = ...) -> None: ...

class WatchKind(_message.Message):
    __slots__ = ["Kind", "LoadSecrets", "Name", "Filter", "SubKind", "Version"]
    class FilterEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    KIND_FIELD_NUMBER: _ClassVar[int]
    LOADSECRETS_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    FILTER_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    LoadSecrets: bool
    Name: str
    Filter: _containers.ScalarMap[str, str]
    SubKind: str
    Version: str
    def __init__(self, Kind: _Optional[str] = ..., LoadSecrets: bool = ..., Name: _Optional[str] = ..., Filter: _Optional[_Mapping[str, str]] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ...) -> None: ...

class WatchStatusV1(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: WatchStatusSpecV1
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[WatchStatusSpecV1, _Mapping]] = ...) -> None: ...

class WatchStatusSpecV1(_message.Message):
    __slots__ = ["Kinds"]
    KINDS_FIELD_NUMBER: _ClassVar[int]
    Kinds: _containers.RepeatedCompositeFieldContainer[WatchKind]
    def __init__(self, Kinds: _Optional[_Iterable[_Union[WatchKind, _Mapping]]] = ...) -> None: ...

class ServerInfoV1(_message.Message):
    __slots__ = ["Kind", "SubKind", "Version", "Metadata", "Spec"]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUBKIND_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    Kind: str
    SubKind: str
    Version: str
    Metadata: Metadata
    Spec: ServerInfoSpecV1
    def __init__(self, Kind: _Optional[str] = ..., SubKind: _Optional[str] = ..., Version: _Optional[str] = ..., Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Spec: _Optional[_Union[ServerInfoSpecV1, _Mapping]] = ...) -> None: ...

class ServerInfoSpecV1(_message.Message):
    __slots__ = ["NewLabels"]
    class NewLabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NEWLABELS_FIELD_NUMBER: _ClassVar[int]
    NewLabels: _containers.ScalarMap[str, str]
    def __init__(self, NewLabels: _Optional[_Mapping[str, str]] = ...) -> None: ...

class JamfSpecV1(_message.Message):
    __slots__ = ["enabled", "name", "sync_delay", "api_endpoint", "username", "password", "inventory"]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    SYNC_DELAY_FIELD_NUMBER: _ClassVar[int]
    API_ENDPOINT_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    INVENTORY_FIELD_NUMBER: _ClassVar[int]
    enabled: bool
    name: str
    sync_delay: int
    api_endpoint: str
    username: str
    password: str
    inventory: _containers.RepeatedCompositeFieldContainer[JamfInventoryEntry]
    def __init__(self, enabled: bool = ..., name: _Optional[str] = ..., sync_delay: _Optional[int] = ..., api_endpoint: _Optional[str] = ..., username: _Optional[str] = ..., password: _Optional[str] = ..., inventory: _Optional[_Iterable[_Union[JamfInventoryEntry, _Mapping]]] = ...) -> None: ...

class JamfInventoryEntry(_message.Message):
    __slots__ = ["filter_rsql", "sync_period_partial", "sync_period_full", "on_missing"]
    FILTER_RSQL_FIELD_NUMBER: _ClassVar[int]
    SYNC_PERIOD_PARTIAL_FIELD_NUMBER: _ClassVar[int]
    SYNC_PERIOD_FULL_FIELD_NUMBER: _ClassVar[int]
    ON_MISSING_FIELD_NUMBER: _ClassVar[int]
    filter_rsql: str
    sync_period_partial: int
    sync_period_full: int
    on_missing: str
    def __init__(self, filter_rsql: _Optional[str] = ..., sync_period_partial: _Optional[int] = ..., sync_period_full: _Optional[int] = ..., on_missing: _Optional[str] = ...) -> None: ...

class MessageWithHeader(_message.Message):
    __slots__ = ["Header"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    Header: ResourceHeader
    def __init__(self, Header: _Optional[_Union[ResourceHeader, _Mapping]] = ...) -> None: ...

class AWSMatcher(_message.Message):
    __slots__ = ["Types", "Regions", "AssumeRole", "Tags", "Params", "SSM"]
    TYPES_FIELD_NUMBER: _ClassVar[int]
    REGIONS_FIELD_NUMBER: _ClassVar[int]
    ASSUMEROLE_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    PARAMS_FIELD_NUMBER: _ClassVar[int]
    SSM_FIELD_NUMBER: _ClassVar[int]
    Types: _containers.RepeatedScalarFieldContainer[str]
    Regions: _containers.RepeatedScalarFieldContainer[str]
    AssumeRole: AssumeRole
    Tags: _wrappers_pb2.LabelValues
    Params: InstallerParams
    SSM: AWSSSM
    def __init__(self, Types: _Optional[_Iterable[str]] = ..., Regions: _Optional[_Iterable[str]] = ..., AssumeRole: _Optional[_Union[AssumeRole, _Mapping]] = ..., Tags: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., Params: _Optional[_Union[InstallerParams, _Mapping]] = ..., SSM: _Optional[_Union[AWSSSM, _Mapping]] = ...) -> None: ...

class AssumeRole(_message.Message):
    __slots__ = ["RoleARN", "ExternalID"]
    ROLEARN_FIELD_NUMBER: _ClassVar[int]
    EXTERNALID_FIELD_NUMBER: _ClassVar[int]
    RoleARN: str
    ExternalID: str
    def __init__(self, RoleARN: _Optional[str] = ..., ExternalID: _Optional[str] = ...) -> None: ...

class InstallerParams(_message.Message):
    __slots__ = ["JoinMethod", "JoinToken", "ScriptName", "InstallTeleport", "SSHDConfig", "PublicProxyAddr", "Azure"]
    JOINMETHOD_FIELD_NUMBER: _ClassVar[int]
    JOINTOKEN_FIELD_NUMBER: _ClassVar[int]
    SCRIPTNAME_FIELD_NUMBER: _ClassVar[int]
    INSTALLTELEPORT_FIELD_NUMBER: _ClassVar[int]
    SSHDCONFIG_FIELD_NUMBER: _ClassVar[int]
    PUBLICPROXYADDR_FIELD_NUMBER: _ClassVar[int]
    AZURE_FIELD_NUMBER: _ClassVar[int]
    JoinMethod: str
    JoinToken: str
    ScriptName: str
    InstallTeleport: bool
    SSHDConfig: str
    PublicProxyAddr: str
    Azure: AzureInstallerParams
    def __init__(self, JoinMethod: _Optional[str] = ..., JoinToken: _Optional[str] = ..., ScriptName: _Optional[str] = ..., InstallTeleport: bool = ..., SSHDConfig: _Optional[str] = ..., PublicProxyAddr: _Optional[str] = ..., Azure: _Optional[_Union[AzureInstallerParams, _Mapping]] = ...) -> None: ...

class AWSSSM(_message.Message):
    __slots__ = ["DocumentName"]
    DOCUMENTNAME_FIELD_NUMBER: _ClassVar[int]
    DocumentName: str
    def __init__(self, DocumentName: _Optional[str] = ...) -> None: ...

class AzureInstallerParams(_message.Message):
    __slots__ = ["ClientID"]
    CLIENTID_FIELD_NUMBER: _ClassVar[int]
    ClientID: str
    def __init__(self, ClientID: _Optional[str] = ...) -> None: ...

class AzureMatcher(_message.Message):
    __slots__ = ["Subscriptions", "ResourceGroups", "Types", "Regions", "ResourceTags", "Params"]
    SUBSCRIPTIONS_FIELD_NUMBER: _ClassVar[int]
    RESOURCEGROUPS_FIELD_NUMBER: _ClassVar[int]
    TYPES_FIELD_NUMBER: _ClassVar[int]
    REGIONS_FIELD_NUMBER: _ClassVar[int]
    RESOURCETAGS_FIELD_NUMBER: _ClassVar[int]
    PARAMS_FIELD_NUMBER: _ClassVar[int]
    Subscriptions: _containers.RepeatedScalarFieldContainer[str]
    ResourceGroups: _containers.RepeatedScalarFieldContainer[str]
    Types: _containers.RepeatedScalarFieldContainer[str]
    Regions: _containers.RepeatedScalarFieldContainer[str]
    ResourceTags: _wrappers_pb2.LabelValues
    Params: InstallerParams
    def __init__(self, Subscriptions: _Optional[_Iterable[str]] = ..., ResourceGroups: _Optional[_Iterable[str]] = ..., Types: _Optional[_Iterable[str]] = ..., Regions: _Optional[_Iterable[str]] = ..., ResourceTags: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., Params: _Optional[_Union[InstallerParams, _Mapping]] = ...) -> None: ...

class GCPMatcher(_message.Message):
    __slots__ = ["Types", "Locations", "Tags", "ProjectIDs", "ServiceAccounts", "Params", "Labels"]
    TYPES_FIELD_NUMBER: _ClassVar[int]
    LOCATIONS_FIELD_NUMBER: _ClassVar[int]
    TAGS_FIELD_NUMBER: _ClassVar[int]
    PROJECTIDS_FIELD_NUMBER: _ClassVar[int]
    SERVICEACCOUNTS_FIELD_NUMBER: _ClassVar[int]
    PARAMS_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    Types: _containers.RepeatedScalarFieldContainer[str]
    Locations: _containers.RepeatedScalarFieldContainer[str]
    Tags: _wrappers_pb2.LabelValues
    ProjectIDs: _containers.RepeatedScalarFieldContainer[str]
    ServiceAccounts: _containers.RepeatedScalarFieldContainer[str]
    Params: InstallerParams
    Labels: _wrappers_pb2.LabelValues
    def __init__(self, Types: _Optional[_Iterable[str]] = ..., Locations: _Optional[_Iterable[str]] = ..., Tags: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., ProjectIDs: _Optional[_Iterable[str]] = ..., ServiceAccounts: _Optional[_Iterable[str]] = ..., Params: _Optional[_Union[InstallerParams, _Mapping]] = ..., Labels: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ...) -> None: ...

class KubernetesMatcher(_message.Message):
    __slots__ = ["Types", "Namespaces", "Labels"]
    TYPES_FIELD_NUMBER: _ClassVar[int]
    NAMESPACES_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    Types: _containers.RepeatedScalarFieldContainer[str]
    Namespaces: _containers.RepeatedScalarFieldContainer[str]
    Labels: _wrappers_pb2.LabelValues
    def __init__(self, Types: _Optional[_Iterable[str]] = ..., Namespaces: _Optional[_Iterable[str]] = ..., Labels: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ...) -> None: ...

class OktaOptions(_message.Message):
    __slots__ = ["SyncPeriod"]
    SYNCPERIOD_FIELD_NUMBER: _ClassVar[int]
    SyncPeriod: int
    def __init__(self, SyncPeriod: _Optional[int] = ...) -> None: ...
