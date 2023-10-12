from teleport.header.v1 import resourceheader_pb2 as _resourceheader_pb2
from teleport.trait.v1 import trait_pb2 as _trait_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class UserLoginState(_message.Message):
    __slots__ = ["header", "spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    header: _resourceheader_pb2.ResourceHeader
    spec: Spec
    def __init__(self, header: _Optional[_Union[_resourceheader_pb2.ResourceHeader, _Mapping]] = ..., spec: _Optional[_Union[Spec, _Mapping]] = ...) -> None: ...

class Spec(_message.Message):
    __slots__ = ["roles", "traits", "user_type"]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    TRAITS_FIELD_NUMBER: _ClassVar[int]
    USER_TYPE_FIELD_NUMBER: _ClassVar[int]
    roles: _containers.RepeatedScalarFieldContainer[str]
    traits: _containers.RepeatedCompositeFieldContainer[_trait_pb2.Trait]
    user_type: str
    def __init__(self, roles: _Optional[_Iterable[str]] = ..., traits: _Optional[_Iterable[_Union[_trait_pb2.Trait, _Mapping]]] = ..., user_type: _Optional[str] = ...) -> None: ...
