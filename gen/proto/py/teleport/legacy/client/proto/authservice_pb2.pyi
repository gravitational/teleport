from gogoproto import gogo_pb2 as _gogo_pb2
from google.protobuf import empty_pb2 as _empty_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from teleport.attestation.v1 import attestation_pb2 as _attestation_pb2
from teleport.legacy.client.proto import certs_pb2 as _certs_pb2
from teleport.legacy.client.proto import event_pb2 as _event_pb2
from teleport.legacy.types.events import events_pb2 as _events_pb2
from teleport.legacy.types import types_pb2 as _types_pb2
from teleport.legacy.types.webauthn import webauthn_pb2 as _webauthn_pb2
from teleport.legacy.types.wrappers import wrappers_pb2 as _wrappers_pb2
from teleport.usageevents.v1 import usageevents_pb2 as _usageevents_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DeviceType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    DEVICE_TYPE_UNSPECIFIED: _ClassVar[DeviceType]
    DEVICE_TYPE_TOTP: _ClassVar[DeviceType]
    DEVICE_TYPE_WEBAUTHN: _ClassVar[DeviceType]

class DeviceUsage(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    DEVICE_USAGE_UNSPECIFIED: _ClassVar[DeviceUsage]
    DEVICE_USAGE_MFA: _ClassVar[DeviceUsage]
    DEVICE_USAGE_PASSWORDLESS: _ClassVar[DeviceUsage]

class MFARequired(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    MFA_REQUIRED_UNSPECIFIED: _ClassVar[MFARequired]
    MFA_REQUIRED_YES: _ClassVar[MFARequired]
    MFA_REQUIRED_NO: _ClassVar[MFARequired]

class Order(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    DESCENDING: _ClassVar[Order]
    ASCENDING: _ClassVar[Order]

class LabelUpdateKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    SSHServer: _ClassVar[LabelUpdateKind]
    SSHServerCloudLabels: _ClassVar[LabelUpdateKind]
DEVICE_TYPE_UNSPECIFIED: DeviceType
DEVICE_TYPE_TOTP: DeviceType
DEVICE_TYPE_WEBAUTHN: DeviceType
DEVICE_USAGE_UNSPECIFIED: DeviceUsage
DEVICE_USAGE_MFA: DeviceUsage
DEVICE_USAGE_PASSWORDLESS: DeviceUsage
MFA_REQUIRED_UNSPECIFIED: MFARequired
MFA_REQUIRED_YES: MFARequired
MFA_REQUIRED_NO: MFARequired
DESCENDING: Order
ASCENDING: Order
SSHServer: LabelUpdateKind
SSHServerCloudLabels: LabelUpdateKind

class Watch(_message.Message):
    __slots__ = ["Kinds", "AllowPartialSuccess"]
    KINDS_FIELD_NUMBER: _ClassVar[int]
    ALLOWPARTIALSUCCESS_FIELD_NUMBER: _ClassVar[int]
    Kinds: _containers.RepeatedCompositeFieldContainer[_types_pb2.WatchKind]
    AllowPartialSuccess: bool
    def __init__(self, Kinds: _Optional[_Iterable[_Union[_types_pb2.WatchKind, _Mapping]]] = ..., AllowPartialSuccess: bool = ...) -> None: ...

class HostCertsRequest(_message.Message):
    __slots__ = ["HostID", "NodeName", "Role", "AdditionalPrincipals", "DNSNames", "PublicTLSKey", "PublicSSHKey", "RemoteAddr", "Rotation", "NoCache", "SystemRoles"]
    HOSTID_FIELD_NUMBER: _ClassVar[int]
    NODENAME_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    ADDITIONALPRINCIPALS_FIELD_NUMBER: _ClassVar[int]
    DNSNAMES_FIELD_NUMBER: _ClassVar[int]
    PUBLICTLSKEY_FIELD_NUMBER: _ClassVar[int]
    PUBLICSSHKEY_FIELD_NUMBER: _ClassVar[int]
    REMOTEADDR_FIELD_NUMBER: _ClassVar[int]
    ROTATION_FIELD_NUMBER: _ClassVar[int]
    NOCACHE_FIELD_NUMBER: _ClassVar[int]
    SYSTEMROLES_FIELD_NUMBER: _ClassVar[int]
    HostID: str
    NodeName: str
    Role: str
    AdditionalPrincipals: _containers.RepeatedScalarFieldContainer[str]
    DNSNames: _containers.RepeatedScalarFieldContainer[str]
    PublicTLSKey: bytes
    PublicSSHKey: bytes
    RemoteAddr: str
    Rotation: _types_pb2.Rotation
    NoCache: bool
    SystemRoles: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, HostID: _Optional[str] = ..., NodeName: _Optional[str] = ..., Role: _Optional[str] = ..., AdditionalPrincipals: _Optional[_Iterable[str]] = ..., DNSNames: _Optional[_Iterable[str]] = ..., PublicTLSKey: _Optional[bytes] = ..., PublicSSHKey: _Optional[bytes] = ..., RemoteAddr: _Optional[str] = ..., Rotation: _Optional[_Union[_types_pb2.Rotation, _Mapping]] = ..., NoCache: bool = ..., SystemRoles: _Optional[_Iterable[str]] = ...) -> None: ...

class OpenSSHCertRequest(_message.Message):
    __slots__ = ["PublicKey", "TTL", "Cluster", "User", "Roles"]
    PUBLICKEY_FIELD_NUMBER: _ClassVar[int]
    TTL_FIELD_NUMBER: _ClassVar[int]
    CLUSTER_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    PublicKey: bytes
    TTL: int
    Cluster: str
    User: _types_pb2.UserV2
    Roles: _containers.RepeatedCompositeFieldContainer[_types_pb2.RoleV6]
    def __init__(self, PublicKey: _Optional[bytes] = ..., TTL: _Optional[int] = ..., Cluster: _Optional[str] = ..., User: _Optional[_Union[_types_pb2.UserV2, _Mapping]] = ..., Roles: _Optional[_Iterable[_Union[_types_pb2.RoleV6, _Mapping]]] = ...) -> None: ...

class OpenSSHCert(_message.Message):
    __slots__ = ["Cert"]
    CERT_FIELD_NUMBER: _ClassVar[int]
    Cert: bytes
    def __init__(self, Cert: _Optional[bytes] = ...) -> None: ...

class UserCertsRequest(_message.Message):
    __slots__ = ["PublicKey", "Username", "Expires", "Format", "RouteToCluster", "AccessRequests", "KubernetesCluster", "RouteToDatabase", "NodeName", "Usage", "RouteToApp", "RoleRequests", "RouteToWindowsDesktop", "UseRoleRequests", "DropAccessRequests", "ConnectionDiagnosticID", "RequesterName", "MFAResponse", "SSHLogin", "attestation_statement"]
    class CertUsage(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = []
        All: _ClassVar[UserCertsRequest.CertUsage]
        SSH: _ClassVar[UserCertsRequest.CertUsage]
        Kubernetes: _ClassVar[UserCertsRequest.CertUsage]
        Database: _ClassVar[UserCertsRequest.CertUsage]
        App: _ClassVar[UserCertsRequest.CertUsage]
        WindowsDesktop: _ClassVar[UserCertsRequest.CertUsage]
    All: UserCertsRequest.CertUsage
    SSH: UserCertsRequest.CertUsage
    Kubernetes: UserCertsRequest.CertUsage
    Database: UserCertsRequest.CertUsage
    App: UserCertsRequest.CertUsage
    WindowsDesktop: UserCertsRequest.CertUsage
    class Requester(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = []
        UNSPECIFIED: _ClassVar[UserCertsRequest.Requester]
        TSH_DB_LOCAL_PROXY_TUNNEL: _ClassVar[UserCertsRequest.Requester]
        TSH_KUBE_LOCAL_PROXY: _ClassVar[UserCertsRequest.Requester]
    UNSPECIFIED: UserCertsRequest.Requester
    TSH_DB_LOCAL_PROXY_TUNNEL: UserCertsRequest.Requester
    TSH_KUBE_LOCAL_PROXY: UserCertsRequest.Requester
    PUBLICKEY_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    FORMAT_FIELD_NUMBER: _ClassVar[int]
    ROUTETOCLUSTER_FIELD_NUMBER: _ClassVar[int]
    ACCESSREQUESTS_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESCLUSTER_FIELD_NUMBER: _ClassVar[int]
    ROUTETODATABASE_FIELD_NUMBER: _ClassVar[int]
    NODENAME_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    ROUTETOAPP_FIELD_NUMBER: _ClassVar[int]
    ROLEREQUESTS_FIELD_NUMBER: _ClassVar[int]
    ROUTETOWINDOWSDESKTOP_FIELD_NUMBER: _ClassVar[int]
    USEROLEREQUESTS_FIELD_NUMBER: _ClassVar[int]
    DROPACCESSREQUESTS_FIELD_NUMBER: _ClassVar[int]
    CONNECTIONDIAGNOSTICID_FIELD_NUMBER: _ClassVar[int]
    REQUESTERNAME_FIELD_NUMBER: _ClassVar[int]
    MFARESPONSE_FIELD_NUMBER: _ClassVar[int]
    SSHLOGIN_FIELD_NUMBER: _ClassVar[int]
    ATTESTATION_STATEMENT_FIELD_NUMBER: _ClassVar[int]
    PublicKey: bytes
    Username: str
    Expires: _timestamp_pb2.Timestamp
    Format: str
    RouteToCluster: str
    AccessRequests: _containers.RepeatedScalarFieldContainer[str]
    KubernetesCluster: str
    RouteToDatabase: RouteToDatabase
    NodeName: str
    Usage: UserCertsRequest.CertUsage
    RouteToApp: RouteToApp
    RoleRequests: _containers.RepeatedScalarFieldContainer[str]
    RouteToWindowsDesktop: RouteToWindowsDesktop
    UseRoleRequests: bool
    DropAccessRequests: _containers.RepeatedScalarFieldContainer[str]
    ConnectionDiagnosticID: str
    RequesterName: UserCertsRequest.Requester
    MFAResponse: MFAAuthenticateResponse
    SSHLogin: str
    attestation_statement: _attestation_pb2.AttestationStatement
    def __init__(self, PublicKey: _Optional[bytes] = ..., Username: _Optional[str] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Format: _Optional[str] = ..., RouteToCluster: _Optional[str] = ..., AccessRequests: _Optional[_Iterable[str]] = ..., KubernetesCluster: _Optional[str] = ..., RouteToDatabase: _Optional[_Union[RouteToDatabase, _Mapping]] = ..., NodeName: _Optional[str] = ..., Usage: _Optional[_Union[UserCertsRequest.CertUsage, str]] = ..., RouteToApp: _Optional[_Union[RouteToApp, _Mapping]] = ..., RoleRequests: _Optional[_Iterable[str]] = ..., RouteToWindowsDesktop: _Optional[_Union[RouteToWindowsDesktop, _Mapping]] = ..., UseRoleRequests: bool = ..., DropAccessRequests: _Optional[_Iterable[str]] = ..., ConnectionDiagnosticID: _Optional[str] = ..., RequesterName: _Optional[_Union[UserCertsRequest.Requester, str]] = ..., MFAResponse: _Optional[_Union[MFAAuthenticateResponse, _Mapping]] = ..., SSHLogin: _Optional[str] = ..., attestation_statement: _Optional[_Union[_attestation_pb2.AttestationStatement, _Mapping]] = ...) -> None: ...

class RouteToDatabase(_message.Message):
    __slots__ = ["ServiceName", "Protocol", "Username", "Database"]
    SERVICENAME_FIELD_NUMBER: _ClassVar[int]
    PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    ServiceName: str
    Protocol: str
    Username: str
    Database: str
    def __init__(self, ServiceName: _Optional[str] = ..., Protocol: _Optional[str] = ..., Username: _Optional[str] = ..., Database: _Optional[str] = ...) -> None: ...

class RouteToWindowsDesktop(_message.Message):
    __slots__ = ["WindowsDesktop", "Login"]
    WINDOWSDESKTOP_FIELD_NUMBER: _ClassVar[int]
    LOGIN_FIELD_NUMBER: _ClassVar[int]
    WindowsDesktop: str
    Login: str
    def __init__(self, WindowsDesktop: _Optional[str] = ..., Login: _Optional[str] = ...) -> None: ...

class RouteToApp(_message.Message):
    __slots__ = ["Name", "SessionID", "PublicAddr", "ClusterName", "AWSRoleARN", "AzureIdentity", "GCPServiceAccount"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    PUBLICADDR_FIELD_NUMBER: _ClassVar[int]
    CLUSTERNAME_FIELD_NUMBER: _ClassVar[int]
    AWSROLEARN_FIELD_NUMBER: _ClassVar[int]
    AZUREIDENTITY_FIELD_NUMBER: _ClassVar[int]
    GCPSERVICEACCOUNT_FIELD_NUMBER: _ClassVar[int]
    Name: str
    SessionID: str
    PublicAddr: str
    ClusterName: str
    AWSRoleARN: str
    AzureIdentity: str
    GCPServiceAccount: str
    def __init__(self, Name: _Optional[str] = ..., SessionID: _Optional[str] = ..., PublicAddr: _Optional[str] = ..., ClusterName: _Optional[str] = ..., AWSRoleARN: _Optional[str] = ..., AzureIdentity: _Optional[str] = ..., GCPServiceAccount: _Optional[str] = ...) -> None: ...

class GetUserRequest(_message.Message):
    __slots__ = ["Name", "WithSecrets"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    WITHSECRETS_FIELD_NUMBER: _ClassVar[int]
    Name: str
    WithSecrets: bool
    def __init__(self, Name: _Optional[str] = ..., WithSecrets: bool = ...) -> None: ...

class GetUsersRequest(_message.Message):
    __slots__ = ["WithSecrets"]
    WITHSECRETS_FIELD_NUMBER: _ClassVar[int]
    WithSecrets: bool
    def __init__(self, WithSecrets: bool = ...) -> None: ...

class ChangePasswordRequest(_message.Message):
    __slots__ = ["User", "OldPassword", "NewPassword", "SecondFactorToken", "Webauthn"]
    USER_FIELD_NUMBER: _ClassVar[int]
    OLDPASSWORD_FIELD_NUMBER: _ClassVar[int]
    NEWPASSWORD_FIELD_NUMBER: _ClassVar[int]
    SECONDFACTORTOKEN_FIELD_NUMBER: _ClassVar[int]
    WEBAUTHN_FIELD_NUMBER: _ClassVar[int]
    User: str
    OldPassword: bytes
    NewPassword: bytes
    SecondFactorToken: str
    Webauthn: _webauthn_pb2.CredentialAssertionResponse
    def __init__(self, User: _Optional[str] = ..., OldPassword: _Optional[bytes] = ..., NewPassword: _Optional[bytes] = ..., SecondFactorToken: _Optional[str] = ..., Webauthn: _Optional[_Union[_webauthn_pb2.CredentialAssertionResponse, _Mapping]] = ...) -> None: ...

class PluginDataSeq(_message.Message):
    __slots__ = ["PluginData"]
    PLUGINDATA_FIELD_NUMBER: _ClassVar[int]
    PluginData: _containers.RepeatedCompositeFieldContainer[_types_pb2.PluginDataV3]
    def __init__(self, PluginData: _Optional[_Iterable[_Union[_types_pb2.PluginDataV3, _Mapping]]] = ...) -> None: ...

class RequestStateSetter(_message.Message):
    __slots__ = ["ID", "State", "Delegator", "Reason", "Annotations", "Roles"]
    ID_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    DELEGATOR_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    ANNOTATIONS_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    ID: str
    State: _types_pb2.RequestState
    Delegator: str
    Reason: str
    Annotations: _wrappers_pb2.LabelValues
    Roles: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, ID: _Optional[str] = ..., State: _Optional[_Union[_types_pb2.RequestState, str]] = ..., Delegator: _Optional[str] = ..., Reason: _Optional[str] = ..., Annotations: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ..., Roles: _Optional[_Iterable[str]] = ...) -> None: ...

class RequestID(_message.Message):
    __slots__ = ["ID"]
    ID_FIELD_NUMBER: _ClassVar[int]
    ID: str
    def __init__(self, ID: _Optional[str] = ...) -> None: ...

class GetResetPasswordTokenRequest(_message.Message):
    __slots__ = ["TokenID"]
    TOKENID_FIELD_NUMBER: _ClassVar[int]
    TokenID: str
    def __init__(self, TokenID: _Optional[str] = ...) -> None: ...

class CreateResetPasswordTokenRequest(_message.Message):
    __slots__ = ["Name", "Type", "TTL"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    TTL_FIELD_NUMBER: _ClassVar[int]
    Name: str
    Type: str
    TTL: int
    def __init__(self, Name: _Optional[str] = ..., Type: _Optional[str] = ..., TTL: _Optional[int] = ...) -> None: ...

class RenewableCertsRequest(_message.Message):
    __slots__ = ["Token", "PublicKey"]
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    PUBLICKEY_FIELD_NUMBER: _ClassVar[int]
    Token: str
    PublicKey: bytes
    def __init__(self, Token: _Optional[str] = ..., PublicKey: _Optional[bytes] = ...) -> None: ...

class CreateBotRequest(_message.Message):
    __slots__ = ["Name", "TTL", "TokenID", "Roles", "Traits"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    TTL_FIELD_NUMBER: _ClassVar[int]
    TOKENID_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    TRAITS_FIELD_NUMBER: _ClassVar[int]
    Name: str
    TTL: int
    TokenID: str
    Roles: _containers.RepeatedScalarFieldContainer[str]
    Traits: _wrappers_pb2.LabelValues
    def __init__(self, Name: _Optional[str] = ..., TTL: _Optional[int] = ..., TokenID: _Optional[str] = ..., Roles: _Optional[_Iterable[str]] = ..., Traits: _Optional[_Union[_wrappers_pb2.LabelValues, _Mapping]] = ...) -> None: ...

class CreateBotResponse(_message.Message):
    __slots__ = ["UserName", "RoleName", "TokenID", "TokenTTL", "JoinMethod"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    ROLENAME_FIELD_NUMBER: _ClassVar[int]
    TOKENID_FIELD_NUMBER: _ClassVar[int]
    TOKENTTL_FIELD_NUMBER: _ClassVar[int]
    JOINMETHOD_FIELD_NUMBER: _ClassVar[int]
    UserName: str
    RoleName: str
    TokenID: str
    TokenTTL: int
    JoinMethod: str
    def __init__(self, UserName: _Optional[str] = ..., RoleName: _Optional[str] = ..., TokenID: _Optional[str] = ..., TokenTTL: _Optional[int] = ..., JoinMethod: _Optional[str] = ...) -> None: ...

class DeleteBotRequest(_message.Message):
    __slots__ = ["Name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Name: str
    def __init__(self, Name: _Optional[str] = ...) -> None: ...

class GetBotUsersRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class PingRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class PingResponse(_message.Message):
    __slots__ = ["ClusterName", "ServerVersion", "ServerFeatures", "ProxyPublicAddr", "IsBoring", "RemoteAddr", "LoadAllCAs"]
    CLUSTERNAME_FIELD_NUMBER: _ClassVar[int]
    SERVERVERSION_FIELD_NUMBER: _ClassVar[int]
    SERVERFEATURES_FIELD_NUMBER: _ClassVar[int]
    PROXYPUBLICADDR_FIELD_NUMBER: _ClassVar[int]
    ISBORING_FIELD_NUMBER: _ClassVar[int]
    REMOTEADDR_FIELD_NUMBER: _ClassVar[int]
    LOADALLCAS_FIELD_NUMBER: _ClassVar[int]
    ClusterName: str
    ServerVersion: str
    ServerFeatures: Features
    ProxyPublicAddr: str
    IsBoring: bool
    RemoteAddr: str
    LoadAllCAs: bool
    def __init__(self, ClusterName: _Optional[str] = ..., ServerVersion: _Optional[str] = ..., ServerFeatures: _Optional[_Union[Features, _Mapping]] = ..., ProxyPublicAddr: _Optional[str] = ..., IsBoring: bool = ..., RemoteAddr: _Optional[str] = ..., LoadAllCAs: bool = ...) -> None: ...

class Features(_message.Message):
    __slots__ = ["Kubernetes", "App", "DB", "OIDC", "SAML", "AccessControls", "AdvancedAccessWorkflows", "Cloud", "HSM", "Desktop", "RecoveryCodes", "Plugins", "AutomaticUpgrades", "IsUsageBased", "Assist", "DeviceTrust", "FeatureHiding", "AccessRequests", "CustomTheme"]
    KUBERNETES_FIELD_NUMBER: _ClassVar[int]
    APP_FIELD_NUMBER: _ClassVar[int]
    DB_FIELD_NUMBER: _ClassVar[int]
    OIDC_FIELD_NUMBER: _ClassVar[int]
    SAML_FIELD_NUMBER: _ClassVar[int]
    ACCESSCONTROLS_FIELD_NUMBER: _ClassVar[int]
    ADVANCEDACCESSWORKFLOWS_FIELD_NUMBER: _ClassVar[int]
    CLOUD_FIELD_NUMBER: _ClassVar[int]
    HSM_FIELD_NUMBER: _ClassVar[int]
    DESKTOP_FIELD_NUMBER: _ClassVar[int]
    RECOVERYCODES_FIELD_NUMBER: _ClassVar[int]
    PLUGINS_FIELD_NUMBER: _ClassVar[int]
    AUTOMATICUPGRADES_FIELD_NUMBER: _ClassVar[int]
    ISUSAGEBASED_FIELD_NUMBER: _ClassVar[int]
    ASSIST_FIELD_NUMBER: _ClassVar[int]
    DEVICETRUST_FIELD_NUMBER: _ClassVar[int]
    FEATUREHIDING_FIELD_NUMBER: _ClassVar[int]
    ACCESSREQUESTS_FIELD_NUMBER: _ClassVar[int]
    CUSTOMTHEME_FIELD_NUMBER: _ClassVar[int]
    Kubernetes: bool
    App: bool
    DB: bool
    OIDC: bool
    SAML: bool
    AccessControls: bool
    AdvancedAccessWorkflows: bool
    Cloud: bool
    HSM: bool
    Desktop: bool
    RecoveryCodes: bool
    Plugins: bool
    AutomaticUpgrades: bool
    IsUsageBased: bool
    Assist: bool
    DeviceTrust: DeviceTrustFeature
    FeatureHiding: bool
    AccessRequests: AccessRequestsFeature
    CustomTheme: str
    def __init__(self, Kubernetes: bool = ..., App: bool = ..., DB: bool = ..., OIDC: bool = ..., SAML: bool = ..., AccessControls: bool = ..., AdvancedAccessWorkflows: bool = ..., Cloud: bool = ..., HSM: bool = ..., Desktop: bool = ..., RecoveryCodes: bool = ..., Plugins: bool = ..., AutomaticUpgrades: bool = ..., IsUsageBased: bool = ..., Assist: bool = ..., DeviceTrust: _Optional[_Union[DeviceTrustFeature, _Mapping]] = ..., FeatureHiding: bool = ..., AccessRequests: _Optional[_Union[AccessRequestsFeature, _Mapping]] = ..., CustomTheme: _Optional[str] = ...) -> None: ...

class DeviceTrustFeature(_message.Message):
    __slots__ = ["enabled", "devices_usage_limit"]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    DEVICES_USAGE_LIMIT_FIELD_NUMBER: _ClassVar[int]
    enabled: bool
    devices_usage_limit: int
    def __init__(self, enabled: bool = ..., devices_usage_limit: _Optional[int] = ...) -> None: ...

class AccessRequestsFeature(_message.Message):
    __slots__ = ["monthly_request_limit"]
    MONTHLY_REQUEST_LIMIT_FIELD_NUMBER: _ClassVar[int]
    monthly_request_limit: int
    def __init__(self, monthly_request_limit: _Optional[int] = ...) -> None: ...

class DeleteUserRequest(_message.Message):
    __slots__ = ["Name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Name: str
    def __init__(self, Name: _Optional[str] = ...) -> None: ...

class Semaphores(_message.Message):
    __slots__ = ["Semaphores"]
    SEMAPHORES_FIELD_NUMBER: _ClassVar[int]
    Semaphores: _containers.RepeatedCompositeFieldContainer[_types_pb2.SemaphoreV3]
    def __init__(self, Semaphores: _Optional[_Iterable[_Union[_types_pb2.SemaphoreV3, _Mapping]]] = ...) -> None: ...

class AuditStreamRequest(_message.Message):
    __slots__ = ["CreateStream", "ResumeStream", "CompleteStream", "FlushAndCloseStream", "Event"]
    CREATESTREAM_FIELD_NUMBER: _ClassVar[int]
    RESUMESTREAM_FIELD_NUMBER: _ClassVar[int]
    COMPLETESTREAM_FIELD_NUMBER: _ClassVar[int]
    FLUSHANDCLOSESTREAM_FIELD_NUMBER: _ClassVar[int]
    EVENT_FIELD_NUMBER: _ClassVar[int]
    CreateStream: CreateStream
    ResumeStream: ResumeStream
    CompleteStream: CompleteStream
    FlushAndCloseStream: FlushAndCloseStream
    Event: _events_pb2.OneOf
    def __init__(self, CreateStream: _Optional[_Union[CreateStream, _Mapping]] = ..., ResumeStream: _Optional[_Union[ResumeStream, _Mapping]] = ..., CompleteStream: _Optional[_Union[CompleteStream, _Mapping]] = ..., FlushAndCloseStream: _Optional[_Union[FlushAndCloseStream, _Mapping]] = ..., Event: _Optional[_Union[_events_pb2.OneOf, _Mapping]] = ...) -> None: ...

class AuditStreamStatus(_message.Message):
    __slots__ = ["UploadID"]
    UPLOADID_FIELD_NUMBER: _ClassVar[int]
    UploadID: str
    def __init__(self, UploadID: _Optional[str] = ...) -> None: ...

class CreateStream(_message.Message):
    __slots__ = ["SessionID"]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    SessionID: str
    def __init__(self, SessionID: _Optional[str] = ...) -> None: ...

class ResumeStream(_message.Message):
    __slots__ = ["SessionID", "UploadID"]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    UPLOADID_FIELD_NUMBER: _ClassVar[int]
    SessionID: str
    UploadID: str
    def __init__(self, SessionID: _Optional[str] = ..., UploadID: _Optional[str] = ...) -> None: ...

class CompleteStream(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class FlushAndCloseStream(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class UpsertApplicationServerRequest(_message.Message):
    __slots__ = ["Server"]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    Server: _types_pb2.AppServerV3
    def __init__(self, Server: _Optional[_Union[_types_pb2.AppServerV3, _Mapping]] = ...) -> None: ...

class DeleteApplicationServerRequest(_message.Message):
    __slots__ = ["Namespace", "HostID", "Name"]
    NAMESPACE_FIELD_NUMBER: _ClassVar[int]
    HOSTID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Namespace: str
    HostID: str
    Name: str
    def __init__(self, Namespace: _Optional[str] = ..., HostID: _Optional[str] = ..., Name: _Optional[str] = ...) -> None: ...

class DeleteAllApplicationServersRequest(_message.Message):
    __slots__ = ["Namespace"]
    NAMESPACE_FIELD_NUMBER: _ClassVar[int]
    Namespace: str
    def __init__(self, Namespace: _Optional[str] = ...) -> None: ...

class GenerateAppTokenRequest(_message.Message):
    __slots__ = ["Username", "Roles", "URI", "Expires", "Traits"]
    class TraitsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: _wrappers_pb2.StringValues
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[_wrappers_pb2.StringValues, _Mapping]] = ...) -> None: ...
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    URI_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    TRAITS_FIELD_NUMBER: _ClassVar[int]
    Username: str
    Roles: _containers.RepeatedScalarFieldContainer[str]
    URI: str
    Expires: _timestamp_pb2.Timestamp
    Traits: _containers.MessageMap[str, _wrappers_pb2.StringValues]
    def __init__(self, Username: _Optional[str] = ..., Roles: _Optional[_Iterable[str]] = ..., URI: _Optional[str] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Traits: _Optional[_Mapping[str, _wrappers_pb2.StringValues]] = ...) -> None: ...

class GenerateAppTokenResponse(_message.Message):
    __slots__ = ["Token"]
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    Token: str
    def __init__(self, Token: _Optional[str] = ...) -> None: ...

class GetAppSessionRequest(_message.Message):
    __slots__ = ["SessionID"]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    SessionID: str
    def __init__(self, SessionID: _Optional[str] = ...) -> None: ...

class GetAppSessionResponse(_message.Message):
    __slots__ = ["Session"]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    Session: _types_pb2.WebSessionV2
    def __init__(self, Session: _Optional[_Union[_types_pb2.WebSessionV2, _Mapping]] = ...) -> None: ...

class ListAppSessionsRequest(_message.Message):
    __slots__ = ["page_size", "page_token", "user"]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    page_size: int
    page_token: str
    user: str
    def __init__(self, page_size: _Optional[int] = ..., page_token: _Optional[str] = ..., user: _Optional[str] = ...) -> None: ...

class ListAppSessionsResponse(_message.Message):
    __slots__ = ["sessions", "next_page_token"]
    SESSIONS_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    sessions: _containers.RepeatedCompositeFieldContainer[_types_pb2.WebSessionV2]
    next_page_token: str
    def __init__(self, sessions: _Optional[_Iterable[_Union[_types_pb2.WebSessionV2, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

class GetSnowflakeSessionsResponse(_message.Message):
    __slots__ = ["Sessions"]
    SESSIONS_FIELD_NUMBER: _ClassVar[int]
    Sessions: _containers.RepeatedCompositeFieldContainer[_types_pb2.WebSessionV2]
    def __init__(self, Sessions: _Optional[_Iterable[_Union[_types_pb2.WebSessionV2, _Mapping]]] = ...) -> None: ...

class ListSAMLIdPSessionsRequest(_message.Message):
    __slots__ = ["page_size", "page_token", "user"]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    page_size: int
    page_token: str
    user: str
    def __init__(self, page_size: _Optional[int] = ..., page_token: _Optional[str] = ..., user: _Optional[str] = ...) -> None: ...

class ListSAMLIdPSessionsResponse(_message.Message):
    __slots__ = ["sessions", "next_page_token"]
    SESSIONS_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    sessions: _containers.RepeatedCompositeFieldContainer[_types_pb2.WebSessionV2]
    next_page_token: str
    def __init__(self, sessions: _Optional[_Iterable[_Union[_types_pb2.WebSessionV2, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

class CreateAppSessionRequest(_message.Message):
    __slots__ = ["Username", "PublicAddr", "ClusterName", "AWSRoleARN", "AzureIdentity", "GCPServiceAccount"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    PUBLICADDR_FIELD_NUMBER: _ClassVar[int]
    CLUSTERNAME_FIELD_NUMBER: _ClassVar[int]
    AWSROLEARN_FIELD_NUMBER: _ClassVar[int]
    AZUREIDENTITY_FIELD_NUMBER: _ClassVar[int]
    GCPSERVICEACCOUNT_FIELD_NUMBER: _ClassVar[int]
    Username: str
    PublicAddr: str
    ClusterName: str
    AWSRoleARN: str
    AzureIdentity: str
    GCPServiceAccount: str
    def __init__(self, Username: _Optional[str] = ..., PublicAddr: _Optional[str] = ..., ClusterName: _Optional[str] = ..., AWSRoleARN: _Optional[str] = ..., AzureIdentity: _Optional[str] = ..., GCPServiceAccount: _Optional[str] = ...) -> None: ...

class CreateAppSessionResponse(_message.Message):
    __slots__ = ["Session"]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    Session: _types_pb2.WebSessionV2
    def __init__(self, Session: _Optional[_Union[_types_pb2.WebSessionV2, _Mapping]] = ...) -> None: ...

class CreateSnowflakeSessionRequest(_message.Message):
    __slots__ = ["Username", "SessionToken", "TokenTTL"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    SESSIONTOKEN_FIELD_NUMBER: _ClassVar[int]
    TOKENTTL_FIELD_NUMBER: _ClassVar[int]
    Username: str
    SessionToken: str
    TokenTTL: int
    def __init__(self, Username: _Optional[str] = ..., SessionToken: _Optional[str] = ..., TokenTTL: _Optional[int] = ...) -> None: ...

class CreateSnowflakeSessionResponse(_message.Message):
    __slots__ = ["Session"]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    Session: _types_pb2.WebSessionV2
    def __init__(self, Session: _Optional[_Union[_types_pb2.WebSessionV2, _Mapping]] = ...) -> None: ...

class CreateSAMLIdPSessionRequest(_message.Message):
    __slots__ = ["SessionID", "Username", "SAMLSession"]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    SAMLSESSION_FIELD_NUMBER: _ClassVar[int]
    SessionID: str
    Username: str
    SAMLSession: _types_pb2.SAMLSessionData
    def __init__(self, SessionID: _Optional[str] = ..., Username: _Optional[str] = ..., SAMLSession: _Optional[_Union[_types_pb2.SAMLSessionData, _Mapping]] = ...) -> None: ...

class CreateSAMLIdPSessionResponse(_message.Message):
    __slots__ = ["Session"]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    Session: _types_pb2.WebSessionV2
    def __init__(self, Session: _Optional[_Union[_types_pb2.WebSessionV2, _Mapping]] = ...) -> None: ...

class GetSnowflakeSessionRequest(_message.Message):
    __slots__ = ["SessionID"]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    SessionID: str
    def __init__(self, SessionID: _Optional[str] = ...) -> None: ...

class GetSnowflakeSessionResponse(_message.Message):
    __slots__ = ["Session"]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    Session: _types_pb2.WebSessionV2
    def __init__(self, Session: _Optional[_Union[_types_pb2.WebSessionV2, _Mapping]] = ...) -> None: ...

class GetSAMLIdPSessionRequest(_message.Message):
    __slots__ = ["SessionID"]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    SessionID: str
    def __init__(self, SessionID: _Optional[str] = ...) -> None: ...

class GetSAMLIdPSessionResponse(_message.Message):
    __slots__ = ["Session"]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    Session: _types_pb2.WebSessionV2
    def __init__(self, Session: _Optional[_Union[_types_pb2.WebSessionV2, _Mapping]] = ...) -> None: ...

class DeleteAppSessionRequest(_message.Message):
    __slots__ = ["SessionID"]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    SessionID: str
    def __init__(self, SessionID: _Optional[str] = ...) -> None: ...

class DeleteSnowflakeSessionRequest(_message.Message):
    __slots__ = ["SessionID"]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    SessionID: str
    def __init__(self, SessionID: _Optional[str] = ...) -> None: ...

class DeleteSAMLIdPSessionRequest(_message.Message):
    __slots__ = ["SessionID"]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    SessionID: str
    def __init__(self, SessionID: _Optional[str] = ...) -> None: ...

class DeleteUserAppSessionsRequest(_message.Message):
    __slots__ = ["Username"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    Username: str
    def __init__(self, Username: _Optional[str] = ...) -> None: ...

class DeleteUserSAMLIdPSessionsRequest(_message.Message):
    __slots__ = ["Username"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    Username: str
    def __init__(self, Username: _Optional[str] = ...) -> None: ...

class GetWebSessionResponse(_message.Message):
    __slots__ = ["Session"]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    Session: _types_pb2.WebSessionV2
    def __init__(self, Session: _Optional[_Union[_types_pb2.WebSessionV2, _Mapping]] = ...) -> None: ...

class GetWebSessionsResponse(_message.Message):
    __slots__ = ["Sessions"]
    SESSIONS_FIELD_NUMBER: _ClassVar[int]
    Sessions: _containers.RepeatedCompositeFieldContainer[_types_pb2.WebSessionV2]
    def __init__(self, Sessions: _Optional[_Iterable[_Union[_types_pb2.WebSessionV2, _Mapping]]] = ...) -> None: ...

class GetWebTokenResponse(_message.Message):
    __slots__ = ["Token"]
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    Token: _types_pb2.WebTokenV3
    def __init__(self, Token: _Optional[_Union[_types_pb2.WebTokenV3, _Mapping]] = ...) -> None: ...

class GetWebTokensResponse(_message.Message):
    __slots__ = ["Tokens"]
    TOKENS_FIELD_NUMBER: _ClassVar[int]
    Tokens: _containers.RepeatedCompositeFieldContainer[_types_pb2.WebTokenV3]
    def __init__(self, Tokens: _Optional[_Iterable[_Union[_types_pb2.WebTokenV3, _Mapping]]] = ...) -> None: ...

class UpsertKubernetesServerRequest(_message.Message):
    __slots__ = ["Server"]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    Server: _types_pb2.KubernetesServerV3
    def __init__(self, Server: _Optional[_Union[_types_pb2.KubernetesServerV3, _Mapping]] = ...) -> None: ...

class DeleteKubernetesServerRequest(_message.Message):
    __slots__ = ["HostID", "Name"]
    HOSTID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    HostID: str
    Name: str
    def __init__(self, HostID: _Optional[str] = ..., Name: _Optional[str] = ...) -> None: ...

class DeleteAllKubernetesServersRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class UpsertDatabaseServerRequest(_message.Message):
    __slots__ = ["Server"]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    Server: _types_pb2.DatabaseServerV3
    def __init__(self, Server: _Optional[_Union[_types_pb2.DatabaseServerV3, _Mapping]] = ...) -> None: ...

class DeleteDatabaseServerRequest(_message.Message):
    __slots__ = ["Namespace", "HostID", "Name"]
    NAMESPACE_FIELD_NUMBER: _ClassVar[int]
    HOSTID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Namespace: str
    HostID: str
    Name: str
    def __init__(self, Namespace: _Optional[str] = ..., HostID: _Optional[str] = ..., Name: _Optional[str] = ...) -> None: ...

class DeleteAllDatabaseServersRequest(_message.Message):
    __slots__ = ["Namespace"]
    NAMESPACE_FIELD_NUMBER: _ClassVar[int]
    Namespace: str
    def __init__(self, Namespace: _Optional[str] = ...) -> None: ...

class DatabaseServiceV1List(_message.Message):
    __slots__ = ["Services"]
    SERVICES_FIELD_NUMBER: _ClassVar[int]
    Services: _containers.RepeatedCompositeFieldContainer[_types_pb2.DatabaseServiceV1]
    def __init__(self, Services: _Optional[_Iterable[_Union[_types_pb2.DatabaseServiceV1, _Mapping]]] = ...) -> None: ...

class UpsertDatabaseServiceRequest(_message.Message):
    __slots__ = ["Service"]
    SERVICE_FIELD_NUMBER: _ClassVar[int]
    Service: _types_pb2.DatabaseServiceV1
    def __init__(self, Service: _Optional[_Union[_types_pb2.DatabaseServiceV1, _Mapping]] = ...) -> None: ...

class DeleteAllDatabaseServicesRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class DatabaseCSRRequest(_message.Message):
    __slots__ = ["CSR", "ClusterName"]
    CSR_FIELD_NUMBER: _ClassVar[int]
    CLUSTERNAME_FIELD_NUMBER: _ClassVar[int]
    CSR: bytes
    ClusterName: str
    def __init__(self, CSR: _Optional[bytes] = ..., ClusterName: _Optional[str] = ...) -> None: ...

class DatabaseCSRResponse(_message.Message):
    __slots__ = ["Cert", "CACerts"]
    CERT_FIELD_NUMBER: _ClassVar[int]
    CACERTS_FIELD_NUMBER: _ClassVar[int]
    Cert: bytes
    CACerts: _containers.RepeatedScalarFieldContainer[bytes]
    def __init__(self, Cert: _Optional[bytes] = ..., CACerts: _Optional[_Iterable[bytes]] = ...) -> None: ...

class DatabaseCertRequest(_message.Message):
    __slots__ = ["CSR", "ServerName", "TTL", "ServerNames", "RequesterName", "CertificateExtensions", "CRLEndpoint"]
    class Requester(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = []
        UNSPECIFIED: _ClassVar[DatabaseCertRequest.Requester]
        TCTL: _ClassVar[DatabaseCertRequest.Requester]
    UNSPECIFIED: DatabaseCertRequest.Requester
    TCTL: DatabaseCertRequest.Requester
    class Extensions(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = []
        NORMAL: _ClassVar[DatabaseCertRequest.Extensions]
        WINDOWS_SMARTCARD: _ClassVar[DatabaseCertRequest.Extensions]
    NORMAL: DatabaseCertRequest.Extensions
    WINDOWS_SMARTCARD: DatabaseCertRequest.Extensions
    CSR_FIELD_NUMBER: _ClassVar[int]
    SERVERNAME_FIELD_NUMBER: _ClassVar[int]
    TTL_FIELD_NUMBER: _ClassVar[int]
    SERVERNAMES_FIELD_NUMBER: _ClassVar[int]
    REQUESTERNAME_FIELD_NUMBER: _ClassVar[int]
    CERTIFICATEEXTENSIONS_FIELD_NUMBER: _ClassVar[int]
    CRLENDPOINT_FIELD_NUMBER: _ClassVar[int]
    CSR: bytes
    ServerName: str
    TTL: int
    ServerNames: _containers.RepeatedScalarFieldContainer[str]
    RequesterName: DatabaseCertRequest.Requester
    CertificateExtensions: DatabaseCertRequest.Extensions
    CRLEndpoint: str
    def __init__(self, CSR: _Optional[bytes] = ..., ServerName: _Optional[str] = ..., TTL: _Optional[int] = ..., ServerNames: _Optional[_Iterable[str]] = ..., RequesterName: _Optional[_Union[DatabaseCertRequest.Requester, str]] = ..., CertificateExtensions: _Optional[_Union[DatabaseCertRequest.Extensions, str]] = ..., CRLEndpoint: _Optional[str] = ...) -> None: ...

class DatabaseCertResponse(_message.Message):
    __slots__ = ["Cert", "CACerts"]
    CERT_FIELD_NUMBER: _ClassVar[int]
    CACERTS_FIELD_NUMBER: _ClassVar[int]
    Cert: bytes
    CACerts: _containers.RepeatedScalarFieldContainer[bytes]
    def __init__(self, Cert: _Optional[bytes] = ..., CACerts: _Optional[_Iterable[bytes]] = ...) -> None: ...

class SnowflakeJWTRequest(_message.Message):
    __slots__ = ["AccountName", "UserName"]
    ACCOUNTNAME_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    AccountName: str
    UserName: str
    def __init__(self, AccountName: _Optional[str] = ..., UserName: _Optional[str] = ...) -> None: ...

class SnowflakeJWTResponse(_message.Message):
    __slots__ = ["Token"]
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    Token: str
    def __init__(self, Token: _Optional[str] = ...) -> None: ...

class GetRoleRequest(_message.Message):
    __slots__ = ["Name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Name: str
    def __init__(self, Name: _Optional[str] = ...) -> None: ...

class GetRolesResponse(_message.Message):
    __slots__ = ["Roles"]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    Roles: _containers.RepeatedCompositeFieldContainer[_types_pb2.RoleV6]
    def __init__(self, Roles: _Optional[_Iterable[_Union[_types_pb2.RoleV6, _Mapping]]] = ...) -> None: ...

class DeleteRoleRequest(_message.Message):
    __slots__ = ["Name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Name: str
    def __init__(self, Name: _Optional[str] = ...) -> None: ...

class MFAAuthenticateChallenge(_message.Message):
    __slots__ = ["TOTP", "WebauthnChallenge", "MFARequired"]
    TOTP_FIELD_NUMBER: _ClassVar[int]
    WEBAUTHNCHALLENGE_FIELD_NUMBER: _ClassVar[int]
    MFAREQUIRED_FIELD_NUMBER: _ClassVar[int]
    TOTP: TOTPChallenge
    WebauthnChallenge: _webauthn_pb2.CredentialAssertion
    MFARequired: MFARequired
    def __init__(self, TOTP: _Optional[_Union[TOTPChallenge, _Mapping]] = ..., WebauthnChallenge: _Optional[_Union[_webauthn_pb2.CredentialAssertion, _Mapping]] = ..., MFARequired: _Optional[_Union[MFARequired, str]] = ...) -> None: ...

class MFAAuthenticateResponse(_message.Message):
    __slots__ = ["TOTP", "Webauthn"]
    TOTP_FIELD_NUMBER: _ClassVar[int]
    WEBAUTHN_FIELD_NUMBER: _ClassVar[int]
    TOTP: TOTPResponse
    Webauthn: _webauthn_pb2.CredentialAssertionResponse
    def __init__(self, TOTP: _Optional[_Union[TOTPResponse, _Mapping]] = ..., Webauthn: _Optional[_Union[_webauthn_pb2.CredentialAssertionResponse, _Mapping]] = ...) -> None: ...

class TOTPChallenge(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class TOTPResponse(_message.Message):
    __slots__ = ["Code"]
    CODE_FIELD_NUMBER: _ClassVar[int]
    Code: str
    def __init__(self, Code: _Optional[str] = ...) -> None: ...

class MFARegisterChallenge(_message.Message):
    __slots__ = ["TOTP", "Webauthn"]
    TOTP_FIELD_NUMBER: _ClassVar[int]
    WEBAUTHN_FIELD_NUMBER: _ClassVar[int]
    TOTP: TOTPRegisterChallenge
    Webauthn: _webauthn_pb2.CredentialCreation
    def __init__(self, TOTP: _Optional[_Union[TOTPRegisterChallenge, _Mapping]] = ..., Webauthn: _Optional[_Union[_webauthn_pb2.CredentialCreation, _Mapping]] = ...) -> None: ...

class MFARegisterResponse(_message.Message):
    __slots__ = ["TOTP", "Webauthn"]
    TOTP_FIELD_NUMBER: _ClassVar[int]
    WEBAUTHN_FIELD_NUMBER: _ClassVar[int]
    TOTP: TOTPRegisterResponse
    Webauthn: _webauthn_pb2.CredentialCreationResponse
    def __init__(self, TOTP: _Optional[_Union[TOTPRegisterResponse, _Mapping]] = ..., Webauthn: _Optional[_Union[_webauthn_pb2.CredentialCreationResponse, _Mapping]] = ...) -> None: ...

class TOTPRegisterChallenge(_message.Message):
    __slots__ = ["Secret", "Issuer", "PeriodSeconds", "Algorithm", "Digits", "Account", "QRCode", "ID"]
    SECRET_FIELD_NUMBER: _ClassVar[int]
    ISSUER_FIELD_NUMBER: _ClassVar[int]
    PERIODSECONDS_FIELD_NUMBER: _ClassVar[int]
    ALGORITHM_FIELD_NUMBER: _ClassVar[int]
    DIGITS_FIELD_NUMBER: _ClassVar[int]
    ACCOUNT_FIELD_NUMBER: _ClassVar[int]
    QRCODE_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    Secret: str
    Issuer: str
    PeriodSeconds: int
    Algorithm: str
    Digits: int
    Account: str
    QRCode: bytes
    ID: str
    def __init__(self, Secret: _Optional[str] = ..., Issuer: _Optional[str] = ..., PeriodSeconds: _Optional[int] = ..., Algorithm: _Optional[str] = ..., Digits: _Optional[int] = ..., Account: _Optional[str] = ..., QRCode: _Optional[bytes] = ..., ID: _Optional[str] = ...) -> None: ...

class TOTPRegisterResponse(_message.Message):
    __slots__ = ["Code", "ID"]
    CODE_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    Code: str
    ID: str
    def __init__(self, Code: _Optional[str] = ..., ID: _Optional[str] = ...) -> None: ...

class AddMFADeviceRequest(_message.Message):
    __slots__ = ["Init", "ExistingMFAResponse", "NewMFARegisterResponse"]
    INIT_FIELD_NUMBER: _ClassVar[int]
    EXISTINGMFARESPONSE_FIELD_NUMBER: _ClassVar[int]
    NEWMFAREGISTERRESPONSE_FIELD_NUMBER: _ClassVar[int]
    Init: AddMFADeviceRequestInit
    ExistingMFAResponse: MFAAuthenticateResponse
    NewMFARegisterResponse: MFARegisterResponse
    def __init__(self, Init: _Optional[_Union[AddMFADeviceRequestInit, _Mapping]] = ..., ExistingMFAResponse: _Optional[_Union[MFAAuthenticateResponse, _Mapping]] = ..., NewMFARegisterResponse: _Optional[_Union[MFARegisterResponse, _Mapping]] = ...) -> None: ...

class AddMFADeviceResponse(_message.Message):
    __slots__ = ["ExistingMFAChallenge", "NewMFARegisterChallenge", "Ack"]
    EXISTINGMFACHALLENGE_FIELD_NUMBER: _ClassVar[int]
    NEWMFAREGISTERCHALLENGE_FIELD_NUMBER: _ClassVar[int]
    ACK_FIELD_NUMBER: _ClassVar[int]
    ExistingMFAChallenge: MFAAuthenticateChallenge
    NewMFARegisterChallenge: MFARegisterChallenge
    Ack: AddMFADeviceResponseAck
    def __init__(self, ExistingMFAChallenge: _Optional[_Union[MFAAuthenticateChallenge, _Mapping]] = ..., NewMFARegisterChallenge: _Optional[_Union[MFARegisterChallenge, _Mapping]] = ..., Ack: _Optional[_Union[AddMFADeviceResponseAck, _Mapping]] = ...) -> None: ...

class AddMFADeviceRequestInit(_message.Message):
    __slots__ = ["DeviceName", "DeviceType", "DeviceUsage"]
    DEVICENAME_FIELD_NUMBER: _ClassVar[int]
    DEVICETYPE_FIELD_NUMBER: _ClassVar[int]
    DEVICEUSAGE_FIELD_NUMBER: _ClassVar[int]
    DeviceName: str
    DeviceType: DeviceType
    DeviceUsage: DeviceUsage
    def __init__(self, DeviceName: _Optional[str] = ..., DeviceType: _Optional[_Union[DeviceType, str]] = ..., DeviceUsage: _Optional[_Union[DeviceUsage, str]] = ...) -> None: ...

class AddMFADeviceResponseAck(_message.Message):
    __slots__ = ["Device"]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    Device: _types_pb2.MFADevice
    def __init__(self, Device: _Optional[_Union[_types_pb2.MFADevice, _Mapping]] = ...) -> None: ...

class DeleteMFADeviceRequest(_message.Message):
    __slots__ = ["Init", "MFAResponse"]
    INIT_FIELD_NUMBER: _ClassVar[int]
    MFARESPONSE_FIELD_NUMBER: _ClassVar[int]
    Init: DeleteMFADeviceRequestInit
    MFAResponse: MFAAuthenticateResponse
    def __init__(self, Init: _Optional[_Union[DeleteMFADeviceRequestInit, _Mapping]] = ..., MFAResponse: _Optional[_Union[MFAAuthenticateResponse, _Mapping]] = ...) -> None: ...

class DeleteMFADeviceResponse(_message.Message):
    __slots__ = ["MFAChallenge", "Ack"]
    MFACHALLENGE_FIELD_NUMBER: _ClassVar[int]
    ACK_FIELD_NUMBER: _ClassVar[int]
    MFAChallenge: MFAAuthenticateChallenge
    Ack: DeleteMFADeviceResponseAck
    def __init__(self, MFAChallenge: _Optional[_Union[MFAAuthenticateChallenge, _Mapping]] = ..., Ack: _Optional[_Union[DeleteMFADeviceResponseAck, _Mapping]] = ...) -> None: ...

class DeleteMFADeviceRequestInit(_message.Message):
    __slots__ = ["DeviceName"]
    DEVICENAME_FIELD_NUMBER: _ClassVar[int]
    DeviceName: str
    def __init__(self, DeviceName: _Optional[str] = ...) -> None: ...

class DeleteMFADeviceResponseAck(_message.Message):
    __slots__ = ["Device"]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    Device: _types_pb2.MFADevice
    def __init__(self, Device: _Optional[_Union[_types_pb2.MFADevice, _Mapping]] = ...) -> None: ...

class DeleteMFADeviceSyncRequest(_message.Message):
    __slots__ = ["TokenID", "DeviceName", "ExistingMFAResponse"]
    TOKENID_FIELD_NUMBER: _ClassVar[int]
    DEVICENAME_FIELD_NUMBER: _ClassVar[int]
    EXISTINGMFARESPONSE_FIELD_NUMBER: _ClassVar[int]
    TokenID: str
    DeviceName: str
    ExistingMFAResponse: MFAAuthenticateResponse
    def __init__(self, TokenID: _Optional[str] = ..., DeviceName: _Optional[str] = ..., ExistingMFAResponse: _Optional[_Union[MFAAuthenticateResponse, _Mapping]] = ...) -> None: ...

class AddMFADeviceSyncRequest(_message.Message):
    __slots__ = ["TokenID", "ContextUser", "NewDeviceName", "NewMFAResponse", "DeviceUsage"]
    TOKENID_FIELD_NUMBER: _ClassVar[int]
    CONTEXTUSER_FIELD_NUMBER: _ClassVar[int]
    NEWDEVICENAME_FIELD_NUMBER: _ClassVar[int]
    NEWMFARESPONSE_FIELD_NUMBER: _ClassVar[int]
    DEVICEUSAGE_FIELD_NUMBER: _ClassVar[int]
    TokenID: str
    ContextUser: ContextUser
    NewDeviceName: str
    NewMFAResponse: MFARegisterResponse
    DeviceUsage: DeviceUsage
    def __init__(self, TokenID: _Optional[str] = ..., ContextUser: _Optional[_Union[ContextUser, _Mapping]] = ..., NewDeviceName: _Optional[str] = ..., NewMFAResponse: _Optional[_Union[MFARegisterResponse, _Mapping]] = ..., DeviceUsage: _Optional[_Union[DeviceUsage, str]] = ...) -> None: ...

class AddMFADeviceSyncResponse(_message.Message):
    __slots__ = ["Device"]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    Device: _types_pb2.MFADevice
    def __init__(self, Device: _Optional[_Union[_types_pb2.MFADevice, _Mapping]] = ...) -> None: ...

class GetMFADevicesRequest(_message.Message):
    __slots__ = ["TokenID"]
    TOKENID_FIELD_NUMBER: _ClassVar[int]
    TokenID: str
    def __init__(self, TokenID: _Optional[str] = ...) -> None: ...

class GetMFADevicesResponse(_message.Message):
    __slots__ = ["Devices"]
    DEVICES_FIELD_NUMBER: _ClassVar[int]
    Devices: _containers.RepeatedCompositeFieldContainer[_types_pb2.MFADevice]
    def __init__(self, Devices: _Optional[_Iterable[_Union[_types_pb2.MFADevice, _Mapping]]] = ...) -> None: ...

class UserSingleUseCertsRequest(_message.Message):
    __slots__ = ["Init", "MFAResponse"]
    INIT_FIELD_NUMBER: _ClassVar[int]
    MFARESPONSE_FIELD_NUMBER: _ClassVar[int]
    Init: UserCertsRequest
    MFAResponse: MFAAuthenticateResponse
    def __init__(self, Init: _Optional[_Union[UserCertsRequest, _Mapping]] = ..., MFAResponse: _Optional[_Union[MFAAuthenticateResponse, _Mapping]] = ...) -> None: ...

class UserSingleUseCertsResponse(_message.Message):
    __slots__ = ["MFAChallenge", "Cert"]
    MFACHALLENGE_FIELD_NUMBER: _ClassVar[int]
    CERT_FIELD_NUMBER: _ClassVar[int]
    MFAChallenge: MFAAuthenticateChallenge
    Cert: SingleUseUserCert
    def __init__(self, MFAChallenge: _Optional[_Union[MFAAuthenticateChallenge, _Mapping]] = ..., Cert: _Optional[_Union[SingleUseUserCert, _Mapping]] = ...) -> None: ...

class IsMFARequiredRequest(_message.Message):
    __slots__ = ["KubernetesCluster", "Database", "Node", "WindowsDesktop"]
    KUBERNETESCLUSTER_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    NODE_FIELD_NUMBER: _ClassVar[int]
    WINDOWSDESKTOP_FIELD_NUMBER: _ClassVar[int]
    KubernetesCluster: str
    Database: RouteToDatabase
    Node: NodeLogin
    WindowsDesktop: RouteToWindowsDesktop
    def __init__(self, KubernetesCluster: _Optional[str] = ..., Database: _Optional[_Union[RouteToDatabase, _Mapping]] = ..., Node: _Optional[_Union[NodeLogin, _Mapping]] = ..., WindowsDesktop: _Optional[_Union[RouteToWindowsDesktop, _Mapping]] = ...) -> None: ...

class StreamSessionEventsRequest(_message.Message):
    __slots__ = ["SessionID", "StartIndex"]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    STARTINDEX_FIELD_NUMBER: _ClassVar[int]
    SessionID: str
    StartIndex: int
    def __init__(self, SessionID: _Optional[str] = ..., StartIndex: _Optional[int] = ...) -> None: ...

class NodeLogin(_message.Message):
    __slots__ = ["Node", "Login"]
    NODE_FIELD_NUMBER: _ClassVar[int]
    LOGIN_FIELD_NUMBER: _ClassVar[int]
    Node: str
    Login: str
    def __init__(self, Node: _Optional[str] = ..., Login: _Optional[str] = ...) -> None: ...

class IsMFARequiredResponse(_message.Message):
    __slots__ = ["Required", "MFARequired"]
    REQUIRED_FIELD_NUMBER: _ClassVar[int]
    MFAREQUIRED_FIELD_NUMBER: _ClassVar[int]
    Required: bool
    MFARequired: MFARequired
    def __init__(self, Required: bool = ..., MFARequired: _Optional[_Union[MFARequired, str]] = ...) -> None: ...

class SingleUseUserCert(_message.Message):
    __slots__ = ["SSH", "TLS"]
    SSH_FIELD_NUMBER: _ClassVar[int]
    TLS_FIELD_NUMBER: _ClassVar[int]
    SSH: bytes
    TLS: bytes
    def __init__(self, SSH: _Optional[bytes] = ..., TLS: _Optional[bytes] = ...) -> None: ...

class GetEventsRequest(_message.Message):
    __slots__ = ["Namespace", "StartDate", "EndDate", "EventTypes", "Limit", "StartKey", "Order"]
    NAMESPACE_FIELD_NUMBER: _ClassVar[int]
    STARTDATE_FIELD_NUMBER: _ClassVar[int]
    ENDDATE_FIELD_NUMBER: _ClassVar[int]
    EVENTTYPES_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    STARTKEY_FIELD_NUMBER: _ClassVar[int]
    ORDER_FIELD_NUMBER: _ClassVar[int]
    Namespace: str
    StartDate: _timestamp_pb2.Timestamp
    EndDate: _timestamp_pb2.Timestamp
    EventTypes: _containers.RepeatedScalarFieldContainer[str]
    Limit: int
    StartKey: str
    Order: Order
    def __init__(self, Namespace: _Optional[str] = ..., StartDate: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., EndDate: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., EventTypes: _Optional[_Iterable[str]] = ..., Limit: _Optional[int] = ..., StartKey: _Optional[str] = ..., Order: _Optional[_Union[Order, str]] = ...) -> None: ...

class GetSessionEventsRequest(_message.Message):
    __slots__ = ["StartDate", "EndDate", "Limit", "StartKey", "Order"]
    STARTDATE_FIELD_NUMBER: _ClassVar[int]
    ENDDATE_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    STARTKEY_FIELD_NUMBER: _ClassVar[int]
    ORDER_FIELD_NUMBER: _ClassVar[int]
    StartDate: _timestamp_pb2.Timestamp
    EndDate: _timestamp_pb2.Timestamp
    Limit: int
    StartKey: str
    Order: Order
    def __init__(self, StartDate: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., EndDate: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Limit: _Optional[int] = ..., StartKey: _Optional[str] = ..., Order: _Optional[_Union[Order, str]] = ...) -> None: ...

class Events(_message.Message):
    __slots__ = ["Items", "LastKey"]
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    LASTKEY_FIELD_NUMBER: _ClassVar[int]
    Items: _containers.RepeatedCompositeFieldContainer[_events_pb2.OneOf]
    LastKey: str
    def __init__(self, Items: _Optional[_Iterable[_Union[_events_pb2.OneOf, _Mapping]]] = ..., LastKey: _Optional[str] = ...) -> None: ...

class GetLocksRequest(_message.Message):
    __slots__ = ["Targets", "InForceOnly"]
    TARGETS_FIELD_NUMBER: _ClassVar[int]
    INFORCEONLY_FIELD_NUMBER: _ClassVar[int]
    Targets: _containers.RepeatedCompositeFieldContainer[_types_pb2.LockTarget]
    InForceOnly: bool
    def __init__(self, Targets: _Optional[_Iterable[_Union[_types_pb2.LockTarget, _Mapping]]] = ..., InForceOnly: bool = ...) -> None: ...

class GetLocksResponse(_message.Message):
    __slots__ = ["Locks"]
    LOCKS_FIELD_NUMBER: _ClassVar[int]
    Locks: _containers.RepeatedCompositeFieldContainer[_types_pb2.LockV2]
    def __init__(self, Locks: _Optional[_Iterable[_Union[_types_pb2.LockV2, _Mapping]]] = ...) -> None: ...

class GetLockRequest(_message.Message):
    __slots__ = ["Name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Name: str
    def __init__(self, Name: _Optional[str] = ...) -> None: ...

class DeleteLockRequest(_message.Message):
    __slots__ = ["Name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Name: str
    def __init__(self, Name: _Optional[str] = ...) -> None: ...

class ReplaceRemoteLocksRequest(_message.Message):
    __slots__ = ["ClusterName", "Locks"]
    CLUSTERNAME_FIELD_NUMBER: _ClassVar[int]
    LOCKS_FIELD_NUMBER: _ClassVar[int]
    ClusterName: str
    Locks: _containers.RepeatedCompositeFieldContainer[_types_pb2.LockV2]
    def __init__(self, ClusterName: _Optional[str] = ..., Locks: _Optional[_Iterable[_Union[_types_pb2.LockV2, _Mapping]]] = ...) -> None: ...

class GetWindowsDesktopServicesResponse(_message.Message):
    __slots__ = ["services"]
    SERVICES_FIELD_NUMBER: _ClassVar[int]
    services: _containers.RepeatedCompositeFieldContainer[_types_pb2.WindowsDesktopServiceV3]
    def __init__(self, services: _Optional[_Iterable[_Union[_types_pb2.WindowsDesktopServiceV3, _Mapping]]] = ...) -> None: ...

class GetWindowsDesktopServiceRequest(_message.Message):
    __slots__ = ["Name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Name: str
    def __init__(self, Name: _Optional[str] = ...) -> None: ...

class GetWindowsDesktopServiceResponse(_message.Message):
    __slots__ = ["service"]
    SERVICE_FIELD_NUMBER: _ClassVar[int]
    service: _types_pb2.WindowsDesktopServiceV3
    def __init__(self, service: _Optional[_Union[_types_pb2.WindowsDesktopServiceV3, _Mapping]] = ...) -> None: ...

class DeleteWindowsDesktopServiceRequest(_message.Message):
    __slots__ = ["Name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Name: str
    def __init__(self, Name: _Optional[str] = ...) -> None: ...

class GetWindowsDesktopsResponse(_message.Message):
    __slots__ = ["Desktops"]
    DESKTOPS_FIELD_NUMBER: _ClassVar[int]
    Desktops: _containers.RepeatedCompositeFieldContainer[_types_pb2.WindowsDesktopV3]
    def __init__(self, Desktops: _Optional[_Iterable[_Union[_types_pb2.WindowsDesktopV3, _Mapping]]] = ...) -> None: ...

class DeleteWindowsDesktopRequest(_message.Message):
    __slots__ = ["Name", "HostID"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    HOSTID_FIELD_NUMBER: _ClassVar[int]
    Name: str
    HostID: str
    def __init__(self, Name: _Optional[str] = ..., HostID: _Optional[str] = ...) -> None: ...

class WindowsDesktopCertRequest(_message.Message):
    __slots__ = ["CSR", "CRLEndpoint", "TTL"]
    CSR_FIELD_NUMBER: _ClassVar[int]
    CRLENDPOINT_FIELD_NUMBER: _ClassVar[int]
    TTL_FIELD_NUMBER: _ClassVar[int]
    CSR: bytes
    CRLEndpoint: str
    TTL: int
    def __init__(self, CSR: _Optional[bytes] = ..., CRLEndpoint: _Optional[str] = ..., TTL: _Optional[int] = ...) -> None: ...

class WindowsDesktopCertResponse(_message.Message):
    __slots__ = ["Cert"]
    CERT_FIELD_NUMBER: _ClassVar[int]
    Cert: bytes
    def __init__(self, Cert: _Optional[bytes] = ...) -> None: ...

class ListSAMLIdPServiceProvidersRequest(_message.Message):
    __slots__ = ["Limit", "NextKey"]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    NEXTKEY_FIELD_NUMBER: _ClassVar[int]
    Limit: int
    NextKey: str
    def __init__(self, Limit: _Optional[int] = ..., NextKey: _Optional[str] = ...) -> None: ...

class ListSAMLIdPServiceProvidersResponse(_message.Message):
    __slots__ = ["ServiceProviders", "NextKey", "TotalCount"]
    SERVICEPROVIDERS_FIELD_NUMBER: _ClassVar[int]
    NEXTKEY_FIELD_NUMBER: _ClassVar[int]
    TOTALCOUNT_FIELD_NUMBER: _ClassVar[int]
    ServiceProviders: _containers.RepeatedCompositeFieldContainer[_types_pb2.SAMLIdPServiceProviderV1]
    NextKey: str
    TotalCount: int
    def __init__(self, ServiceProviders: _Optional[_Iterable[_Union[_types_pb2.SAMLIdPServiceProviderV1, _Mapping]]] = ..., NextKey: _Optional[str] = ..., TotalCount: _Optional[int] = ...) -> None: ...

class GetSAMLIdPServiceProviderRequest(_message.Message):
    __slots__ = ["Name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Name: str
    def __init__(self, Name: _Optional[str] = ...) -> None: ...

class DeleteSAMLIdPServiceProviderRequest(_message.Message):
    __slots__ = ["Name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Name: str
    def __init__(self, Name: _Optional[str] = ...) -> None: ...

class ListUserGroupsRequest(_message.Message):
    __slots__ = ["Limit", "NextKey"]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    NEXTKEY_FIELD_NUMBER: _ClassVar[int]
    Limit: int
    NextKey: str
    def __init__(self, Limit: _Optional[int] = ..., NextKey: _Optional[str] = ...) -> None: ...

class ListUserGroupsResponse(_message.Message):
    __slots__ = ["UserGroups", "NextKey", "TotalCount"]
    USERGROUPS_FIELD_NUMBER: _ClassVar[int]
    NEXTKEY_FIELD_NUMBER: _ClassVar[int]
    TOTALCOUNT_FIELD_NUMBER: _ClassVar[int]
    UserGroups: _containers.RepeatedCompositeFieldContainer[_types_pb2.UserGroupV1]
    NextKey: str
    TotalCount: int
    def __init__(self, UserGroups: _Optional[_Iterable[_Union[_types_pb2.UserGroupV1, _Mapping]]] = ..., NextKey: _Optional[str] = ..., TotalCount: _Optional[int] = ...) -> None: ...

class GetUserGroupRequest(_message.Message):
    __slots__ = ["Name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Name: str
    def __init__(self, Name: _Optional[str] = ...) -> None: ...

class DeleteUserGroupRequest(_message.Message):
    __slots__ = ["Name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Name: str
    def __init__(self, Name: _Optional[str] = ...) -> None: ...

class CertAuthorityRequest(_message.Message):
    __slots__ = ["Type"]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    Type: str
    def __init__(self, Type: _Optional[str] = ...) -> None: ...

class CRL(_message.Message):
    __slots__ = ["CRL"]
    CRL_FIELD_NUMBER: _ClassVar[int]
    CRL: bytes
    def __init__(self, CRL: _Optional[bytes] = ...) -> None: ...

class ChangeUserAuthenticationRequest(_message.Message):
    __slots__ = ["TokenID", "NewPassword", "NewMFARegisterResponse", "NewDeviceName", "LoginIP"]
    TOKENID_FIELD_NUMBER: _ClassVar[int]
    NEWPASSWORD_FIELD_NUMBER: _ClassVar[int]
    NEWMFAREGISTERRESPONSE_FIELD_NUMBER: _ClassVar[int]
    NEWDEVICENAME_FIELD_NUMBER: _ClassVar[int]
    LOGINIP_FIELD_NUMBER: _ClassVar[int]
    TokenID: str
    NewPassword: bytes
    NewMFARegisterResponse: MFARegisterResponse
    NewDeviceName: str
    LoginIP: str
    def __init__(self, TokenID: _Optional[str] = ..., NewPassword: _Optional[bytes] = ..., NewMFARegisterResponse: _Optional[_Union[MFARegisterResponse, _Mapping]] = ..., NewDeviceName: _Optional[str] = ..., LoginIP: _Optional[str] = ...) -> None: ...

class ChangeUserAuthenticationResponse(_message.Message):
    __slots__ = ["WebSession", "Recovery", "PrivateKeyPolicyEnabled"]
    WEBSESSION_FIELD_NUMBER: _ClassVar[int]
    RECOVERY_FIELD_NUMBER: _ClassVar[int]
    PRIVATEKEYPOLICYENABLED_FIELD_NUMBER: _ClassVar[int]
    WebSession: _types_pb2.WebSessionV2
    Recovery: RecoveryCodes
    PrivateKeyPolicyEnabled: bool
    def __init__(self, WebSession: _Optional[_Union[_types_pb2.WebSessionV2, _Mapping]] = ..., Recovery: _Optional[_Union[RecoveryCodes, _Mapping]] = ..., PrivateKeyPolicyEnabled: bool = ...) -> None: ...

class StartAccountRecoveryRequest(_message.Message):
    __slots__ = ["Username", "RecoveryCode", "RecoverType"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    RECOVERYCODE_FIELD_NUMBER: _ClassVar[int]
    RECOVERTYPE_FIELD_NUMBER: _ClassVar[int]
    Username: str
    RecoveryCode: bytes
    RecoverType: _types_pb2.UserTokenUsage
    def __init__(self, Username: _Optional[str] = ..., RecoveryCode: _Optional[bytes] = ..., RecoverType: _Optional[_Union[_types_pb2.UserTokenUsage, str]] = ...) -> None: ...

class VerifyAccountRecoveryRequest(_message.Message):
    __slots__ = ["RecoveryStartTokenID", "Username", "Password", "MFAAuthenticateResponse"]
    RECOVERYSTARTTOKENID_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    MFAAUTHENTICATERESPONSE_FIELD_NUMBER: _ClassVar[int]
    RecoveryStartTokenID: str
    Username: str
    Password: bytes
    MFAAuthenticateResponse: MFAAuthenticateResponse
    def __init__(self, RecoveryStartTokenID: _Optional[str] = ..., Username: _Optional[str] = ..., Password: _Optional[bytes] = ..., MFAAuthenticateResponse: _Optional[_Union[MFAAuthenticateResponse, _Mapping]] = ...) -> None: ...

class CompleteAccountRecoveryRequest(_message.Message):
    __slots__ = ["RecoveryApprovedTokenID", "NewDeviceName", "NewPassword", "NewMFAResponse"]
    RECOVERYAPPROVEDTOKENID_FIELD_NUMBER: _ClassVar[int]
    NEWDEVICENAME_FIELD_NUMBER: _ClassVar[int]
    NEWPASSWORD_FIELD_NUMBER: _ClassVar[int]
    NEWMFARESPONSE_FIELD_NUMBER: _ClassVar[int]
    RecoveryApprovedTokenID: str
    NewDeviceName: str
    NewPassword: bytes
    NewMFAResponse: MFARegisterResponse
    def __init__(self, RecoveryApprovedTokenID: _Optional[str] = ..., NewDeviceName: _Optional[str] = ..., NewPassword: _Optional[bytes] = ..., NewMFAResponse: _Optional[_Union[MFARegisterResponse, _Mapping]] = ...) -> None: ...

class RecoveryCodes(_message.Message):
    __slots__ = ["Codes", "Created"]
    CODES_FIELD_NUMBER: _ClassVar[int]
    CREATED_FIELD_NUMBER: _ClassVar[int]
    Codes: _containers.RepeatedScalarFieldContainer[str]
    Created: _timestamp_pb2.Timestamp
    def __init__(self, Codes: _Optional[_Iterable[str]] = ..., Created: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class CreateAccountRecoveryCodesRequest(_message.Message):
    __slots__ = ["TokenID"]
    TOKENID_FIELD_NUMBER: _ClassVar[int]
    TokenID: str
    def __init__(self, TokenID: _Optional[str] = ...) -> None: ...

class GetAccountRecoveryTokenRequest(_message.Message):
    __slots__ = ["RecoveryTokenID"]
    RECOVERYTOKENID_FIELD_NUMBER: _ClassVar[int]
    RecoveryTokenID: str
    def __init__(self, RecoveryTokenID: _Optional[str] = ...) -> None: ...

class GetAccountRecoveryCodesRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class UserCredentials(_message.Message):
    __slots__ = ["Username", "Password"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    Username: str
    Password: bytes
    def __init__(self, Username: _Optional[str] = ..., Password: _Optional[bytes] = ...) -> None: ...

class ContextUser(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class Passwordless(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class CreateAuthenticateChallengeRequest(_message.Message):
    __slots__ = ["UserCredentials", "RecoveryStartTokenID", "ContextUser", "Passwordless", "MFARequiredCheck"]
    USERCREDENTIALS_FIELD_NUMBER: _ClassVar[int]
    RECOVERYSTARTTOKENID_FIELD_NUMBER: _ClassVar[int]
    CONTEXTUSER_FIELD_NUMBER: _ClassVar[int]
    PASSWORDLESS_FIELD_NUMBER: _ClassVar[int]
    MFAREQUIREDCHECK_FIELD_NUMBER: _ClassVar[int]
    UserCredentials: UserCredentials
    RecoveryStartTokenID: str
    ContextUser: ContextUser
    Passwordless: Passwordless
    MFARequiredCheck: IsMFARequiredRequest
    def __init__(self, UserCredentials: _Optional[_Union[UserCredentials, _Mapping]] = ..., RecoveryStartTokenID: _Optional[str] = ..., ContextUser: _Optional[_Union[ContextUser, _Mapping]] = ..., Passwordless: _Optional[_Union[Passwordless, _Mapping]] = ..., MFARequiredCheck: _Optional[_Union[IsMFARequiredRequest, _Mapping]] = ...) -> None: ...

class CreatePrivilegeTokenRequest(_message.Message):
    __slots__ = ["ExistingMFAResponse"]
    EXISTINGMFARESPONSE_FIELD_NUMBER: _ClassVar[int]
    ExistingMFAResponse: MFAAuthenticateResponse
    def __init__(self, ExistingMFAResponse: _Optional[_Union[MFAAuthenticateResponse, _Mapping]] = ...) -> None: ...

class CreateRegisterChallengeRequest(_message.Message):
    __slots__ = ["TokenID", "ExistingMFAResponse", "DeviceType", "DeviceUsage"]
    TOKENID_FIELD_NUMBER: _ClassVar[int]
    EXISTINGMFARESPONSE_FIELD_NUMBER: _ClassVar[int]
    DEVICETYPE_FIELD_NUMBER: _ClassVar[int]
    DEVICEUSAGE_FIELD_NUMBER: _ClassVar[int]
    TokenID: str
    ExistingMFAResponse: MFAAuthenticateResponse
    DeviceType: DeviceType
    DeviceUsage: DeviceUsage
    def __init__(self, TokenID: _Optional[str] = ..., ExistingMFAResponse: _Optional[_Union[MFAAuthenticateResponse, _Mapping]] = ..., DeviceType: _Optional[_Union[DeviceType, str]] = ..., DeviceUsage: _Optional[_Union[DeviceUsage, str]] = ...) -> None: ...

class PaginatedResource(_message.Message):
    __slots__ = ["DatabaseServer", "AppServer", "Node", "WindowsDesktop", "KubeCluster", "KubernetesServer", "WindowsDesktopService", "DatabaseService", "UserGroup", "AppServerOrSAMLIdPServiceProvider"]
    DATABASESERVER_FIELD_NUMBER: _ClassVar[int]
    APPSERVER_FIELD_NUMBER: _ClassVar[int]
    NODE_FIELD_NUMBER: _ClassVar[int]
    WINDOWSDESKTOP_FIELD_NUMBER: _ClassVar[int]
    KUBECLUSTER_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESSERVER_FIELD_NUMBER: _ClassVar[int]
    WINDOWSDESKTOPSERVICE_FIELD_NUMBER: _ClassVar[int]
    DATABASESERVICE_FIELD_NUMBER: _ClassVar[int]
    USERGROUP_FIELD_NUMBER: _ClassVar[int]
    APPSERVERORSAMLIDPSERVICEPROVIDER_FIELD_NUMBER: _ClassVar[int]
    DatabaseServer: _types_pb2.DatabaseServerV3
    AppServer: _types_pb2.AppServerV3
    Node: _types_pb2.ServerV2
    WindowsDesktop: _types_pb2.WindowsDesktopV3
    KubeCluster: _types_pb2.KubernetesClusterV3
    KubernetesServer: _types_pb2.KubernetesServerV3
    WindowsDesktopService: _types_pb2.WindowsDesktopServiceV3
    DatabaseService: _types_pb2.DatabaseServiceV1
    UserGroup: _types_pb2.UserGroupV1
    AppServerOrSAMLIdPServiceProvider: _types_pb2.AppServerOrSAMLIdPServiceProviderV1
    def __init__(self, DatabaseServer: _Optional[_Union[_types_pb2.DatabaseServerV3, _Mapping]] = ..., AppServer: _Optional[_Union[_types_pb2.AppServerV3, _Mapping]] = ..., Node: _Optional[_Union[_types_pb2.ServerV2, _Mapping]] = ..., WindowsDesktop: _Optional[_Union[_types_pb2.WindowsDesktopV3, _Mapping]] = ..., KubeCluster: _Optional[_Union[_types_pb2.KubernetesClusterV3, _Mapping]] = ..., KubernetesServer: _Optional[_Union[_types_pb2.KubernetesServerV3, _Mapping]] = ..., WindowsDesktopService: _Optional[_Union[_types_pb2.WindowsDesktopServiceV3, _Mapping]] = ..., DatabaseService: _Optional[_Union[_types_pb2.DatabaseServiceV1, _Mapping]] = ..., UserGroup: _Optional[_Union[_types_pb2.UserGroupV1, _Mapping]] = ..., AppServerOrSAMLIdPServiceProvider: _Optional[_Union[_types_pb2.AppServerOrSAMLIdPServiceProviderV1, _Mapping]] = ...) -> None: ...

class ListUnifiedResourcesRequest(_message.Message):
    __slots__ = ["Kinds", "Limit", "StartKey", "Labels", "PredicateExpression", "SearchKeywords", "SortBy", "WindowsDesktopFilter", "UseSearchAsRoles", "UsePreviewAsRoles"]
    class LabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    KINDS_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    STARTKEY_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    PREDICATEEXPRESSION_FIELD_NUMBER: _ClassVar[int]
    SEARCHKEYWORDS_FIELD_NUMBER: _ClassVar[int]
    SORTBY_FIELD_NUMBER: _ClassVar[int]
    WINDOWSDESKTOPFILTER_FIELD_NUMBER: _ClassVar[int]
    USESEARCHASROLES_FIELD_NUMBER: _ClassVar[int]
    USEPREVIEWASROLES_FIELD_NUMBER: _ClassVar[int]
    Kinds: _containers.RepeatedScalarFieldContainer[str]
    Limit: int
    StartKey: str
    Labels: _containers.ScalarMap[str, str]
    PredicateExpression: str
    SearchKeywords: _containers.RepeatedScalarFieldContainer[str]
    SortBy: _types_pb2.SortBy
    WindowsDesktopFilter: _types_pb2.WindowsDesktopFilter
    UseSearchAsRoles: bool
    UsePreviewAsRoles: bool
    def __init__(self, Kinds: _Optional[_Iterable[str]] = ..., Limit: _Optional[int] = ..., StartKey: _Optional[str] = ..., Labels: _Optional[_Mapping[str, str]] = ..., PredicateExpression: _Optional[str] = ..., SearchKeywords: _Optional[_Iterable[str]] = ..., SortBy: _Optional[_Union[_types_pb2.SortBy, _Mapping]] = ..., WindowsDesktopFilter: _Optional[_Union[_types_pb2.WindowsDesktopFilter, _Mapping]] = ..., UseSearchAsRoles: bool = ..., UsePreviewAsRoles: bool = ...) -> None: ...

class ListUnifiedResourcesResponse(_message.Message):
    __slots__ = ["Resources", "NextKey"]
    RESOURCES_FIELD_NUMBER: _ClassVar[int]
    NEXTKEY_FIELD_NUMBER: _ClassVar[int]
    Resources: _containers.RepeatedCompositeFieldContainer[PaginatedResource]
    NextKey: str
    def __init__(self, Resources: _Optional[_Iterable[_Union[PaginatedResource, _Mapping]]] = ..., NextKey: _Optional[str] = ...) -> None: ...

class ListResourcesRequest(_message.Message):
    __slots__ = ["ResourceType", "Namespace", "Limit", "StartKey", "Labels", "PredicateExpression", "SearchKeywords", "SortBy", "NeedTotalCount", "WindowsDesktopFilter", "UseSearchAsRoles", "UsePreviewAsRoles"]
    class LabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    RESOURCETYPE_FIELD_NUMBER: _ClassVar[int]
    NAMESPACE_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    STARTKEY_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    PREDICATEEXPRESSION_FIELD_NUMBER: _ClassVar[int]
    SEARCHKEYWORDS_FIELD_NUMBER: _ClassVar[int]
    SORTBY_FIELD_NUMBER: _ClassVar[int]
    NEEDTOTALCOUNT_FIELD_NUMBER: _ClassVar[int]
    WINDOWSDESKTOPFILTER_FIELD_NUMBER: _ClassVar[int]
    USESEARCHASROLES_FIELD_NUMBER: _ClassVar[int]
    USEPREVIEWASROLES_FIELD_NUMBER: _ClassVar[int]
    ResourceType: str
    Namespace: str
    Limit: int
    StartKey: str
    Labels: _containers.ScalarMap[str, str]
    PredicateExpression: str
    SearchKeywords: _containers.RepeatedScalarFieldContainer[str]
    SortBy: _types_pb2.SortBy
    NeedTotalCount: bool
    WindowsDesktopFilter: _types_pb2.WindowsDesktopFilter
    UseSearchAsRoles: bool
    UsePreviewAsRoles: bool
    def __init__(self, ResourceType: _Optional[str] = ..., Namespace: _Optional[str] = ..., Limit: _Optional[int] = ..., StartKey: _Optional[str] = ..., Labels: _Optional[_Mapping[str, str]] = ..., PredicateExpression: _Optional[str] = ..., SearchKeywords: _Optional[_Iterable[str]] = ..., SortBy: _Optional[_Union[_types_pb2.SortBy, _Mapping]] = ..., NeedTotalCount: bool = ..., WindowsDesktopFilter: _Optional[_Union[_types_pb2.WindowsDesktopFilter, _Mapping]] = ..., UseSearchAsRoles: bool = ..., UsePreviewAsRoles: bool = ...) -> None: ...

class GetSSHTargetsRequest(_message.Message):
    __slots__ = ["Host", "Port"]
    HOST_FIELD_NUMBER: _ClassVar[int]
    PORT_FIELD_NUMBER: _ClassVar[int]
    Host: str
    Port: str
    def __init__(self, Host: _Optional[str] = ..., Port: _Optional[str] = ...) -> None: ...

class GetSSHTargetsResponse(_message.Message):
    __slots__ = ["Servers"]
    SERVERS_FIELD_NUMBER: _ClassVar[int]
    Servers: _containers.RepeatedCompositeFieldContainer[_types_pb2.ServerV2]
    def __init__(self, Servers: _Optional[_Iterable[_Union[_types_pb2.ServerV2, _Mapping]]] = ...) -> None: ...

class ListResourcesResponse(_message.Message):
    __slots__ = ["Resources", "NextKey", "TotalCount"]
    RESOURCES_FIELD_NUMBER: _ClassVar[int]
    NEXTKEY_FIELD_NUMBER: _ClassVar[int]
    TOTALCOUNT_FIELD_NUMBER: _ClassVar[int]
    Resources: _containers.RepeatedCompositeFieldContainer[PaginatedResource]
    NextKey: str
    TotalCount: int
    def __init__(self, Resources: _Optional[_Iterable[_Union[PaginatedResource, _Mapping]]] = ..., NextKey: _Optional[str] = ..., TotalCount: _Optional[int] = ...) -> None: ...

class CreateSessionTrackerRequest(_message.Message):
    __slots__ = ["SessionTracker"]
    SESSIONTRACKER_FIELD_NUMBER: _ClassVar[int]
    SessionTracker: _types_pb2.SessionTrackerV1
    def __init__(self, SessionTracker: _Optional[_Union[_types_pb2.SessionTrackerV1, _Mapping]] = ...) -> None: ...

class GetSessionTrackerRequest(_message.Message):
    __slots__ = ["SessionID"]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    SessionID: str
    def __init__(self, SessionID: _Optional[str] = ...) -> None: ...

class RemoveSessionTrackerRequest(_message.Message):
    __slots__ = ["SessionID"]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    SessionID: str
    def __init__(self, SessionID: _Optional[str] = ...) -> None: ...

class SessionTrackerUpdateState(_message.Message):
    __slots__ = ["State"]
    STATE_FIELD_NUMBER: _ClassVar[int]
    State: _types_pb2.SessionState
    def __init__(self, State: _Optional[_Union[_types_pb2.SessionState, str]] = ...) -> None: ...

class SessionTrackerAddParticipant(_message.Message):
    __slots__ = ["Participant"]
    PARTICIPANT_FIELD_NUMBER: _ClassVar[int]
    Participant: _types_pb2.Participant
    def __init__(self, Participant: _Optional[_Union[_types_pb2.Participant, _Mapping]] = ...) -> None: ...

class SessionTrackerRemoveParticipant(_message.Message):
    __slots__ = ["ParticipantID"]
    PARTICIPANTID_FIELD_NUMBER: _ClassVar[int]
    ParticipantID: str
    def __init__(self, ParticipantID: _Optional[str] = ...) -> None: ...

class SessionTrackerUpdateExpiry(_message.Message):
    __slots__ = ["Expires"]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    Expires: _timestamp_pb2.Timestamp
    def __init__(self, Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class UpdateSessionTrackerRequest(_message.Message):
    __slots__ = ["SessionID", "UpdateState", "AddParticipant", "RemoveParticipant", "UpdateExpiry"]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    UPDATESTATE_FIELD_NUMBER: _ClassVar[int]
    ADDPARTICIPANT_FIELD_NUMBER: _ClassVar[int]
    REMOVEPARTICIPANT_FIELD_NUMBER: _ClassVar[int]
    UPDATEEXPIRY_FIELD_NUMBER: _ClassVar[int]
    SessionID: str
    UpdateState: SessionTrackerUpdateState
    AddParticipant: SessionTrackerAddParticipant
    RemoveParticipant: SessionTrackerRemoveParticipant
    UpdateExpiry: SessionTrackerUpdateExpiry
    def __init__(self, SessionID: _Optional[str] = ..., UpdateState: _Optional[_Union[SessionTrackerUpdateState, _Mapping]] = ..., AddParticipant: _Optional[_Union[SessionTrackerAddParticipant, _Mapping]] = ..., RemoveParticipant: _Optional[_Union[SessionTrackerRemoveParticipant, _Mapping]] = ..., UpdateExpiry: _Optional[_Union[SessionTrackerUpdateExpiry, _Mapping]] = ...) -> None: ...

class PresenceMFAChallengeRequest(_message.Message):
    __slots__ = ["SessionID"]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    SessionID: str
    def __init__(self, SessionID: _Optional[str] = ...) -> None: ...

class PresenceMFAChallengeSend(_message.Message):
    __slots__ = ["ChallengeRequest", "ChallengeResponse"]
    CHALLENGEREQUEST_FIELD_NUMBER: _ClassVar[int]
    CHALLENGERESPONSE_FIELD_NUMBER: _ClassVar[int]
    ChallengeRequest: PresenceMFAChallengeRequest
    ChallengeResponse: MFAAuthenticateResponse
    def __init__(self, ChallengeRequest: _Optional[_Union[PresenceMFAChallengeRequest, _Mapping]] = ..., ChallengeResponse: _Optional[_Union[MFAAuthenticateResponse, _Mapping]] = ...) -> None: ...

class GetDomainNameResponse(_message.Message):
    __slots__ = ["DomainName"]
    DOMAINNAME_FIELD_NUMBER: _ClassVar[int]
    DomainName: str
    def __init__(self, DomainName: _Optional[str] = ...) -> None: ...

class GetClusterCACertResponse(_message.Message):
    __slots__ = ["TLSCA"]
    TLSCA_FIELD_NUMBER: _ClassVar[int]
    TLSCA: bytes
    def __init__(self, TLSCA: _Optional[bytes] = ...) -> None: ...

class GetLicenseResponse(_message.Message):
    __slots__ = ["License"]
    LICENSE_FIELD_NUMBER: _ClassVar[int]
    License: bytes
    def __init__(self, License: _Optional[bytes] = ...) -> None: ...

class ListReleasesResponse(_message.Message):
    __slots__ = ["releases"]
    RELEASES_FIELD_NUMBER: _ClassVar[int]
    releases: _containers.RepeatedCompositeFieldContainer[_types_pb2.Release]
    def __init__(self, releases: _Optional[_Iterable[_Union[_types_pb2.Release, _Mapping]]] = ...) -> None: ...

class GetOIDCAuthRequestRequest(_message.Message):
    __slots__ = ["StateToken"]
    STATETOKEN_FIELD_NUMBER: _ClassVar[int]
    StateToken: str
    def __init__(self, StateToken: _Optional[str] = ...) -> None: ...

class GetSAMLAuthRequestRequest(_message.Message):
    __slots__ = ["ID"]
    ID_FIELD_NUMBER: _ClassVar[int]
    ID: str
    def __init__(self, ID: _Optional[str] = ...) -> None: ...

class GetGithubAuthRequestRequest(_message.Message):
    __slots__ = ["StateToken"]
    STATETOKEN_FIELD_NUMBER: _ClassVar[int]
    StateToken: str
    def __init__(self, StateToken: _Optional[str] = ...) -> None: ...

class GetSSODiagnosticInfoRequest(_message.Message):
    __slots__ = ["AuthRequestKind", "AuthRequestID"]
    AUTHREQUESTKIND_FIELD_NUMBER: _ClassVar[int]
    AUTHREQUESTID_FIELD_NUMBER: _ClassVar[int]
    AuthRequestKind: str
    AuthRequestID: str
    def __init__(self, AuthRequestKind: _Optional[str] = ..., AuthRequestID: _Optional[str] = ...) -> None: ...

class UpstreamInventoryOneOf(_message.Message):
    __slots__ = ["Hello", "Heartbeat", "Pong", "AgentMetadata"]
    HELLO_FIELD_NUMBER: _ClassVar[int]
    HEARTBEAT_FIELD_NUMBER: _ClassVar[int]
    PONG_FIELD_NUMBER: _ClassVar[int]
    AGENTMETADATA_FIELD_NUMBER: _ClassVar[int]
    Hello: UpstreamInventoryHello
    Heartbeat: InventoryHeartbeat
    Pong: UpstreamInventoryPong
    AgentMetadata: UpstreamInventoryAgentMetadata
    def __init__(self, Hello: _Optional[_Union[UpstreamInventoryHello, _Mapping]] = ..., Heartbeat: _Optional[_Union[InventoryHeartbeat, _Mapping]] = ..., Pong: _Optional[_Union[UpstreamInventoryPong, _Mapping]] = ..., AgentMetadata: _Optional[_Union[UpstreamInventoryAgentMetadata, _Mapping]] = ...) -> None: ...

class DownstreamInventoryOneOf(_message.Message):
    __slots__ = ["Hello", "Ping", "UpdateLabels"]
    HELLO_FIELD_NUMBER: _ClassVar[int]
    PING_FIELD_NUMBER: _ClassVar[int]
    UPDATELABELS_FIELD_NUMBER: _ClassVar[int]
    Hello: DownstreamInventoryHello
    Ping: DownstreamInventoryPing
    UpdateLabels: DownstreamInventoryUpdateLabels
    def __init__(self, Hello: _Optional[_Union[DownstreamInventoryHello, _Mapping]] = ..., Ping: _Optional[_Union[DownstreamInventoryPing, _Mapping]] = ..., UpdateLabels: _Optional[_Union[DownstreamInventoryUpdateLabels, _Mapping]] = ...) -> None: ...

class DownstreamInventoryPing(_message.Message):
    __slots__ = ["ID"]
    ID_FIELD_NUMBER: _ClassVar[int]
    ID: int
    def __init__(self, ID: _Optional[int] = ...) -> None: ...

class UpstreamInventoryPong(_message.Message):
    __slots__ = ["ID"]
    ID_FIELD_NUMBER: _ClassVar[int]
    ID: int
    def __init__(self, ID: _Optional[int] = ...) -> None: ...

class UpstreamInventoryHello(_message.Message):
    __slots__ = ["Version", "ServerID", "Services", "Hostname", "ExternalUpgrader"]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    SERVERID_FIELD_NUMBER: _ClassVar[int]
    SERVICES_FIELD_NUMBER: _ClassVar[int]
    HOSTNAME_FIELD_NUMBER: _ClassVar[int]
    EXTERNALUPGRADER_FIELD_NUMBER: _ClassVar[int]
    Version: str
    ServerID: str
    Services: _containers.RepeatedScalarFieldContainer[str]
    Hostname: str
    ExternalUpgrader: str
    def __init__(self, Version: _Optional[str] = ..., ServerID: _Optional[str] = ..., Services: _Optional[_Iterable[str]] = ..., Hostname: _Optional[str] = ..., ExternalUpgrader: _Optional[str] = ...) -> None: ...

class UpstreamInventoryAgentMetadata(_message.Message):
    __slots__ = ["OS", "OSVersion", "HostArchitecture", "GlibcVersion", "InstallMethods", "ContainerRuntime", "ContainerOrchestrator", "CloudEnvironment"]
    OS_FIELD_NUMBER: _ClassVar[int]
    OSVERSION_FIELD_NUMBER: _ClassVar[int]
    HOSTARCHITECTURE_FIELD_NUMBER: _ClassVar[int]
    GLIBCVERSION_FIELD_NUMBER: _ClassVar[int]
    INSTALLMETHODS_FIELD_NUMBER: _ClassVar[int]
    CONTAINERRUNTIME_FIELD_NUMBER: _ClassVar[int]
    CONTAINERORCHESTRATOR_FIELD_NUMBER: _ClassVar[int]
    CLOUDENVIRONMENT_FIELD_NUMBER: _ClassVar[int]
    OS: str
    OSVersion: str
    HostArchitecture: str
    GlibcVersion: str
    InstallMethods: _containers.RepeatedScalarFieldContainer[str]
    ContainerRuntime: str
    ContainerOrchestrator: str
    CloudEnvironment: str
    def __init__(self, OS: _Optional[str] = ..., OSVersion: _Optional[str] = ..., HostArchitecture: _Optional[str] = ..., GlibcVersion: _Optional[str] = ..., InstallMethods: _Optional[_Iterable[str]] = ..., ContainerRuntime: _Optional[str] = ..., ContainerOrchestrator: _Optional[str] = ..., CloudEnvironment: _Optional[str] = ...) -> None: ...

class DownstreamInventoryHello(_message.Message):
    __slots__ = ["Version", "ServerID"]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    SERVERID_FIELD_NUMBER: _ClassVar[int]
    Version: str
    ServerID: str
    def __init__(self, Version: _Optional[str] = ..., ServerID: _Optional[str] = ...) -> None: ...

class InventoryUpdateLabelsRequest(_message.Message):
    __slots__ = ["ServerID", "Kind", "Labels"]
    class LabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    SERVERID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    ServerID: str
    Kind: LabelUpdateKind
    Labels: _containers.ScalarMap[str, str]
    def __init__(self, ServerID: _Optional[str] = ..., Kind: _Optional[_Union[LabelUpdateKind, str]] = ..., Labels: _Optional[_Mapping[str, str]] = ...) -> None: ...

class DownstreamInventoryUpdateLabels(_message.Message):
    __slots__ = ["Kind", "Labels"]
    class LabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    KIND_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    Kind: LabelUpdateKind
    Labels: _containers.ScalarMap[str, str]
    def __init__(self, Kind: _Optional[_Union[LabelUpdateKind, str]] = ..., Labels: _Optional[_Mapping[str, str]] = ...) -> None: ...

class InventoryHeartbeat(_message.Message):
    __slots__ = ["SSHServer"]
    SSHSERVER_FIELD_NUMBER: _ClassVar[int]
    SSHServer: _types_pb2.ServerV2
    def __init__(self, SSHServer: _Optional[_Union[_types_pb2.ServerV2, _Mapping]] = ...) -> None: ...

class InventoryStatusRequest(_message.Message):
    __slots__ = ["Connected"]
    CONNECTED_FIELD_NUMBER: _ClassVar[int]
    Connected: bool
    def __init__(self, Connected: bool = ...) -> None: ...

class InventoryStatusSummary(_message.Message):
    __slots__ = ["Connected", "InstanceCount", "VersionCounts", "UpgraderCounts", "ServiceCounts"]
    class VersionCountsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: int
        def __init__(self, key: _Optional[str] = ..., value: _Optional[int] = ...) -> None: ...
    class UpgraderCountsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: int
        def __init__(self, key: _Optional[str] = ..., value: _Optional[int] = ...) -> None: ...
    class ServiceCountsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: int
        def __init__(self, key: _Optional[str] = ..., value: _Optional[int] = ...) -> None: ...
    CONNECTED_FIELD_NUMBER: _ClassVar[int]
    INSTANCECOUNT_FIELD_NUMBER: _ClassVar[int]
    VERSIONCOUNTS_FIELD_NUMBER: _ClassVar[int]
    UPGRADERCOUNTS_FIELD_NUMBER: _ClassVar[int]
    SERVICECOUNTS_FIELD_NUMBER: _ClassVar[int]
    Connected: _containers.RepeatedCompositeFieldContainer[UpstreamInventoryHello]
    InstanceCount: int
    VersionCounts: _containers.ScalarMap[str, int]
    UpgraderCounts: _containers.ScalarMap[str, int]
    ServiceCounts: _containers.ScalarMap[str, int]
    def __init__(self, Connected: _Optional[_Iterable[_Union[UpstreamInventoryHello, _Mapping]]] = ..., InstanceCount: _Optional[int] = ..., VersionCounts: _Optional[_Mapping[str, int]] = ..., UpgraderCounts: _Optional[_Mapping[str, int]] = ..., ServiceCounts: _Optional[_Mapping[str, int]] = ...) -> None: ...

class InventoryConnectedServiceCountsRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class InventoryConnectedServiceCounts(_message.Message):
    __slots__ = ["ServiceCounts"]
    class ServiceCountsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: int
        def __init__(self, key: _Optional[str] = ..., value: _Optional[int] = ...) -> None: ...
    SERVICECOUNTS_FIELD_NUMBER: _ClassVar[int]
    ServiceCounts: _containers.ScalarMap[str, int]
    def __init__(self, ServiceCounts: _Optional[_Mapping[str, int]] = ...) -> None: ...

class InventoryPingRequest(_message.Message):
    __slots__ = ["ServerID", "ControlLog"]
    SERVERID_FIELD_NUMBER: _ClassVar[int]
    CONTROLLOG_FIELD_NUMBER: _ClassVar[int]
    ServerID: str
    ControlLog: bool
    def __init__(self, ServerID: _Optional[str] = ..., ControlLog: bool = ...) -> None: ...

class InventoryPingResponse(_message.Message):
    __slots__ = ["Duration"]
    DURATION_FIELD_NUMBER: _ClassVar[int]
    Duration: int
    def __init__(self, Duration: _Optional[int] = ...) -> None: ...

class GetClusterAlertsResponse(_message.Message):
    __slots__ = ["Alerts"]
    ALERTS_FIELD_NUMBER: _ClassVar[int]
    Alerts: _containers.RepeatedCompositeFieldContainer[_types_pb2.ClusterAlert]
    def __init__(self, Alerts: _Optional[_Iterable[_Union[_types_pb2.ClusterAlert, _Mapping]]] = ...) -> None: ...

class GetAlertAcksRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class GetAlertAcksResponse(_message.Message):
    __slots__ = ["Acks"]
    ACKS_FIELD_NUMBER: _ClassVar[int]
    Acks: _containers.RepeatedCompositeFieldContainer[_types_pb2.AlertAcknowledgement]
    def __init__(self, Acks: _Optional[_Iterable[_Union[_types_pb2.AlertAcknowledgement, _Mapping]]] = ...) -> None: ...

class ClearAlertAcksRequest(_message.Message):
    __slots__ = ["AlertID"]
    ALERTID_FIELD_NUMBER: _ClassVar[int]
    AlertID: str
    def __init__(self, AlertID: _Optional[str] = ...) -> None: ...

class UpsertClusterAlertRequest(_message.Message):
    __slots__ = ["Alert"]
    ALERT_FIELD_NUMBER: _ClassVar[int]
    Alert: _types_pb2.ClusterAlert
    def __init__(self, Alert: _Optional[_Union[_types_pb2.ClusterAlert, _Mapping]] = ...) -> None: ...

class GetConnectionDiagnosticRequest(_message.Message):
    __slots__ = ["Name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    Name: str
    def __init__(self, Name: _Optional[str] = ...) -> None: ...

class AppendDiagnosticTraceRequest(_message.Message):
    __slots__ = ["Name", "Trace"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    TRACE_FIELD_NUMBER: _ClassVar[int]
    Name: str
    Trace: _types_pb2.ConnectionDiagnosticTrace
    def __init__(self, Name: _Optional[str] = ..., Trace: _Optional[_Union[_types_pb2.ConnectionDiagnosticTrace, _Mapping]] = ...) -> None: ...

class SubmitUsageEventRequest(_message.Message):
    __slots__ = ["Event"]
    EVENT_FIELD_NUMBER: _ClassVar[int]
    Event: _usageevents_pb2.UsageEventOneOf
    def __init__(self, Event: _Optional[_Union[_usageevents_pb2.UsageEventOneOf, _Mapping]] = ...) -> None: ...

class GetLicenseRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class ListReleasesRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class CreateTokenV2Request(_message.Message):
    __slots__ = ["V2"]
    V2_FIELD_NUMBER: _ClassVar[int]
    V2: _types_pb2.ProvisionTokenV2
    def __init__(self, V2: _Optional[_Union[_types_pb2.ProvisionTokenV2, _Mapping]] = ...) -> None: ...

class UpsertTokenV2Request(_message.Message):
    __slots__ = ["V2"]
    V2_FIELD_NUMBER: _ClassVar[int]
    V2: _types_pb2.ProvisionTokenV2
    def __init__(self, V2: _Optional[_Union[_types_pb2.ProvisionTokenV2, _Mapping]] = ...) -> None: ...

class GetHeadlessAuthenticationRequest(_message.Message):
    __slots__ = ["id"]
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class UpdateHeadlessAuthenticationStateRequest(_message.Message):
    __slots__ = ["id", "state", "mfa_response"]
    ID_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    MFA_RESPONSE_FIELD_NUMBER: _ClassVar[int]
    id: str
    state: _types_pb2.HeadlessAuthenticationState
    mfa_response: MFAAuthenticateResponse
    def __init__(self, id: _Optional[str] = ..., state: _Optional[_Union[_types_pb2.HeadlessAuthenticationState, str]] = ..., mfa_response: _Optional[_Union[MFAAuthenticateResponse, _Mapping]] = ...) -> None: ...

class ExportUpgradeWindowsRequest(_message.Message):
    __slots__ = ["TeleportVersion", "UpgraderKind"]
    TELEPORTVERSION_FIELD_NUMBER: _ClassVar[int]
    UPGRADERKIND_FIELD_NUMBER: _ClassVar[int]
    TeleportVersion: str
    UpgraderKind: str
    def __init__(self, TeleportVersion: _Optional[str] = ..., UpgraderKind: _Optional[str] = ...) -> None: ...

class ExportUpgradeWindowsResponse(_message.Message):
    __slots__ = ["CanonicalSchedule", "KubeControllerSchedule", "SystemdUnitSchedule"]
    CANONICALSCHEDULE_FIELD_NUMBER: _ClassVar[int]
    KUBECONTROLLERSCHEDULE_FIELD_NUMBER: _ClassVar[int]
    SYSTEMDUNITSCHEDULE_FIELD_NUMBER: _ClassVar[int]
    CanonicalSchedule: _types_pb2.AgentUpgradeSchedule
    KubeControllerSchedule: str
    SystemdUnitSchedule: str
    def __init__(self, CanonicalSchedule: _Optional[_Union[_types_pb2.AgentUpgradeSchedule, _Mapping]] = ..., KubeControllerSchedule: _Optional[str] = ..., SystemdUnitSchedule: _Optional[str] = ...) -> None: ...

class AccessRequestAllowedPromotionRequest(_message.Message):
    __slots__ = ["accessRequestID"]
    ACCESSREQUESTID_FIELD_NUMBER: _ClassVar[int]
    accessRequestID: str
    def __init__(self, accessRequestID: _Optional[str] = ...) -> None: ...

class AccessRequestAllowedPromotionResponse(_message.Message):
    __slots__ = ["allowedPromotions"]
    ALLOWEDPROMOTIONS_FIELD_NUMBER: _ClassVar[int]
    allowedPromotions: _types_pb2.AccessRequestAllowedPromotions
    def __init__(self, allowedPromotions: _Optional[_Union[_types_pb2.AccessRequestAllowedPromotions, _Mapping]] = ...) -> None: ...
