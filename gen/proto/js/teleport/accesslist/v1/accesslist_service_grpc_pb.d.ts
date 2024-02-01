// package: teleport.accesslist.v1
// file: teleport/accesslist/v1/accesslist_service.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as teleport_accesslist_v1_accesslist_service_pb from "../../../teleport/accesslist/v1/accesslist_service_pb";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";
import * as teleport_accesslist_v1_accesslist_pb from "../../../teleport/accesslist/v1/accesslist_pb";
import * as teleport_legacy_types_types_pb from "../../../teleport/legacy/types/types_pb";

interface IAccessListServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    getAccessLists: IAccessListServiceService_IGetAccessLists;
    listAccessLists: IAccessListServiceService_IListAccessLists;
    getAccessList: IAccessListServiceService_IGetAccessList;
    upsertAccessList: IAccessListServiceService_IUpsertAccessList;
    deleteAccessList: IAccessListServiceService_IDeleteAccessList;
    deleteAllAccessLists: IAccessListServiceService_IDeleteAllAccessLists;
    getAccessListsToReview: IAccessListServiceService_IGetAccessListsToReview;
    listAccessListMembers: IAccessListServiceService_IListAccessListMembers;
    listAllAccessListMembers: IAccessListServiceService_IListAllAccessListMembers;
    getAccessListMember: IAccessListServiceService_IGetAccessListMember;
    upsertAccessListMember: IAccessListServiceService_IUpsertAccessListMember;
    deleteAccessListMember: IAccessListServiceService_IDeleteAccessListMember;
    deleteAllAccessListMembersForAccessList: IAccessListServiceService_IDeleteAllAccessListMembersForAccessList;
    deleteAllAccessListMembers: IAccessListServiceService_IDeleteAllAccessListMembers;
    upsertAccessListWithMembers: IAccessListServiceService_IUpsertAccessListWithMembers;
    listAccessListReviews: IAccessListServiceService_IListAccessListReviews;
    listAllAccessListReviews: IAccessListServiceService_IListAllAccessListReviews;
    createAccessListReview: IAccessListServiceService_ICreateAccessListReview;
    deleteAccessListReview: IAccessListServiceService_IDeleteAccessListReview;
    accessRequestPromote: IAccessListServiceService_IAccessRequestPromote;
    getSuggestedAccessLists: IAccessListServiceService_IGetSuggestedAccessLists;
}

interface IAccessListServiceService_IGetAccessLists extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.GetAccessListsRequest, teleport_accesslist_v1_accesslist_service_pb.GetAccessListsResponse> {
    path: "/teleport.accesslist.v1.AccessListService/GetAccessLists";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.GetAccessListsRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.GetAccessListsRequest>;
    responseSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.GetAccessListsResponse>;
    responseDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.GetAccessListsResponse>;
}
interface IAccessListServiceService_IListAccessLists extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.ListAccessListsRequest, teleport_accesslist_v1_accesslist_service_pb.ListAccessListsResponse> {
    path: "/teleport.accesslist.v1.AccessListService/ListAccessLists";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.ListAccessListsRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.ListAccessListsRequest>;
    responseSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.ListAccessListsResponse>;
    responseDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.ListAccessListsResponse>;
}
interface IAccessListServiceService_IGetAccessList extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.GetAccessListRequest, teleport_accesslist_v1_accesslist_pb.AccessList> {
    path: "/teleport.accesslist.v1.AccessListService/GetAccessList";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.GetAccessListRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.GetAccessListRequest>;
    responseSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_pb.AccessList>;
    responseDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_pb.AccessList>;
}
interface IAccessListServiceService_IUpsertAccessList extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListRequest, teleport_accesslist_v1_accesslist_pb.AccessList> {
    path: "/teleport.accesslist.v1.AccessListService/UpsertAccessList";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListRequest>;
    responseSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_pb.AccessList>;
    responseDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_pb.AccessList>;
}
interface IAccessListServiceService_IDeleteAccessList extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListRequest, google_protobuf_empty_pb.Empty> {
    path: "/teleport.accesslist.v1.AccessListService/DeleteAccessList";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListRequest>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}
