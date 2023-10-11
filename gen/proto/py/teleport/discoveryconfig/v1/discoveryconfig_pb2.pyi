from teleport.header.v1 import resourceheader_pb2 as _resourceheader_pb2
from teleport.legacy.types import types_pb2 as _types_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DiscoveryConfig(_message.Message):
    __slots__ = ["header", "spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    header: _resourceheader_pb2.ResourceHeader
    spec: DiscoveryConfigSpec
    def __init__(self, header: _Optional[_Union[_resourceheader_pb2.ResourceHeader, _Mapping]] = ..., spec: _Optional[_Union[DiscoveryConfigSpec, _Mapping]] = ...) -> None: ...

class DiscoveryConfigSpec(_message.Message):
    __slots__ = ["discovery_group", "aws", "azure", "gcp", "kube"]
    DISCOVERY_GROUP_FIELD_NUMBER: _ClassVar[int]
    AWS_FIELD_NUMBER: _ClassVar[int]
    AZURE_FIELD_NUMBER: _ClassVar[int]
    GCP_FIELD_NUMBER: _ClassVar[int]
    KUBE_FIELD_NUMBER: _ClassVar[int]
    discovery_group: str
    aws: _containers.RepeatedCompositeFieldContainer[_types_pb2.AWSMatcher]
    azure: _containers.RepeatedCompositeFieldContainer[_types_pb2.AzureMatcher]
    gcp: _containers.RepeatedCompositeFieldContainer[_types_pb2.GCPMatcher]
    kube: _containers.RepeatedCompositeFieldContainer[_types_pb2.KubernetesMatcher]
    def __init__(self, discovery_group: _Optional[str] = ..., aws: _Optional[_Iterable[_Union[_types_pb2.AWSMatcher, _Mapping]]] = ..., azure: _Optional[_Iterable[_Union[_types_pb2.AzureMatcher, _Mapping]]] = ..., gcp: _Optional[_Iterable[_Union[_types_pb2.GCPMatcher, _Mapping]]] = ..., kube: _Optional[_Iterable[_Union[_types_pb2.KubernetesMatcher, _Mapping]]] = ...) -> None: ...
