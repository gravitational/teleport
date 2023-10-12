from google.protobuf import empty_pb2 as _empty_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from teleport.accesslist.v1 import accesslist_pb2 as _accesslist_pb2
from teleport.legacy.types import types_pb2 as _types_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetAccessListsRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class GetAccessListsResponse(_message.Message):
    __slots__ = ["access_lists"]
    ACCESS_LISTS_FIELD_NUMBER: _ClassVar[int]
    access_lists: _containers.RepeatedCompositeFieldContainer[_accesslist_pb2.AccessList]
    def __init__(self, access_lists: _Optional[_Iterable[_Union[_accesslist_pb2.AccessList, _Mapping]]] = ...) -> None: ...

class ListAccessListsRequest(_message.Message):
    __slots__ = ["page_size", "next_token"]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    NEXT_TOKEN_FIELD_NUMBER: _ClassVar[int]
    page_size: int
    next_token: str
    def __init__(self, page_size: _Optional[int] = ..., next_token: _Optional[str] = ...) -> None: ...

class ListAccessListsResponse(_message.Message):
    __slots__ = ["access_lists", "next_token"]
    ACCESS_LISTS_FIELD_NUMBER: _ClassVar[int]
    NEXT_TOKEN_FIELD_NUMBER: _ClassVar[int]
    access_lists: _containers.RepeatedCompositeFieldContainer[_accesslist_pb2.AccessList]
    next_token: str
    def __init__(self, access_lists: _Optional[_Iterable[_Union[_accesslist_pb2.AccessList, _Mapping]]] = ..., next_token: _Optional[str] = ...) -> None: ...

