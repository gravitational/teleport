// package: teleport.accesslist.v1
// file: teleport/accesslist/v1/accesslist_service.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";
import * as teleport_accesslist_v1_accesslist_pb from "../../../teleport/accesslist/v1/accesslist_pb";
import * as teleport_legacy_types_types_pb from "../../../teleport/legacy/types/types_pb";

export class GetAccessListsRequest extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetAccessListsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetAccessListsRequest): GetAccessListsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetAccessListsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetAccessListsRequest;
    static deserializeBinaryFromReader(message: GetAccessListsRequest, reader: jspb.BinaryReader): GetAccessListsRequest;
}

export namespace GetAccessListsRequest {
    export type AsObject = {
    }
}

export class GetAccessListsResponse extends jspb.Message { 
    clearAccessListsList(): void;
    getAccessListsList(): Array<teleport_accesslist_v1_accesslist_pb.AccessList>;
    setAccessListsList(value: Array<teleport_accesslist_v1_accesslist_pb.AccessList>): GetAccessListsResponse;
    addAccessLists(value?: teleport_accesslist_v1_accesslist_pb.AccessList, index?: number): teleport_accesslist_v1_accesslist_pb.AccessList;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetAccessListsResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetAccessListsResponse): GetAccessListsResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetAccessListsResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetAccessListsResponse;
    static deserializeBinaryFromReader(message: GetAccessListsResponse, reader: jspb.BinaryReader): GetAccessListsResponse;
}

export namespace GetAccessListsResponse {
    export type AsObject = {
        accessListsList: Array<teleport_accesslist_v1_accesslist_pb.AccessList.AsObject>,
    }
}

export class ListAccessListsRequest extends jspb.Message { 
    getPageSize(): number;
    setPageSize(value: number): ListAccessListsRequest;

    getNextToken(): string;
    setNextToken(value: string): ListAccessListsRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListAccessListsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ListAccessListsRequest): ListAccessListsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListAccessListsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListAccessListsRequest;
    static deserializeBinaryFromReader(message: ListAccessListsRequest, reader: jspb.BinaryReader): ListAccessListsRequest;
}

export namespace ListAccessListsRequest {
    export type AsObject = {
        pageSize: number,
        nextToken: string,
    }
}

export class ListAccessListsResponse extends jspb.Message { 
    clearAccessListsList(): void;
    getAccessListsList(): Array<teleport_accesslist_v1_accesslist_pb.AccessList>;
    setAccessListsList(value: Array<teleport_accesslist_v1_accesslist_pb.AccessList>): ListAccessListsResponse;
    addAccessLists(value?: teleport_accesslist_v1_accesslist_pb.AccessList, index?: number): teleport_accesslist_v1_accesslist_pb.AccessList;

    getNextToken(): string;
    setNextToken(value: string): ListAccessListsResponse;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListAccessListsResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ListAccessListsResponse): ListAccessListsResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListAccessListsResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListAccessListsResponse;
    static deserializeBinaryFromReader(message: ListAccessListsResponse, reader: jspb.BinaryReader): ListAccessListsResponse;
}

export namespace ListAccessListsResponse {
    export type AsObject = {
        accessListsList: Array<teleport_accesslist_v1_accesslist_pb.AccessList.AsObject>,
        nextToken: string,
    }
}

export class GetAccessListRequest extends jspb.Message { 
    getName(): string;
    setName(value: string): GetAccessListRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetAccessListRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetAccessListRequest): GetAccessListRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetAccessListRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetAccessListRequest;
    static deserializeBinaryFromReader(message: GetAccessListRequest, reader: jspb.BinaryReader): GetAccessListRequest;
}

export namespace GetAccessListRequest {
    export type AsObject = {
        name: string,
    }
}

export class UpsertAccessListRequest extends jspb.Message { 