interface IAccessListServiceService_IDeleteAllAccessLists extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListsRequest, google_protobuf_empty_pb.Empty> {
    path: "/teleport.accesslist.v1.AccessListService/DeleteAllAccessLists";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListsRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListsRequest>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}
interface IAccessListServiceService_IGetAccessListsToReview extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewRequest, teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewResponse> {
    path: "/teleport.accesslist.v1.AccessListService/GetAccessListsToReview";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewRequest>;
    responseSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewResponse>;
    responseDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewResponse>;
}
interface IAccessListServiceService_IListAccessListMembers extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersRequest, teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersResponse> {
    path: "/teleport.accesslist.v1.AccessListService/ListAccessListMembers";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersRequest>;
    responseSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersResponse>;
    responseDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersResponse>;
}
interface IAccessListServiceService_IListAllAccessListMembers extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersRequest, teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersResponse> {
    path: "/teleport.accesslist.v1.AccessListService/ListAllAccessListMembers";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersRequest>;
    responseSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersResponse>;
    responseDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersResponse>;
}
interface IAccessListServiceService_IGetAccessListMember extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.GetAccessListMemberRequest, teleport_accesslist_v1_accesslist_pb.Member> {
    path: "/teleport.accesslist.v1.AccessListService/GetAccessListMember";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.GetAccessListMemberRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.GetAccessListMemberRequest>;
    responseSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_pb.Member>;
    responseDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_pb.Member>;
}
interface IAccessListServiceService_IUpsertAccessListMember extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListMemberRequest, teleport_accesslist_v1_accesslist_pb.Member> {
    path: "/teleport.accesslist.v1.AccessListService/UpsertAccessListMember";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListMemberRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListMemberRequest>;
    responseSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_pb.Member>;
    responseDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_pb.Member>;
}
interface IAccessListServiceService_IDeleteAccessListMember extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListMemberRequest, google_protobuf_empty_pb.Empty> {
    path: "/teleport.accesslist.v1.AccessListService/DeleteAccessListMember";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListMemberRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListMemberRequest>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}
interface IAccessListServiceService_IDeleteAllAccessListMembersForAccessList extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersForAccessListRequest, google_protobuf_empty_pb.Empty> {
    path: "/teleport.accesslist.v1.AccessListService/DeleteAllAccessListMembersForAccessList";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersForAccessListRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersForAccessListRequest>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}
interface IAccessListServiceService_IDeleteAllAccessListMembers extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersRequest, google_protobuf_empty_pb.Empty> {
    path: "/teleport.accesslist.v1.AccessListService/DeleteAllAccessListMembers";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersRequest>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}
interface IAccessListServiceService_IUpsertAccessListWithMembers extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersRequest, teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersResponse> {
    path: "/teleport.accesslist.v1.AccessListService/UpsertAccessListWithMembers";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersRequest>;
    responseSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersResponse>;
    responseDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersResponse>;
}
interface IAccessListServiceService_IListAccessListReviews extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsRequest, teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsResponse> {
    path: "/teleport.accesslist.v1.AccessListService/ListAccessListReviews";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsRequest>;
    responseSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsResponse>;
    responseDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsResponse>;
}
interface IAccessListServiceService_IListAllAccessListReviews extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsRequest, teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsResponse> {
    path: "/teleport.accesslist.v1.AccessListService/ListAllAccessListReviews";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsRequest>;
    responseSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsResponse>;
    responseDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsResponse>;
}
interface IAccessListServiceService_ICreateAccessListReview extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewRequest, teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewResponse> {
    path: "/teleport.accesslist.v1.AccessListService/CreateAccessListReview";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewRequest>;
    responseSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewResponse>;
    responseDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewResponse>;
}
interface IAccessListServiceService_IDeleteAccessListReview extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListReviewRequest, google_protobuf_empty_pb.Empty> {
    path: "/teleport.accesslist.v1.AccessListService/DeleteAccessListReview";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListReviewRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListReviewRequest>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}
