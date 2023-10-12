from google.protobuf import timestamp_pb2 as _timestamp_pb2
from teleport.header.v1 import resourceheader_pb2 as _resourceheader_pb2
from teleport.trait.v1 import trait_pb2 as _trait_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ReviewFrequency(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    REVIEW_FREQUENCY_UNSPECIFIED: _ClassVar[ReviewFrequency]
    REVIEW_FREQUENCY_ONE_MONTH: _ClassVar[ReviewFrequency]
    REVIEW_FREQUENCY_THREE_MONTHS: _ClassVar[ReviewFrequency]
    REVIEW_FREQUENCY_SIX_MONTHS: _ClassVar[ReviewFrequency]
    REVIEW_FREQUENCY_ONE_YEAR: _ClassVar[ReviewFrequency]

class ReviewDayOfMonth(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    REVIEW_DAY_OF_MONTH_UNSPECIFIED: _ClassVar[ReviewDayOfMonth]
    REVIEW_DAY_OF_MONTH_FIRST: _ClassVar[ReviewDayOfMonth]
    REVIEW_DAY_OF_MONTH_FIFTEENTH: _ClassVar[ReviewDayOfMonth]
    REVIEW_DAY_OF_MONTH_LAST: _ClassVar[ReviewDayOfMonth]

class IneligibleStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    INELIGIBLE_STATUS_UNSPECIFIED: _ClassVar[IneligibleStatus]
    INELIGIBLE_STATUS_ELIGIBLE: _ClassVar[IneligibleStatus]
    INELIGIBLE_STATUS_USER_NOT_EXIST: _ClassVar[IneligibleStatus]
    INELIGIBLE_STATUS_MISSING_REQUIREMENTS: _ClassVar[IneligibleStatus]
    INELIGIBLE_STATUS_EXPIRED: _ClassVar[IneligibleStatus]
REVIEW_FREQUENCY_UNSPECIFIED: ReviewFrequency
REVIEW_FREQUENCY_ONE_MONTH: ReviewFrequency
REVIEW_FREQUENCY_THREE_MONTHS: ReviewFrequency
REVIEW_FREQUENCY_SIX_MONTHS: ReviewFrequency
REVIEW_FREQUENCY_ONE_YEAR: ReviewFrequency
REVIEW_DAY_OF_MONTH_UNSPECIFIED: ReviewDayOfMonth
REVIEW_DAY_OF_MONTH_FIRST: ReviewDayOfMonth
REVIEW_DAY_OF_MONTH_FIFTEENTH: ReviewDayOfMonth
REVIEW_DAY_OF_MONTH_LAST: ReviewDayOfMonth
INELIGIBLE_STATUS_UNSPECIFIED: IneligibleStatus
INELIGIBLE_STATUS_ELIGIBLE: IneligibleStatus
INELIGIBLE_STATUS_USER_NOT_EXIST: IneligibleStatus
INELIGIBLE_STATUS_MISSING_REQUIREMENTS: IneligibleStatus
INELIGIBLE_STATUS_EXPIRED: IneligibleStatus

class AccessList(_message.Message):
    __slots__ = ["header", "spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    header: _resourceheader_pb2.ResourceHeader
    spec: AccessListSpec
    def __init__(self, header: _Optional[_Union[_resourceheader_pb2.ResourceHeader, _Mapping]] = ..., spec: _Optional[_Union[AccessListSpec, _Mapping]] = ...) -> None: ...

class AccessListSpec(_message.Message):
    __slots__ = ["description", "owners", "audit", "membership_requires", "ownership_requires", "grants", "title"]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    OWNERS_FIELD_NUMBER: _ClassVar[int]
    AUDIT_FIELD_NUMBER: _ClassVar[int]
    MEMBERSHIP_REQUIRES_FIELD_NUMBER: _ClassVar[int]
    OWNERSHIP_REQUIRES_FIELD_NUMBER: _ClassVar[int]
    GRANTS_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    description: str
    owners: _containers.RepeatedCompositeFieldContainer[AccessListOwner]
    audit: AccessListAudit
    membership_requires: AccessListRequires
    ownership_requires: AccessListRequires
    grants: AccessListGrants
    title: str
    def __init__(self, description: _Optional[str] = ..., owners: _Optional[_Iterable[_Union[AccessListOwner, _Mapping]]] = ..., audit: _Optional[_Union[AccessListAudit, _Mapping]] = ..., membership_requires: _Optional[_Union[AccessListRequires, _Mapping]] = ..., ownership_requires: _Optional[_Union[AccessListRequires, _Mapping]] = ..., grants: _Optional[_Union[AccessListGrants, _Mapping]] = ..., title: _Optional[str] = ...) -> None: ...

class AccessListOwner(_message.Message):
    __slots__ = ["name", "description", "ineligible_status"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    INELIGIBLE_STATUS_FIELD_NUMBER: _ClassVar[int]
    name: str
    description: str
    ineligible_status: IneligibleStatus
    def __init__(self, name: _Optional[str] = ..., description: _Optional[str] = ..., ineligible_status: _Optional[_Union[IneligibleStatus, str]] = ...) -> None: ...

class AccessListAudit(_message.Message):
    __slots__ = ["next_audit_date", "recurrence"]
    NEXT_AUDIT_DATE_FIELD_NUMBER: _ClassVar[int]
    RECURRENCE_FIELD_NUMBER: _ClassVar[int]
    next_audit_date: _timestamp_pb2.Timestamp
    recurrence: Recurrence
    def __init__(self, next_audit_date: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., recurrence: _Optional[_Union[Recurrence, _Mapping]] = ...) -> None: ...

class Recurrence(_message.Message):
    __slots__ = ["frequency", "day_of_month"]
    FREQUENCY_FIELD_NUMBER: _ClassVar[int]
    DAY_OF_MONTH_FIELD_NUMBER: _ClassVar[int]
    frequency: ReviewFrequency
    day_of_month: ReviewDayOfMonth
    def __init__(self, frequency: _Optional[_Union[ReviewFrequency, str]] = ..., day_of_month: _Optional[_Union[ReviewDayOfMonth, str]] = ...) -> None: ...

class AccessListRequires(_message.Message):
    __slots__ = ["roles", "traits"]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    TRAITS_FIELD_NUMBER: _ClassVar[int]
    roles: _containers.RepeatedScalarFieldContainer[str]
    traits: _containers.RepeatedCompositeFieldContainer[_trait_pb2.Trait]
    def __init__(self, roles: _Optional[_Iterable[str]] = ..., traits: _Optional[_Iterable[_Union[_trait_pb2.Trait, _Mapping]]] = ...) -> None: ...

class AccessListGrants(_message.Message):
    __slots__ = ["roles", "traits"]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    TRAITS_FIELD_NUMBER: _ClassVar[int]
    roles: _containers.RepeatedScalarFieldContainer[str]
    traits: _containers.RepeatedCompositeFieldContainer[_trait_pb2.Trait]
    def __init__(self, roles: _Optional[_Iterable[str]] = ..., traits: _Optional[_Iterable[_Union[_trait_pb2.Trait, _Mapping]]] = ...) -> None: ...

class Member(_message.Message):
    __slots__ = ["header", "spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    header: _resourceheader_pb2.ResourceHeader
    spec: MemberSpec
    def __init__(self, header: _Optional[_Union[_resourceheader_pb2.ResourceHeader, _Mapping]] = ..., spec: _Optional[_Union[MemberSpec, _Mapping]] = ...) -> None: ...

class MemberSpec(_message.Message):
    __slots__ = ["access_list", "name", "joined", "expires", "reason", "added_by", "ineligible_status"]
    ACCESS_LIST_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    JOINED_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    ADDED_BY_FIELD_NUMBER: _ClassVar[int]
    INELIGIBLE_STATUS_FIELD_NUMBER: _ClassVar[int]
    access_list: str
    name: str
    joined: _timestamp_pb2.Timestamp
    expires: _timestamp_pb2.Timestamp
    reason: str
    added_by: str
    ineligible_status: IneligibleStatus
    def __init__(self, access_list: _Optional[str] = ..., name: _Optional[str] = ..., joined: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., expires: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., reason: _Optional[str] = ..., added_by: _Optional[str] = ..., ineligible_status: _Optional[_Union[IneligibleStatus, str]] = ...) -> None: ...

class Review(_message.Message):
    __slots__ = ["header", "spec"]
    HEADER_FIELD_NUMBER: _ClassVar[int]
    SPEC_FIELD_NUMBER: _ClassVar[int]
    header: _resourceheader_pb2.ResourceHeader
    spec: ReviewSpec
    def __init__(self, header: _Optional[_Union[_resourceheader_pb2.ResourceHeader, _Mapping]] = ..., spec: _Optional[_Union[ReviewSpec, _Mapping]] = ...) -> None: ...

class ReviewSpec(_message.Message):
    __slots__ = ["access_list", "reviewers", "review_date", "notes", "changes"]
    ACCESS_LIST_FIELD_NUMBER: _ClassVar[int]
    REVIEWERS_FIELD_NUMBER: _ClassVar[int]
    REVIEW_DATE_FIELD_NUMBER: _ClassVar[int]
    NOTES_FIELD_NUMBER: _ClassVar[int]
    CHANGES_FIELD_NUMBER: _ClassVar[int]
    access_list: str
    reviewers: _containers.RepeatedScalarFieldContainer[str]
    review_date: _timestamp_pb2.Timestamp
    notes: str
    changes: ReviewChanges
    def __init__(self, access_list: _Optional[str] = ..., reviewers: _Optional[_Iterable[str]] = ..., review_date: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ..., notes: _Optional[str] = ..., changes: _Optional[_Union[ReviewChanges, _Mapping]] = ...) -> None: ...

class ReviewChanges(_message.Message):
    __slots__ = ["membership_requirements_changed", "removed_members", "review_frequency_changed", "review_day_of_month_changed"]
    MEMBERSHIP_REQUIREMENTS_CHANGED_FIELD_NUMBER: _ClassVar[int]
    REMOVED_MEMBERS_FIELD_NUMBER: _ClassVar[int]
    REVIEW_FREQUENCY_CHANGED_FIELD_NUMBER: _ClassVar[int]
    REVIEW_DAY_OF_MONTH_CHANGED_FIELD_NUMBER: _ClassVar[int]
    membership_requirements_changed: AccessListRequires
    removed_members: _containers.RepeatedScalarFieldContainer[str]
    review_frequency_changed: ReviewFrequency
    review_day_of_month_changed: ReviewDayOfMonth
    def __init__(self, membership_requirements_changed: _Optional[_Union[AccessListRequires, _Mapping]] = ..., removed_members: _Optional[_Iterable[str]] = ..., review_frequency_changed: _Optional[_Union[ReviewFrequency, str]] = ..., review_day_of_month_changed: _Optional[_Union[ReviewDayOfMonth, str]] = ...) -> None: ...