    hasAccessList(): boolean;
    clearAccessList(): void;
    getAccessList(): teleport_accesslist_v1_accesslist_pb.AccessList | undefined;
    setAccessList(value?: teleport_accesslist_v1_accesslist_pb.AccessList): UpsertAccessListRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UpsertAccessListRequest.AsObject;
    static toObject(includeInstance: boolean, msg: UpsertAccessListRequest): UpsertAccessListRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UpsertAccessListRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UpsertAccessListRequest;
    static deserializeBinaryFromReader(message: UpsertAccessListRequest, reader: jspb.BinaryReader): UpsertAccessListRequest;
}

export namespace UpsertAccessListRequest {
    export type AsObject = {
        accessList?: teleport_accesslist_v1_accesslist_pb.AccessList.AsObject,
    }
}

export class DeleteAccessListRequest extends jspb.Message { 
    getName(): string;
    setName(value: string): DeleteAccessListRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DeleteAccessListRequest.AsObject;
    static toObject(includeInstance: boolean, msg: DeleteAccessListRequest): DeleteAccessListRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DeleteAccessListRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DeleteAccessListRequest;
    static deserializeBinaryFromReader(message: DeleteAccessListRequest, reader: jspb.BinaryReader): DeleteAccessListRequest;
}

export namespace DeleteAccessListRequest {
    export type AsObject = {
        name: string,
    }
}

export class DeleteAllAccessListsRequest extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DeleteAllAccessListsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: DeleteAllAccessListsRequest): DeleteAllAccessListsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DeleteAllAccessListsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DeleteAllAccessListsRequest;
    static deserializeBinaryFromReader(message: DeleteAllAccessListsRequest, reader: jspb.BinaryReader): DeleteAllAccessListsRequest;
}

export namespace DeleteAllAccessListsRequest {
    export type AsObject = {
    }
}

export class GetAccessListsToReviewRequest extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetAccessListsToReviewRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetAccessListsToReviewRequest): GetAccessListsToReviewRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetAccessListsToReviewRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetAccessListsToReviewRequest;
    static deserializeBinaryFromReader(message: GetAccessListsToReviewRequest, reader: jspb.BinaryReader): GetAccessListsToReviewRequest;
}

export namespace GetAccessListsToReviewRequest {
    export type AsObject = {
    }
}

export class GetAccessListsToReviewResponse extends jspb.Message { 
    clearAccessListsList(): void;
    getAccessListsList(): Array<teleport_accesslist_v1_accesslist_pb.AccessList>;
    setAccessListsList(value: Array<teleport_accesslist_v1_accesslist_pb.AccessList>): GetAccessListsToReviewResponse;
    addAccessLists(value?: teleport_accesslist_v1_accesslist_pb.AccessList, index?: number): teleport_accesslist_v1_accesslist_pb.AccessList;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetAccessListsToReviewResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetAccessListsToReviewResponse): GetAccessListsToReviewResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetAccessListsToReviewResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetAccessListsToReviewResponse;
    static deserializeBinaryFromReader(message: GetAccessListsToReviewResponse, reader: jspb.BinaryReader): GetAccessListsToReviewResponse;
}

export namespace GetAccessListsToReviewResponse {
    export type AsObject = {
        accessListsList: Array<teleport_accesslist_v1_accesslist_pb.AccessList.AsObject>,
    }
}

export class ListAccessListMembersRequest extends jspb.Message { 
    getPageSize(): number;
    setPageSize(value: number): ListAccessListMembersRequest;

    getPageToken(): string;
    setPageToken(value: string): ListAccessListMembersRequest;

    getAccessList(): string;
    setAccessList(value: string): ListAccessListMembersRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListAccessListMembersRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ListAccessListMembersRequest): ListAccessListMembersRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListAccessListMembersRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListAccessListMembersRequest;
    static deserializeBinaryFromReader(message: ListAccessListMembersRequest, reader: jspb.BinaryReader): ListAccessListMembersRequest;
}

