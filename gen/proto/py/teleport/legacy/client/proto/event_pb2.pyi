from teleport.accesslist.v1 import accesslist_pb2 as _accesslist_pb2
from teleport.discoveryconfig.v1 import discoveryconfig_pb2 as _discoveryconfig_pb2
from teleport.legacy.types import types_pb2 as _types_pb2
from teleport.userloginstate.v1 import userloginstate_pb2 as _userloginstate_pb2
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Operation(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    INIT: _ClassVar[Operation]
    PUT: _ClassVar[Operation]
    DELETE: _ClassVar[Operation]
INIT: Operation
PUT: Operation
DELETE: Operation

class Event(_message.Message):
    __slots__ = ["Type", "ResourceHeader", "CertAuthority", "StaticTokens", "ProvisionToken", "ClusterName", "User", "Role", "Namespace", "Server", "ReverseTunnel", "TunnelConnection", "AccessRequest", "AppSession", "RemoteCluster", "DatabaseServer", "WebSession", "WebToken", "ClusterNetworkingConfig", "SessionRecordingConfig", "AuthPreference", "ClusterAuditConfig", "Lock", "NetworkRestrictions", "WindowsDesktopService", "WindowsDesktop", "Database", "AppServer", "App", "SnowflakeSession", "KubernetesServer", "KubernetesCluster", "Installer", "DatabaseService", "SAMLIdPServiceProvider", "SAMLIdPSession", "UserGroup", "UIConfig", "OktaImportRule", "OktaAssignment", "Integration", "WatchStatus", "HeadlessAuthentication", "AccessList", "UserLoginState", "AccessListMember", "DiscoveryConfig"]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    RESOURCEHEADER_FIELD_NUMBER: _ClassVar[int]
    CERTAUTHORITY_FIELD_NUMBER: _ClassVar[int]
    STATICTOKENS_FIELD_NUMBER: _ClassVar[int]
    PROVISIONTOKEN_FIELD_NUMBER: _ClassVar[int]
    CLUSTERNAME_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    NAMESPACE_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    REVERSETUNNEL_FIELD_NUMBER: _ClassVar[int]
    TUNNELCONNECTION_FIELD_NUMBER: _ClassVar[int]
    ACCESSREQUEST_FIELD_NUMBER: _ClassVar[int]
    APPSESSION_FIELD_NUMBER: _ClassVar[int]
    REMOTECLUSTER_FIELD_NUMBER: _ClassVar[int]
    DATABASESERVER_FIELD_NUMBER: _ClassVar[int]
    WEBSESSION_FIELD_NUMBER: _ClassVar[int]
    WEBTOKEN_FIELD_NUMBER: _ClassVar[int]
    CLUSTERNETWORKINGCONFIG_FIELD_NUMBER: _ClassVar[int]
    SESSIONRECORDINGCONFIG_FIELD_NUMBER: _ClassVar[int]
    AUTHPREFERENCE_FIELD_NUMBER: _ClassVar[int]
    CLUSTERAUDITCONFIG_FIELD_NUMBER: _ClassVar[int]
    LOCK_FIELD_NUMBER: _ClassVar[int]
    NETWORKRESTRICTIONS_FIELD_NUMBER: _ClassVar[int]
    WINDOWSDESKTOPSERVICE_FIELD_NUMBER: _ClassVar[int]
    WINDOWSDESKTOP_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    APPSERVER_FIELD_NUMBER: _ClassVar[int]
    APP_FIELD_NUMBER: _ClassVar[int]
    SNOWFLAKESESSION_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESSERVER_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESCLUSTER_FIELD_NUMBER: _ClassVar[int]
    INSTALLER_FIELD_NUMBER: _ClassVar[int]
    DATABASESERVICE_FIELD_NUMBER: _ClassVar[int]
    SAMLIDPSERVICEPROVIDER_FIELD_NUMBER: _ClassVar[int]
    SAMLIDPSESSION_FIELD_NUMBER: _ClassVar[int]
    USERGROUP_FIELD_NUMBER: _ClassVar[int]
    UICONFIG_FIELD_NUMBER: _ClassVar[int]
    OKTAIMPORTRULE_FIELD_NUMBER: _ClassVar[int]
    OKTAASSIGNMENT_FIELD_NUMBER: _ClassVar[int]
    INTEGRATION_FIELD_NUMBER: _ClassVar[int]
    WATCHSTATUS_FIELD_NUMBER: _ClassVar[int]
    HEADLESSAUTHENTICATION_FIELD_NUMBER: _ClassVar[int]
    ACCESSLIST_FIELD_NUMBER: _ClassVar[int]
    USERLOGINSTATE_FIELD_NUMBER: _ClassVar[int]
    ACCESSLISTMEMBER_FIELD_NUMBER: _ClassVar[int]
    DISCOVERYCONFIG_FIELD_NUMBER: _ClassVar[int]
    Type: Operation
    ResourceHeader: _types_pb2.ResourceHeader
    CertAuthority: _types_pb2.CertAuthorityV2
    StaticTokens: _types_pb2.StaticTokensV2
    ProvisionToken: _types_pb2.ProvisionTokenV2
    ClusterName: _types_pb2.ClusterNameV2
    User: _types_pb2.UserV2
    Role: _types_pb2.RoleV6
    Namespace: _types_pb2.Namespace
    Server: _types_pb2.ServerV2
    ReverseTunnel: _types_pb2.ReverseTunnelV2
    TunnelConnection: _types_pb2.TunnelConnectionV2
    AccessRequest: _types_pb2.AccessRequestV3
    AppSession: _types_pb2.WebSessionV2
    RemoteCluster: _types_pb2.RemoteClusterV3
    DatabaseServer: _types_pb2.DatabaseServerV3
    WebSession: _types_pb2.WebSessionV2
    WebToken: _types_pb2.WebTokenV3
    ClusterNetworkingConfig: _types_pb2.ClusterNetworkingConfigV2
    SessionRecordingConfig: _types_pb2.SessionRecordingConfigV2
    AuthPreference: _types_pb2.AuthPreferenceV2
    ClusterAuditConfig: _types_pb2.ClusterAuditConfigV2
    Lock: _types_pb2.LockV2
    NetworkRestrictions: _types_pb2.NetworkRestrictionsV4
    WindowsDesktopService: _types_pb2.WindowsDesktopServiceV3
    WindowsDesktop: _types_pb2.WindowsDesktopV3
    Database: _types_pb2.DatabaseV3
    AppServer: _types_pb2.AppServerV3
    App: _types_pb2.AppV3
    SnowflakeSession: _types_pb2.WebSessionV2
    KubernetesServer: _types_pb2.KubernetesServerV3
    KubernetesCluster: _types_pb2.KubernetesClusterV3
    Installer: _types_pb2.InstallerV1
    DatabaseService: _types_pb2.DatabaseServiceV1
    SAMLIdPServiceProvider: _types_pb2.SAMLIdPServiceProviderV1
    SAMLIdPSession: _types_pb2.WebSessionV2
    UserGroup: _types_pb2.UserGroupV1
    UIConfig: _types_pb2.UIConfigV1
    OktaImportRule: _types_pb2.OktaImportRuleV1
    OktaAssignment: _types_pb2.OktaAssignmentV1
    Integration: _types_pb2.IntegrationV1
    WatchStatus: _types_pb2.WatchStatusV1
    HeadlessAuthentication: _types_pb2.HeadlessAuthentication
    AccessList: _accesslist_pb2.AccessList
    UserLoginState: _userloginstate_pb2.UserLoginState
    AccessListMember: _accesslist_pb2.Member
    DiscoveryConfig: _discoveryconfig_pb2.DiscoveryConfig
    def __init__(self, Type: _Optional[_Union[Operation, str]] = ..., ResourceHeader: _Optional[_Union[_types_pb2.ResourceHeader, _Mapping]] = ..., CertAuthority: _Optional[_Union[_types_pb2.CertAuthorityV2, _Mapping]] = ..., StaticTokens: _Optional[_Union[_types_pb2.StaticTokensV2, _Mapping]] = ..., ProvisionToken: _Optional[_Union[_types_pb2.ProvisionTokenV2, _Mapping]] = ..., ClusterName: _Optional[_Union[_types_pb2.ClusterNameV2, _Mapping]] = ..., User: _Optional[_Union[_types_pb2.UserV2, _Mapping]] = ..., Role: _Optional[_Union[_types_pb2.RoleV6, _Mapping]] = ..., Namespace: _Optional[_Union[_types_pb2.Namespace, _Mapping]] = ..., Server: _Optional[_Union[_types_pb2.ServerV2, _Mapping]] = ..., ReverseTunnel: _Optional[_Union[_types_pb2.ReverseTunnelV2, _Mapping]] = ..., TunnelConnection: _Optional[_Union[_types_pb2.TunnelConnectionV2, _Mapping]] = ..., AccessRequest: _Optional[_Union[_types_pb2.AccessRequestV3, _Mapping]] = ..., AppSession: _Optional[_Union[_types_pb2.WebSessionV2, _Mapping]] = ..., RemoteCluster: _Optional[_Union[_types_pb2.RemoteClusterV3, _Mapping]] = ..., DatabaseServer: _Optional[_Union[_types_pb2.DatabaseServerV3, _Mapping]] = ..., WebSession: _Optional[_Union[_types_pb2.WebSessionV2, _Mapping]] = ..., WebToken: _Optional[_Union[_types_pb2.WebTokenV3, _Mapping]] = ..., ClusterNetworkingConfig: _Optional[_Union[_types_pb2.ClusterNetworkingConfigV2, _Mapping]] = ..., SessionRecordingConfig: _Optional[_Union[_types_pb2.SessionRecordingConfigV2, _Mapping]] = ..., AuthPreference: _Optional[_Union[_types_pb2.AuthPreferenceV2, _Mapping]] = ..., ClusterAuditConfig: _Optional[_Union[_types_pb2.ClusterAuditConfigV2, _Mapping]] = ..., Lock: _Optional[_Union[_types_pb2.LockV2, _Mapping]] = ..., NetworkRestrictions: _Optional[_Union[_types_pb2.NetworkRestrictionsV4, _Mapping]] = ..., WindowsDesktopService: _Optional[_Union[_types_pb2.WindowsDesktopServiceV3, _Mapping]] = ..., WindowsDesktop: _Optional[_Union[_types_pb2.WindowsDesktopV3, _Mapping]] = ..., Database: _Optional[_Union[_types_pb2.DatabaseV3, _Mapping]] = ..., AppServer: _Optional[_Union[_types_pb2.AppServerV3, _Mapping]] = ..., App: _Optional[_Union[_types_pb2.AppV3, _Mapping]] = ..., SnowflakeSession: _Optional[_Union[_types_pb2.WebSessionV2, _Mapping]] = ..., KubernetesServer: _Optional[_Union[_types_pb2.KubernetesServerV3, _Mapping]] = ..., KubernetesCluster: _Optional[_Union[_types_pb2.KubernetesClusterV3, _Mapping]] = ..., Installer: _Optional[_Union[_types_pb2.InstallerV1, _Mapping]] = ..., DatabaseService: _Optional[_Union[_types_pb2.DatabaseServiceV1, _Mapping]] = ..., SAMLIdPServiceProvider: _Optional[_Union[_types_pb2.SAMLIdPServiceProviderV1, _Mapping]] = ..., SAMLIdPSession: _Optional[_Union[_types_pb2.WebSessionV2, _Mapping]] = ..., UserGroup: _Optional[_Union[_types_pb2.UserGroupV1, _Mapping]] = ..., UIConfig: _Optional[_Union[_types_pb2.UIConfigV1, _Mapping]] = ..., OktaImportRule: _Optional[_Union[_types_pb2.OktaImportRuleV1, _Mapping]] = ..., OktaAssignment: _Optional[_Union[_types_pb2.OktaAssignmentV1, _Mapping]] = ..., Integration: _Optional[_Union[_types_pb2.IntegrationV1, _Mapping]] = ..., WatchStatus: _Optional[_Union[_types_pb2.WatchStatusV1, _Mapping]] = ..., HeadlessAuthentication: _Optional[_Union[_types_pb2.HeadlessAuthentication, _Mapping]] = ..., AccessList: _Optional[_Union[_accesslist_pb2.AccessList, _Mapping]] = ..., UserLoginState: _Optional[_Union[_userloginstate_pb2.UserLoginState, _Mapping]] = ..., AccessListMember: _Optional[_Union[_accesslist_pb2.Member, _Mapping]] = ..., DiscoveryConfig: _Optional[_Union[_discoveryconfig_pb2.DiscoveryConfig, _Mapping]] = ...) -> None: ...
