from teleport.legacy.client.proto import event_pb2 as _event_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Node(_message.Message):
    __slots__ = ["id", "kind", "sub_kind", "name", "labels", "hostname"]
    class LabelsEntry(_message.Message):
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
    id: str
    kind: str
    sub_kind: str
    name: str
    labels: _containers.ScalarMap[str, str]
    hostname: str
    def __init__(self, id: _Optional[str] = ..., kind: _Optional[str] = ..., sub_kind: _Optional[str] = ..., name: _Optional[str] = ..., labels: _Optional[_Mapping[str, str]] = ..., hostname: _Optional[str] = ...) -> None: ...

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

class SendEventRequest(_message.Message):
    __slots__ = ["event"]
    EVENT_FIELD_NUMBER: _ClassVar[int]
    event: _event_pb2.Event
    def __init__(self, event: _Optional[_Union[_event_pb2.Event, _Mapping]] = ...) -> None: ...

class SendEventResponse(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...