export namespace ListAccessListMembersRequest {
    export type AsObject = {
        pageSize: number,
        pageToken: string,
        accessList: string,
    }
}

export class ListAccessListMembersResponse extends jspb.Message { 
    clearMembersList(): void;
    getMembersList(): Array<teleport_accesslist_v1_accesslist_pb.Member>;
    setMembersList(value: Array<teleport_accesslist_v1_accesslist_pb.Member>): ListAccessListMembersResponse;
    addMembers(value?: teleport_accesslist_v1_accesslist_pb.Member, index?: number): teleport_accesslist_v1_accesslist_pb.Member;

    getNextPageToken(): string;
    setNextPageToken(value: string): ListAccessListMembersResponse;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListAccessListMembersResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ListAccessListMembersResponse): ListAccessListMembersResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListAccessListMembersResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListAccessListMembersResponse;
    static deserializeBinaryFromReader(message: ListAccessListMembersResponse, reader: jspb.BinaryReader): ListAccessListMembersResponse;
}

export namespace ListAccessListMembersResponse {
    export type AsObject = {
        membersList: Array<teleport_accesslist_v1_accesslist_pb.Member.AsObject>,
        nextPageToken: string,
    }
}

export class ListAllAccessListMembersRequest extends jspb.Message { 
    getPageSize(): number;
    setPageSize(value: number): ListAllAccessListMembersRequest;

    getPageToken(): string;
    setPageToken(value: string): ListAllAccessListMembersRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListAllAccessListMembersRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ListAllAccessListMembersRequest): ListAllAccessListMembersRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListAllAccessListMembersRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListAllAccessListMembersRequest;
    static deserializeBinaryFromReader(message: ListAllAccessListMembersRequest, reader: jspb.BinaryReader): ListAllAccessListMembersRequest;
}

export namespace ListAllAccessListMembersRequest {
    export type AsObject = {
        pageSize: number,
        pageToken: string,
    }
}

export class ListAllAccessListMembersResponse extends jspb.Message { 
    clearMembersList(): void;
    getMembersList(): Array<teleport_accesslist_v1_accesslist_pb.Member>;
    setMembersList(value: Array<teleport_accesslist_v1_accesslist_pb.Member>): ListAllAccessListMembersResponse;
    addMembers(value?: teleport_accesslist_v1_accesslist_pb.Member, index?: number): teleport_accesslist_v1_accesslist_pb.Member;

    getNextPageToken(): string;
    setNextPageToken(value: string): ListAllAccessListMembersResponse;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListAllAccessListMembersResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ListAllAccessListMembersResponse): ListAllAccessListMembersResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListAllAccessListMembersResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListAllAccessListMembersResponse;
    static deserializeBinaryFromReader(message: ListAllAccessListMembersResponse, reader: jspb.BinaryReader): ListAllAccessListMembersResponse;
}

export namespace ListAllAccessListMembersResponse {
    export type AsObject = {
        membersList: Array<teleport_accesslist_v1_accesslist_pb.Member.AsObject>,
        nextPageToken: string,
    }
}

export class UpsertAccessListWithMembersRequest extends jspb.Message { 

    hasAccessList(): boolean;
    clearAccessList(): void;
    getAccessList(): teleport_accesslist_v1_accesslist_pb.AccessList | undefined;
    setAccessList(value?: teleport_accesslist_v1_accesslist_pb.AccessList): UpsertAccessListWithMembersRequest;

    clearMembersList(): void;
    getMembersList(): Array<teleport_accesslist_v1_accesslist_pb.Member>;
    setMembersList(value: Array<teleport_accesslist_v1_accesslist_pb.Member>): UpsertAccessListWithMembersRequest;
    addMembers(value?: teleport_accesslist_v1_accesslist_pb.Member, index?: number): teleport_accesslist_v1_accesslist_pb.Member;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UpsertAccessListWithMembersRequest.AsObject;
    static toObject(includeInstance: boolean, msg: UpsertAccessListWithMembersRequest): UpsertAccessListWithMembersRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UpsertAccessListWithMembersRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UpsertAccessListWithMembersRequest;
    static deserializeBinaryFromReader(message: UpsertAccessListWithMembersRequest, reader: jspb.BinaryReader): UpsertAccessListWithMembersRequest;
}