interface IAccessListServiceService_IAccessRequestPromote extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteRequest, teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteResponse> {
    path: "/teleport.accesslist.v1.AccessListService/AccessRequestPromote";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteRequest>;
    responseSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteResponse>;
    responseDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteResponse>;
}
interface IAccessListServiceService_IGetSuggestedAccessLists extends grpc.MethodDefinition<teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsRequest, teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsResponse> {
    path: "/teleport.accesslist.v1.AccessListService/GetSuggestedAccessLists";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsRequest>;
    requestDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsRequest>;
    responseSerialize: grpc.serialize<teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsResponse>;
    responseDeserialize: grpc.deserialize<teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsResponse>;
}

export const AccessListServiceService: IAccessListServiceService;

export interface IAccessListServiceServer {
    getAccessLists: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.GetAccessListsRequest, teleport_accesslist_v1_accesslist_service_pb.GetAccessListsResponse>;
    listAccessLists: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.ListAccessListsRequest, teleport_accesslist_v1_accesslist_service_pb.ListAccessListsResponse>;
    getAccessList: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.GetAccessListRequest, teleport_accesslist_v1_accesslist_pb.AccessList>;
    upsertAccessList: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListRequest, teleport_accesslist_v1_accesslist_pb.AccessList>;
    deleteAccessList: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListRequest, google_protobuf_empty_pb.Empty>;
    deleteAllAccessLists: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListsRequest, google_protobuf_empty_pb.Empty>;
    getAccessListsToReview: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewRequest, teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewResponse>;
    listAccessListMembers: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersRequest, teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersResponse>;
    listAllAccessListMembers: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersRequest, teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersResponse>;
    getAccessListMember: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.GetAccessListMemberRequest, teleport_accesslist_v1_accesslist_pb.Member>;
    upsertAccessListMember: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListMemberRequest, teleport_accesslist_v1_accesslist_pb.Member>;
    deleteAccessListMember: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListMemberRequest, google_protobuf_empty_pb.Empty>;
    deleteAllAccessListMembersForAccessList: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersForAccessListRequest, google_protobuf_empty_pb.Empty>;
    deleteAllAccessListMembers: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersRequest, google_protobuf_empty_pb.Empty>;
    upsertAccessListWithMembers: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersRequest, teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersResponse>;
    listAccessListReviews: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsRequest, teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsResponse>;
    listAllAccessListReviews: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsRequest, teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsResponse>;
    createAccessListReview: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewRequest, teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewResponse>;
    deleteAccessListReview: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListReviewRequest, google_protobuf_empty_pb.Empty>;
    accessRequestPromote: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteRequest, teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteResponse>;
    getSuggestedAccessLists: grpc.handleUnaryCall<teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsRequest, teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsResponse>;
}

