from google.protobuf import duration_pb2 as _duration_pb2
from google.protobuf import empty_pb2 as _empty_pb2
from teleport.legacy.types import types_pb2 as _types_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ListOktaImportRulesRequest(_message.Message):
    __slots__ = ["page_size", "page_token"]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    page_size: int
    page_token: str
    def __init__(self, page_size: _Optional[int] = ..., page_token: _Optional[str] = ...) -> None: ...

class ListOktaImportRulesResponse(_message.Message):
    __slots__ = ["import_rules", "next_page_token"]
    IMPORT_RULES_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    import_rules: _containers.RepeatedCompositeFieldContainer[_types_pb2.OktaImportRuleV1]
    next_page_token: str
    def __init__(self, import_rules: _Optional[_Iterable[_Union[_types_pb2.OktaImportRuleV1, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

class GetOktaImportRuleRequest(_message.Message):
    __slots__ = ["name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class CreateOktaImportRuleRequest(_message.Message):
    __slots__ = ["import_rule"]
    IMPORT_RULE_FIELD_NUMBER: _ClassVar[int]
    import_rule: _types_pb2.OktaImportRuleV1
    def __init__(self, import_rule: _Optional[_Union[_types_pb2.OktaImportRuleV1, _Mapping]] = ...) -> None: ...

class UpdateOktaImportRuleRequest(_message.Message):
    __slots__ = ["import_rule"]
    IMPORT_RULE_FIELD_NUMBER: _ClassVar[int]
    import_rule: _types_pb2.OktaImportRuleV1
    def __init__(self, import_rule: _Optional[_Union[_types_pb2.OktaImportRuleV1, _Mapping]] = ...) -> None: ...

class DeleteOktaImportRuleRequest(_message.Message):
    __slots__ = ["name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class DeleteAllOktaImportRulesRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class ListOktaAssignmentsRequest(_message.Message):
    __slots__ = ["page_size", "page_token"]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    page_size: int
    page_token: str
    def __init__(self, page_size: _Optional[int] = ..., page_token: _Optional[str] = ...) -> None: ...

class ListOktaAssignmentsResponse(_message.Message):
    __slots__ = ["assignments", "next_page_token"]
    ASSIGNMENTS_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    assignments: _containers.RepeatedCompositeFieldContainer[_types_pb2.OktaAssignmentV1]
    next_page_token: str
    def __init__(self, assignments: _Optional[_Iterable[_Union[_types_pb2.OktaAssignmentV1, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

class GetOktaAssignmentRequest(_message.Message):
    __slots__ = ["name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class CreateOktaAssignmentRequest(_message.Message):
    __slots__ = ["assignment"]
    ASSIGNMENT_FIELD_NUMBER: _ClassVar[int]
    assignment: _types_pb2.OktaAssignmentV1
    def __init__(self, assignment: _Optional[_Union[_types_pb2.OktaAssignmentV1, _Mapping]] = ...) -> None: ...

class UpdateOktaAssignmentRequest(_message.Message):
    __slots__ = ["assignment"]
    ASSIGNMENT_FIELD_NUMBER: _ClassVar[int]
    assignment: _types_pb2.OktaAssignmentV1
    def __init__(self, assignment: _Optional[_Union[_types_pb2.OktaAssignmentV1, _Mapping]] = ...) -> None: ...

class UpdateOktaAssignmentStatusRequest(_message.Message):
    __slots__ = ["name", "status", "time_has_passed"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    TIME_HAS_PASSED_FIELD_NUMBER: _ClassVar[int]
    name: str
    status: _types_pb2.OktaAssignmentSpecV1.OktaAssignmentStatus
    time_has_passed: _duration_pb2.Duration
    def __init__(self, name: _Optional[str] = ..., status: _Optional[_Union[_types_pb2.OktaAssignmentSpecV1.OktaAssignmentStatus, str]] = ..., time_has_passed: _Optional[_Union[_duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class DeleteOktaAssignmentRequest(_message.Message):
    __slots__ = ["name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class DeleteAllOktaAssignmentsRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...