export namespace UpsertAccessListWithMembersRequest {
    export type AsObject = {
        accessList?: teleport_accesslist_v1_accesslist_pb.AccessList.AsObject,
        membersList: Array<teleport_accesslist_v1_accesslist_pb.Member.AsObject>,
    }
}

export class UpsertAccessListWithMembersResponse extends jspb.Message { 

    hasAccessList(): boolean;
    clearAccessList(): void;
    getAccessList(): teleport_accesslist_v1_accesslist_pb.AccessList | undefined;
    setAccessList(value?: teleport_accesslist_v1_accesslist_pb.AccessList): UpsertAccessListWithMembersResponse;

    clearMembersList(): void;
    getMembersList(): Array<teleport_accesslist_v1_accesslist_pb.Member>;
    setMembersList(value: Array<teleport_accesslist_v1_accesslist_pb.Member>): UpsertAccessListWithMembersResponse;
    addMembers(value?: teleport_accesslist_v1_accesslist_pb.Member, index?: number): teleport_accesslist_v1_accesslist_pb.Member;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UpsertAccessListWithMembersResponse.AsObject;
    static toObject(includeInstance: boolean, msg: UpsertAccessListWithMembersResponse): UpsertAccessListWithMembersResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UpsertAccessListWithMembersResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UpsertAccessListWithMembersResponse;
    static deserializeBinaryFromReader(message: UpsertAccessListWithMembersResponse, reader: jspb.BinaryReader): UpsertAccessListWithMembersResponse;
}

export namespace UpsertAccessListWithMembersResponse {
    export type AsObject = {
        accessList?: teleport_accesslist_v1_accesslist_pb.AccessList.AsObject,
        membersList: Array<teleport_accesslist_v1_accesslist_pb.Member.AsObject>,
    }
}

export class GetAccessListMemberRequest extends jspb.Message { 
    getAccessList(): string;
    setAccessList(value: string): GetAccessListMemberRequest;

    getMemberName(): string;
    setMemberName(value: string): GetAccessListMemberRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetAccessListMemberRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetAccessListMemberRequest): GetAccessListMemberRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetAccessListMemberRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetAccessListMemberRequest;
    static deserializeBinaryFromReader(message: GetAccessListMemberRequest, reader: jspb.BinaryReader): GetAccessListMemberRequest;
}

export namespace GetAccessListMemberRequest {
    export type AsObject = {
        accessList: string,
        memberName: string,
    }
}

export class UpsertAccessListMemberRequest extends jspb.Message { 

    hasMember(): boolean;
    clearMember(): void;
    getMember(): teleport_accesslist_v1_accesslist_pb.Member | undefined;
    setMember(value?: teleport_accesslist_v1_accesslist_pb.Member): UpsertAccessListMemberRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UpsertAccessListMemberRequest.AsObject;
    static toObject(includeInstance: boolean, msg: UpsertAccessListMemberRequest): UpsertAccessListMemberRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UpsertAccessListMemberRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UpsertAccessListMemberRequest;
    static deserializeBinaryFromReader(message: UpsertAccessListMemberRequest, reader: jspb.BinaryReader): UpsertAccessListMemberRequest;
}

export namespace UpsertAccessListMemberRequest {
    export type AsObject = {
        member?: teleport_accesslist_v1_accesslist_pb.Member.AsObject,
    }
}

export class DeleteAccessListMemberRequest extends jspb.Message { 
    getAccessList(): string;
    setAccessList(value: string): DeleteAccessListMemberRequest;

