from google.protobuf import struct_pb2 as _struct_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Order(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    ORDER_DESCENDING_UNSPECIFIED: _ClassVar[Order]
    ORDER_ASCENDING: _ClassVar[Order]
ORDER_DESCENDING_UNSPECIFIED: Order
ORDER_ASCENDING: Order

class StreamUnstructuredSessionEventsRequest(_message.Message):
    __slots__ = ["session_id", "start_index"]
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    START_INDEX_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    start_index: int
    def __init__(self, session_id: _Optional[str] = ..., start_index: _Optional[int] = ...) -> None: ...

class GetUnstructuredEventsRequest(_message.Message):
    __slots__ = ["namespace", "start_date", "end_date", "event_types", "limit", "start_key", "order"]
    NAMESPACE_FIELD_NUMBER: _ClassVar[int]
    START_DATE_FIELD_NUMBER: _ClassVar[int]
    END_DATE_FIELD_NUMBER: _ClassVar[int]
    EVENT_TYPES_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    START_KEY_FIELD_NUMBER: _ClassVar[int]
    ORDER_FIELD_NUMBER: _ClassVar[int]
    namespace: str
    start_date: _timestamp_pb2.Timestamp
    end_date: _timestamp_pb2.Timestamp
    event_types: _containers.RepeatedScalarFieldContainer[str]
    limit: int
    start_key: str
    order: Order
    def __init__(self, namespace: _Optional[str] = ..., start_date: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., end_date: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., event_types: _Optional[_Iterable[str]] = ..., limit: _Optional[int] = ..., start_key: _Optional[str] = ..., order: _Optional[_Union[Order, str]] = ...) -> None: ...

class EventsUnstructured(_message.Message):
    __slots__ = ["items", "last_key"]
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    LAST_KEY_FIELD_NUMBER: _ClassVar[int]
    items: _containers.RepeatedCompositeFieldContainer[EventUnstructured]
    last_key: str
    def __init__(self, items: _Optional[_Iterable[_Union[EventUnstructured, _Mapping]]] = ..., last_key: _Optional[str] = ...) -> None: ...

class EventUnstructured(_message.Message):
    __slots__ = ["type", "id", "time", "index", "unstructured"]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    TIME_FIELD_NUMBER: _ClassVar[int]
    INDEX_FIELD_NUMBER: _ClassVar[int]
    UNSTRUCTURED_FIELD_NUMBER: _ClassVar[int]
    type: str
    id: str
    time: _timestamp_pb2.Timestamp
    index: int
    unstructured: _struct_pb2.Struct
    def __init__(self, type: _Optional[str] = ..., id: _Optional[str] = ..., time: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., index: _Optional[int] = ..., unstructured: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...