export interface IAccessListServiceClient {
    getAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsResponse) => void): grpc.ClientUnaryCall;
    getAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsResponse) => void): grpc.ClientUnaryCall;
    getAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsResponse) => void): grpc.ClientUnaryCall;
    listAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListsRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListsResponse) => void): grpc.ClientUnaryCall;
    listAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListsResponse) => void): grpc.ClientUnaryCall;
    listAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListsResponse) => void): grpc.ClientUnaryCall;
    getAccessList(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.AccessList) => void): grpc.ClientUnaryCall;
    getAccessList(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.AccessList) => void): grpc.ClientUnaryCall;
    getAccessList(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.AccessList) => void): grpc.ClientUnaryCall;
    upsertAccessList(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.AccessList) => void): grpc.ClientUnaryCall;
    upsertAccessList(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.AccessList) => void): grpc.ClientUnaryCall;
    upsertAccessList(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.AccessList) => void): grpc.ClientUnaryCall;
    deleteAccessList(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    deleteAccessList(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    deleteAccessList(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    deleteAllAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListsRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    deleteAllAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    deleteAllAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    getAccessListsToReview(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewResponse) => void): grpc.ClientUnaryCall;
    getAccessListsToReview(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewResponse) => void): grpc.ClientUnaryCall;
    getAccessListsToReview(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewResponse) => void): grpc.ClientUnaryCall;
    listAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersResponse) => void): grpc.ClientUnaryCall;
    listAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersResponse) => void): grpc.ClientUnaryCall;
    listAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersResponse) => void): grpc.ClientUnaryCall;
    listAllAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersResponse) => void): grpc.ClientUnaryCall;
    listAllAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersResponse) => void): grpc.ClientUnaryCall;
    listAllAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersResponse) => void): grpc.ClientUnaryCall;
    getAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListMemberRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.Member) => void): grpc.ClientUnaryCall;
    getAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListMemberRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.Member) => void): grpc.ClientUnaryCall;
    getAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListMemberRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.Member) => void): grpc.ClientUnaryCall;
    upsertAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListMemberRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.Member) => void): grpc.ClientUnaryCall;
    upsertAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListMemberRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.Member) => void): grpc.ClientUnaryCall;
    upsertAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListMemberRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.Member) => void): grpc.ClientUnaryCall;
    deleteAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListMemberRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    deleteAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListMemberRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    deleteAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListMemberRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    deleteAllAccessListMembersForAccessList(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersForAccessListRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    deleteAllAccessListMembersForAccessList(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersForAccessListRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    deleteAllAccessListMembersForAccessList(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersForAccessListRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    deleteAllAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    deleteAllAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    deleteAllAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    upsertAccessListWithMembers(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersResponse) => void): grpc.ClientUnaryCall;
    upsertAccessListWithMembers(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersResponse) => void): grpc.ClientUnaryCall;
    upsertAccessListWithMembers(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersResponse) => void): grpc.ClientUnaryCall;
    listAccessListReviews(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsResponse) => void): grpc.ClientUnaryCall;
    listAccessListReviews(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsResponse) => void): grpc.ClientUnaryCall;
    listAccessListReviews(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsResponse) => void): grpc.ClientUnaryCall;
    listAllAccessListReviews(request: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsResponse) => void): grpc.ClientUnaryCall;
    listAllAccessListReviews(request: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsResponse) => void): grpc.ClientUnaryCall;
    listAllAccessListReviews(request: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsResponse) => void): grpc.ClientUnaryCall;
    createAccessListReview(request: teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewResponse) => void): grpc.ClientUnaryCall;
    createAccessListReview(request: teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewResponse) => void): grpc.ClientUnaryCall;
    createAccessListReview(request: teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewResponse) => void): grpc.ClientUnaryCall;
    deleteAccessListReview(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListReviewRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    deleteAccessListReview(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListReviewRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    deleteAccessListReview(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListReviewRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    accessRequestPromote(request: teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteResponse) => void): grpc.ClientUnaryCall;
    accessRequestPromote(request: teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteResponse) => void): grpc.ClientUnaryCall;
    accessRequestPromote(request: teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteResponse) => void): grpc.ClientUnaryCall;
    getSuggestedAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsResponse) => void): grpc.ClientUnaryCall;
    getSuggestedAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsResponse) => void): grpc.ClientUnaryCall;
    getSuggestedAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsResponse) => void): grpc.ClientUnaryCall;
}

