from teleport.legacy.client.proto import event_pb2 as _event_pb2
from teleport.legacy.types import types_pb2 as _types_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Node(_message.Message):
    __slots__ = ["id", "kind", "sub_kind", "name", "labels", "hostname", "properties"]
    class LabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    class PropertiesEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUB_KIND_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    HOSTNAME_FIELD_NUMBER: _ClassVar[int]
    PROPERTIES_FIELD_NUMBER: _ClassVar[int]
    id: str
    kind: str
    sub_kind: str
    name: str
    labels: _containers.ScalarMap[str, str]
    hostname: str
    properties: _containers.ScalarMap[str, str]
    def __init__(self, id: _Optional[str] = ..., kind: _Optional[str] = ..., sub_kind: _Optional[str] = ..., name: _Optional[str] = ..., labels: _Optional[_Mapping[str, str]] = ..., hostname: _Optional[str] = ..., properties: _Optional[_Mapping[str, str]] = ...) -> None: ...

class Edge(_message.Message):
    __slots__ = ["to", "type"]
    FROM_FIELD_NUMBER: _ClassVar[int]
    TO_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    to: str
    type: str
    def __init__(self, to: _Optional[str] = ..., type: _Optional[str] = ..., **kwargs) -> None: ...

class QueryRequest(_message.Message):
    __slots__ = ["query"]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    query: str
    def __init__(self, query: _Optional[str] = ...) -> None: ...

class QueryResponse(_message.Message):
    __slots__ = ["nodes", "edges"]
    NODES_FIELD_NUMBER: _ClassVar[int]
    EDGES_FIELD_NUMBER: _ClassVar[int]
    nodes: _containers.RepeatedCompositeFieldContainer[Node]
    edges: _containers.RepeatedCompositeFieldContainer[Edge]
    def __init__(self, nodes: _Optional[_Iterable[_Union[Node, _Mapping]]] = ..., edges: _Optional[_Iterable[_Union[Edge, _Mapping]]] = ...) -> None: ...

class GetFileRequest(_message.Message):
    __slots__ = ["filepath"]
    FILEPATH_FIELD_NUMBER: _ClassVar[int]
    filepath: str
    def __init__(self, filepath: _Optional[str] = ...) -> None: ...

class GetFileResponse(_message.Message):
    __slots__ = ["data"]
    DATA_FIELD_NUMBER: _ClassVar[int]
    data: bytes
    def __init__(self, data: _Optional[bytes] = ...) -> None: ...

class SendEventRequest(_message.Message):
    __slots__ = ["event"]
    EVENT_FIELD_NUMBER: _ClassVar[int]
    event: _event_pb2.Event
    def __init__(self, event: _Optional[_Union[_event_pb2.Event, _Mapping]] = ...) -> None: ...

class SendEventResponse(_message.Message):
    __slots__ = ["cache_initialized"]
    CACHE_INITIALIZED_FIELD_NUMBER: _ClassVar[int]
    cache_initialized: bool
    def __init__(self, cache_initialized: bool = ...) -> None: ...

class SendResourceRequest(_message.Message):
    __slots__ = ["end", "resource_header", "user", "role", "server", "access_request"]
    END_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_HEADER_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    SERVER_FIELD_NUMBER: _ClassVar[int]
    ACCESS_REQUEST_FIELD_NUMBER: _ClassVar[int]
    end: SendResourceEnd
    resource_header: _types_pb2.ResourceHeader
    user: _types_pb2.UserV2
    role: _types_pb2.RoleV6
    server: _types_pb2.ServerV2
    access_request: _types_pb2.AccessRequestV3
    def __init__(self, end: _Optional[_Union[SendResourceEnd, _Mapping]] = ..., resource_header: _Optional[_Union[_types_pb2.ResourceHeader, _Mapping]] = ..., user: _Optional[_Union[_types_pb2.UserV2, _Mapping]] = ..., role: _Optional[_Union[_types_pb2.RoleV6, _Mapping]] = ..., server: _Optional[_Union[_types_pb2.ServerV2, _Mapping]] = ..., access_request: _Optional[_Union[_types_pb2.AccessRequestV3, _Mapping]] = ...) -> None: ...

class SendResourceEnd(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class SendResourceResponse(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...
