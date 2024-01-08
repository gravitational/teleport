// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
'use strict';
var grpc = require('@grpc/grpc-js');
var teleport_accesslist_v1_accesslist_service_pb = require('../../../teleport/accesslist/v1/accesslist_service_pb.js');
var google_protobuf_empty_pb = require('google-protobuf/google/protobuf/empty_pb.js');
var google_protobuf_timestamp_pb = require('google-protobuf/google/protobuf/timestamp_pb.js');
var teleport_accesslist_v1_accesslist_pb = require('../../../teleport/accesslist/v1/accesslist_pb.js');
var teleport_legacy_types_types_pb = require('../../../teleport/legacy/types/types_pb.js');

function serialize_google_protobuf_Empty(arg) {
  if (!(arg instanceof google_protobuf_empty_pb.Empty)) {
    throw new Error('Expected argument of type google.protobuf.Empty');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_google_protobuf_Empty(buffer_arg) {
  return google_protobuf_empty_pb.Empty.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_AccessList(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_pb.AccessList)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.AccessList');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_AccessList(buffer_arg) {
  return teleport_accesslist_v1_accesslist_pb.AccessList.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_AccessRequestPromoteRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.AccessRequestPromoteRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_AccessRequestPromoteRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_AccessRequestPromoteResponse(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteResponse)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.AccessRequestPromoteResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_AccessRequestPromoteResponse(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_CreateAccessListReviewRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.CreateAccessListReviewRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_CreateAccessListReviewRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_CreateAccessListReviewResponse(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewResponse)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.CreateAccessListReviewResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_CreateAccessListReviewResponse(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_DeleteAccessListMemberRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListMemberRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.DeleteAccessListMemberRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_DeleteAccessListMemberRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListMemberRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_DeleteAccessListRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.DeleteAccessListRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_DeleteAccessListRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_DeleteAccessListReviewRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListReviewRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.DeleteAccessListReviewRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_DeleteAccessListReviewRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListReviewRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_DeleteAllAccessListMembersForAccessListRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersForAccessListRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.DeleteAllAccessListMembersForAccessListRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_DeleteAllAccessListMembersForAccessListRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersForAccessListRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_DeleteAllAccessListMembersRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.DeleteAllAccessListMembersRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_DeleteAllAccessListMembersRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_DeleteAllAccessListsRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListsRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.DeleteAllAccessListsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_DeleteAllAccessListsRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_GetAccessListMemberRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.GetAccessListMemberRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.GetAccessListMemberRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_GetAccessListMemberRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.GetAccessListMemberRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_GetAccessListRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.GetAccessListRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.GetAccessListRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_GetAccessListRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.GetAccessListRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_GetAccessListsRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.GetAccessListsRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.GetAccessListsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_GetAccessListsRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.GetAccessListsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_GetAccessListsResponse(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.GetAccessListsResponse)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.GetAccessListsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_GetAccessListsResponse(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.GetAccessListsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_GetAccessListsToReviewRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.GetAccessListsToReviewRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_GetAccessListsToReviewRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_GetAccessListsToReviewResponse(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewResponse)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.GetAccessListsToReviewResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_GetAccessListsToReviewResponse(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_GetSuggestedAccessListsRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.GetSuggestedAccessListsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_GetSuggestedAccessListsRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_GetSuggestedAccessListsResponse(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsResponse)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.GetSuggestedAccessListsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_GetSuggestedAccessListsResponse(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_ListAccessListMembersRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.ListAccessListMembersRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_ListAccessListMembersRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_ListAccessListMembersResponse(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersResponse)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.ListAccessListMembersResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_ListAccessListMembersResponse(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_ListAccessListReviewsRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.ListAccessListReviewsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_ListAccessListReviewsRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_ListAccessListReviewsResponse(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsResponse)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.ListAccessListReviewsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_ListAccessListReviewsResponse(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_ListAccessListsRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.ListAccessListsRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.ListAccessListsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_ListAccessListsRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.ListAccessListsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_ListAccessListsResponse(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.ListAccessListsResponse)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.ListAccessListsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_ListAccessListsResponse(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.ListAccessListsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_ListAllAccessListMembersRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.ListAllAccessListMembersRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_ListAllAccessListMembersRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_ListAllAccessListMembersResponse(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersResponse)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.ListAllAccessListMembersResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_ListAllAccessListMembersResponse(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_ListAllAccessListReviewsRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.ListAllAccessListReviewsRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_ListAllAccessListReviewsRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_ListAllAccessListReviewsResponse(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsResponse)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.ListAllAccessListReviewsResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_ListAllAccessListReviewsResponse(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_Member(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_pb.Member)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.Member');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_Member(buffer_arg) {
  return teleport_accesslist_v1_accesslist_pb.Member.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_UpsertAccessListMemberRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListMemberRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.UpsertAccessListMemberRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_UpsertAccessListMemberRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListMemberRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_UpsertAccessListRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.UpsertAccessListRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_UpsertAccessListRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_UpsertAccessListWithMembersRequest(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersRequest)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.UpsertAccessListWithMembersRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_UpsertAccessListWithMembersRequest(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_teleport_accesslist_v1_UpsertAccessListWithMembersResponse(arg) {
  if (!(arg instanceof teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersResponse)) {
    throw new Error('Expected argument of type teleport.accesslist.v1.UpsertAccessListWithMembersResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_teleport_accesslist_v1_UpsertAccessListWithMembersResponse(buffer_arg) {
  return teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// AccessListService provides CRUD methods for Access List resources.
var AccessListServiceService = exports.AccessListServiceService = {
  // GetAccessLists returns a list of all access lists.
getAccessLists: {
    path: '/teleport.accesslist.v1.AccessListService/GetAccessLists',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsRequest,
    responseType: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsResponse,
    requestSerialize: serialize_teleport_accesslist_v1_GetAccessListsRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_GetAccessListsRequest,
    responseSerialize: serialize_teleport_accesslist_v1_GetAccessListsResponse,
    responseDeserialize: deserialize_teleport_accesslist_v1_GetAccessListsResponse,
  },
  // ListAccessLists returns a paginated list of all access lists.
listAccessLists: {
    path: '/teleport.accesslist.v1.AccessListService/ListAccessLists',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.ListAccessListsRequest,
    responseType: teleport_accesslist_v1_accesslist_service_pb.ListAccessListsResponse,
    requestSerialize: serialize_teleport_accesslist_v1_ListAccessListsRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_ListAccessListsRequest,
    responseSerialize: serialize_teleport_accesslist_v1_ListAccessListsResponse,
    responseDeserialize: deserialize_teleport_accesslist_v1_ListAccessListsResponse,
  },
  // GetAccessList returns the specified access list resource.
getAccessList: {
    path: '/teleport.accesslist.v1.AccessListService/GetAccessList',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.GetAccessListRequest,
    responseType: teleport_accesslist_v1_accesslist_pb.AccessList,
    requestSerialize: serialize_teleport_accesslist_v1_GetAccessListRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_GetAccessListRequest,
    responseSerialize: serialize_teleport_accesslist_v1_AccessList,
    responseDeserialize: deserialize_teleport_accesslist_v1_AccessList,
  },
  // UpsertAccessList creates or updates an access list resource.
upsertAccessList: {
    path: '/teleport.accesslist.v1.AccessListService/UpsertAccessList',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListRequest,
    responseType: teleport_accesslist_v1_accesslist_pb.AccessList,
    requestSerialize: serialize_teleport_accesslist_v1_UpsertAccessListRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_UpsertAccessListRequest,
    responseSerialize: serialize_teleport_accesslist_v1_AccessList,
    responseDeserialize: deserialize_teleport_accesslist_v1_AccessList,
  },
  // DeleteAccessList hard deletes the specified access list resource.
deleteAccessList: {
    path: '/teleport.accesslist.v1.AccessListService/DeleteAccessList',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListRequest,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_teleport_accesslist_v1_DeleteAccessListRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_DeleteAccessListRequest,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
  // DeleteAllAccessLists hard deletes all access lists.
deleteAllAccessLists: {
    path: '/teleport.accesslist.v1.AccessListService/DeleteAllAccessLists',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListsRequest,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_teleport_accesslist_v1_DeleteAllAccessListsRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_DeleteAllAccessListsRequest,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
  // GetAccessListsToReview will return access lists that need to be reviewed by the current user.
getAccessListsToReview: {
    path: '/teleport.accesslist.v1.AccessListService/GetAccessListsToReview',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewRequest,
    responseType: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewResponse,
    requestSerialize: serialize_teleport_accesslist_v1_GetAccessListsToReviewRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_GetAccessListsToReviewRequest,
    responseSerialize: serialize_teleport_accesslist_v1_GetAccessListsToReviewResponse,
    responseDeserialize: deserialize_teleport_accesslist_v1_GetAccessListsToReviewResponse,
  },
  // ListAccessListMembers returns a paginated list of all access list members.
listAccessListMembers: {
    path: '/teleport.accesslist.v1.AccessListService/ListAccessListMembers',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersRequest,
    responseType: teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersResponse,
    requestSerialize: serialize_teleport_accesslist_v1_ListAccessListMembersRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_ListAccessListMembersRequest,
    responseSerialize: serialize_teleport_accesslist_v1_ListAccessListMembersResponse,
    responseDeserialize: deserialize_teleport_accesslist_v1_ListAccessListMembersResponse,
  },
  // ListAllAccessListMembers returns a paginated list of all access list members for all access lists.
listAllAccessListMembers: {
    path: '/teleport.accesslist.v1.AccessListService/ListAllAccessListMembers',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersRequest,
    responseType: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersResponse,
    requestSerialize: serialize_teleport_accesslist_v1_ListAllAccessListMembersRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_ListAllAccessListMembersRequest,
    responseSerialize: serialize_teleport_accesslist_v1_ListAllAccessListMembersResponse,
    responseDeserialize: deserialize_teleport_accesslist_v1_ListAllAccessListMembersResponse,
  },
  // GetAccessListMember returns the specified access list member resource.
getAccessListMember: {
    path: '/teleport.accesslist.v1.AccessListService/GetAccessListMember',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.GetAccessListMemberRequest,
    responseType: teleport_accesslist_v1_accesslist_pb.Member,
    requestSerialize: serialize_teleport_accesslist_v1_GetAccessListMemberRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_GetAccessListMemberRequest,
    responseSerialize: serialize_teleport_accesslist_v1_Member,
    responseDeserialize: deserialize_teleport_accesslist_v1_Member,
  },
  // UpsertAccessListMember creates or updates an access list member resource.
upsertAccessListMember: {
    path: '/teleport.accesslist.v1.AccessListService/UpsertAccessListMember',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListMemberRequest,
    responseType: teleport_accesslist_v1_accesslist_pb.Member,
    requestSerialize: serialize_teleport_accesslist_v1_UpsertAccessListMemberRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_UpsertAccessListMemberRequest,
    responseSerialize: serialize_teleport_accesslist_v1_Member,
    responseDeserialize: deserialize_teleport_accesslist_v1_Member,
  },
  // DeleteAccessListMember hard deletes the specified access list member resource.
deleteAccessListMember: {
    path: '/teleport.accesslist.v1.AccessListService/DeleteAccessListMember',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListMemberRequest,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_teleport_accesslist_v1_DeleteAccessListMemberRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_DeleteAccessListMemberRequest,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
  // DeleteAllAccessListMembers hard deletes all access list members for an access list.
deleteAllAccessListMembersForAccessList: {
    path: '/teleport.accesslist.v1.AccessListService/DeleteAllAccessListMembersForAccessList',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersForAccessListRequest,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_teleport_accesslist_v1_DeleteAllAccessListMembersForAccessListRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_DeleteAllAccessListMembersForAccessListRequest,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
  // DeleteAllAccessListMembers hard deletes all access list members for an access list.
deleteAllAccessListMembers: {
    path: '/teleport.accesslist.v1.AccessListService/DeleteAllAccessListMembers',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersRequest,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_teleport_accesslist_v1_DeleteAllAccessListMembersRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_DeleteAllAccessListMembersRequest,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
  // UpsertAccessListWithMembers creates or updates an access list with members.
upsertAccessListWithMembers: {
    path: '/teleport.accesslist.v1.AccessListService/UpsertAccessListWithMembers',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersRequest,
    responseType: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersResponse,
    requestSerialize: serialize_teleport_accesslist_v1_UpsertAccessListWithMembersRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_UpsertAccessListWithMembersRequest,
    responseSerialize: serialize_teleport_accesslist_v1_UpsertAccessListWithMembersResponse,
    responseDeserialize: deserialize_teleport_accesslist_v1_UpsertAccessListWithMembersResponse,
  },
  // ListAccessListReviews will list access list reviews for a particular access list.
listAccessListReviews: {
    path: '/teleport.accesslist.v1.AccessListService/ListAccessListReviews',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsRequest,
    responseType: teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsResponse,
    requestSerialize: serialize_teleport_accesslist_v1_ListAccessListReviewsRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_ListAccessListReviewsRequest,
    responseSerialize: serialize_teleport_accesslist_v1_ListAccessListReviewsResponse,
    responseDeserialize: deserialize_teleport_accesslist_v1_ListAccessListReviewsResponse,
  },
  // ListAllAccessListReviews will list access list reviews for all access lists.
listAllAccessListReviews: {
    path: '/teleport.accesslist.v1.AccessListService/ListAllAccessListReviews',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsRequest,
    responseType: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsResponse,
    requestSerialize: serialize_teleport_accesslist_v1_ListAllAccessListReviewsRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_ListAllAccessListReviewsRequest,
    responseSerialize: serialize_teleport_accesslist_v1_ListAllAccessListReviewsResponse,
    responseDeserialize: deserialize_teleport_accesslist_v1_ListAllAccessListReviewsResponse,
  },
  // CreateAccessListReview will create a new review for an access list. It will also modify the original access list
// and its members depending on the details of the review.
createAccessListReview: {
    path: '/teleport.accesslist.v1.AccessListService/CreateAccessListReview',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewRequest,
    responseType: teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewResponse,
    requestSerialize: serialize_teleport_accesslist_v1_CreateAccessListReviewRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_CreateAccessListReviewRequest,
    responseSerialize: serialize_teleport_accesslist_v1_CreateAccessListReviewResponse,
    responseDeserialize: deserialize_teleport_accesslist_v1_CreateAccessListReviewResponse,
  },
  // DeleteAccessListReview will delete an access list review from the backend.
deleteAccessListReview: {
    path: '/teleport.accesslist.v1.AccessListService/DeleteAccessListReview',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListReviewRequest,
    responseType: google_protobuf_empty_pb.Empty,
    requestSerialize: serialize_teleport_accesslist_v1_DeleteAccessListReviewRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_DeleteAccessListReviewRequest,
    responseSerialize: serialize_google_protobuf_Empty,
    responseDeserialize: deserialize_google_protobuf_Empty,
  },
  // AccessRequestPromote promotes an access request to an access list.
accessRequestPromote: {
    path: '/teleport.accesslist.v1.AccessListService/AccessRequestPromote',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteRequest,
    responseType: teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteResponse,
    requestSerialize: serialize_teleport_accesslist_v1_AccessRequestPromoteRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_AccessRequestPromoteRequest,
    responseSerialize: serialize_teleport_accesslist_v1_AccessRequestPromoteResponse,
    responseDeserialize: deserialize_teleport_accesslist_v1_AccessRequestPromoteResponse,
  },
  // GetSuggestedAccessLists returns suggested access lists for an access request.
getSuggestedAccessLists: {
    path: '/teleport.accesslist.v1.AccessListService/GetSuggestedAccessLists',
    requestStream: false,
    responseStream: false,
    requestType: teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsRequest,
    responseType: teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsResponse,
    requestSerialize: serialize_teleport_accesslist_v1_GetSuggestedAccessListsRequest,
    requestDeserialize: deserialize_teleport_accesslist_v1_GetSuggestedAccessListsRequest,
    responseSerialize: serialize_teleport_accesslist_v1_GetSuggestedAccessListsResponse,
    responseDeserialize: deserialize_teleport_accesslist_v1_GetSuggestedAccessListsResponse,
  },
};

exports.AccessListServiceClient = grpc.makeGenericClientConstructor(AccessListServiceService);