    getMemberName(): string;
    setMemberName(value: string): DeleteAccessListMemberRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DeleteAccessListMemberRequest.AsObject;
    static toObject(includeInstance: boolean, msg: DeleteAccessListMemberRequest): DeleteAccessListMemberRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DeleteAccessListMemberRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DeleteAccessListMemberRequest;
    static deserializeBinaryFromReader(message: DeleteAccessListMemberRequest, reader: jspb.BinaryReader): DeleteAccessListMemberRequest;
}

export namespace DeleteAccessListMemberRequest {
    export type AsObject = {
        accessList: string,
        memberName: string,
    }
}

export class DeleteAllAccessListMembersForAccessListRequest extends jspb.Message { 
    getAccessList(): string;
    setAccessList(value: string): DeleteAllAccessListMembersForAccessListRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DeleteAllAccessListMembersForAccessListRequest.AsObject;
    static toObject(includeInstance: boolean, msg: DeleteAllAccessListMembersForAccessListRequest): DeleteAllAccessListMembersForAccessListRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DeleteAllAccessListMembersForAccessListRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DeleteAllAccessListMembersForAccessListRequest;
    static deserializeBinaryFromReader(message: DeleteAllAccessListMembersForAccessListRequest, reader: jspb.BinaryReader): DeleteAllAccessListMembersForAccessListRequest;
}

export namespace DeleteAllAccessListMembersForAccessListRequest {
    export type AsObject = {
        accessList: string,
    }
}

export class DeleteAllAccessListMembersRequest extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DeleteAllAccessListMembersRequest.AsObject;
    static toObject(includeInstance: boolean, msg: DeleteAllAccessListMembersRequest): DeleteAllAccessListMembersRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DeleteAllAccessListMembersRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DeleteAllAccessListMembersRequest;
    static deserializeBinaryFromReader(message: DeleteAllAccessListMembersRequest, reader: jspb.BinaryReader): DeleteAllAccessListMembersRequest;
}

export namespace DeleteAllAccessListMembersRequest {
    export type AsObject = {
    }
}

export class ListAccessListReviewsRequest extends jspb.Message { 
    getAccessList(): string;
    setAccessList(value: string): ListAccessListReviewsRequest;

    getPageSize(): number;
    setPageSize(value: number): ListAccessListReviewsRequest;

    getNextToken(): string;
    setNextToken(value: string): ListAccessListReviewsRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListAccessListReviewsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ListAccessListReviewsRequest): ListAccessListReviewsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListAccessListReviewsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListAccessListReviewsRequest;
    static deserializeBinaryFromReader(message: ListAccessListReviewsRequest, reader: jspb.BinaryReader): ListAccessListReviewsRequest;
}

export namespace ListAccessListReviewsRequest {
    export type AsObject = {
        accessList: string,
        pageSize: number,
        nextToken: string,
    }
}

export class ListAccessListReviewsResponse extends jspb.Message { 
    clearReviewsList(): void;
    getReviewsList(): Array<teleport_accesslist_v1_accesslist_pb.Review>;
    setReviewsList(value: Array<teleport_accesslist_v1_accesslist_pb.Review>): ListAccessListReviewsResponse;
    addReviews(value?: teleport_accesslist_v1_accesslist_pb.Review, index?: number): teleport_accesslist_v1_accesslist_pb.Review;

    getNextToken(): string;
    setNextToken(value: string): ListAccessListReviewsResponse;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListAccessListReviewsResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ListAccessListReviewsResponse): ListAccessListReviewsResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListAccessListReviewsResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListAccessListReviewsResponse;
    static deserializeBinaryFromReader(message: ListAccessListReviewsResponse, reader: jspb.BinaryReader): ListAccessListReviewsResponse;
}

export namespace ListAccessListReviewsResponse {
    export type AsObject = {
        reviewsList: Array<teleport_accesslist_v1_accesslist_pb.Review.AsObject>,
        nextToken: string,
    }
}

export class ListAllAccessListReviewsRequest extends jspb.Message { 
    getPageSize(): number;
    setPageSize(value: number): ListAllAccessListReviewsRequest;

