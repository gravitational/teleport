from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Resource(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    RESOURCE_UNSPECIFIED: _ClassVar[Resource]
    RESOURCE_WINDOWS_DESKTOPS: _ClassVar[Resource]
    RESOURCE_SERVER_SSH: _ClassVar[Resource]
    RESOURCE_DATABASES: _ClassVar[Resource]
    RESOURCE_KUBERNETES: _ClassVar[Resource]
    RESOURCE_WEB_APPLICATIONS: _ClassVar[Resource]
RESOURCE_UNSPECIFIED: Resource
RESOURCE_WINDOWS_DESKTOPS: Resource
RESOURCE_SERVER_SSH: Resource
RESOURCE_DATABASES: Resource
RESOURCE_KUBERNETES: Resource
RESOURCE_WEB_APPLICATIONS: Resource

class MarketingParams(_message.Message):
    __slots__ = ["campaign", "source", "medium", "intent"]
    CAMPAIGN_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    MEDIUM_FIELD_NUMBER: _ClassVar[int]
    INTENT_FIELD_NUMBER: _ClassVar[int]
    campaign: str
    source: str
    medium: str
    intent: str
    def __init__(self, campaign: _Optional[str] = ..., source: _Optional[str] = ..., medium: _Optional[str] = ..., intent: _Optional[str] = ...) -> None: ...

class OnboardUserPreferences(_message.Message):
    __slots__ = ["preferred_resources", "marketing_params"]
    PREFERRED_RESOURCES_FIELD_NUMBER: _ClassVar[int]
    MARKETING_PARAMS_FIELD_NUMBER: _ClassVar[int]
    preferred_resources: _containers.RepeatedScalarFieldContainer[Resource]
    marketing_params: MarketingParams
    def __init__(self, preferred_resources: _Optional[_Iterable[_Union[Resource, str]]] = ..., marketing_params: _Optional[_Union[MarketingParams, _Mapping]] = ...) -> None: ...
