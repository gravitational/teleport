from google.protobuf import empty_pb2 as _empty_pb2
from teleport.legacy.types import types_pb2 as _types_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class PluginType(_message.Message):
    __slots__ = ["type", "oauth_client_id"]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    OAUTH_CLIENT_ID_FIELD_NUMBER: _ClassVar[int]
    type: str
    oauth_client_id: str
    def __init__(self, type: _Optional[str] = ..., oauth_client_id: _Optional[str] = ...) -> None: ...

class CreatePluginRequest(_message.Message):
    __slots__ = ["plugin", "bootstrap_credentials", "static_credentials"]
    PLUGIN_FIELD_NUMBER: _ClassVar[int]
    BOOTSTRAP_CREDENTIALS_FIELD_NUMBER: _ClassVar[int]
    STATIC_CREDENTIALS_FIELD_NUMBER: _ClassVar[int]
    plugin: _types_pb2.PluginV1
    bootstrap_credentials: _types_pb2.PluginBootstrapCredentialsV1
    static_credentials: _types_pb2.PluginStaticCredentialsV1
    def __init__(self, plugin: _Optional[_Union[_types_pb2.PluginV1, _Mapping]] = ..., bootstrap_credentials: _Optional[_Union[_types_pb2.PluginBootstrapCredentialsV1, _Mapping]] = ..., static_credentials: _Optional[_Union[_types_pb2.PluginStaticCredentialsV1, _Mapping]] = ...) -> None: ...

class GetPluginRequest(_message.Message):
    __slots__ = ["name", "with_secrets"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    WITH_SECRETS_FIELD_NUMBER: _ClassVar[int]
    name: str
    with_secrets: bool
    def __init__(self, name: _Optional[str] = ..., with_secrets: bool = ...) -> None: ...

class ListPluginsRequest(_message.Message):
    __slots__ = ["page_size", "start_key", "with_secrets"]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    START_KEY_FIELD_NUMBER: _ClassVar[int]
    WITH_SECRETS_FIELD_NUMBER: _ClassVar[int]
    page_size: int
    start_key: str
    with_secrets: bool
    def __init__(self, page_size: _Optional[int] = ..., start_key: _Optional[str] = ..., with_secrets: bool = ...) -> None: ...

class ListPluginsResponse(_message.Message):
    __slots__ = ["plugins", "next_key"]
    PLUGINS_FIELD_NUMBER: _ClassVar[int]
    NEXT_KEY_FIELD_NUMBER: _ClassVar[int]
    plugins: _containers.RepeatedCompositeFieldContainer[_types_pb2.PluginV1]
    next_key: str
    def __init__(self, plugins: _Optional[_Iterable[_Union[_types_pb2.PluginV1, _Mapping]]] = ..., next_key: _Optional[str] = ...) -> None: ...

class DeletePluginRequest(_message.Message):
    __slots__ = ["name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class SetPluginCredentialsRequest(_message.Message):
    __slots__ = ["name", "credentials"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    CREDENTIALS_FIELD_NUMBER: _ClassVar[int]
    name: str
    credentials: _types_pb2.PluginCredentialsV1
    def __init__(self, name: _Optional[str] = ..., credentials: _Optional[_Union[_types_pb2.PluginCredentialsV1, _Mapping]] = ...) -> None: ...

class SetPluginStatusRequest(_message.Message):
    __slots__ = ["name", "status"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    name: str
    status: _types_pb2.PluginStatusV1
    def __init__(self, name: _Optional[str] = ..., status: _Optional[_Union[_types_pb2.PluginStatusV1, _Mapping]] = ...) -> None: ...

class GetAvailablePluginTypesRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class GetAvailablePluginTypesResponse(_message.Message):
    __slots__ = ["plugin_types"]
    PLUGIN_TYPES_FIELD_NUMBER: _ClassVar[int]
    plugin_types: _containers.RepeatedCompositeFieldContainer[PluginType]
    def __init__(self, plugin_types: _Optional[_Iterable[_Union[PluginType, _Mapping]]] = ...) -> None: ...

class SearchPluginStaticCredentialsRequest(_message.Message):
    __slots__ = ["labels"]
    class LabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    LABELS_FIELD_NUMBER: _ClassVar[int]
    labels: _containers.ScalarMap[str, str]
    def __init__(self, labels: _Optional[_Mapping[str, str]] = ...) -> None: ...

class SearchPluginStaticCredentialsResponse(_message.Message):
    __slots__ = ["credentials"]
    CREDENTIALS_FIELD_NUMBER: _ClassVar[int]
    credentials: _containers.RepeatedCompositeFieldContainer[_types_pb2.PluginStaticCredentialsV1]
    def __init__(self, credentials: _Optional[_Iterable[_Union[_types_pb2.PluginStaticCredentialsV1, _Mapping]]] = ...) -> None: ...