    getNextToken(): string;
    setNextToken(value: string): ListAllAccessListReviewsRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListAllAccessListReviewsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ListAllAccessListReviewsRequest): ListAllAccessListReviewsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListAllAccessListReviewsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListAllAccessListReviewsRequest;
    static deserializeBinaryFromReader(message: ListAllAccessListReviewsRequest, reader: jspb.BinaryReader): ListAllAccessListReviewsRequest;
}

export namespace ListAllAccessListReviewsRequest {
    export type AsObject = {
        pageSize: number,
        nextToken: string,
    }
}

export class ListAllAccessListReviewsResponse extends jspb.Message { 
    clearReviewsList(): void;
    getReviewsList(): Array<teleport_accesslist_v1_accesslist_pb.Review>;
    setReviewsList(value: Array<teleport_accesslist_v1_accesslist_pb.Review>): ListAllAccessListReviewsResponse;
    addReviews(value?: teleport_accesslist_v1_accesslist_pb.Review, index?: number): teleport_accesslist_v1_accesslist_pb.Review;

    getNextToken(): string;
    setNextToken(value: string): ListAllAccessListReviewsResponse;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListAllAccessListReviewsResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ListAllAccessListReviewsResponse): ListAllAccessListReviewsResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListAllAccessListReviewsResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListAllAccessListReviewsResponse;
    static deserializeBinaryFromReader(message: ListAllAccessListReviewsResponse, reader: jspb.BinaryReader): ListAllAccessListReviewsResponse;
}

export namespace ListAllAccessListReviewsResponse {
    export type AsObject = {
        reviewsList: Array<teleport_accesslist_v1_accesslist_pb.Review.AsObject>,
        nextToken: string,
    }
}

export class CreateAccessListReviewRequest extends jspb.Message { 

    hasReview(): boolean;
    clearReview(): void;
    getReview(): teleport_accesslist_v1_accesslist_pb.Review | undefined;
    setReview(value?: teleport_accesslist_v1_accesslist_pb.Review): CreateAccessListReviewRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CreateAccessListReviewRequest.AsObject;
    static toObject(includeInstance: boolean, msg: CreateAccessListReviewRequest): CreateAccessListReviewRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CreateAccessListReviewRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CreateAccessListReviewRequest;
    static deserializeBinaryFromReader(message: CreateAccessListReviewRequest, reader: jspb.BinaryReader): CreateAccessListReviewRequest;
}

export namespace CreateAccessListReviewRequest {
    export type AsObject = {
        review?: teleport_accesslist_v1_accesslist_pb.Review.AsObject,
    }
}

export class CreateAccessListReviewResponse extends jspb.Message { 
    getReviewName(): string;
    setReviewName(value: string): CreateAccessListReviewResponse;


    hasNextAuditDate(): boolean;
    clearNextAuditDate(): void;
    getNextAuditDate(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setNextAuditDate(value?: google_protobuf_timestamp_pb.Timestamp): CreateAccessListReviewResponse;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CreateAccessListReviewResponse.AsObject;
    static toObject(includeInstance: boolean, msg: CreateAccessListReviewResponse): CreateAccessListReviewResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CreateAccessListReviewResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CreateAccessListReviewResponse;
    static deserializeBinaryFromReader(message: CreateAccessListReviewResponse, reader: jspb.BinaryReader): CreateAccessListReviewResponse;
}

export namespace CreateAccessListReviewResponse {
    export type AsObject = {
        reviewName: string,
        nextAuditDate?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    }
}

export class DeleteAccessListReviewRequest extends jspb.Message { 
    getReviewName(): string;
    setReviewName(value: string): DeleteAccessListReviewRequest;

