// package: teleport.lib.teleterm.v1
// file: teleport/lib/teleterm/v1/access_request.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class AccessRequest extends jspb.Message { 
    getId(): string;
    setId(value: string): AccessRequest;

    getState(): string;
    setState(value: string): AccessRequest;

    getResolveReason(): string;
    setResolveReason(value: string): AccessRequest;

    getRequestReason(): string;
    setRequestReason(value: string): AccessRequest;

    getUser(): string;
    setUser(value: string): AccessRequest;

    clearRolesList(): void;
    getRolesList(): Array<string>;
    setRolesList(value: Array<string>): AccessRequest;
    addRoles(value: string, index?: number): string;


    hasCreated(): boolean;
    clearCreated(): void;
    getCreated(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setCreated(value?: google_protobuf_timestamp_pb.Timestamp): AccessRequest;


    hasExpires(): boolean;
    clearExpires(): void;
    getExpires(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setExpires(value?: google_protobuf_timestamp_pb.Timestamp): AccessRequest;

    clearReviewsList(): void;
    getReviewsList(): Array<AccessRequestReview>;
    setReviewsList(value: Array<AccessRequestReview>): AccessRequest;
    addReviews(value?: AccessRequestReview, index?: number): AccessRequestReview;

    clearSuggestedReviewersList(): void;
    getSuggestedReviewersList(): Array<string>;
    setSuggestedReviewersList(value: Array<string>): AccessRequest;
    addSuggestedReviewers(value: string, index?: number): string;

    clearThresholdNamesList(): void;
    getThresholdNamesList(): Array<string>;
    setThresholdNamesList(value: Array<string>): AccessRequest;
    addThresholdNames(value: string, index?: number): string;

    clearResourceIdsList(): void;
    getResourceIdsList(): Array<ResourceID>;
    setResourceIdsList(value: Array<ResourceID>): AccessRequest;
    addResourceIds(value?: ResourceID, index?: number): ResourceID;

    clearResourcesList(): void;
    getResourcesList(): Array<Resource>;
    setResourcesList(value: Array<Resource>): AccessRequest;
    addResources(value?: Resource, index?: number): Resource;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AccessRequest.AsObject;
    static toObject(includeInstance: boolean, msg: AccessRequest): AccessRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AccessRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AccessRequest;
    static deserializeBinaryFromReader(message: AccessRequest, reader: jspb.BinaryReader): AccessRequest;
}

export namespace AccessRequest {
    export type AsObject = {
        id: string,
        state: string,
        resolveReason: string,
        requestReason: string,
        user: string,
        rolesList: Array<string>,
        created?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        expires?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        reviewsList: Array<AccessRequestReview.AsObject>,
        suggestedReviewersList: Array<string>,
        thresholdNamesList: Array<string>,
        resourceIdsList: Array<ResourceID.AsObject>,
        resourcesList: Array<Resource.AsObject>,
    }
}

export class AccessRequestReview extends jspb.Message { 
    getAuthor(): string;
    setAuthor(value: string): AccessRequestReview;

    clearRolesList(): void;
    getRolesList(): Array<string>;
    setRolesList(value: Array<string>): AccessRequestReview;
    addRoles(value: string, index?: number): string;

    getState(): string;
    setState(value: string): AccessRequestReview;

    getReason(): string;
    setReason(value: string): AccessRequestReview;


    hasCreated(): boolean;
    clearCreated(): void;
    getCreated(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setCreated(value?: google_protobuf_timestamp_pb.Timestamp): AccessRequestReview;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AccessRequestReview.AsObject;
    static toObject(includeInstance: boolean, msg: AccessRequestReview): AccessRequestReview.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AccessRequestReview, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AccessRequestReview;
    static deserializeBinaryFromReader(message: AccessRequestReview, reader: jspb.BinaryReader): AccessRequestReview;
}

export namespace AccessRequestReview {
    export type AsObject = {
        author: string,
        rolesList: Array<string>,
        state: string,
        reason: string,
        created?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    }
}

export class ResourceID extends jspb.Message { 
    getKind(): string;
    setKind(value: string): ResourceID;

    getName(): string;
    setName(value: string): ResourceID;

    getClusterName(): string;
    setClusterName(value: string): ResourceID;

    getSubResourceName(): string;
    setSubResourceName(value: string): ResourceID;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ResourceID.AsObject;
    static toObject(includeInstance: boolean, msg: ResourceID): ResourceID.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ResourceID, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ResourceID;
    static deserializeBinaryFromReader(message: ResourceID, reader: jspb.BinaryReader): ResourceID;
}

export namespace ResourceID {
    export type AsObject = {
        kind: string,
        name: string,
        clusterName: string,
        subResourceName: string,
    }
}

export class ResourceDetails extends jspb.Message { 
    getHostname(): string;
    setHostname(value: string): ResourceDetails;

    getFriendlyName(): string;
    setFriendlyName(value: string): ResourceDetails;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ResourceDetails.AsObject;
    static toObject(includeInstance: boolean, msg: ResourceDetails): ResourceDetails.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ResourceDetails, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ResourceDetails;
    static deserializeBinaryFromReader(message: ResourceDetails, reader: jspb.BinaryReader): ResourceDetails;
}

export namespace ResourceDetails {
    export type AsObject = {
        hostname: string,
        friendlyName: string,
    }
}

export class Resource extends jspb.Message { 

    hasId(): boolean;
    clearId(): void;
    getId(): ResourceID | undefined;
    setId(value?: ResourceID): Resource;


    hasDetails(): boolean;
    clearDetails(): void;
    getDetails(): ResourceDetails | undefined;
    setDetails(value?: ResourceDetails): Resource;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Resource.AsObject;
    static toObject(includeInstance: boolean, msg: Resource): Resource.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Resource, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Resource;
    static deserializeBinaryFromReader(message: Resource, reader: jspb.BinaryReader): Resource;
}

export namespace Resource {
    export type AsObject = {
        id?: ResourceID.AsObject,
        details?: ResourceDetails.AsObject,
    }
}
