from gogoproto import gogo_pb2 as _gogo_pb2
from google.protobuf import struct_pb2 as _struct_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf import wrappers_pb2 as _wrappers_pb2
from teleport.legacy.types import types_pb2 as _types_pb2
from teleport.legacy.types.wrappers import wrappers_pb2 as _wrappers_pb2_1
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class EventAction(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    OBSERVED: _ClassVar[EventAction]
    DENIED: _ClassVar[EventAction]

class SFTPAction(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    INVALID: _ClassVar[SFTPAction]
    OPEN: _ClassVar[SFTPAction]
    CLOSE: _ClassVar[SFTPAction]
    READ: _ClassVar[SFTPAction]
    WRITE: _ClassVar[SFTPAction]
    LSTAT: _ClassVar[SFTPAction]
    FSTAT: _ClassVar[SFTPAction]
    SETSTAT: _ClassVar[SFTPAction]
    FSETSTAT: _ClassVar[SFTPAction]
    OPENDIR: _ClassVar[SFTPAction]
    READDIR: _ClassVar[SFTPAction]
    REMOVE: _ClassVar[SFTPAction]
    MKDIR: _ClassVar[SFTPAction]
    RMDIR: _ClassVar[SFTPAction]
    REALPATH: _ClassVar[SFTPAction]
    STAT: _ClassVar[SFTPAction]
    RENAME: _ClassVar[SFTPAction]
    READLINK: _ClassVar[SFTPAction]
    SYMLINK: _ClassVar[SFTPAction]
    LINK: _ClassVar[SFTPAction]

class OSType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    OS_TYPE_UNSPECIFIED: _ClassVar[OSType]
    OS_TYPE_LINUX: _ClassVar[OSType]
    OS_TYPE_MACOS: _ClassVar[OSType]
    OS_TYPE_WINDOWS: _ClassVar[OSType]

class DeviceOrigin(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    DEVICE_ORIGIN_UNSPECIFIED: _ClassVar[DeviceOrigin]
    DEVICE_ORIGIN_API: _ClassVar[DeviceOrigin]
    DEVICE_ORIGIN_JAMF: _ClassVar[DeviceOrigin]
    DEVICE_ORIGIN_INTUNE: _ClassVar[DeviceOrigin]

class ElasticsearchCategory(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    ELASTICSEARCH_CATEGORY_GENERAL: _ClassVar[ElasticsearchCategory]
    ELASTICSEARCH_CATEGORY_SECURITY: _ClassVar[ElasticsearchCategory]
    ELASTICSEARCH_CATEGORY_SEARCH: _ClassVar[ElasticsearchCategory]
    ELASTICSEARCH_CATEGORY_SQL: _ClassVar[ElasticsearchCategory]

class OpenSearchCategory(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    OPEN_SEARCH_CATEGORY_GENERAL: _ClassVar[OpenSearchCategory]
    OPEN_SEARCH_CATEGORY_SECURITY: _ClassVar[OpenSearchCategory]
    OPEN_SEARCH_CATEGORY_SEARCH: _ClassVar[OpenSearchCategory]
    OPEN_SEARCH_CATEGORY_SQL: _ClassVar[OpenSearchCategory]
OBSERVED: EventAction
DENIED: EventAction
INVALID: SFTPAction
OPEN: SFTPAction
CLOSE: SFTPAction
READ: SFTPAction
WRITE: SFTPAction
LSTAT: SFTPAction
FSTAT: SFTPAction
SETSTAT: SFTPAction
FSETSTAT: SFTPAction
OPENDIR: SFTPAction
READDIR: SFTPAction
REMOVE: SFTPAction
MKDIR: SFTPAction
RMDIR: SFTPAction
REALPATH: SFTPAction
STAT: SFTPAction
RENAME: SFTPAction
READLINK: SFTPAction
SYMLINK: SFTPAction
LINK: SFTPAction
OS_TYPE_UNSPECIFIED: OSType
OS_TYPE_LINUX: OSType
OS_TYPE_MACOS: OSType
OS_TYPE_WINDOWS: OSType
DEVICE_ORIGIN_UNSPECIFIED: DeviceOrigin
DEVICE_ORIGIN_API: DeviceOrigin
DEVICE_ORIGIN_JAMF: DeviceOrigin
DEVICE_ORIGIN_INTUNE: DeviceOrigin
ELASTICSEARCH_CATEGORY_GENERAL: ElasticsearchCategory
ELASTICSEARCH_CATEGORY_SECURITY: ElasticsearchCategory
ELASTICSEARCH_CATEGORY_SEARCH: ElasticsearchCategory
ELASTICSEARCH_CATEGORY_SQL: ElasticsearchCategory
OPEN_SEARCH_CATEGORY_GENERAL: OpenSearchCategory
OPEN_SEARCH_CATEGORY_SECURITY: OpenSearchCategory
OPEN_SEARCH_CATEGORY_SEARCH: OpenSearchCategory
OPEN_SEARCH_CATEGORY_SQL: OpenSearchCategory

class Metadata(_message.Message):
    __slots__ = ["Index", "Type", "ID", "Code", "Time", "ClusterName"]
    INDEX_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    CODE_FIELD_NUMBER: _ClassVar[int]
    TIME_FIELD_NUMBER: _ClassVar[int]
    CLUSTERNAME_FIELD_NUMBER: _ClassVar[int]
    Index: int
    Type: str
    ID: str
    Code: str
    Time: _timestamp_pb2.Timestamp
    ClusterName: str
    def __init__(self, Index: _Optional[int] = ..., Type: _Optional[str] = ..., ID: _Optional[str] = ..., Code: _Optional[str] = ..., Time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., ClusterName: _Optional[str] = ...) -> None: ...

class SessionMetadata(_message.Message):
    __slots__ = ["SessionID", "WithMFA"]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    WITHMFA_FIELD_NUMBER: _ClassVar[int]
    SessionID: str
    WithMFA: str
    def __init__(self, SessionID: _Optional[str] = ..., WithMFA: _Optional[str] = ...) -> None: ...

class UserMetadata(_message.Message):
    __slots__ = ["User", "Login", "Impersonator", "AWSRoleARN", "AccessRequests", "AzureIdentity", "GCPServiceAccount", "TrustedDevice"]
    USER_FIELD_NUMBER: _ClassVar[int]
    LOGIN_FIELD_NUMBER: _ClassVar[int]
    IMPERSONATOR_FIELD_NUMBER: _ClassVar[int]
    AWSROLEARN_FIELD_NUMBER: _ClassVar[int]
    ACCESSREQUESTS_FIELD_NUMBER: _ClassVar[int]
    AZUREIDENTITY_FIELD_NUMBER: _ClassVar[int]
    GCPSERVICEACCOUNT_FIELD_NUMBER: _ClassVar[int]
    TRUSTEDDEVICE_FIELD_NUMBER: _ClassVar[int]
    User: str
    Login: str
    Impersonator: str
    AWSRoleARN: str
    AccessRequests: _containers.RepeatedScalarFieldContainer[str]
    AzureIdentity: str
    GCPServiceAccount: str
    TrustedDevice: DeviceMetadata
    def __init__(self, User: _Optional[str] = ..., Login: _Optional[str] = ..., Impersonator: _Optional[str] = ..., AWSRoleARN: _Optional[str] = ..., AccessRequests: _Optional[_Iterable[str]] = ..., AzureIdentity: _Optional[str] = ..., GCPServiceAccount: _Optional[str] = ..., TrustedDevice: _Optional[_Union[DeviceMetadata, _Mapping]] = ...) -> None: ...

class ServerMetadata(_message.Message):
    __slots__ = ["ServerNamespace", "ServerID", "ServerHostname", "ServerAddr", "ServerLabels", "ForwardedBy", "ServerSubKind"]
    class ServerLabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    SERVERNAMESPACE_FIELD_NUMBER: _ClassVar[int]
    SERVERID_FIELD_NUMBER: _ClassVar[int]
    SERVERHOSTNAME_FIELD_NUMBER: _ClassVar[int]
    SERVERADDR_FIELD_NUMBER: _ClassVar[int]
    SERVERLABELS_FIELD_NUMBER: _ClassVar[int]
    FORWARDEDBY_FIELD_NUMBER: _ClassVar[int]
    SERVERSUBKIND_FIELD_NUMBER: _ClassVar[int]
    ServerNamespace: str
    ServerID: str
    ServerHostname: str
    ServerAddr: str
    ServerLabels: _containers.ScalarMap[str, str]
    ForwardedBy: str
    ServerSubKind: str
    def __init__(self, ServerNamespace: _Optional[str] = ..., ServerID: _Optional[str] = ..., ServerHostname: _Optional[str] = ..., ServerAddr: _Optional[str] = ..., ServerLabels: _Optional[_Mapping[str, str]] = ..., ForwardedBy: _Optional[str] = ..., ServerSubKind: _Optional[str] = ...) -> None: ...

class ConnectionMetadata(_message.Message):
    __slots__ = ["LocalAddr", "RemoteAddr", "Protocol"]
    LOCALADDR_FIELD_NUMBER: _ClassVar[int]
    REMOTEADDR_FIELD_NUMBER: _ClassVar[int]
    PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    LocalAddr: str
    RemoteAddr: str
    Protocol: str
    def __init__(self, LocalAddr: _Optional[str] = ..., RemoteAddr: _Optional[str] = ..., Protocol: _Optional[str] = ...) -> None: ...

class ClientMetadata(_message.Message):
    __slots__ = ["UserAgent"]
    USERAGENT_FIELD_NUMBER: _ClassVar[int]
    UserAgent: str
    def __init__(self, UserAgent: _Optional[str] = ...) -> None: ...

class KubernetesClusterMetadata(_message.Message):
    __slots__ = ["KubernetesCluster", "KubernetesUsers", "KubernetesGroups", "KubernetesLabels"]
    class KubernetesLabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    KUBERNETESCLUSTER_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESUSERS_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESGROUPS_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESLABELS_FIELD_NUMBER: _ClassVar[int]
    KubernetesCluster: str
    KubernetesUsers: _containers.RepeatedScalarFieldContainer[str]
    KubernetesGroups: _containers.RepeatedScalarFieldContainer[str]
    KubernetesLabels: _containers.ScalarMap[str, str]
    def __init__(self, KubernetesCluster: _Optional[str] = ..., KubernetesUsers: _Optional[_Iterable[str]] = ..., KubernetesGroups: _Optional[_Iterable[str]] = ..., KubernetesLabels: _Optional[_Mapping[str, str]] = ...) -> None: ...

class KubernetesPodMetadata(_message.Message):
    __slots__ = ["KubernetesPodName", "KubernetesPodNamespace", "KubernetesContainerName", "KubernetesContainerImage", "KubernetesNodeName"]
    KUBERNETESPODNAME_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESPODNAMESPACE_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESCONTAINERNAME_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESCONTAINERIMAGE_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESNODENAME_FIELD_NUMBER: _ClassVar[int]
    KubernetesPodName: str
    KubernetesPodNamespace: str
    KubernetesContainerName: str
    KubernetesContainerImage: str
    KubernetesNodeName: str
    def __init__(self, KubernetesPodName: _Optional[str] = ..., KubernetesPodNamespace: _Optional[str] = ..., KubernetesContainerName: _Optional[str] = ..., KubernetesContainerImage: _Optional[str] = ..., KubernetesNodeName: _Optional[str] = ...) -> None: ...

class SAMLIdPServiceProviderMetadata(_message.Message):
    __slots__ = ["ServiceProviderEntityID", "ServiceProviderShortcut"]
    SERVICEPROVIDERENTITYID_FIELD_NUMBER: _ClassVar[int]
    SERVICEPROVIDERSHORTCUT_FIELD_NUMBER: _ClassVar[int]
    ServiceProviderEntityID: str
    ServiceProviderShortcut: str
    def __init__(self, ServiceProviderEntityID: _Optional[str] = ..., ServiceProviderShortcut: _Optional[str] = ...) -> None: ...

class OktaResourcesUpdatedMetadata(_message.Message):
    __slots__ = ["Added", "Updated", "Deleted", "AddedResources", "UpdatedResources", "DeletedResources"]
    ADDED_FIELD_NUMBER: _ClassVar[int]
    UPDATED_FIELD_NUMBER: _ClassVar[int]
    DELETED_FIELD_NUMBER: _ClassVar[int]
    ADDEDRESOURCES_FIELD_NUMBER: _ClassVar[int]
    UPDATEDRESOURCES_FIELD_NUMBER: _ClassVar[int]
    DELETEDRESOURCES_FIELD_NUMBER: _ClassVar[int]
    Added: int
    Updated: int
    Deleted: int
    AddedResources: _containers.RepeatedCompositeFieldContainer[OktaResource]
    UpdatedResources: _containers.RepeatedCompositeFieldContainer[OktaResource]
    DeletedResources: _containers.RepeatedCompositeFieldContainer[OktaResource]
    def __init__(self, Added: _Optional[int] = ..., Updated: _Optional[int] = ..., Deleted: _Optional[int] = ..., AddedResources: _Optional[_Iterable[_Union[OktaResource, _Mapping]]] = ..., UpdatedResources: _Optional[_Iterable[_Union[OktaResource, _Mapping]]] = ..., DeletedResources: _Optional[_Iterable[_Union[OktaResource, _Mapping]]] = ...) -> None: ...

class OktaResource(_message.Message):
    __slots__ = ["ID", "Description"]
    ID_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    ID: str
    Description: str
    def __init__(self, ID: _Optional[str] = ..., Description: _Optional[str] = ...) -> None: ...

class OktaAssignmentMetadata(_message.Message):
    __slots__ = ["Source", "User", "StartingStatus", "EndingStatus"]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    STARTINGSTATUS_FIELD_NUMBER: _ClassVar[int]
    ENDINGSTATUS_FIELD_NUMBER: _ClassVar[int]
    Source: str
    User: str
    StartingStatus: str
    EndingStatus: str
    def __init__(self, Source: _Optional[str] = ..., User: _Optional[str] = ..., StartingStatus: _Optional[str] = ..., EndingStatus: _Optional[str] = ...) -> None: ...

class AccessListMemberMetadata(_message.Message):
    __slots__ = ["AccessListName", "Members"]
    ACCESSLISTNAME_FIELD_NUMBER: _ClassVar[int]
    MEMBERS_FIELD_NUMBER: _ClassVar[int]
    AccessListName: str
    Members: _containers.RepeatedCompositeFieldContainer[AccessListMember]
    def __init__(self, AccessListName: _Optional[str] = ..., Members: _Optional[_Iterable[_Union[AccessListMember, _Mapping]]] = ...) -> None: ...

class AccessListMember(_message.Message):
    __slots__ = ["JoinedOn", "RemovedOn", "Reason", "MemberName"]
    JOINEDON_FIELD_NUMBER: _ClassVar[int]
    REMOVEDON_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    MEMBERNAME_FIELD_NUMBER: _ClassVar[int]
    JoinedOn: _timestamp_pb2.Timestamp
    RemovedOn: _timestamp_pb2.Timestamp
    Reason: str
    MemberName: str
    def __init__(self, JoinedOn: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., RemovedOn: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., Reason: _Optional[str] = ..., MemberName: _Optional[str] = ...) -> None: ...

class AccessListReviewMetadata(_message.Message):
    __slots__ = ["Message"]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    Message: str
    def __init__(self, Message: _Optional[str] = ...) -> None: ...

class SessionStart(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Server", "Connection", "TerminalSize", "KubernetesCluster", "KubernetesPod", "InitialCommand", "SessionRecording"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    TERMINALSIZE_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESCLUSTER_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESPOD_FIELD_NUMBER: _ClassVar[int]
    INITIALCOMMAND_FIELD_NUMBER: _ClassVar[int]
    SESSIONRECORDING_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Server: ServerMetadata
    Connection: ConnectionMetadata
    TerminalSize: str
    KubernetesCluster: KubernetesClusterMetadata
    KubernetesPod: KubernetesPodMetadata
    InitialCommand: _containers.RepeatedScalarFieldContainer[str]
    SessionRecording: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., TerminalSize: _Optional[str] = ..., KubernetesCluster: _Optional[_Union[KubernetesClusterMetadata, _Mapping]] = ..., KubernetesPod: _Optional[_Union[KubernetesPodMetadata, _Mapping]] = ..., InitialCommand: _Optional[_Iterable[str]] = ..., SessionRecording: _Optional[str] = ...) -> None: ...

class SessionJoin(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Server", "Connection", "KubernetesCluster"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESCLUSTER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Server: ServerMetadata
    Connection: ConnectionMetadata
    KubernetesCluster: KubernetesClusterMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., KubernetesCluster: _Optional[_Union[KubernetesClusterMetadata, _Mapping]] = ...) -> None: ...

class SessionPrint(_message.Message):
    __slots__ = ["Metadata", "ChunkIndex", "Data", "Bytes", "DelayMilliseconds", "Offset"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    CHUNKINDEX_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    BYTES_FIELD_NUMBER: _ClassVar[int]
    DELAYMILLISECONDS_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    ChunkIndex: int
    Data: bytes
    Bytes: int
    DelayMilliseconds: int
    Offset: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., ChunkIndex: _Optional[int] = ..., Data: _Optional[bytes] = ..., Bytes: _Optional[int] = ..., DelayMilliseconds: _Optional[int] = ..., Offset: _Optional[int] = ...) -> None: ...

class DesktopRecording(_message.Message):
    __slots__ = ["Metadata", "Message", "DelayMilliseconds"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    DELAYMILLISECONDS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Message: bytes
    DelayMilliseconds: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Message: _Optional[bytes] = ..., DelayMilliseconds: _Optional[int] = ...) -> None: ...

class DesktopClipboardReceive(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Connection", "DesktopAddr", "Length"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    DESKTOPADDR_FIELD_NUMBER: _ClassVar[int]
    LENGTH_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Connection: ConnectionMetadata
    DesktopAddr: str
    Length: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., DesktopAddr: _Optional[str] = ..., Length: _Optional[int] = ...) -> None: ...

class DesktopClipboardSend(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Connection", "DesktopAddr", "Length"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    DESKTOPADDR_FIELD_NUMBER: _ClassVar[int]
    LENGTH_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Connection: ConnectionMetadata
    DesktopAddr: str
    Length: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., DesktopAddr: _Optional[str] = ..., Length: _Optional[int] = ...) -> None: ...

class DesktopSharedDirectoryStart(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Connection", "Status", "DesktopAddr", "DirectoryName", "DirectoryID"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    DESKTOPADDR_FIELD_NUMBER: _ClassVar[int]
    DIRECTORYNAME_FIELD_NUMBER: _ClassVar[int]
    DIRECTORYID_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Connection: ConnectionMetadata
    Status: Status
    DesktopAddr: str
    DirectoryName: str
    DirectoryID: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ..., DesktopAddr: _Optional[str] = ..., DirectoryName: _Optional[str] = ..., DirectoryID: _Optional[int] = ...) -> None: ...

class DesktopSharedDirectoryRead(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Connection", "Status", "DesktopAddr", "DirectoryName", "DirectoryID", "Path", "Length", "Offset"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    DESKTOPADDR_FIELD_NUMBER: _ClassVar[int]
    DIRECTORYNAME_FIELD_NUMBER: _ClassVar[int]
    DIRECTORYID_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    LENGTH_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Connection: ConnectionMetadata
    Status: Status
    DesktopAddr: str
    DirectoryName: str
    DirectoryID: int
    Path: str
    Length: int
    Offset: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ..., DesktopAddr: _Optional[str] = ..., DirectoryName: _Optional[str] = ..., DirectoryID: _Optional[int] = ..., Path: _Optional[str] = ..., Length: _Optional[int] = ..., Offset: _Optional[int] = ...) -> None: ...

class DesktopSharedDirectoryWrite(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Connection", "Status", "DesktopAddr", "DirectoryName", "DirectoryID", "Path", "Length", "Offset"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    DESKTOPADDR_FIELD_NUMBER: _ClassVar[int]
    DIRECTORYNAME_FIELD_NUMBER: _ClassVar[int]
    DIRECTORYID_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    LENGTH_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Connection: ConnectionMetadata
    Status: Status
    DesktopAddr: str
    DirectoryName: str
    DirectoryID: int
    Path: str
    Length: int
    Offset: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ..., DesktopAddr: _Optional[str] = ..., DirectoryName: _Optional[str] = ..., DirectoryID: _Optional[int] = ..., Path: _Optional[str] = ..., Length: _Optional[int] = ..., Offset: _Optional[int] = ...) -> None: ...

class SessionReject(_message.Message):
    __slots__ = ["Metadata", "User", "Server", "Connection", "Reason", "Maximum"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    MAXIMUM_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Server: ServerMetadata
    Connection: ConnectionMetadata
    Reason: str
    Maximum: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., Reason: _Optional[str] = ..., Maximum: _Optional[int] = ...) -> None: ...

class SessionConnect(_message.Message):
    __slots__ = ["Metadata", "Server", "Connection"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Server: ServerMetadata
    Connection: ConnectionMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ...) -> None: ...

class FileTransferRequestEvent(_message.Message):
    __slots__ = ["Metadata", "Session", "RequestID", "Approvers", "Requester", "Location", "Download", "Filename"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    REQUESTID_FIELD_NUMBER: _ClassVar[int]
    APPROVERS_FIELD_NUMBER: _ClassVar[int]
    REQUESTER_FIELD_NUMBER: _ClassVar[int]
    LOCATION_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_FIELD_NUMBER: _ClassVar[int]
    FILENAME_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Session: SessionMetadata
    RequestID: str
    Approvers: _containers.RepeatedScalarFieldContainer[str]
    Requester: str
    Location: str
    Download: bool
    Filename: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., RequestID: _Optional[str] = ..., Approvers: _Optional[_Iterable[str]] = ..., Requester: _Optional[str] = ..., Location: _Optional[str] = ..., Download: bool = ..., Filename: _Optional[str] = ...) -> None: ...

class Resize(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Connection", "Server", "TerminalSize", "KubernetesCluster", "KubernetesPod"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    TERMINALSIZE_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESCLUSTER_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESPOD_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Connection: ConnectionMetadata
    Server: ServerMetadata
    TerminalSize: str
    KubernetesCluster: KubernetesClusterMetadata
    KubernetesPod: KubernetesPodMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., TerminalSize: _Optional[str] = ..., KubernetesCluster: _Optional[_Union[KubernetesClusterMetadata, _Mapping]] = ..., KubernetesPod: _Optional[_Union[KubernetesPodMetadata, _Mapping]] = ...) -> None: ...

class SessionEnd(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Connection", "Server", "EnhancedRecording", "Interactive", "Participants", "StartTime", "EndTime", "KubernetesCluster", "KubernetesPod", "InitialCommand", "SessionRecording"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    ENHANCEDRECORDING_FIELD_NUMBER: _ClassVar[int]
    INTERACTIVE_FIELD_NUMBER: _ClassVar[int]
    PARTICIPANTS_FIELD_NUMBER: _ClassVar[int]
    STARTTIME_FIELD_NUMBER: _ClassVar[int]
    ENDTIME_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESCLUSTER_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESPOD_FIELD_NUMBER: _ClassVar[int]
    INITIALCOMMAND_FIELD_NUMBER: _ClassVar[int]
    SESSIONRECORDING_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Connection: ConnectionMetadata
    Server: ServerMetadata
    EnhancedRecording: bool
    Interactive: bool
    Participants: _containers.RepeatedScalarFieldContainer[str]
    StartTime: _timestamp_pb2.Timestamp
    EndTime: _timestamp_pb2.Timestamp
    KubernetesCluster: KubernetesClusterMetadata
    KubernetesPod: KubernetesPodMetadata
    InitialCommand: _containers.RepeatedScalarFieldContainer[str]
    SessionRecording: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., EnhancedRecording: bool = ..., Interactive: bool = ..., Participants: _Optional[_Iterable[str]] = ..., StartTime: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., EndTime: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., KubernetesCluster: _Optional[_Union[KubernetesClusterMetadata, _Mapping]] = ..., KubernetesPod: _Optional[_Union[KubernetesPodMetadata, _Mapping]] = ..., InitialCommand: _Optional[_Iterable[str]] = ..., SessionRecording: _Optional[str] = ...) -> None: ...

class BPFMetadata(_message.Message):
    __slots__ = ["PID", "CgroupID", "Program"]
    PID_FIELD_NUMBER: _ClassVar[int]
    CGROUPID_FIELD_NUMBER: _ClassVar[int]
    PROGRAM_FIELD_NUMBER: _ClassVar[int]
    PID: int
    CgroupID: int
    Program: str
    def __init__(self, PID: _Optional[int] = ..., CgroupID: _Optional[int] = ..., Program: _Optional[str] = ...) -> None: ...

class Status(_message.Message):
    __slots__ = ["Success", "Error", "UserMessage"]
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    USERMESSAGE_FIELD_NUMBER: _ClassVar[int]
    Success: bool
    Error: str
    UserMessage: str
    def __init__(self, Success: bool = ..., Error: _Optional[str] = ..., UserMessage: _Optional[str] = ...) -> None: ...

class SessionCommand(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Server", "BPF", "PPID", "Path", "Argv", "ReturnCode"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    BPF_FIELD_NUMBER: _ClassVar[int]
    PPID_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    ARGV_FIELD_NUMBER: _ClassVar[int]
    RETURNCODE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Server: ServerMetadata
    BPF: BPFMetadata
    PPID: int
    Path: str
    Argv: _containers.RepeatedScalarFieldContainer[str]
    ReturnCode: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., BPF: _Optional[_Union[BPFMetadata, _Mapping]] = ..., PPID: _Optional[int] = ..., Path: _Optional[str] = ..., Argv: _Optional[_Iterable[str]] = ..., ReturnCode: _Optional[int] = ...) -> None: ...

class SessionDisk(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Server", "BPF", "Path", "Flags", "ReturnCode"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    BPF_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    FLAGS_FIELD_NUMBER: _ClassVar[int]
    RETURNCODE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Server: ServerMetadata
    BPF: BPFMetadata
    Path: str
    Flags: int
    ReturnCode: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., BPF: _Optional[_Union[BPFMetadata, _Mapping]] = ..., Path: _Optional[str] = ..., Flags: _Optional[int] = ..., ReturnCode: _Optional[int] = ...) -> None: ...

class SessionNetwork(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Server", "BPF", "SrcAddr", "DstAddr", "DstPort", "TCPVersion", "Operation", "Action"]
    class NetworkOperation(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = []
        CONNECT: _ClassVar[SessionNetwork.NetworkOperation]
        SEND: _ClassVar[SessionNetwork.NetworkOperation]
    CONNECT: SessionNetwork.NetworkOperation
    SEND: SessionNetwork.NetworkOperation
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    BPF_FIELD_NUMBER: _ClassVar[int]
    SRCADDR_FIELD_NUMBER: _ClassVar[int]
    DSTADDR_FIELD_NUMBER: _ClassVar[int]
    DSTPORT_FIELD_NUMBER: _ClassVar[int]
    TCPVERSION_FIELD_NUMBER: _ClassVar[int]
    OPERATION_FIELD_NUMBER: _ClassVar[int]
    ACTION_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Server: ServerMetadata
    BPF: BPFMetadata
    SrcAddr: str
    DstAddr: str
    DstPort: int
    TCPVersion: int
    Operation: SessionNetwork.NetworkOperation
    Action: EventAction
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., BPF: _Optional[_Union[BPFMetadata, _Mapping]] = ..., SrcAddr: _Optional[str] = ..., DstAddr: _Optional[str] = ..., DstPort: _Optional[int] = ..., TCPVersion: _Optional[int] = ..., Operation: _Optional[_Union[SessionNetwork.NetworkOperation, str]] = ..., Action: _Optional[_Union[EventAction, str]] = ...) -> None: ...

class SessionData(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Server", "Connection", "BytesTransmitted", "BytesReceived"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    BYTESTRANSMITTED_FIELD_NUMBER: _ClassVar[int]
    BYTESRECEIVED_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Server: ServerMetadata
    Connection: ConnectionMetadata
    BytesTransmitted: int
    BytesReceived: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., BytesTransmitted: _Optional[int] = ..., BytesReceived: _Optional[int] = ...) -> None: ...

class SessionLeave(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Server", "Connection"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Server: ServerMetadata
    Connection: ConnectionMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ...) -> None: ...

class UserLogin(_message.Message):
    __slots__ = ["Metadata", "User", "Status", "Method", "IdentityAttributes", "MFADevice", "Client", "Connection", "AppliedLoginRules"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    IDENTITYATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    MFADEVICE_FIELD_NUMBER: _ClassVar[int]
    CLIENT_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    APPLIEDLOGINRULES_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Status: Status
    Method: str
    IdentityAttributes: _struct_pb2.Struct
    MFADevice: MFADeviceMetadata
    Client: ClientMetadata
    Connection: ConnectionMetadata
    AppliedLoginRules: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ..., Method: _Optional[str] = ..., IdentityAttributes: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., MFADevice: _Optional[_Union[MFADeviceMetadata, _Mapping]] = ..., Client: _Optional[_Union[ClientMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., AppliedLoginRules: _Optional[_Iterable[str]] = ...) -> None: ...

class ResourceMetadata(_message.Message):
    __slots__ = ["Name", "Expires", "UpdatedBy", "TTL"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    UPDATEDBY_FIELD_NUMBER: _ClassVar[int]
    TTL_FIELD_NUMBER: _ClassVar[int]
    Name: str
    Expires: _timestamp_pb2.Timestamp
    UpdatedBy: str
    TTL: str
    def __init__(self, Name: _Optional[str] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., UpdatedBy: _Optional[str] = ..., TTL: _Optional[str] = ...) -> None: ...

class UserCreate(_message.Message):
    __slots__ = ["Metadata", "User", "Resource", "Roles", "Connector"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    CONNECTOR_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Resource: ResourceMetadata
    Roles: _containers.RepeatedScalarFieldContainer[str]
    Connector: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., Roles: _Optional[_Iterable[str]] = ..., Connector: _Optional[str] = ...) -> None: ...

class UserDelete(_message.Message):
    __slots__ = ["Metadata", "User", "Resource"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Resource: ResourceMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ...) -> None: ...

class UserPasswordChange(_message.Message):
    __slots__ = ["Metadata", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class AccessRequestCreate(_message.Message):
    __slots__ = ["Metadata", "User", "Resource", "Roles", "RequestID", "RequestState", "Delegator", "Reason", "Annotations", "Reviewer", "ProposedState", "RequestedResourceIDs", "MaxDuration", "PromotedAccessListName"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    REQUESTID_FIELD_NUMBER: _ClassVar[int]
    REQUESTSTATE_FIELD_NUMBER: _ClassVar[int]
    DELEGATOR_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    ANNOTATIONS_FIELD_NUMBER: _ClassVar[int]
    REVIEWER_FIELD_NUMBER: _ClassVar[int]
    PROPOSEDSTATE_FIELD_NUMBER: _ClassVar[int]
    REQUESTEDRESOURCEIDS_FIELD_NUMBER: _ClassVar[int]
    MAXDURATION_FIELD_NUMBER: _ClassVar[int]
    PROMOTEDACCESSLISTNAME_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Resource: ResourceMetadata
    Roles: _containers.RepeatedScalarFieldContainer[str]
    RequestID: str
    RequestState: str
    Delegator: str
    Reason: str
    Annotations: _struct_pb2.Struct
    Reviewer: str
    ProposedState: str
    RequestedResourceIDs: _containers.RepeatedCompositeFieldContainer[ResourceID]
    MaxDuration: _timestamp_pb2.Timestamp
    PromotedAccessListName: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., Roles: _Optional[_Iterable[str]] = ..., RequestID: _Optional[str] = ..., RequestState: _Optional[str] = ..., Delegator: _Optional[str] = ..., Reason: _Optional[str] = ..., Annotations: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., Reviewer: _Optional[str] = ..., ProposedState: _Optional[str] = ..., RequestedResourceIDs: _Optional[_Iterable[_Union[ResourceID, _Mapping]]] = ..., MaxDuration: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., PromotedAccessListName: _Optional[str] = ...) -> None: ...

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

class AccessRequestDelete(_message.Message):
    __slots__ = ["Metadata", "User", "RequestID"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    REQUESTID_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    RequestID: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., RequestID: _Optional[str] = ...) -> None: ...

class PortForward(_message.Message):
    __slots__ = ["Metadata", "User", "Connection", "Status", "Addr"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ADDR_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Connection: ConnectionMetadata
    Status: Status
    Addr: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ..., Addr: _Optional[str] = ...) -> None: ...

class X11Forward(_message.Message):
    __slots__ = ["Metadata", "User", "Connection", "Status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Connection: ConnectionMetadata
    Status: Status
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ...) -> None: ...

class CommandMetadata(_message.Message):
    __slots__ = ["Command", "ExitCode", "Error"]
    COMMAND_FIELD_NUMBER: _ClassVar[int]
    EXITCODE_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    Command: str
    ExitCode: str
    Error: str
    def __init__(self, Command: _Optional[str] = ..., ExitCode: _Optional[str] = ..., Error: _Optional[str] = ...) -> None: ...

class Exec(_message.Message):
    __slots__ = ["Metadata", "User", "Connection", "Session", "Server", "Command", "KubernetesCluster", "KubernetesPod"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    COMMAND_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESCLUSTER_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESPOD_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Connection: ConnectionMetadata
    Session: SessionMetadata
    Server: ServerMetadata
    Command: CommandMetadata
    KubernetesCluster: KubernetesClusterMetadata
    KubernetesPod: KubernetesPodMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., Command: _Optional[_Union[CommandMetadata, _Mapping]] = ..., KubernetesCluster: _Optional[_Union[KubernetesClusterMetadata, _Mapping]] = ..., KubernetesPod: _Optional[_Union[KubernetesPodMetadata, _Mapping]] = ...) -> None: ...

class SCP(_message.Message):
    __slots__ = ["Metadata", "User", "Connection", "Session", "Server", "Command", "Path", "Action"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    COMMAND_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    ACTION_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Connection: ConnectionMetadata
    Session: SessionMetadata
    Server: ServerMetadata
    Command: CommandMetadata
    Path: str
    Action: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., Command: _Optional[_Union[CommandMetadata, _Mapping]] = ..., Path: _Optional[str] = ..., Action: _Optional[str] = ...) -> None: ...

class SFTPAttributes(_message.Message):
    __slots__ = ["FileSize", "UID", "GID", "Permissions", "AccessTime", "ModificationTime"]
    FILESIZE_FIELD_NUMBER: _ClassVar[int]
    UID_FIELD_NUMBER: _ClassVar[int]
    GID_FIELD_NUMBER: _ClassVar[int]
    PERMISSIONS_FIELD_NUMBER: _ClassVar[int]
    ACCESSTIME_FIELD_NUMBER: _ClassVar[int]
    MODIFICATIONTIME_FIELD_NUMBER: _ClassVar[int]
    FileSize: _wrappers_pb2.UInt64Value
    UID: _wrappers_pb2.UInt32Value
    GID: _wrappers_pb2.UInt32Value
    Permissions: _wrappers_pb2.UInt32Value
    AccessTime: _timestamp_pb2.Timestamp
    ModificationTime: _timestamp_pb2.Timestamp
    def __init__(self, FileSize: _Optional[_Union[_wrappers_pb2.UInt64Value, _Mapping]] = ..., UID: _Optional[_Union[_wrappers_pb2.UInt32Value, _Mapping]] = ..., GID: _Optional[_Union[_wrappers_pb2.UInt32Value, _Mapping]] = ..., Permissions: _Optional[_Union[_wrappers_pb2.UInt32Value, _Mapping]] = ..., AccessTime: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., ModificationTime: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class SFTP(_message.Message):
    __slots__ = ["Metadata", "User", "Connection", "Session", "Server", "WorkingDirectory", "Path", "TargetPath", "Flags", "Attributes", "Action", "Error"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    WORKINGDIRECTORY_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    TARGETPATH_FIELD_NUMBER: _ClassVar[int]
    FLAGS_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    ACTION_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Connection: ConnectionMetadata
    Session: SessionMetadata
    Server: ServerMetadata
    WorkingDirectory: str
    Path: str
    TargetPath: str
    Flags: int
    Attributes: SFTPAttributes
    Action: SFTPAction
    Error: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., WorkingDirectory: _Optional[str] = ..., Path: _Optional[str] = ..., TargetPath: _Optional[str] = ..., Flags: _Optional[int] = ..., Attributes: _Optional[_Union[SFTPAttributes, _Mapping]] = ..., Action: _Optional[_Union[SFTPAction, str]] = ..., Error: _Optional[str] = ...) -> None: ...

class Subsystem(_message.Message):
    __slots__ = ["Metadata", "User", "Connection", "Name", "Error"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Connection: ConnectionMetadata
    Name: str
    Error: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., Name: _Optional[str] = ..., Error: _Optional[str] = ...) -> None: ...

class ClientDisconnect(_message.Message):
    __slots__ = ["Metadata", "User", "Connection", "Server", "Reason"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Connection: ConnectionMetadata
    Server: ServerMetadata
    Reason: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., Reason: _Optional[str] = ...) -> None: ...

class AuthAttempt(_message.Message):
    __slots__ = ["Metadata", "User", "Connection", "Status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Connection: ConnectionMetadata
    Status: Status
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ...) -> None: ...

class UserTokenCreate(_message.Message):
    __slots__ = ["Metadata", "Resource", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class RoleCreate(_message.Message):
    __slots__ = ["Metadata", "Resource", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class RoleDelete(_message.Message):
    __slots__ = ["Metadata", "Resource", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class TrustedClusterCreate(_message.Message):
    __slots__ = ["Metadata", "Resource", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class TrustedClusterDelete(_message.Message):
    __slots__ = ["Metadata", "Resource", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class ProvisionTokenCreate(_message.Message):
    __slots__ = ["Metadata", "Resource", "User", "Roles", "JoinMethod"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    JOINMETHOD_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    User: UserMetadata
    Roles: _containers.RepeatedScalarFieldContainer[str]
    JoinMethod: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Roles: _Optional[_Iterable[str]] = ..., JoinMethod: _Optional[str] = ...) -> None: ...

class TrustedClusterTokenCreate(_message.Message):
    __slots__ = ["Metadata", "Resource", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class GithubConnectorCreate(_message.Message):
    __slots__ = ["Metadata", "Resource", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class GithubConnectorDelete(_message.Message):
    __slots__ = ["Metadata", "Resource", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class OIDCConnectorCreate(_message.Message):
    __slots__ = ["Metadata", "Resource", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class OIDCConnectorDelete(_message.Message):
    __slots__ = ["Metadata", "Resource", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class SAMLConnectorCreate(_message.Message):
    __slots__ = ["Metadata", "Resource", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class SAMLConnectorDelete(_message.Message):
    __slots__ = ["Metadata", "Resource", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class KubeRequest(_message.Message):
    __slots__ = ["Metadata", "User", "Connection", "Server", "RequestPath", "Verb", "ResourceAPIGroup", "ResourceNamespace", "ResourceKind", "ResourceName", "ResponseCode", "Kubernetes"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    REQUESTPATH_FIELD_NUMBER: _ClassVar[int]
    VERB_FIELD_NUMBER: _ClassVar[int]
    RESOURCEAPIGROUP_FIELD_NUMBER: _ClassVar[int]
    RESOURCENAMESPACE_FIELD_NUMBER: _ClassVar[int]
    RESOURCEKIND_FIELD_NUMBER: _ClassVar[int]
    RESOURCENAME_FIELD_NUMBER: _ClassVar[int]
    RESPONSECODE_FIELD_NUMBER: _ClassVar[int]
    KUBERNETES_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Connection: ConnectionMetadata
    Server: ServerMetadata
    RequestPath: str
    Verb: str
    ResourceAPIGroup: str
    ResourceNamespace: str
    ResourceKind: str
    ResourceName: str
    ResponseCode: int
    Kubernetes: KubernetesClusterMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., RequestPath: _Optional[str] = ..., Verb: _Optional[str] = ..., ResourceAPIGroup: _Optional[str] = ..., ResourceNamespace: _Optional[str] = ..., ResourceKind: _Optional[str] = ..., ResourceName: _Optional[str] = ..., ResponseCode: _Optional[int] = ..., Kubernetes: _Optional[_Union[KubernetesClusterMetadata, _Mapping]] = ...) -> None: ...

class AppMetadata(_message.Message):
    __slots__ = ["AppURI", "AppPublicAddr", "AppLabels", "AppName"]
    class AppLabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    APPURI_FIELD_NUMBER: _ClassVar[int]
    APPPUBLICADDR_FIELD_NUMBER: _ClassVar[int]
    APPLABELS_FIELD_NUMBER: _ClassVar[int]
    APPNAME_FIELD_NUMBER: _ClassVar[int]
    AppURI: str
    AppPublicAddr: str
    AppLabels: _containers.ScalarMap[str, str]
    AppName: str
    def __init__(self, AppURI: _Optional[str] = ..., AppPublicAddr: _Optional[str] = ..., AppLabels: _Optional[_Mapping[str, str]] = ..., AppName: _Optional[str] = ...) -> None: ...

class AppCreate(_message.Message):
    __slots__ = ["Metadata", "User", "Resource", "App"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    APP_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Resource: ResourceMetadata
    App: AppMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., App: _Optional[_Union[AppMetadata, _Mapping]] = ...) -> None: ...

class AppUpdate(_message.Message):
    __slots__ = ["Metadata", "User", "Resource", "App"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    APP_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Resource: ResourceMetadata
    App: AppMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., App: _Optional[_Union[AppMetadata, _Mapping]] = ...) -> None: ...

class AppDelete(_message.Message):
    __slots__ = ["Metadata", "User", "Resource"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Resource: ResourceMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ...) -> None: ...

class AppSessionStart(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Server", "Connection", "PublicAddr", "App"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    PUBLICADDR_FIELD_NUMBER: _ClassVar[int]
    APP_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Server: ServerMetadata
    Connection: ConnectionMetadata
    PublicAddr: str
    App: AppMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., PublicAddr: _Optional[str] = ..., App: _Optional[_Union[AppMetadata, _Mapping]] = ...) -> None: ...

class AppSessionEnd(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Server", "Connection", "App"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    APP_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Server: ServerMetadata
    Connection: ConnectionMetadata
    App: AppMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., App: _Optional[_Union[AppMetadata, _Mapping]] = ...) -> None: ...

class AppSessionChunk(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Server", "Connection", "SessionChunkID", "App"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    SESSIONCHUNKID_FIELD_NUMBER: _ClassVar[int]
    APP_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Server: ServerMetadata
    Connection: ConnectionMetadata
    SessionChunkID: str
    App: AppMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., SessionChunkID: _Optional[str] = ..., App: _Optional[_Union[AppMetadata, _Mapping]] = ...) -> None: ...

class AppSessionRequest(_message.Message):
    __slots__ = ["Metadata", "StatusCode", "Path", "RawQuery", "Method", "App", "AWS"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    STATUSCODE_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    RAWQUERY_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    APP_FIELD_NUMBER: _ClassVar[int]
    AWS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    StatusCode: int
    Path: str
    RawQuery: str
    Method: str
    App: AppMetadata
    AWS: AWSRequestMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., StatusCode: _Optional[int] = ..., Path: _Optional[str] = ..., RawQuery: _Optional[str] = ..., Method: _Optional[str] = ..., App: _Optional[_Union[AppMetadata, _Mapping]] = ..., AWS: _Optional[_Union[AWSRequestMetadata, _Mapping]] = ...) -> None: ...

class AWSRequestMetadata(_message.Message):
    __slots__ = ["AWSRegion", "AWSService", "AWSHost", "AWSAssumedRole"]
    AWSREGION_FIELD_NUMBER: _ClassVar[int]
    AWSSERVICE_FIELD_NUMBER: _ClassVar[int]
    AWSHOST_FIELD_NUMBER: _ClassVar[int]
    AWSASSUMEDROLE_FIELD_NUMBER: _ClassVar[int]
    AWSRegion: str
    AWSService: str
    AWSHost: str
    AWSAssumedRole: str
    def __init__(self, AWSRegion: _Optional[str] = ..., AWSService: _Optional[str] = ..., AWSHost: _Optional[str] = ..., AWSAssumedRole: _Optional[str] = ...) -> None: ...

class DatabaseMetadata(_message.Message):
    __slots__ = ["DatabaseService", "DatabaseProtocol", "DatabaseURI", "DatabaseName", "DatabaseUser", "DatabaseLabels", "DatabaseAWSRegion", "DatabaseAWSRedshiftClusterID", "DatabaseGCPProjectID", "DatabaseGCPInstanceID", "DatabaseRoles", "DatabaseType", "DatabaseOrigin"]
    class DatabaseLabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    DATABASESERVICE_FIELD_NUMBER: _ClassVar[int]
    DATABASEPROTOCOL_FIELD_NUMBER: _ClassVar[int]
    DATABASEURI_FIELD_NUMBER: _ClassVar[int]
    DATABASENAME_FIELD_NUMBER: _ClassVar[int]
    DATABASEUSER_FIELD_NUMBER: _ClassVar[int]
    DATABASELABELS_FIELD_NUMBER: _ClassVar[int]
    DATABASEAWSREGION_FIELD_NUMBER: _ClassVar[int]
    DATABASEAWSREDSHIFTCLUSTERID_FIELD_NUMBER: _ClassVar[int]
    DATABASEGCPPROJECTID_FIELD_NUMBER: _ClassVar[int]
    DATABASEGCPINSTANCEID_FIELD_NUMBER: _ClassVar[int]
    DATABASEROLES_FIELD_NUMBER: _ClassVar[int]
    DATABASETYPE_FIELD_NUMBER: _ClassVar[int]
    DATABASEORIGIN_FIELD_NUMBER: _ClassVar[int]
    DatabaseService: str
    DatabaseProtocol: str
    DatabaseURI: str
    DatabaseName: str
    DatabaseUser: str
    DatabaseLabels: _containers.ScalarMap[str, str]
    DatabaseAWSRegion: str
    DatabaseAWSRedshiftClusterID: str
    DatabaseGCPProjectID: str
    DatabaseGCPInstanceID: str
    DatabaseRoles: _containers.RepeatedScalarFieldContainer[str]
    DatabaseType: str
    DatabaseOrigin: str
    def __init__(self, DatabaseService: _Optional[str] = ..., DatabaseProtocol: _Optional[str] = ..., DatabaseURI: _Optional[str] = ..., DatabaseName: _Optional[str] = ..., DatabaseUser: _Optional[str] = ..., DatabaseLabels: _Optional[_Mapping[str, str]] = ..., DatabaseAWSRegion: _Optional[str] = ..., DatabaseAWSRedshiftClusterID: _Optional[str] = ..., DatabaseGCPProjectID: _Optional[str] = ..., DatabaseGCPInstanceID: _Optional[str] = ..., DatabaseRoles: _Optional[_Iterable[str]] = ..., DatabaseType: _Optional[str] = ..., DatabaseOrigin: _Optional[str] = ...) -> None: ...

class DatabaseCreate(_message.Message):
    __slots__ = ["Metadata", "User", "Resource", "Database"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Resource: ResourceMetadata
    Database: DatabaseMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ...) -> None: ...

class DatabaseUpdate(_message.Message):
    __slots__ = ["Metadata", "User", "Resource", "Database"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Resource: ResourceMetadata
    Database: DatabaseMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ...) -> None: ...

class DatabaseDelete(_message.Message):
    __slots__ = ["Metadata", "User", "Resource"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Resource: ResourceMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ...) -> None: ...

class DatabaseSessionStart(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Server", "Connection", "Status", "Database"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Server: ServerMetadata
    Connection: ConnectionMetadata
    Status: Status
    Database: DatabaseMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ...) -> None: ...

class DatabaseSessionQuery(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "DatabaseQuery", "DatabaseQueryParameters", "Status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    DATABASEQUERY_FIELD_NUMBER: _ClassVar[int]
    DATABASEQUERYPARAMETERS_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    DatabaseQuery: str
    DatabaseQueryParameters: _containers.RepeatedScalarFieldContainer[str]
    Status: Status
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., DatabaseQuery: _Optional[str] = ..., DatabaseQueryParameters: _Optional[_Iterable[str]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ...) -> None: ...

class PostgresParse(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "StatementName", "Query"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    STATEMENTNAME_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    StatementName: str
    Query: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., StatementName: _Optional[str] = ..., Query: _Optional[str] = ...) -> None: ...

class PostgresBind(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "StatementName", "PortalName", "Parameters"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    STATEMENTNAME_FIELD_NUMBER: _ClassVar[int]
    PORTALNAME_FIELD_NUMBER: _ClassVar[int]
    PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    StatementName: str
    PortalName: str
    Parameters: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., StatementName: _Optional[str] = ..., PortalName: _Optional[str] = ..., Parameters: _Optional[_Iterable[str]] = ...) -> None: ...

class PostgresExecute(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "PortalName"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    PORTALNAME_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    PortalName: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., PortalName: _Optional[str] = ...) -> None: ...

class PostgresClose(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "StatementName", "PortalName"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    STATEMENTNAME_FIELD_NUMBER: _ClassVar[int]
    PORTALNAME_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    StatementName: str
    PortalName: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., StatementName: _Optional[str] = ..., PortalName: _Optional[str] = ...) -> None: ...

class PostgresFunctionCall(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "FunctionOID", "FunctionArgs"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    FUNCTIONOID_FIELD_NUMBER: _ClassVar[int]
    FUNCTIONARGS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    FunctionOID: int
    FunctionArgs: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., FunctionOID: _Optional[int] = ..., FunctionArgs: _Optional[_Iterable[str]] = ...) -> None: ...

class WindowsDesktopSessionStart(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Connection", "Status", "WindowsDesktopService", "DesktopAddr", "Domain", "WindowsUser", "DesktopLabels", "DesktopName"]
    class DesktopLabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    CONNECTION_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    WINDOWSDESKTOPSERVICE_FIELD_NUMBER: _ClassVar[int]
    DESKTOPADDR_FIELD_NUMBER: _ClassVar[int]
    DOMAIN_FIELD_NUMBER: _ClassVar[int]
    WINDOWSUSER_FIELD_NUMBER: _ClassVar[int]
    DESKTOPLABELS_FIELD_NUMBER: _ClassVar[int]
    DESKTOPNAME_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Connection: ConnectionMetadata
    Status: Status
    WindowsDesktopService: str
    DesktopAddr: str
    Domain: str
    WindowsUser: str
    DesktopLabels: _containers.ScalarMap[str, str]
    DesktopName: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Connection: _Optional[_Union[ConnectionMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ..., WindowsDesktopService: _Optional[str] = ..., DesktopAddr: _Optional[str] = ..., Domain: _Optional[str] = ..., WindowsUser: _Optional[str] = ..., DesktopLabels: _Optional[_Mapping[str, str]] = ..., DesktopName: _Optional[str] = ...) -> None: ...

class DatabaseSessionEnd(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ...) -> None: ...

class MFADeviceMetadata(_message.Message):
    __slots__ = ["DeviceName", "DeviceID", "DeviceType"]
    DEVICENAME_FIELD_NUMBER: _ClassVar[int]
    DEVICEID_FIELD_NUMBER: _ClassVar[int]
    DEVICETYPE_FIELD_NUMBER: _ClassVar[int]
    DeviceName: str
    DeviceID: str
    DeviceType: str
    def __init__(self, DeviceName: _Optional[str] = ..., DeviceID: _Optional[str] = ..., DeviceType: _Optional[str] = ...) -> None: ...

class MFADeviceAdd(_message.Message):
    __slots__ = ["Metadata", "User", "Device"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Device: MFADeviceMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Device: _Optional[_Union[MFADeviceMetadata, _Mapping]] = ...) -> None: ...

class MFADeviceDelete(_message.Message):
    __slots__ = ["Metadata", "User", "Device"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Device: MFADeviceMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Device: _Optional[_Union[MFADeviceMetadata, _Mapping]] = ...) -> None: ...

class BillingInformationUpdate(_message.Message):
    __slots__ = ["Metadata", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class BillingCardCreate(_message.Message):
    __slots__ = ["Metadata", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class BillingCardDelete(_message.Message):
    __slots__ = ["Metadata", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class LockCreate(_message.Message):
    __slots__ = ["Metadata", "Resource", "User", "Target"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    TARGET_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    User: UserMetadata
    Target: _types_pb2.LockTarget
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Target: _Optional[_Union[_types_pb2.LockTarget, _Mapping]] = ...) -> None: ...

class LockDelete(_message.Message):
    __slots__ = ["Metadata", "Resource", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class RecoveryCodeGenerate(_message.Message):
    __slots__ = ["Metadata", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class RecoveryCodeUsed(_message.Message):
    __slots__ = ["Metadata", "User", "Status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Status: Status
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ...) -> None: ...

class WindowsDesktopSessionEnd(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "WindowsDesktopService", "DesktopAddr", "Domain", "WindowsUser", "DesktopLabels", "StartTime", "EndTime", "DesktopName", "Recorded", "Participants"]
    class DesktopLabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    WINDOWSDESKTOPSERVICE_FIELD_NUMBER: _ClassVar[int]
    DESKTOPADDR_FIELD_NUMBER: _ClassVar[int]
    DOMAIN_FIELD_NUMBER: _ClassVar[int]
    WINDOWSUSER_FIELD_NUMBER: _ClassVar[int]
    DESKTOPLABELS_FIELD_NUMBER: _ClassVar[int]
    STARTTIME_FIELD_NUMBER: _ClassVar[int]
    ENDTIME_FIELD_NUMBER: _ClassVar[int]
    DESKTOPNAME_FIELD_NUMBER: _ClassVar[int]
    RECORDED_FIELD_NUMBER: _ClassVar[int]
    PARTICIPANTS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    WindowsDesktopService: str
    DesktopAddr: str
    Domain: str
    WindowsUser: str
    DesktopLabels: _containers.ScalarMap[str, str]
    StartTime: _timestamp_pb2.Timestamp
    EndTime: _timestamp_pb2.Timestamp
    DesktopName: str
    Recorded: bool
    Participants: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., WindowsDesktopService: _Optional[str] = ..., DesktopAddr: _Optional[str] = ..., Domain: _Optional[str] = ..., WindowsUser: _Optional[str] = ..., DesktopLabels: _Optional[_Mapping[str, str]] = ..., StartTime: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., EndTime: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., DesktopName: _Optional[str] = ..., Recorded: bool = ..., Participants: _Optional[_Iterable[str]] = ...) -> None: ...

class CertificateCreate(_message.Message):
    __slots__ = ["Metadata", "CertificateType", "Identity"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    CERTIFICATETYPE_FIELD_NUMBER: _ClassVar[int]
    IDENTITY_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    CertificateType: str
    Identity: Identity
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., CertificateType: _Optional[str] = ..., Identity: _Optional[_Union[Identity, _Mapping]] = ...) -> None: ...

class RenewableCertificateGenerationMismatch(_message.Message):
    __slots__ = ["Metadata", "UserMetadata"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USERMETADATA_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    UserMetadata: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., UserMetadata: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class BotJoin(_message.Message):
    __slots__ = ["Metadata", "Status", "BotName", "Method", "TokenName", "Attributes"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    BOTNAME_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    TOKENNAME_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Status: Status
    BotName: str
    Method: str
    TokenName: str
    Attributes: _struct_pb2.Struct
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ..., BotName: _Optional[str] = ..., Method: _Optional[str] = ..., TokenName: _Optional[str] = ..., Attributes: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class InstanceJoin(_message.Message):
    __slots__ = ["Metadata", "Status", "HostID", "NodeName", "Role", "Method", "TokenName", "Attributes"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    HOSTID_FIELD_NUMBER: _ClassVar[int]
    NODENAME_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    TOKENNAME_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Status: Status
    HostID: str
    NodeName: str
    Role: str
    Method: str
    TokenName: str
    Attributes: _struct_pb2.Struct
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ..., HostID: _Optional[str] = ..., NodeName: _Optional[str] = ..., Role: _Optional[str] = ..., Method: _Optional[str] = ..., TokenName: _Optional[str] = ..., Attributes: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class Unknown(_message.Message):
    __slots__ = ["Metadata", "UnknownType", "UnknownCode", "Data"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    UNKNOWNTYPE_FIELD_NUMBER: _ClassVar[int]
    UNKNOWNCODE_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    UnknownType: str
    UnknownCode: str
    Data: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., UnknownType: _Optional[str] = ..., UnknownCode: _Optional[str] = ..., Data: _Optional[str] = ...) -> None: ...

class DeviceMetadata(_message.Message):
    __slots__ = ["device_id", "os_type", "asset_tag", "credential_id", "device_origin"]
    DEVICE_ID_FIELD_NUMBER: _ClassVar[int]
    OS_TYPE_FIELD_NUMBER: _ClassVar[int]
    ASSET_TAG_FIELD_NUMBER: _ClassVar[int]
    CREDENTIAL_ID_FIELD_NUMBER: _ClassVar[int]
    DEVICE_ORIGIN_FIELD_NUMBER: _ClassVar[int]
    device_id: str
    os_type: OSType
    asset_tag: str
    credential_id: str
    device_origin: DeviceOrigin
    def __init__(self, device_id: _Optional[str] = ..., os_type: _Optional[_Union[OSType, str]] = ..., asset_tag: _Optional[str] = ..., credential_id: _Optional[str] = ..., device_origin: _Optional[_Union[DeviceOrigin, str]] = ...) -> None: ...

class DeviceEvent(_message.Message):
    __slots__ = ["metadata", "status", "device", "user"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    metadata: Metadata
    status: Status
    device: DeviceMetadata
    user: UserMetadata
    def __init__(self, metadata: _Optional[_Union[Metadata, _Mapping]] = ..., status: _Optional[_Union[Status, _Mapping]] = ..., device: _Optional[_Union[DeviceMetadata, _Mapping]] = ..., user: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class DeviceEvent2(_message.Message):
    __slots__ = ["metadata", "device", "status", "user"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    metadata: Metadata
    device: DeviceMetadata
    status: Status
    user: UserMetadata
    def __init__(self, metadata: _Optional[_Union[Metadata, _Mapping]] = ..., device: _Optional[_Union[DeviceMetadata, _Mapping]] = ..., status: _Optional[_Union[Status, _Mapping]] = ..., user: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class OneOf(_message.Message):
    __slots__ = ["UserLogin", "UserCreate", "UserDelete", "UserPasswordChange", "SessionStart", "SessionJoin", "SessionPrint", "SessionReject", "Resize", "SessionEnd", "SessionCommand", "SessionDisk", "SessionNetwork", "SessionData", "SessionLeave", "PortForward", "X11Forward", "SCP", "Exec", "Subsystem", "ClientDisconnect", "AuthAttempt", "AccessRequestCreate", "UserTokenCreate", "RoleCreate", "RoleDelete", "TrustedClusterCreate", "TrustedClusterDelete", "TrustedClusterTokenCreate", "GithubConnectorCreate", "GithubConnectorDelete", "OIDCConnectorCreate", "OIDCConnectorDelete", "SAMLConnectorCreate", "SAMLConnectorDelete", "KubeRequest", "AppSessionStart", "AppSessionChunk", "AppSessionRequest", "DatabaseSessionStart", "DatabaseSessionEnd", "DatabaseSessionQuery", "SessionUpload", "MFADeviceAdd", "MFADeviceDelete", "BillingInformationUpdate", "BillingCardCreate", "BillingCardDelete", "LockCreate", "LockDelete", "RecoveryCodeGenerate", "RecoveryCodeUsed", "DatabaseCreate", "DatabaseUpdate", "DatabaseDelete", "AppCreate", "AppUpdate", "AppDelete", "WindowsDesktopSessionStart", "WindowsDesktopSessionEnd", "PostgresParse", "PostgresBind", "PostgresExecute", "PostgresClose", "PostgresFunctionCall", "AccessRequestDelete", "SessionConnect", "CertificateCreate", "DesktopRecording", "DesktopClipboardSend", "DesktopClipboardReceive", "MySQLStatementPrepare", "MySQLStatementExecute", "MySQLStatementSendLongData", "MySQLStatementClose", "MySQLStatementReset", "MySQLStatementFetch", "MySQLStatementBulkExecute", "RenewableCertificateGenerationMismatch", "Unknown", "MySQLInitDB", "MySQLCreateDB", "MySQLDropDB", "MySQLShutDown", "MySQLProcessKill", "MySQLDebug", "MySQLRefresh", "AccessRequestResourceSearch", "SQLServerRPCRequest", "DatabaseSessionMalformedPacket", "SFTP", "UpgradeWindowStartUpdate", "AppSessionEnd", "SessionRecordingAccess", "KubernetesClusterCreate", "KubernetesClusterUpdate", "KubernetesClusterDelete", "SSMRun", "ElasticsearchRequest", "CassandraBatch", "CassandraPrepare", "CassandraRegister", "CassandraExecute", "AppSessionDynamoDBRequest", "DesktopSharedDirectoryStart", "DesktopSharedDirectoryRead", "DesktopSharedDirectoryWrite", "DynamoDBRequest", "BotJoin", "InstanceJoin", "DeviceEvent", "LoginRuleCreate", "LoginRuleDelete", "SAMLIdPAuthAttempt", "SAMLIdPServiceProviderCreate", "SAMLIdPServiceProviderUpdate", "SAMLIdPServiceProviderDelete", "SAMLIdPServiceProviderDeleteAll", "OpenSearchRequest", "DeviceEvent2", "OktaResourcesUpdate", "OktaSyncFailure", "OktaAssignmentResult", "ProvisionTokenCreate", "AccessListCreate", "AccessListUpdate", "AccessListDelete", "AccessListReview", "AccessListMemberCreate", "AccessListMemberUpdate", "AccessListMemberDelete", "AccessListMemberDeleteAllForAccessList"]
    USERLOGIN_FIELD_NUMBER: _ClassVar[int]
    USERCREATE_FIELD_NUMBER: _ClassVar[int]
    USERDELETE_FIELD_NUMBER: _ClassVar[int]
    USERPASSWORDCHANGE_FIELD_NUMBER: _ClassVar[int]
    SESSIONSTART_FIELD_NUMBER: _ClassVar[int]
    SESSIONJOIN_FIELD_NUMBER: _ClassVar[int]
    SESSIONPRINT_FIELD_NUMBER: _ClassVar[int]
    SESSIONREJECT_FIELD_NUMBER: _ClassVar[int]
    RESIZE_FIELD_NUMBER: _ClassVar[int]
    SESSIONEND_FIELD_NUMBER: _ClassVar[int]
    SESSIONCOMMAND_FIELD_NUMBER: _ClassVar[int]
    SESSIONDISK_FIELD_NUMBER: _ClassVar[int]
    SESSIONNETWORK_FIELD_NUMBER: _ClassVar[int]
    SESSIONDATA_FIELD_NUMBER: _ClassVar[int]
    SESSIONLEAVE_FIELD_NUMBER: _ClassVar[int]
    PORTFORWARD_FIELD_NUMBER: _ClassVar[int]
    X11FORWARD_FIELD_NUMBER: _ClassVar[int]
    SCP_FIELD_NUMBER: _ClassVar[int]
    EXEC_FIELD_NUMBER: _ClassVar[int]
    SUBSYSTEM_FIELD_NUMBER: _ClassVar[int]
    CLIENTDISCONNECT_FIELD_NUMBER: _ClassVar[int]
    AUTHATTEMPT_FIELD_NUMBER: _ClassVar[int]
    ACCESSREQUESTCREATE_FIELD_NUMBER: _ClassVar[int]
    USERTOKENCREATE_FIELD_NUMBER: _ClassVar[int]
    ROLECREATE_FIELD_NUMBER: _ClassVar[int]
    ROLEDELETE_FIELD_NUMBER: _ClassVar[int]
    TRUSTEDCLUSTERCREATE_FIELD_NUMBER: _ClassVar[int]
    TRUSTEDCLUSTERDELETE_FIELD_NUMBER: _ClassVar[int]
    TRUSTEDCLUSTERTOKENCREATE_FIELD_NUMBER: _ClassVar[int]
    GITHUBCONNECTORCREATE_FIELD_NUMBER: _ClassVar[int]
    GITHUBCONNECTORDELETE_FIELD_NUMBER: _ClassVar[int]
    OIDCCONNECTORCREATE_FIELD_NUMBER: _ClassVar[int]
    OIDCCONNECTORDELETE_FIELD_NUMBER: _ClassVar[int]
    SAMLCONNECTORCREATE_FIELD_NUMBER: _ClassVar[int]
    SAMLCONNECTORDELETE_FIELD_NUMBER: _ClassVar[int]
    KUBEREQUEST_FIELD_NUMBER: _ClassVar[int]
    APPSESSIONSTART_FIELD_NUMBER: _ClassVar[int]
    APPSESSIONCHUNK_FIELD_NUMBER: _ClassVar[int]
    APPSESSIONREQUEST_FIELD_NUMBER: _ClassVar[int]
    DATABASESESSIONSTART_FIELD_NUMBER: _ClassVar[int]
    DATABASESESSIONEND_FIELD_NUMBER: _ClassVar[int]
    DATABASESESSIONQUERY_FIELD_NUMBER: _ClassVar[int]
    SESSIONUPLOAD_FIELD_NUMBER: _ClassVar[int]
    MFADEVICEADD_FIELD_NUMBER: _ClassVar[int]
    MFADEVICEDELETE_FIELD_NUMBER: _ClassVar[int]
    BILLINGINFORMATIONUPDATE_FIELD_NUMBER: _ClassVar[int]
    BILLINGCARDCREATE_FIELD_NUMBER: _ClassVar[int]
    BILLINGCARDDELETE_FIELD_NUMBER: _ClassVar[int]
    LOCKCREATE_FIELD_NUMBER: _ClassVar[int]
    LOCKDELETE_FIELD_NUMBER: _ClassVar[int]
    RECOVERYCODEGENERATE_FIELD_NUMBER: _ClassVar[int]
    RECOVERYCODEUSED_FIELD_NUMBER: _ClassVar[int]
    DATABASECREATE_FIELD_NUMBER: _ClassVar[int]
    DATABASEUPDATE_FIELD_NUMBER: _ClassVar[int]
    DATABASEDELETE_FIELD_NUMBER: _ClassVar[int]
    APPCREATE_FIELD_NUMBER: _ClassVar[int]
    APPUPDATE_FIELD_NUMBER: _ClassVar[int]
    APPDELETE_FIELD_NUMBER: _ClassVar[int]
    WINDOWSDESKTOPSESSIONSTART_FIELD_NUMBER: _ClassVar[int]
    WINDOWSDESKTOPSESSIONEND_FIELD_NUMBER: _ClassVar[int]
    POSTGRESPARSE_FIELD_NUMBER: _ClassVar[int]
    POSTGRESBIND_FIELD_NUMBER: _ClassVar[int]
    POSTGRESEXECUTE_FIELD_NUMBER: _ClassVar[int]
    POSTGRESCLOSE_FIELD_NUMBER: _ClassVar[int]
    POSTGRESFUNCTIONCALL_FIELD_NUMBER: _ClassVar[int]
    ACCESSREQUESTDELETE_FIELD_NUMBER: _ClassVar[int]
    SESSIONCONNECT_FIELD_NUMBER: _ClassVar[int]
    CERTIFICATECREATE_FIELD_NUMBER: _ClassVar[int]
    DESKTOPRECORDING_FIELD_NUMBER: _ClassVar[int]
    DESKTOPCLIPBOARDSEND_FIELD_NUMBER: _ClassVar[int]
    DESKTOPCLIPBOARDRECEIVE_FIELD_NUMBER: _ClassVar[int]
    MYSQLSTATEMENTPREPARE_FIELD_NUMBER: _ClassVar[int]
    MYSQLSTATEMENTEXECUTE_FIELD_NUMBER: _ClassVar[int]
    MYSQLSTATEMENTSENDLONGDATA_FIELD_NUMBER: _ClassVar[int]
    MYSQLSTATEMENTCLOSE_FIELD_NUMBER: _ClassVar[int]
    MYSQLSTATEMENTRESET_FIELD_NUMBER: _ClassVar[int]
    MYSQLSTATEMENTFETCH_FIELD_NUMBER: _ClassVar[int]
    MYSQLSTATEMENTBULKEXECUTE_FIELD_NUMBER: _ClassVar[int]
    RENEWABLECERTIFICATEGENERATIONMISMATCH_FIELD_NUMBER: _ClassVar[int]
    UNKNOWN_FIELD_NUMBER: _ClassVar[int]
    MYSQLINITDB_FIELD_NUMBER: _ClassVar[int]
    MYSQLCREATEDB_FIELD_NUMBER: _ClassVar[int]
    MYSQLDROPDB_FIELD_NUMBER: _ClassVar[int]
    MYSQLSHUTDOWN_FIELD_NUMBER: _ClassVar[int]
    MYSQLPROCESSKILL_FIELD_NUMBER: _ClassVar[int]
    MYSQLDEBUG_FIELD_NUMBER: _ClassVar[int]
    MYSQLREFRESH_FIELD_NUMBER: _ClassVar[int]
    ACCESSREQUESTRESOURCESEARCH_FIELD_NUMBER: _ClassVar[int]
    SQLSERVERRPCREQUEST_FIELD_NUMBER: _ClassVar[int]
    DATABASESESSIONMALFORMEDPACKET_FIELD_NUMBER: _ClassVar[int]
    SFTP_FIELD_NUMBER: _ClassVar[int]
    UPGRADEWINDOWSTARTUPDATE_FIELD_NUMBER: _ClassVar[int]
    APPSESSIONEND_FIELD_NUMBER: _ClassVar[int]
    SESSIONRECORDINGACCESS_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESCLUSTERCREATE_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESCLUSTERUPDATE_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESCLUSTERDELETE_FIELD_NUMBER: _ClassVar[int]
    SSMRUN_FIELD_NUMBER: _ClassVar[int]
    ELASTICSEARCHREQUEST_FIELD_NUMBER: _ClassVar[int]
    CASSANDRABATCH_FIELD_NUMBER: _ClassVar[int]
    CASSANDRAPREPARE_FIELD_NUMBER: _ClassVar[int]
    CASSANDRAREGISTER_FIELD_NUMBER: _ClassVar[int]
    CASSANDRAEXECUTE_FIELD_NUMBER: _ClassVar[int]
    APPSESSIONDYNAMODBREQUEST_FIELD_NUMBER: _ClassVar[int]
    DESKTOPSHAREDDIRECTORYSTART_FIELD_NUMBER: _ClassVar[int]
    DESKTOPSHAREDDIRECTORYREAD_FIELD_NUMBER: _ClassVar[int]
    DESKTOPSHAREDDIRECTORYWRITE_FIELD_NUMBER: _ClassVar[int]
    DYNAMODBREQUEST_FIELD_NUMBER: _ClassVar[int]
    BOTJOIN_FIELD_NUMBER: _ClassVar[int]
    INSTANCEJOIN_FIELD_NUMBER: _ClassVar[int]
    DEVICEEVENT_FIELD_NUMBER: _ClassVar[int]
    LOGINRULECREATE_FIELD_NUMBER: _ClassVar[int]
    LOGINRULEDELETE_FIELD_NUMBER: _ClassVar[int]
    SAMLIDPAUTHATTEMPT_FIELD_NUMBER: _ClassVar[int]
    SAMLIDPSERVICEPROVIDERCREATE_FIELD_NUMBER: _ClassVar[int]
    SAMLIDPSERVICEPROVIDERUPDATE_FIELD_NUMBER: _ClassVar[int]
    SAMLIDPSERVICEPROVIDERDELETE_FIELD_NUMBER: _ClassVar[int]
    SAMLIDPSERVICEPROVIDERDELETEALL_FIELD_NUMBER: _ClassVar[int]
    OPENSEARCHREQUEST_FIELD_NUMBER: _ClassVar[int]
    DEVICEEVENT2_FIELD_NUMBER: _ClassVar[int]
    OKTARESOURCESUPDATE_FIELD_NUMBER: _ClassVar[int]
    OKTASYNCFAILURE_FIELD_NUMBER: _ClassVar[int]
    OKTAASSIGNMENTRESULT_FIELD_NUMBER: _ClassVar[int]
    PROVISIONTOKENCREATE_FIELD_NUMBER: _ClassVar[int]
    ACCESSLISTCREATE_FIELD_NUMBER: _ClassVar[int]
    ACCESSLISTUPDATE_FIELD_NUMBER: _ClassVar[int]
    ACCESSLISTDELETE_FIELD_NUMBER: _ClassVar[int]
    ACCESSLISTREVIEW_FIELD_NUMBER: _ClassVar[int]
    ACCESSLISTMEMBERCREATE_FIELD_NUMBER: _ClassVar[int]
    ACCESSLISTMEMBERUPDATE_FIELD_NUMBER: _ClassVar[int]
    ACCESSLISTMEMBERDELETE_FIELD_NUMBER: _ClassVar[int]
    ACCESSLISTMEMBERDELETEALLFORACCESSLIST_FIELD_NUMBER: _ClassVar[int]
    UserLogin: UserLogin
    UserCreate: UserCreate
    UserDelete: UserDelete
    UserPasswordChange: UserPasswordChange
    SessionStart: SessionStart
    SessionJoin: SessionJoin
    SessionPrint: SessionPrint
    SessionReject: SessionReject
    Resize: Resize
    SessionEnd: SessionEnd
    SessionCommand: SessionCommand
    SessionDisk: SessionDisk
    SessionNetwork: SessionNetwork
    SessionData: SessionData
    SessionLeave: SessionLeave
    PortForward: PortForward
    X11Forward: X11Forward
    SCP: SCP
    Exec: Exec
    Subsystem: Subsystem
    ClientDisconnect: ClientDisconnect
    AuthAttempt: AuthAttempt
    AccessRequestCreate: AccessRequestCreate
    UserTokenCreate: UserTokenCreate
    RoleCreate: RoleCreate
    RoleDelete: RoleDelete
    TrustedClusterCreate: TrustedClusterCreate
    TrustedClusterDelete: TrustedClusterDelete
    TrustedClusterTokenCreate: TrustedClusterTokenCreate
    GithubConnectorCreate: GithubConnectorCreate
    GithubConnectorDelete: GithubConnectorDelete
    OIDCConnectorCreate: OIDCConnectorCreate
    OIDCConnectorDelete: OIDCConnectorDelete
    SAMLConnectorCreate: SAMLConnectorCreate
    SAMLConnectorDelete: SAMLConnectorDelete
    KubeRequest: KubeRequest
    AppSessionStart: AppSessionStart
    AppSessionChunk: AppSessionChunk
    AppSessionRequest: AppSessionRequest
    DatabaseSessionStart: DatabaseSessionStart
    DatabaseSessionEnd: DatabaseSessionEnd
    DatabaseSessionQuery: DatabaseSessionQuery
    SessionUpload: SessionUpload
    MFADeviceAdd: MFADeviceAdd
    MFADeviceDelete: MFADeviceDelete
    BillingInformationUpdate: BillingInformationUpdate
    BillingCardCreate: BillingCardCreate
    BillingCardDelete: BillingCardDelete
    LockCreate: LockCreate
    LockDelete: LockDelete
    RecoveryCodeGenerate: RecoveryCodeGenerate
    RecoveryCodeUsed: RecoveryCodeUsed
    DatabaseCreate: DatabaseCreate
    DatabaseUpdate: DatabaseUpdate
    DatabaseDelete: DatabaseDelete
    AppCreate: AppCreate
    AppUpdate: AppUpdate
    AppDelete: AppDelete
    WindowsDesktopSessionStart: WindowsDesktopSessionStart
    WindowsDesktopSessionEnd: WindowsDesktopSessionEnd
    PostgresParse: PostgresParse
    PostgresBind: PostgresBind
    PostgresExecute: PostgresExecute
    PostgresClose: PostgresClose
    PostgresFunctionCall: PostgresFunctionCall
    AccessRequestDelete: AccessRequestDelete
    SessionConnect: SessionConnect
    CertificateCreate: CertificateCreate
    DesktopRecording: DesktopRecording
    DesktopClipboardSend: DesktopClipboardSend
    DesktopClipboardReceive: DesktopClipboardReceive
    MySQLStatementPrepare: MySQLStatementPrepare
    MySQLStatementExecute: MySQLStatementExecute
    MySQLStatementSendLongData: MySQLStatementSendLongData
    MySQLStatementClose: MySQLStatementClose
    MySQLStatementReset: MySQLStatementReset
    MySQLStatementFetch: MySQLStatementFetch
    MySQLStatementBulkExecute: MySQLStatementBulkExecute
    RenewableCertificateGenerationMismatch: RenewableCertificateGenerationMismatch
    Unknown: Unknown
    MySQLInitDB: MySQLInitDB
    MySQLCreateDB: MySQLCreateDB
    MySQLDropDB: MySQLDropDB
    MySQLShutDown: MySQLShutDown
    MySQLProcessKill: MySQLProcessKill
    MySQLDebug: MySQLDebug
    MySQLRefresh: MySQLRefresh
    AccessRequestResourceSearch: AccessRequestResourceSearch
    SQLServerRPCRequest: SQLServerRPCRequest
    DatabaseSessionMalformedPacket: DatabaseSessionMalformedPacket
    SFTP: SFTP
    UpgradeWindowStartUpdate: UpgradeWindowStartUpdate
    AppSessionEnd: AppSessionEnd
    SessionRecordingAccess: SessionRecordingAccess
    KubernetesClusterCreate: KubernetesClusterCreate
    KubernetesClusterUpdate: KubernetesClusterUpdate
    KubernetesClusterDelete: KubernetesClusterDelete
    SSMRun: SSMRun
    ElasticsearchRequest: ElasticsearchRequest
    CassandraBatch: CassandraBatch
    CassandraPrepare: CassandraPrepare
    CassandraRegister: CassandraRegister
    CassandraExecute: CassandraExecute
    AppSessionDynamoDBRequest: AppSessionDynamoDBRequest
    DesktopSharedDirectoryStart: DesktopSharedDirectoryStart
    DesktopSharedDirectoryRead: DesktopSharedDirectoryRead
    DesktopSharedDirectoryWrite: DesktopSharedDirectoryWrite
    DynamoDBRequest: DynamoDBRequest
    BotJoin: BotJoin
    InstanceJoin: InstanceJoin
    DeviceEvent: DeviceEvent
    LoginRuleCreate: LoginRuleCreate
    LoginRuleDelete: LoginRuleDelete
    SAMLIdPAuthAttempt: SAMLIdPAuthAttempt
    SAMLIdPServiceProviderCreate: SAMLIdPServiceProviderCreate
    SAMLIdPServiceProviderUpdate: SAMLIdPServiceProviderUpdate
    SAMLIdPServiceProviderDelete: SAMLIdPServiceProviderDelete
    SAMLIdPServiceProviderDeleteAll: SAMLIdPServiceProviderDeleteAll
    OpenSearchRequest: OpenSearchRequest
    DeviceEvent2: DeviceEvent2
    OktaResourcesUpdate: OktaResourcesUpdate
    OktaSyncFailure: OktaSyncFailure
    OktaAssignmentResult: OktaAssignmentResult
    ProvisionTokenCreate: ProvisionTokenCreate
    AccessListCreate: AccessListCreate
    AccessListUpdate: AccessListUpdate
    AccessListDelete: AccessListDelete
    AccessListReview: AccessListReview
    AccessListMemberCreate: AccessListMemberCreate
    AccessListMemberUpdate: AccessListMemberUpdate
    AccessListMemberDelete: AccessListMemberDelete
    AccessListMemberDeleteAllForAccessList: AccessListMemberDeleteAllForAccessList
    def __init__(self, UserLogin: _Optional[_Union[UserLogin, _Mapping]] = ..., UserCreate: _Optional[_Union[UserCreate, _Mapping]] = ..., UserDelete: _Optional[_Union[UserDelete, _Mapping]] = ..., UserPasswordChange: _Optional[_Union[UserPasswordChange, _Mapping]] = ..., SessionStart: _Optional[_Union[SessionStart, _Mapping]] = ..., SessionJoin: _Optional[_Union[SessionJoin, _Mapping]] = ..., SessionPrint: _Optional[_Union[SessionPrint, _Mapping]] = ..., SessionReject: _Optional[_Union[SessionReject, _Mapping]] = ..., Resize: _Optional[_Union[Resize, _Mapping]] = ..., SessionEnd: _Optional[_Union[SessionEnd, _Mapping]] = ..., SessionCommand: _Optional[_Union[SessionCommand, _Mapping]] = ..., SessionDisk: _Optional[_Union[SessionDisk, _Mapping]] = ..., SessionNetwork: _Optional[_Union[SessionNetwork, _Mapping]] = ..., SessionData: _Optional[_Union[SessionData, _Mapping]] = ..., SessionLeave: _Optional[_Union[SessionLeave, _Mapping]] = ..., PortForward: _Optional[_Union[PortForward, _Mapping]] = ..., X11Forward: _Optional[_Union[X11Forward, _Mapping]] = ..., SCP: _Optional[_Union[SCP, _Mapping]] = ..., Exec: _Optional[_Union[Exec, _Mapping]] = ..., Subsystem: _Optional[_Union[Subsystem, _Mapping]] = ..., ClientDisconnect: _Optional[_Union[ClientDisconnect, _Mapping]] = ..., AuthAttempt: _Optional[_Union[AuthAttempt, _Mapping]] = ..., AccessRequestCreate: _Optional[_Union[AccessRequestCreate, _Mapping]] = ..., UserTokenCreate: _Optional[_Union[UserTokenCreate, _Mapping]] = ..., RoleCreate: _Optional[_Union[RoleCreate, _Mapping]] = ..., RoleDelete: _Optional[_Union[RoleDelete, _Mapping]] = ..., TrustedClusterCreate: _Optional[_Union[TrustedClusterCreate, _Mapping]] = ..., TrustedClusterDelete: _Optional[_Union[TrustedClusterDelete, _Mapping]] = ..., TrustedClusterTokenCreate: _Optional[_Union[TrustedClusterTokenCreate, _Mapping]] = ..., GithubConnectorCreate: _Optional[_Union[GithubConnectorCreate, _Mapping]] = ..., GithubConnectorDelete: _Optional[_Union[GithubConnectorDelete, _Mapping]] = ..., OIDCConnectorCreate: _Optional[_Union[OIDCConnectorCreate, _Mapping]] = ..., OIDCConnectorDelete: _Optional[_Union[OIDCConnectorDelete, _Mapping]] = ..., SAMLConnectorCreate: _Optional[_Union[SAMLConnectorCreate, _Mapping]] = ..., SAMLConnectorDelete: _Optional[_Union[SAMLConnectorDelete, _Mapping]] = ..., KubeRequest: _Optional[_Union[KubeRequest, _Mapping]] = ..., AppSessionStart: _Optional[_Union[AppSessionStart, _Mapping]] = ..., AppSessionChunk: _Optional[_Union[AppSessionChunk, _Mapping]] = ..., AppSessionRequest: _Optional[_Union[AppSessionRequest, _Mapping]] = ..., DatabaseSessionStart: _Optional[_Union[DatabaseSessionStart, _Mapping]] = ..., DatabaseSessionEnd: _Optional[_Union[DatabaseSessionEnd, _Mapping]] = ..., DatabaseSessionQuery: _Optional[_Union[DatabaseSessionQuery, _Mapping]] = ..., SessionUpload: _Optional[_Union[SessionUpload, _Mapping]] = ..., MFADeviceAdd: _Optional[_Union[MFADeviceAdd, _Mapping]] = ..., MFADeviceDelete: _Optional[_Union[MFADeviceDelete, _Mapping]] = ..., BillingInformationUpdate: _Optional[_Union[BillingInformationUpdate, _Mapping]] = ..., BillingCardCreate: _Optional[_Union[BillingCardCreate, _Mapping]] = ..., BillingCardDelete: _Optional[_Union[BillingCardDelete, _Mapping]] = ..., LockCreate: _Optional[_Union[LockCreate, _Mapping]] = ..., LockDelete: _Optional[_Union[LockDelete, _Mapping]] = ..., RecoveryCodeGenerate: _Optional[_Union[RecoveryCodeGenerate, _Mapping]] = ..., RecoveryCodeUsed: _Optional[_Union[RecoveryCodeUsed, _Mapping]] = ..., DatabaseCreate: _Optional[_Union[DatabaseCreate, _Mapping]] = ..., DatabaseUpdate: _Optional[_Union[DatabaseUpdate, _Mapping]] = ..., DatabaseDelete: _Optional[_Union[DatabaseDelete, _Mapping]] = ..., AppCreate: _Optional[_Union[AppCreate, _Mapping]] = ..., AppUpdate: _Optional[_Union[AppUpdate, _Mapping]] = ..., AppDelete: _Optional[_Union[AppDelete, _Mapping]] = ..., WindowsDesktopSessionStart: _Optional[_Union[WindowsDesktopSessionStart, _Mapping]] = ..., WindowsDesktopSessionEnd: _Optional[_Union[WindowsDesktopSessionEnd, _Mapping]] = ..., PostgresParse: _Optional[_Union[PostgresParse, _Mapping]] = ..., PostgresBind: _Optional[_Union[PostgresBind, _Mapping]] = ..., PostgresExecute: _Optional[_Union[PostgresExecute, _Mapping]] = ..., PostgresClose: _Optional[_Union[PostgresClose, _Mapping]] = ..., PostgresFunctionCall: _Optional[_Union[PostgresFunctionCall, _Mapping]] = ..., AccessRequestDelete: _Optional[_Union[AccessRequestDelete, _Mapping]] = ..., SessionConnect: _Optional[_Union[SessionConnect, _Mapping]] = ..., CertificateCreate: _Optional[_Union[CertificateCreate, _Mapping]] = ..., DesktopRecording: _Optional[_Union[DesktopRecording, _Mapping]] = ..., DesktopClipboardSend: _Optional[_Union[DesktopClipboardSend, _Mapping]] = ..., DesktopClipboardReceive: _Optional[_Union[DesktopClipboardReceive, _Mapping]] = ..., MySQLStatementPrepare: _Optional[_Union[MySQLStatementPrepare, _Mapping]] = ..., MySQLStatementExecute: _Optional[_Union[MySQLStatementExecute, _Mapping]] = ..., MySQLStatementSendLongData: _Optional[_Union[MySQLStatementSendLongData, _Mapping]] = ..., MySQLStatementClose: _Optional[_Union[MySQLStatementClose, _Mapping]] = ..., MySQLStatementReset: _Optional[_Union[MySQLStatementReset, _Mapping]] = ..., MySQLStatementFetch: _Optional[_Union[MySQLStatementFetch, _Mapping]] = ..., MySQLStatementBulkExecute: _Optional[_Union[MySQLStatementBulkExecute, _Mapping]] = ..., RenewableCertificateGenerationMismatch: _Optional[_Union[RenewableCertificateGenerationMismatch, _Mapping]] = ..., Unknown: _Optional[_Union[Unknown, _Mapping]] = ..., MySQLInitDB: _Optional[_Union[MySQLInitDB, _Mapping]] = ..., MySQLCreateDB: _Optional[_Union[MySQLCreateDB, _Mapping]] = ..., MySQLDropDB: _Optional[_Union[MySQLDropDB, _Mapping]] = ..., MySQLShutDown: _Optional[_Union[MySQLShutDown, _Mapping]] = ..., MySQLProcessKill: _Optional[_Union[MySQLProcessKill, _Mapping]] = ..., MySQLDebug: _Optional[_Union[MySQLDebug, _Mapping]] = ..., MySQLRefresh: _Optional[_Union[MySQLRefresh, _Mapping]] = ..., AccessRequestResourceSearch: _Optional[_Union[AccessRequestResourceSearch, _Mapping]] = ..., SQLServerRPCRequest: _Optional[_Union[SQLServerRPCRequest, _Mapping]] = ..., DatabaseSessionMalformedPacket: _Optional[_Union[DatabaseSessionMalformedPacket, _Mapping]] = ..., SFTP: _Optional[_Union[SFTP, _Mapping]] = ..., UpgradeWindowStartUpdate: _Optional[_Union[UpgradeWindowStartUpdate, _Mapping]] = ..., AppSessionEnd: _Optional[_Union[AppSessionEnd, _Mapping]] = ..., SessionRecordingAccess: _Optional[_Union[SessionRecordingAccess, _Mapping]] = ..., KubernetesClusterCreate: _Optional[_Union[KubernetesClusterCreate, _Mapping]] = ..., KubernetesClusterUpdate: _Optional[_Union[KubernetesClusterUpdate, _Mapping]] = ..., KubernetesClusterDelete: _Optional[_Union[KubernetesClusterDelete, _Mapping]] = ..., SSMRun: _Optional[_Union[SSMRun, _Mapping]] = ..., ElasticsearchRequest: _Optional[_Union[ElasticsearchRequest, _Mapping]] = ..., CassandraBatch: _Optional[_Union[CassandraBatch, _Mapping]] = ..., CassandraPrepare: _Optional[_Union[CassandraPrepare, _Mapping]] = ..., CassandraRegister: _Optional[_Union[CassandraRegister, _Mapping]] = ..., CassandraExecute: _Optional[_Union[CassandraExecute, _Mapping]] = ..., AppSessionDynamoDBRequest: _Optional[_Union[AppSessionDynamoDBRequest, _Mapping]] = ..., DesktopSharedDirectoryStart: _Optional[_Union[DesktopSharedDirectoryStart, _Mapping]] = ..., DesktopSharedDirectoryRead: _Optional[_Union[DesktopSharedDirectoryRead, _Mapping]] = ..., DesktopSharedDirectoryWrite: _Optional[_Union[DesktopSharedDirectoryWrite, _Mapping]] = ..., DynamoDBRequest: _Optional[_Union[DynamoDBRequest, _Mapping]] = ..., BotJoin: _Optional[_Union[BotJoin, _Mapping]] = ..., InstanceJoin: _Optional[_Union[InstanceJoin, _Mapping]] = ..., DeviceEvent: _Optional[_Union[DeviceEvent, _Mapping]] = ..., LoginRuleCreate: _Optional[_Union[LoginRuleCreate, _Mapping]] = ..., LoginRuleDelete: _Optional[_Union[LoginRuleDelete, _Mapping]] = ..., SAMLIdPAuthAttempt: _Optional[_Union[SAMLIdPAuthAttempt, _Mapping]] = ..., SAMLIdPServiceProviderCreate: _Optional[_Union[SAMLIdPServiceProviderCreate, _Mapping]] = ..., SAMLIdPServiceProviderUpdate: _Optional[_Union[SAMLIdPServiceProviderUpdate, _Mapping]] = ..., SAMLIdPServiceProviderDelete: _Optional[_Union[SAMLIdPServiceProviderDelete, _Mapping]] = ..., SAMLIdPServiceProviderDeleteAll: _Optional[_Union[SAMLIdPServiceProviderDeleteAll, _Mapping]] = ..., OpenSearchRequest: _Optional[_Union[OpenSearchRequest, _Mapping]] = ..., DeviceEvent2: _Optional[_Union[DeviceEvent2, _Mapping]] = ..., OktaResourcesUpdate: _Optional[_Union[OktaResourcesUpdate, _Mapping]] = ..., OktaSyncFailure: _Optional[_Union[OktaSyncFailure, _Mapping]] = ..., OktaAssignmentResult: _Optional[_Union[OktaAssignmentResult, _Mapping]] = ..., ProvisionTokenCreate: _Optional[_Union[ProvisionTokenCreate, _Mapping]] = ..., AccessListCreate: _Optional[_Union[AccessListCreate, _Mapping]] = ..., AccessListUpdate: _Optional[_Union[AccessListUpdate, _Mapping]] = ..., AccessListDelete: _Optional[_Union[AccessListDelete, _Mapping]] = ..., AccessListReview: _Optional[_Union[AccessListReview, _Mapping]] = ..., AccessListMemberCreate: _Optional[_Union[AccessListMemberCreate, _Mapping]] = ..., AccessListMemberUpdate: _Optional[_Union[AccessListMemberUpdate, _Mapping]] = ..., AccessListMemberDelete: _Optional[_Union[AccessListMemberDelete, _Mapping]] = ..., AccessListMemberDeleteAllForAccessList: _Optional[_Union[AccessListMemberDeleteAllForAccessList, _Mapping]] = ...) -> None: ...

class StreamStatus(_message.Message):
    __slots__ = ["UploadID", "LastEventIndex", "LastUploadTime"]
    UPLOADID_FIELD_NUMBER: _ClassVar[int]
    LASTEVENTINDEX_FIELD_NUMBER: _ClassVar[int]
    LASTUPLOADTIME_FIELD_NUMBER: _ClassVar[int]
    UploadID: str
    LastEventIndex: int
    LastUploadTime: _timestamp_pb2.Timestamp
    def __init__(self, UploadID: _Optional[str] = ..., LastEventIndex: _Optional[int] = ..., LastUploadTime: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class SessionUpload(_message.Message):
    __slots__ = ["Metadata", "SessionMetadata", "SessionURL"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SESSIONMETADATA_FIELD_NUMBER: _ClassVar[int]
    SESSIONURL_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    SessionMetadata: SessionMetadata
    SessionURL: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., SessionMetadata: _Optional[_Union[SessionMetadata, _Mapping]] = ..., SessionURL: _Optional[str] = ...) -> None: ...

class Identity(_message.Message):
    __slots__ = ["User", "Impersonator", "Roles", "Usage", "Logins", "KubernetesGroups", "KubernetesUsers", "Expires", "RouteToCluster", "KubernetesCluster", "Traits", "RouteToApp", "TeleportCluster", "RouteToDatabase", "DatabaseNames", "DatabaseUsers", "MFADeviceUUID", "ClientIP", "AWSRoleARNs", "AccessRequests", "DisallowReissue", "AllowedResourceIDs", "PreviousIdentityExpires", "AzureIdentities", "GCPServiceAccounts"]
    USER_FIELD_NUMBER: _ClassVar[int]
    IMPERSONATOR_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    LOGINS_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESGROUPS_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESUSERS_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    ROUTETOCLUSTER_FIELD_NUMBER: _ClassVar[int]
    KUBERNETESCLUSTER_FIELD_NUMBER: _ClassVar[int]
    TRAITS_FIELD_NUMBER: _ClassVar[int]
    ROUTETOAPP_FIELD_NUMBER: _ClassVar[int]
    TELEPORTCLUSTER_FIELD_NUMBER: _ClassVar[int]
    ROUTETODATABASE_FIELD_NUMBER: _ClassVar[int]
    DATABASENAMES_FIELD_NUMBER: _ClassVar[int]
    DATABASEUSERS_FIELD_NUMBER: _ClassVar[int]
    MFADEVICEUUID_FIELD_NUMBER: _ClassVar[int]
    CLIENTIP_FIELD_NUMBER: _ClassVar[int]
    AWSROLEARNS_FIELD_NUMBER: _ClassVar[int]
    ACCESSREQUESTS_FIELD_NUMBER: _ClassVar[int]
    DISALLOWREISSUE_FIELD_NUMBER: _ClassVar[int]
    ALLOWEDRESOURCEIDS_FIELD_NUMBER: _ClassVar[int]
    PREVIOUSIDENTITYEXPIRES_FIELD_NUMBER: _ClassVar[int]
    AZUREIDENTITIES_FIELD_NUMBER: _ClassVar[int]
    GCPSERVICEACCOUNTS_FIELD_NUMBER: _ClassVar[int]
    User: str
    Impersonator: str
    Roles: _containers.RepeatedScalarFieldContainer[str]
    Usage: _containers.RepeatedScalarFieldContainer[str]
    Logins: _containers.RepeatedScalarFieldContainer[str]
    KubernetesGroups: _containers.RepeatedScalarFieldContainer[str]
    KubernetesUsers: _containers.RepeatedScalarFieldContainer[str]
    Expires: _timestamp_pb2.Timestamp
    RouteToCluster: str
    KubernetesCluster: str
    Traits: _wrappers_pb2_1.LabelValues
    RouteToApp: RouteToApp
    TeleportCluster: str
    RouteToDatabase: RouteToDatabase
    DatabaseNames: _containers.RepeatedScalarFieldContainer[str]
    DatabaseUsers: _containers.RepeatedScalarFieldContainer[str]
    MFADeviceUUID: str
    ClientIP: str
    AWSRoleARNs: _containers.RepeatedScalarFieldContainer[str]
    AccessRequests: _containers.RepeatedScalarFieldContainer[str]
    DisallowReissue: bool
    AllowedResourceIDs: _containers.RepeatedCompositeFieldContainer[ResourceID]
    PreviousIdentityExpires: _timestamp_pb2.Timestamp
    AzureIdentities: _containers.RepeatedScalarFieldContainer[str]
    GCPServiceAccounts: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, User: _Optional[str] = ..., Impersonator: _Optional[str] = ..., Roles: _Optional[_Iterable[str]] = ..., Usage: _Optional[_Iterable[str]] = ..., Logins: _Optional[_Iterable[str]] = ..., KubernetesGroups: _Optional[_Iterable[str]] = ..., KubernetesUsers: _Optional[_Iterable[str]] = ..., Expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., RouteToCluster: _Optional[str] = ..., KubernetesCluster: _Optional[str] = ..., Traits: _Optional[_Union[_wrappers_pb2_1.LabelValues, _Mapping]] = ..., RouteToApp: _Optional[_Union[RouteToApp, _Mapping]] = ..., TeleportCluster: _Optional[str] = ..., RouteToDatabase: _Optional[_Union[RouteToDatabase, _Mapping]] = ..., DatabaseNames: _Optional[_Iterable[str]] = ..., DatabaseUsers: _Optional[_Iterable[str]] = ..., MFADeviceUUID: _Optional[str] = ..., ClientIP: _Optional[str] = ..., AWSRoleARNs: _Optional[_Iterable[str]] = ..., AccessRequests: _Optional[_Iterable[str]] = ..., DisallowReissue: bool = ..., AllowedResourceIDs: _Optional[_Iterable[_Union[ResourceID, _Mapping]]] = ..., PreviousIdentityExpires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., AzureIdentities: _Optional[_Iterable[str]] = ..., GCPServiceAccounts: _Optional[_Iterable[str]] = ...) -> None: ...

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

class AccessRequestResourceSearch(_message.Message):
    __slots__ = ["Metadata", "User", "SearchAsRoles", "ResourceType", "Namespace", "Labels", "PredicateExpression", "SearchKeywords"]
    class LabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SEARCHASROLES_FIELD_NUMBER: _ClassVar[int]
    RESOURCETYPE_FIELD_NUMBER: _ClassVar[int]
    NAMESPACE_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    PREDICATEEXPRESSION_FIELD_NUMBER: _ClassVar[int]
    SEARCHKEYWORDS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    SearchAsRoles: _containers.RepeatedScalarFieldContainer[str]
    ResourceType: str
    Namespace: str
    Labels: _containers.ScalarMap[str, str]
    PredicateExpression: str
    SearchKeywords: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., SearchAsRoles: _Optional[_Iterable[str]] = ..., ResourceType: _Optional[str] = ..., Namespace: _Optional[str] = ..., Labels: _Optional[_Mapping[str, str]] = ..., PredicateExpression: _Optional[str] = ..., SearchKeywords: _Optional[_Iterable[str]] = ...) -> None: ...

class MySQLStatementPrepare(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "Query"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    Query: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., Query: _Optional[str] = ...) -> None: ...

class MySQLStatementExecute(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "StatementID", "Parameters"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    STATEMENTID_FIELD_NUMBER: _ClassVar[int]
    PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    StatementID: int
    Parameters: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., StatementID: _Optional[int] = ..., Parameters: _Optional[_Iterable[str]] = ...) -> None: ...

class MySQLStatementSendLongData(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "StatementID", "ParameterID", "DataSize"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    STATEMENTID_FIELD_NUMBER: _ClassVar[int]
    PARAMETERID_FIELD_NUMBER: _ClassVar[int]
    DATASIZE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    StatementID: int
    ParameterID: int
    DataSize: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., StatementID: _Optional[int] = ..., ParameterID: _Optional[int] = ..., DataSize: _Optional[int] = ...) -> None: ...

class MySQLStatementClose(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "StatementID"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    STATEMENTID_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    StatementID: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., StatementID: _Optional[int] = ...) -> None: ...

class MySQLStatementReset(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "StatementID"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    STATEMENTID_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    StatementID: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., StatementID: _Optional[int] = ...) -> None: ...

class MySQLStatementFetch(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "StatementID", "RowsCount"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    STATEMENTID_FIELD_NUMBER: _ClassVar[int]
    ROWSCOUNT_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    StatementID: int
    RowsCount: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., StatementID: _Optional[int] = ..., RowsCount: _Optional[int] = ...) -> None: ...

class MySQLStatementBulkExecute(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "StatementID", "Parameters"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    STATEMENTID_FIELD_NUMBER: _ClassVar[int]
    PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    StatementID: int
    Parameters: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., StatementID: _Optional[int] = ..., Parameters: _Optional[_Iterable[str]] = ...) -> None: ...

class MySQLInitDB(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "SchemaName"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    SCHEMANAME_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    SchemaName: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., SchemaName: _Optional[str] = ...) -> None: ...

class MySQLCreateDB(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "SchemaName"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    SCHEMANAME_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    SchemaName: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., SchemaName: _Optional[str] = ...) -> None: ...

class MySQLDropDB(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "SchemaName"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    SCHEMANAME_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    SchemaName: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., SchemaName: _Optional[str] = ...) -> None: ...

class MySQLShutDown(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ...) -> None: ...

class MySQLProcessKill(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "ProcessID"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    PROCESSID_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    ProcessID: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., ProcessID: _Optional[int] = ...) -> None: ...

class MySQLDebug(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ...) -> None: ...

class MySQLRefresh(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "Subcommand"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    SUBCOMMAND_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    Subcommand: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., Subcommand: _Optional[str] = ...) -> None: ...

class SQLServerRPCRequest(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "Procname", "Parameters"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    PROCNAME_FIELD_NUMBER: _ClassVar[int]
    PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    Procname: str
    Parameters: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., Procname: _Optional[str] = ..., Parameters: _Optional[_Iterable[str]] = ...) -> None: ...

class DatabaseSessionMalformedPacket(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "Payload"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    PAYLOAD_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    Payload: bytes
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., Payload: _Optional[bytes] = ...) -> None: ...

class ElasticsearchRequest(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "Path", "RawQuery", "Method", "Body", "Headers", "Category", "Target", "Query", "StatusCode"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    RAWQUERY_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    HEADERS_FIELD_NUMBER: _ClassVar[int]
    CATEGORY_FIELD_NUMBER: _ClassVar[int]
    TARGET_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    STATUSCODE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    Path: str
    RawQuery: str
    Method: str
    Body: bytes
    Headers: _wrappers_pb2_1.LabelValues
    Category: ElasticsearchCategory
    Target: str
    Query: str
    StatusCode: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., Path: _Optional[str] = ..., RawQuery: _Optional[str] = ..., Method: _Optional[str] = ..., Body: _Optional[bytes] = ..., Headers: _Optional[_Union[_wrappers_pb2_1.LabelValues, _Mapping]] = ..., Category: _Optional[_Union[ElasticsearchCategory, str]] = ..., Target: _Optional[str] = ..., Query: _Optional[str] = ..., StatusCode: _Optional[int] = ...) -> None: ...

class OpenSearchRequest(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "Path", "RawQuery", "Method", "Body", "Headers", "Category", "Target", "Query", "StatusCode"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    RAWQUERY_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    HEADERS_FIELD_NUMBER: _ClassVar[int]
    CATEGORY_FIELD_NUMBER: _ClassVar[int]
    TARGET_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    STATUSCODE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    Path: str
    RawQuery: str
    Method: str
    Body: bytes
    Headers: _wrappers_pb2_1.LabelValues
    Category: OpenSearchCategory
    Target: str
    Query: str
    StatusCode: int
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., Path: _Optional[str] = ..., RawQuery: _Optional[str] = ..., Method: _Optional[str] = ..., Body: _Optional[bytes] = ..., Headers: _Optional[_Union[_wrappers_pb2_1.LabelValues, _Mapping]] = ..., Category: _Optional[_Union[OpenSearchCategory, str]] = ..., Target: _Optional[str] = ..., Query: _Optional[str] = ..., StatusCode: _Optional[int] = ...) -> None: ...

class DynamoDBRequest(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "StatusCode", "Path", "RawQuery", "Method", "Target", "Body"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    STATUSCODE_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    RAWQUERY_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    TARGET_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    StatusCode: int
    Path: str
    RawQuery: str
    Method: str
    Target: str
    Body: _struct_pb2.Struct
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., StatusCode: _Optional[int] = ..., Path: _Optional[str] = ..., RawQuery: _Optional[str] = ..., Method: _Optional[str] = ..., Target: _Optional[str] = ..., Body: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class AppSessionDynamoDBRequest(_message.Message):
    __slots__ = ["Metadata", "User", "App", "AWS", "SessionChunkID", "StatusCode", "Path", "RawQuery", "Method", "Target", "Body"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    APP_FIELD_NUMBER: _ClassVar[int]
    AWS_FIELD_NUMBER: _ClassVar[int]
    SESSIONCHUNKID_FIELD_NUMBER: _ClassVar[int]
    STATUSCODE_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    RAWQUERY_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    TARGET_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    App: AppMetadata
    AWS: AWSRequestMetadata
    SessionChunkID: str
    StatusCode: int
    Path: str
    RawQuery: str
    Method: str
    Target: str
    Body: _struct_pb2.Struct
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., App: _Optional[_Union[AppMetadata, _Mapping]] = ..., AWS: _Optional[_Union[AWSRequestMetadata, _Mapping]] = ..., SessionChunkID: _Optional[str] = ..., StatusCode: _Optional[int] = ..., Path: _Optional[str] = ..., RawQuery: _Optional[str] = ..., Method: _Optional[str] = ..., Target: _Optional[str] = ..., Body: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class UpgradeWindowStartMetadata(_message.Message):
    __slots__ = ["UpgradeWindowStart"]
    UPGRADEWINDOWSTART_FIELD_NUMBER: _ClassVar[int]
    UpgradeWindowStart: str
    def __init__(self, UpgradeWindowStart: _Optional[str] = ...) -> None: ...

class UpgradeWindowStartUpdate(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "UpgradeWindowStart"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    UPGRADEWINDOWSTART_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    UpgradeWindowStart: UpgradeWindowStartMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., UpgradeWindowStart: _Optional[_Union[UpgradeWindowStartMetadata, _Mapping]] = ...) -> None: ...

class SessionRecordingAccess(_message.Message):
    __slots__ = ["Metadata", "SessionID", "UserMetadata"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SESSIONID_FIELD_NUMBER: _ClassVar[int]
    USERMETADATA_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    SessionID: str
    UserMetadata: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., SessionID: _Optional[str] = ..., UserMetadata: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class KubeClusterMetadata(_message.Message):
    __slots__ = ["KubeLabels"]
    class KubeLabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    KUBELABELS_FIELD_NUMBER: _ClassVar[int]
    KubeLabels: _containers.ScalarMap[str, str]
    def __init__(self, KubeLabels: _Optional[_Mapping[str, str]] = ...) -> None: ...

class KubernetesClusterCreate(_message.Message):
    __slots__ = ["Metadata", "User", "Resource", "KubeClusterMetadata"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    KUBECLUSTERMETADATA_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Resource: ResourceMetadata
    KubeClusterMetadata: KubeClusterMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., KubeClusterMetadata: _Optional[_Union[KubeClusterMetadata, _Mapping]] = ...) -> None: ...

class KubernetesClusterUpdate(_message.Message):
    __slots__ = ["Metadata", "User", "Resource", "KubeClusterMetadata"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    KUBECLUSTERMETADATA_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Resource: ResourceMetadata
    KubeClusterMetadata: KubeClusterMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., KubeClusterMetadata: _Optional[_Union[KubeClusterMetadata, _Mapping]] = ...) -> None: ...

class KubernetesClusterDelete(_message.Message):
    __slots__ = ["Metadata", "User", "Resource"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Resource: ResourceMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ...) -> None: ...

class SSMRun(_message.Message):
    __slots__ = ["Metadata", "CommandID", "InstanceID", "ExitCode", "Status", "AccountID", "Region"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    COMMANDID_FIELD_NUMBER: _ClassVar[int]
    INSTANCEID_FIELD_NUMBER: _ClassVar[int]
    EXITCODE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ACCOUNTID_FIELD_NUMBER: _ClassVar[int]
    REGION_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    CommandID: str
    InstanceID: str
    ExitCode: int
    Status: str
    AccountID: str
    Region: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., CommandID: _Optional[str] = ..., InstanceID: _Optional[str] = ..., ExitCode: _Optional[int] = ..., Status: _Optional[str] = ..., AccountID: _Optional[str] = ..., Region: _Optional[str] = ...) -> None: ...

class CassandraPrepare(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "Query", "Keyspace"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    KEYSPACE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    Query: str
    Keyspace: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., Query: _Optional[str] = ..., Keyspace: _Optional[str] = ...) -> None: ...

class CassandraExecute(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "QueryId"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    QUERYID_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    QueryId: str
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., QueryId: _Optional[str] = ...) -> None: ...

class CassandraBatch(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "Consistency", "Keyspace", "BatchType", "Children"]
    class BatchChild(_message.Message):
        __slots__ = ["ID", "Query", "Values"]
        class Value(_message.Message):
            __slots__ = ["Type", "Contents"]
            TYPE_FIELD_NUMBER: _ClassVar[int]
            CONTENTS_FIELD_NUMBER: _ClassVar[int]
            Type: int
            Contents: bytes
            def __init__(self, Type: _Optional[int] = ..., Contents: _Optional[bytes] = ...) -> None: ...
        ID_FIELD_NUMBER: _ClassVar[int]
        QUERY_FIELD_NUMBER: _ClassVar[int]
        VALUES_FIELD_NUMBER: _ClassVar[int]
        ID: str
        Query: str
        Values: _containers.RepeatedCompositeFieldContainer[CassandraBatch.BatchChild.Value]
        def __init__(self, ID: _Optional[str] = ..., Query: _Optional[str] = ..., Values: _Optional[_Iterable[_Union[CassandraBatch.BatchChild.Value, _Mapping]]] = ...) -> None: ...
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    CONSISTENCY_FIELD_NUMBER: _ClassVar[int]
    KEYSPACE_FIELD_NUMBER: _ClassVar[int]
    BATCHTYPE_FIELD_NUMBER: _ClassVar[int]
    CHILDREN_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    Consistency: str
    Keyspace: str
    BatchType: str
    Children: _containers.RepeatedCompositeFieldContainer[CassandraBatch.BatchChild]
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., Consistency: _Optional[str] = ..., Keyspace: _Optional[str] = ..., BatchType: _Optional[str] = ..., Children: _Optional[_Iterable[_Union[CassandraBatch.BatchChild, _Mapping]]] = ...) -> None: ...

class CassandraRegister(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Database", "EventTypes"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    EVENTTYPES_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Database: DatabaseMetadata
    EventTypes: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Database: _Optional[_Union[DatabaseMetadata, _Mapping]] = ..., EventTypes: _Optional[_Iterable[str]] = ...) -> None: ...

class LoginRuleCreate(_message.Message):
    __slots__ = ["Metadata", "Resource", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class LoginRuleDelete(_message.Message):
    __slots__ = ["Metadata", "Resource", "User"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    User: UserMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ...) -> None: ...

class SAMLIdPAuthAttempt(_message.Message):
    __slots__ = ["Metadata", "User", "Session", "Status", "ServiceProvider"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    SESSION_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    SERVICEPROVIDER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    User: UserMetadata
    Session: SessionMetadata
    Status: Status
    ServiceProvider: SAMLIdPServiceProviderMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., User: _Optional[_Union[UserMetadata, _Mapping]] = ..., Session: _Optional[_Union[SessionMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ..., ServiceProvider: _Optional[_Union[SAMLIdPServiceProviderMetadata, _Mapping]] = ...) -> None: ...

class SAMLIdPServiceProviderCreate(_message.Message):
    __slots__ = ["Metadata", "Resource", "ServiceProvider"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    SERVICEPROVIDER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    ServiceProvider: SAMLIdPServiceProviderMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., ServiceProvider: _Optional[_Union[SAMLIdPServiceProviderMetadata, _Mapping]] = ...) -> None: ...

class SAMLIdPServiceProviderUpdate(_message.Message):
    __slots__ = ["Metadata", "Resource", "ServiceProvider"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    SERVICEPROVIDER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    ServiceProvider: SAMLIdPServiceProviderMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., ServiceProvider: _Optional[_Union[SAMLIdPServiceProviderMetadata, _Mapping]] = ...) -> None: ...

class SAMLIdPServiceProviderDelete(_message.Message):
    __slots__ = ["Metadata", "Resource", "ServiceProvider"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    SERVICEPROVIDER_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    ServiceProvider: SAMLIdPServiceProviderMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., ServiceProvider: _Optional[_Union[SAMLIdPServiceProviderMetadata, _Mapping]] = ...) -> None: ...

class SAMLIdPServiceProviderDeleteAll(_message.Message):
    __slots__ = ["Metadata", "Resource"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ...) -> None: ...

class OktaResourcesUpdate(_message.Message):
    __slots__ = ["Metadata", "Server", "Updated"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    UPDATED_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Server: ServerMetadata
    Updated: OktaResourcesUpdatedMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., Updated: _Optional[_Union[OktaResourcesUpdatedMetadata, _Mapping]] = ...) -> None: ...

class OktaSyncFailure(_message.Message):
    __slots__ = ["Metadata", "Server", "Status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Server: ServerMetadata
    Status: Status
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ...) -> None: ...

class OktaAssignmentResult(_message.Message):
    __slots__ = ["Metadata", "Server", "Resource", "Status", "OktaAssignment"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    OKTAASSIGNMENT_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Server: ServerMetadata
    Resource: ResourceMetadata
    Status: Status
    OktaAssignment: OktaAssignmentMetadata
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Server: _Optional[_Union[ServerMetadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ..., OktaAssignment: _Optional[_Union[OktaAssignmentMetadata, _Mapping]] = ...) -> None: ...

class AccessListCreate(_message.Message):
    __slots__ = ["Metadata", "Resource", "Status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    Status: Status
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ...) -> None: ...

class AccessListUpdate(_message.Message):
    __slots__ = ["Metadata", "Resource", "Status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    Status: Status
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ...) -> None: ...

class AccessListDelete(_message.Message):
    __slots__ = ["Metadata", "Resource", "Status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    Status: Status
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ...) -> None: ...

class AccessListMemberCreate(_message.Message):
    __slots__ = ["Metadata", "Resource", "AccessListMember", "Status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    ACCESSLISTMEMBER_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    AccessListMember: AccessListMemberMetadata
    Status: Status
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., AccessListMember: _Optional[_Union[AccessListMemberMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ...) -> None: ...

class AccessListMemberUpdate(_message.Message):
    __slots__ = ["Metadata", "Resource", "AccessListMember", "Status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    ACCESSLISTMEMBER_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    AccessListMember: AccessListMemberMetadata
    Status: Status
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., AccessListMember: _Optional[_Union[AccessListMemberMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ...) -> None: ...

class AccessListMemberDelete(_message.Message):
    __slots__ = ["Metadata", "Resource", "AccessListMember", "Status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    ACCESSLISTMEMBER_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    AccessListMember: AccessListMemberMetadata
    Status: Status
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., AccessListMember: _Optional[_Union[AccessListMemberMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ...) -> None: ...

class AccessListMemberDeleteAllForAccessList(_message.Message):
    __slots__ = ["Metadata", "Resource", "AccessListMember", "Status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    ACCESSLISTMEMBER_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    AccessListMember: AccessListMemberMetadata
    Status: Status
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., AccessListMember: _Optional[_Union[AccessListMemberMetadata, _Mapping]] = ..., Status: _Optional[_Union[Status, _Mapping]] = ...) -> None: ...

class AccessListReview(_message.Message):
    __slots__ = ["Metadata", "Resource", "Review", "status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    REVIEW_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    Metadata: Metadata
    Resource: ResourceMetadata
    Review: AccessListReviewMetadata
    status: Status
    def __init__(self, Metadata: _Optional[_Union[Metadata, _Mapping]] = ..., Resource: _Optional[_Union[ResourceMetadata, _Mapping]] = ..., Review: _Optional[_Union[AccessListReviewMetadata, _Mapping]] = ..., status: _Optional[_Union[Status, _Mapping]] = ...) -> None: ...