    getAccessListName(): string;
    setAccessListName(value: string): DeleteAccessListReviewRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DeleteAccessListReviewRequest.AsObject;
    static toObject(includeInstance: boolean, msg: DeleteAccessListReviewRequest): DeleteAccessListReviewRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DeleteAccessListReviewRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DeleteAccessListReviewRequest;
    static deserializeBinaryFromReader(message: DeleteAccessListReviewRequest, reader: jspb.BinaryReader): DeleteAccessListReviewRequest;
}

export namespace DeleteAccessListReviewRequest {
    export type AsObject = {
        reviewName: string,
        accessListName: string,
    }
}

export class AccessRequestPromoteRequest extends jspb.Message { 
    getRequestId(): string;
    setRequestId(value: string): AccessRequestPromoteRequest;

    getAccessListName(): string;
    setAccessListName(value: string): AccessRequestPromoteRequest;

    getReason(): string;
    setReason(value: string): AccessRequestPromoteRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AccessRequestPromoteRequest.AsObject;
    static toObject(includeInstance: boolean, msg: AccessRequestPromoteRequest): AccessRequestPromoteRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AccessRequestPromoteRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AccessRequestPromoteRequest;
    static deserializeBinaryFromReader(message: AccessRequestPromoteRequest, reader: jspb.BinaryReader): AccessRequestPromoteRequest;
}

export namespace AccessRequestPromoteRequest {
    export type AsObject = {
        requestId: string,
        accessListName: string,
        reason: string,
    }
}

export class AccessRequestPromoteResponse extends jspb.Message { 

    hasAccessRequest(): boolean;
    clearAccessRequest(): void;
    getAccessRequest(): teleport_legacy_types_types_pb.AccessRequestV3 | undefined;
    setAccessRequest(value?: teleport_legacy_types_types_pb.AccessRequestV3): AccessRequestPromoteResponse;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AccessRequestPromoteResponse.AsObject;
    static toObject(includeInstance: boolean, msg: AccessRequestPromoteResponse): AccessRequestPromoteResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AccessRequestPromoteResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AccessRequestPromoteResponse;
    static deserializeBinaryFromReader(message: AccessRequestPromoteResponse, reader: jspb.BinaryReader): AccessRequestPromoteResponse;
}

export namespace AccessRequestPromoteResponse {
    export type AsObject = {
        accessRequest?: teleport_legacy_types_types_pb.AccessRequestV3.AsObject,
    }
}

export class GetSuggestedAccessListsRequest extends jspb.Message { 
    getAccessRequestId(): string;
    setAccessRequestId(value: string): GetSuggestedAccessListsRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetSuggestedAccessListsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetSuggestedAccessListsRequest): GetSuggestedAccessListsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetSuggestedAccessListsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetSuggestedAccessListsRequest;
    static deserializeBinaryFromReader(message: GetSuggestedAccessListsRequest, reader: jspb.BinaryReader): GetSuggestedAccessListsRequest;
}

export namespace GetSuggestedAccessListsRequest {
    export type AsObject = {
        accessRequestId: string,
    }
}

export class GetSuggestedAccessListsResponse extends jspb.Message { 
    clearAccessListsList(): void;
    getAccessListsList(): Array<teleport_accesslist_v1_accesslist_pb.AccessList>;
    setAccessListsList(value: Array<teleport_accesslist_v1_accesslist_pb.AccessList>): GetSuggestedAccessListsResponse;
    addAccessLists(value?: teleport_accesslist_v1_accesslist_pb.AccessList, index?: number): teleport_accesslist_v1_accesslist_pb.AccessList;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetSuggestedAccessListsResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetSuggestedAccessListsResponse): GetSuggestedAccessListsResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetSuggestedAccessListsResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetSuggestedAccessListsResponse;
    static deserializeBinaryFromReader(message: GetSuggestedAccessListsResponse, reader: jspb.BinaryReader): GetSuggestedAccessListsResponse;
}

export namespace GetSuggestedAccessListsResponse {
    export type AsObject = {
        accessListsList: Array<teleport_accesslist_v1_accesslist_pb.AccessList.AsObject>,
    }
}
