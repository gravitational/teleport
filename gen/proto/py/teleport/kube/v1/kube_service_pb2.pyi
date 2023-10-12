from teleport.legacy.types import types_pb2 as _types_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ListKubernetesResourcesRequest(_message.Message):
    __slots__ = ["resource_type", "limit", "start_key", "labels", "predicate_expression", "search_keywords", "sort_by", "need_total_count", "use_search_as_roles", "use_preview_as_roles", "teleport_cluster", "kubernetes_cluster", "kubernetes_namespace"]
    class LabelsEntry(_message.Message):
        __slots__ = ["key", "value"]
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    RESOURCE_TYPE_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    START_KEY_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    PREDICATE_EXPRESSION_FIELD_NUMBER: _ClassVar[int]
    SEARCH_KEYWORDS_FIELD_NUMBER: _ClassVar[int]
    SORT_BY_FIELD_NUMBER: _ClassVar[int]
    NEED_TOTAL_COUNT_FIELD_NUMBER: _ClassVar[int]
    USE_SEARCH_AS_ROLES_FIELD_NUMBER: _ClassVar[int]
    USE_PREVIEW_AS_ROLES_FIELD_NUMBER: _ClassVar[int]
    TELEPORT_CLUSTER_FIELD_NUMBER: _ClassVar[int]
    KUBERNETES_CLUSTER_FIELD_NUMBER: _ClassVar[int]
    KUBERNETES_NAMESPACE_FIELD_NUMBER: _ClassVar[int]
    resource_type: str
    limit: int
    start_key: str
    labels: _containers.ScalarMap[str, str]
    predicate_expression: str
    search_keywords: _containers.RepeatedScalarFieldContainer[str]
    sort_by: _types_pb2.SortBy
    need_total_count: bool
    use_search_as_roles: bool
    use_preview_as_roles: bool
    teleport_cluster: str
    kubernetes_cluster: str
    kubernetes_namespace: str
    def __init__(self, resource_type: _Optional[str] = ..., limit: _Optional[int] = ..., start_key: _Optional[str] = ..., labels: _Optional[_Mapping[str, str]] = ..., predicate_expression: _Optional[str] = ..., search_keywords: _Optional[_Iterable[str]] = ..., sort_by: _Optional[_Union[_types_pb2.SortBy, _Mapping]] = ..., need_total_count: bool = ..., use_search_as_roles: bool = ..., use_preview_as_roles: bool = ..., teleport_cluster: _Optional[str] = ..., kubernetes_cluster: _Optional[str] = ..., kubernetes_namespace: _Optional[str] = ...) -> None: ...

class ListKubernetesResourcesResponse(_message.Message):
    __slots__ = ["resources", "next_key", "total_count"]
    RESOURCES_FIELD_NUMBER: _ClassVar[int]
    NEXT_KEY_FIELD_NUMBER: _ClassVar[int]
    TOTAL_COUNT_FIELD_NUMBER: _ClassVar[int]
    resources: _containers.RepeatedCompositeFieldContainer[_types_pb2.KubernetesResourceV1]
    next_key: str
    total_count: int
    def __init__(self, resources: _Optional[_Iterable[_Union[_types_pb2.KubernetesResourceV1, _Mapping]]] = ..., next_key: _Optional[str] = ..., total_count: _Optional[int] = ...) -> None: ...
