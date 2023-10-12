from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ProxySSHRequest(_message.Message):
    __slots__ = ["dial_target", "ssh", "agent"]
    DIAL_TARGET_FIELD_NUMBER: _ClassVar[int]
    SSH_FIELD_NUMBER: _ClassVar[int]
    AGENT_FIELD_NUMBER: _ClassVar[int]
    dial_target: TargetHost
    ssh: Frame
    agent: Frame
    def __init__(self, dial_target: _Optional[_Union[TargetHost, _Mapping]] = ..., ssh: _Optional[_Union[Frame, _Mapping]] = ..., agent: _Optional[_Union[Frame, _Mapping]] = ...) -> None: ...

class ProxySSHResponse(_message.Message):
    __slots__ = ["details", "ssh", "agent"]
    DETAILS_FIELD_NUMBER: _ClassVar[int]
    SSH_FIELD_NUMBER: _ClassVar[int]
    AGENT_FIELD_NUMBER: _ClassVar[int]
    details: ClusterDetails
    ssh: Frame
    agent: Frame
    def __init__(self, details: _Optional[_Union[ClusterDetails, _Mapping]] = ..., ssh: _Optional[_Union[Frame, _Mapping]] = ..., agent: _Optional[_Union[Frame, _Mapping]] = ...) -> None: ...

class ProxyClusterRequest(_message.Message):
    __slots__ = ["cluster", "frame"]
    CLUSTER_FIELD_NUMBER: _ClassVar[int]
    FRAME_FIELD_NUMBER: _ClassVar[int]
    cluster: str
    frame: Frame
    def __init__(self, cluster: _Optional[str] = ..., frame: _Optional[_Union[Frame, _Mapping]] = ...) -> None: ...

class ProxyClusterResponse(_message.Message):
    __slots__ = ["frame"]
    FRAME_FIELD_NUMBER: _ClassVar[int]
    frame: Frame
    def __init__(self, frame: _Optional[_Union[Frame, _Mapping]] = ...) -> None: ...

class Frame(_message.Message):
    __slots__ = ["payload"]
    PAYLOAD_FIELD_NUMBER: _ClassVar[int]
    payload: bytes
    def __init__(self, payload: _Optional[bytes] = ...) -> None: ...

class TargetHost(_message.Message):
    __slots__ = ["host_port", "cluster"]
    HOST_PORT_FIELD_NUMBER: _ClassVar[int]
    CLUSTER_FIELD_NUMBER: _ClassVar[int]
    host_port: str
    cluster: str
    def __init__(self, host_port: _Optional[str] = ..., cluster: _Optional[str] = ...) -> None: ...

class GetClusterDetailsRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class GetClusterDetailsResponse(_message.Message):
    __slots__ = ["details"]
    DETAILS_FIELD_NUMBER: _ClassVar[int]
    details: ClusterDetails
    def __init__(self, details: _Optional[_Union[ClusterDetails, _Mapping]] = ...) -> None: ...

class ClusterDetails(_message.Message):
    __slots__ = ["fips_enabled"]
    FIPS_ENABLED_FIELD_NUMBER: _ClassVar[int]
    fips_enabled: bool
    def __init__(self, fips_enabled: bool = ...) -> None: ...
