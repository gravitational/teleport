from gogoproto import gogo_pb2 as _gogo_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class StringValues(_message.Message):
    __slots__ = ["Values"]
    VALUES_FIELD_NUMBER: _ClassVar[int]
    Values: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, Values: _Optional[_Iterable[str]] = ...) -> None: ...

class LabelValues(_message.Message):
    __slots__ = ["Values"]
    class ValuesEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: StringValues
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[StringValues, _Mapping]] = ...) -> None: ...
    VALUES_FIELD_NUMBER: _ClassVar[int]
    Values: _containers.MessageMap[str, StringValues]
    def __init__(self, Values: _Optional[_Mapping[str, StringValues]] = ...) -> None: ...

class CustomType(_message.Message):
    __slots__ = ["Bytes"]
    BYTES_FIELD_NUMBER: _ClassVar[int]
    Bytes: bytes
    def __init__(self, Bytes: _Optional[bytes] = ...) -> None: ...
