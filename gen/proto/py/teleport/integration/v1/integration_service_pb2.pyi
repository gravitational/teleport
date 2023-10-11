from google.protobuf import empty_pb2 as _empty_pb2
from teleport.legacy.types import types_pb2 as _types_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ListIntegrationsRequest(_message.Message):
    __slots__ = ["limit", "next_key"]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    NEXT_KEY_FIELD_NUMBER: _ClassVar[int]
    limit: int
    next_key: str
    def __init__(self, limit: _Optional[int] = ..., next_key: _Optional[str] = ...) -> None: ...

class ListIntegrationsResponse(_message.Message):
    __slots__ = ["integrations", "next_key", "total_count"]
    INTEGRATIONS_FIELD_NUMBER: _ClassVar[int]
    NEXT_KEY_FIELD_NUMBER: _ClassVar[int]
    TOTAL_COUNT_FIELD_NUMBER: _ClassVar[int]
    integrations: _containers.RepeatedCompositeFieldContainer[_types_pb2.IntegrationV1]
    next_key: str
    total_count: int
    def __init__(self, integrations: _Optional[_Iterable[_Union[_types_pb2.IntegrationV1, _Mapping]]] = ..., next_key: _Optional[str] = ..., total_count: _Optional[int] = ...) -> None: ...

class GetIntegrationRequest(_message.Message):
    __slots__ = ["name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class CreateIntegrationRequest(_message.Message):
    __slots__ = ["integration"]
    INTEGRATION_FIELD_NUMBER: _ClassVar[int]
    integration: _types_pb2.IntegrationV1
    def __init__(self, integration: _Optional[_Union[_types_pb2.IntegrationV1, _Mapping]] = ...) -> None: ...

class UpdateIntegrationRequest(_message.Message):
    __slots__ = ["integration"]
    INTEGRATION_FIELD_NUMBER: _ClassVar[int]
    integration: _types_pb2.IntegrationV1
    def __init__(self, integration: _Optional[_Union[_types_pb2.IntegrationV1, _Mapping]] = ...) -> None: ...

class DeleteIntegrationRequest(_message.Message):
    __slots__ = ["name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class DeleteAllIntegrationsRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class GenerateAWSOIDCTokenRequest(_message.Message):
    __slots__ = ["issuer"]
    ISSUER_FIELD_NUMBER: _ClassVar[int]
    issuer: str
    def __init__(self, issuer: _Optional[str] = ...) -> None: ...

class GenerateAWSOIDCTokenResponse(_message.Message):
    __slots__ = ["token"]
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    token: str
    def __init__(self, token: _Optional[str] = ...) -> None: ...