export class AccessListServiceClient extends grpc.Client implements IAccessListServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public getAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsResponse) => void): grpc.ClientUnaryCall;
    public getAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsResponse) => void): grpc.ClientUnaryCall;
    public getAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsResponse) => void): grpc.ClientUnaryCall;
    public listAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListsRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListsResponse) => void): grpc.ClientUnaryCall;
    public listAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListsResponse) => void): grpc.ClientUnaryCall;
    public listAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListsResponse) => void): grpc.ClientUnaryCall;
    public getAccessList(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.AccessList) => void): grpc.ClientUnaryCall;
    public getAccessList(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.AccessList) => void): grpc.ClientUnaryCall;
    public getAccessList(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.AccessList) => void): grpc.ClientUnaryCall;
    public upsertAccessList(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.AccessList) => void): grpc.ClientUnaryCall;
    public upsertAccessList(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.AccessList) => void): grpc.ClientUnaryCall;
    public upsertAccessList(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.AccessList) => void): grpc.ClientUnaryCall;
    public deleteAccessList(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public deleteAccessList(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public deleteAccessList(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public deleteAllAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListsRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public deleteAllAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public deleteAllAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public getAccessListsToReview(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewResponse) => void): grpc.ClientUnaryCall;
    public getAccessListsToReview(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewResponse) => void): grpc.ClientUnaryCall;
    public getAccessListsToReview(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetAccessListsToReviewResponse) => void): grpc.ClientUnaryCall;
    public listAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersResponse) => void): grpc.ClientUnaryCall;
    public listAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersResponse) => void): grpc.ClientUnaryCall;
    public listAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListMembersResponse) => void): grpc.ClientUnaryCall;
    public listAllAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersResponse) => void): grpc.ClientUnaryCall;
    public listAllAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersResponse) => void): grpc.ClientUnaryCall;
    public listAllAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListMembersResponse) => void): grpc.ClientUnaryCall;
    public getAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListMemberRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.Member) => void): grpc.ClientUnaryCall;
    public getAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListMemberRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.Member) => void): grpc.ClientUnaryCall;
    public getAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.GetAccessListMemberRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.Member) => void): grpc.ClientUnaryCall;
    public upsertAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListMemberRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.Member) => void): grpc.ClientUnaryCall;
    public upsertAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListMemberRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.Member) => void): grpc.ClientUnaryCall;
    public upsertAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListMemberRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_pb.Member) => void): grpc.ClientUnaryCall;
    public deleteAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListMemberRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public deleteAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListMemberRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public deleteAccessListMember(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListMemberRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public deleteAllAccessListMembersForAccessList(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersForAccessListRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public deleteAllAccessListMembersForAccessList(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersForAccessListRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public deleteAllAccessListMembersForAccessList(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersForAccessListRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public deleteAllAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public deleteAllAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public deleteAllAccessListMembers(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAllAccessListMembersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public upsertAccessListWithMembers(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersResponse) => void): grpc.ClientUnaryCall;
    public upsertAccessListWithMembers(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersResponse) => void): grpc.ClientUnaryCall;
    public upsertAccessListWithMembers(request: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.UpsertAccessListWithMembersResponse) => void): grpc.ClientUnaryCall;
    public listAccessListReviews(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsResponse) => void): grpc.ClientUnaryCall;
    public listAccessListReviews(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsResponse) => void): grpc.ClientUnaryCall;
    public listAccessListReviews(request: teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAccessListReviewsResponse) => void): grpc.ClientUnaryCall;
    public listAllAccessListReviews(request: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsResponse) => void): grpc.ClientUnaryCall;
    public listAllAccessListReviews(request: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsResponse) => void): grpc.ClientUnaryCall;
    public listAllAccessListReviews(request: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.ListAllAccessListReviewsResponse) => void): grpc.ClientUnaryCall;
    public createAccessListReview(request: teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewResponse) => void): grpc.ClientUnaryCall;
    public createAccessListReview(request: teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewResponse) => void): grpc.ClientUnaryCall;
    public createAccessListReview(request: teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.CreateAccessListReviewResponse) => void): grpc.ClientUnaryCall;
    public deleteAccessListReview(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListReviewRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public deleteAccessListReview(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListReviewRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public deleteAccessListReview(request: teleport_accesslist_v1_accesslist_service_pb.DeleteAccessListReviewRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public accessRequestPromote(request: teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteResponse) => void): grpc.ClientUnaryCall;
    public accessRequestPromote(request: teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteResponse) => void): grpc.ClientUnaryCall;
    public accessRequestPromote(request: teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.AccessRequestPromoteResponse) => void): grpc.ClientUnaryCall;
    public getSuggestedAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsRequest, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsResponse) => void): grpc.ClientUnaryCall;
    public getSuggestedAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsResponse) => void): grpc.ClientUnaryCall;
    public getSuggestedAccessLists(request: teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_accesslist_v1_accesslist_service_pb.GetSuggestedAccessListsResponse) => void): grpc.ClientUnaryCall;
}
