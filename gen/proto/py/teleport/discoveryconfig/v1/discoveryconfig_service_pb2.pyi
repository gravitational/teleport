from google.protobuf import empty_pb2 as _empty_pb2
from teleport.discoveryconfig.v1 import discoveryconfig_pb2 as _discoveryconfig_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ListDiscoveryConfigsRequest(_message.Message):
    __slots__ = ["page_size", "next_token"]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    NEXT_TOKEN_FIELD_NUMBER: _ClassVar[int]
    page_size: int
    next_token: str
    def __init__(self, page_size: _Optional[int] = ..., next_token: _Optional[str] = ...) -> None: ...

class ListDiscoveryConfigsResponse(_message.Message):
    __slots__ = ["discovery_configs", "next_key", "total_count"]
    DISCOVERY_CONFIGS_FIELD_NUMBER: _ClassVar[int]
    NEXT_KEY_FIELD_NUMBER: _ClassVar[int]
    TOTAL_COUNT_FIELD_NUMBER: _ClassVar[int]
    discovery_configs: _containers.RepeatedCompositeFieldContainer[_discoveryconfig_pb2.DiscoveryConfig]
    next_key: str
    total_count: int
    def __init__(self, discovery_configs: _Optional[_Iterable[_Union[_discoveryconfig_pb2.DiscoveryConfig, _Mapping]]] = ..., next_key: _Optional[str] = ..., total_count: _Optional[int] = ...) -> None: ...

class GetDiscoveryConfigRequest(_message.Message):
    __slots__ = ["name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class CreateDiscoveryConfigRequest(_message.Message):
    __slots__ = ["discovery_config"]
    DISCOVERY_CONFIG_FIELD_NUMBER: _ClassVar[int]
    discovery_config: _discoveryconfig_pb2.DiscoveryConfig
    def __init__(self, discovery_config: _Optional[_Union[_discoveryconfig_pb2.DiscoveryConfig, _Mapping]] = ...) -> None: ...

class UpdateDiscoveryConfigRequest(_message.Message):
    __slots__ = ["discovery_config"]
    DISCOVERY_CONFIG_FIELD_NUMBER: _ClassVar[int]
    discovery_config: _discoveryconfig_pb2.DiscoveryConfig
    def __init__(self, discovery_config: _Optional[_Union[_discoveryconfig_pb2.DiscoveryConfig, _Mapping]] = ...) -> None: ...

class DeleteDiscoveryConfigRequest(_message.Message):
    __slots__ = ["name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class DeleteAllDiscoveryConfigsRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...