class GetAccessListRequest(_message.Message):
    __slots__ = ["name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class UpsertAccessListRequest(_message.Message):
    __slots__ = ["access_list"]
    ACCESS_LIST_FIELD_NUMBER: _ClassVar[int]
    access_list: _accesslist_pb2.AccessList
    def __init__(self, access_list: _Optional[_Union[_accesslist_pb2.AccessList, _Mapping]] = ...) -> None: ...

class DeleteAccessListRequest(_message.Message):
    __slots__ = ["name"]
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class DeleteAllAccessListsRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class ListAccessListMembersRequest(_message.Message):
    __slots__ = ["page_size", "page_token", "access_list"]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    ACCESS_LIST_FIELD_NUMBER: _ClassVar[int]
    page_size: int
    page_token: str
    access_list: str
    def __init__(self, page_size: _Optional[int] = ..., page_token: _Optional[str] = ..., access_list: _Optional[str] = ...) -> None: ...

class ListAccessListMembersResponse(_message.Message):
    __slots__ = ["members", "next_page_token"]
    MEMBERS_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    members: _containers.RepeatedCompositeFieldContainer[_accesslist_pb2.Member]
    next_page_token: str
    def __init__(self, members: _Optional[_Iterable[_Union[_accesslist_pb2.Member, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

class UpsertAccessListWithMembersRequest(_message.Message):
    __slots__ = ["access_list", "members"]
    ACCESS_LIST_FIELD_NUMBER: _ClassVar[int]
    MEMBERS_FIELD_NUMBER: _ClassVar[int]
    access_list: _accesslist_pb2.AccessList
    members: _containers.RepeatedCompositeFieldContainer[_accesslist_pb2.Member]
    def __init__(self, access_list: _Optional[_Union[_accesslist_pb2.AccessList, _Mapping]] = ..., members: _Optional[_Iterable[_Union[_accesslist_pb2.Member, _Mapping]]] = ...) -> None: ...

class UpsertAccessListWithMembersResponse(_message.Message):
    __slots__ = ["access_list", "members"]
    ACCESS_LIST_FIELD_NUMBER: _ClassVar[int]
    MEMBERS_FIELD_NUMBER: _ClassVar[int]
    access_list: _accesslist_pb2.AccessList
    members: _containers.RepeatedCompositeFieldContainer[_accesslist_pb2.Member]
    def __init__(self, access_list: _Optional[_Union[_accesslist_pb2.AccessList, _Mapping]] = ..., members: _Optional[_Iterable[_Union[_accesslist_pb2.Member, _Mapping]]] = ...) -> None: ...

class GetAccessListMemberRequest(_message.Message):
    __slots__ = ["access_list", "member_name"]
    ACCESS_LIST_FIELD_NUMBER: _ClassVar[int]
    MEMBER_NAME_FIELD_NUMBER: _ClassVar[int]
    access_list: str
    member_name: str
    def __init__(self, access_list: _Optional[str] = ..., member_name: _Optional[str] = ...) -> None: ...

class UpsertAccessListMemberRequest(_message.Message):
    __slots__ = ["member"]
    MEMBER_FIELD_NUMBER: _ClassVar[int]
    member: _accesslist_pb2.Member
    def __init__(self, member: _Optional[_Union[_accesslist_pb2.Member, _Mapping]] = ...) -> None: ...

class DeleteAccessListMemberRequest(_message.Message):
    __slots__ = ["access_list", "member_name"]
    ACCESS_LIST_FIELD_NUMBER: _ClassVar[int]
    MEMBER_NAME_FIELD_NUMBER: _ClassVar[int]
    access_list: str
    member_name: str
    def __init__(self, access_list: _Optional[str] = ..., member_name: _Optional[str] = ...) -> None: ...

class DeleteAllAccessListMembersForAccessListRequest(_message.Message):
    __slots__ = ["access_list"]
    ACCESS_LIST_FIELD_NUMBER: _ClassVar[int]
    access_list: str
    def __init__(self, access_list: _Optional[str] = ...) -> None: ...

class DeleteAllAccessListMembersRequest(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class ListAccessListReviewsRequest(_message.Message):
    __slots__ = ["access_list", "page_size", "next_token"]
    ACCESS_LIST_FIELD_NUMBER: _ClassVar[int]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    NEXT_TOKEN_FIELD_NUMBER: _ClassVar[int]
    access_list: str
    page_size: int
    next_token: str
    def __init__(self, access_list: _Optional[str] = ..., page_size: _Optional[int] = ..., next_token: _Optional[str] = ...) -> None: ...

class ListAccessListReviewsResponse(_message.Message):
    __slots__ = ["reviews", "next_token"]
    REVIEWS_FIELD_NUMBER: _ClassVar[int]
    NEXT_TOKEN_FIELD_NUMBER: _ClassVar[int]
    reviews: _containers.RepeatedCompositeFieldContainer[_accesslist_pb2.Review]
    next_token: str
    def __init__(self, reviews: _Optional[_Iterable[_Union[_accesslist_pb2.Review, _Mapping]]] = ..., next_token: _Optional[str] = ...) -> None: ...

class CreateAccessListReviewRequest(_message.Message):
    __slots__ = ["review"]
    REVIEW_FIELD_NUMBER: _ClassVar[int]
    review: _accesslist_pb2.Review
    def __init__(self, review: _Optional[_Union[_accesslist_pb2.Review, _Mapping]] = ...) -> None: ...

class CreateAccessListReviewResponse(_message.Message):
    __slots__ = ["review_name", "next_audit_date"]
    REVIEW_NAME_FIELD_NUMBER: _ClassVar[int]
    NEXT_AUDIT_DATE_FIELD_NUMBER: _ClassVar[int]
    review_name: str
    next_audit_date: _timestamp_pb2.Timestamp
    def __init__(self, review_name: _Optional[str] = ..., next_audit_date: _Optional[_Union[_timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class DeleteAccessListReviewRequest(_message.Message):
    __slots__ = ["review_name", "access_list_name"]
    REVIEW_NAME_FIELD_NUMBER: _ClassVar[int]
    ACCESS_LIST_NAME_FIELD_NUMBER: _ClassVar[int]
    review_name: str
    access_list_name: str
    def __init__(self, review_name: _Optional[str] = ..., access_list_name: _Optional[str] = ...) -> None: ...

class AccessRequestPromoteRequest(_message.Message):
    __slots__ = ["request_id", "access_list_name", "reason"]
    REQUEST_ID_FIELD_NUMBER: _ClassVar[int]
    ACCESS_LIST_NAME_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    request_id: str
    access_list_name: str
    reason: str
    def __init__(self, request_id: _Optional[str] = ..., access_list_name: _Optional[str] = ..., reason: _Optional[str] = ...) -> None: ...

class AccessRequestPromoteResponse(_message.Message):
    __slots__ = ["access_request"]
    ACCESS_REQUEST_FIELD_NUMBER: _ClassVar[int]
    access_request: _types_pb2.AccessRequestV3
    def __init__(self, access_request: _Optional[_Union[_types_pb2.AccessRequestV3, _Mapping]] = ...) -> None: ...
