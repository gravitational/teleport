// package: prehog.v1alpha
// file: prehog/v1alpha/connect.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class ConnectUserLoginEvent extends jspb.Message { 
    getClusterName(): string;
    setClusterName(value: string): ConnectUserLoginEvent;

    getUserName(): string;
    setUserName(value: string): ConnectUserLoginEvent;

    getArch(): string;
    setArch(value: string): ConnectUserLoginEvent;

    getOs(): string;
    setOs(value: string): ConnectUserLoginEvent;

    getOsVersion(): string;
    setOsVersion(value: string): ConnectUserLoginEvent;

    getConnectVersion(): string;
    setConnectVersion(value: string): ConnectUserLoginEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConnectUserLoginEvent.AsObject;
    static toObject(includeInstance: boolean, msg: ConnectUserLoginEvent): ConnectUserLoginEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConnectUserLoginEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConnectUserLoginEvent;
    static deserializeBinaryFromReader(message: ConnectUserLoginEvent, reader: jspb.BinaryReader): ConnectUserLoginEvent;
}

export namespace ConnectUserLoginEvent {
    export type AsObject = {
        clusterName: string,
        userName: string,
        arch: string,
        os: string,
        osVersion: string,
        connectVersion: string,
    }
}

export class ConnectProtocolUseEvent extends jspb.Message { 
    getClusterName(): string;
    setClusterName(value: string): ConnectProtocolUseEvent;

    getUserName(): string;
    setUserName(value: string): ConnectProtocolUseEvent;

    getProtocol(): string;
    setProtocol(value: string): ConnectProtocolUseEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConnectProtocolUseEvent.AsObject;
    static toObject(includeInstance: boolean, msg: ConnectProtocolUseEvent): ConnectProtocolUseEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConnectProtocolUseEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConnectProtocolUseEvent;
    static deserializeBinaryFromReader(message: ConnectProtocolUseEvent, reader: jspb.BinaryReader): ConnectProtocolUseEvent;
}

export namespace ConnectProtocolUseEvent {
    export type AsObject = {
        clusterName: string,
        userName: string,
        protocol: string,
    }
}

export class ConnectAccessRequestCreateEvent extends jspb.Message { 
    getClusterName(): string;
    setClusterName(value: string): ConnectAccessRequestCreateEvent;

    getUserName(): string;
    setUserName(value: string): ConnectAccessRequestCreateEvent;

    getKind(): string;
    setKind(value: string): ConnectAccessRequestCreateEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConnectAccessRequestCreateEvent.AsObject;
    static toObject(includeInstance: boolean, msg: ConnectAccessRequestCreateEvent): ConnectAccessRequestCreateEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConnectAccessRequestCreateEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConnectAccessRequestCreateEvent;
    static deserializeBinaryFromReader(message: ConnectAccessRequestCreateEvent, reader: jspb.BinaryReader): ConnectAccessRequestCreateEvent;
}

export namespace ConnectAccessRequestCreateEvent {
    export type AsObject = {
        clusterName: string,
        userName: string,
        kind: string,
    }
}

export class ConnectAccessRequestReviewEvent extends jspb.Message { 
    getClusterName(): string;
    setClusterName(value: string): ConnectAccessRequestReviewEvent;

    getUserName(): string;
    setUserName(value: string): ConnectAccessRequestReviewEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConnectAccessRequestReviewEvent.AsObject;
    static toObject(includeInstance: boolean, msg: ConnectAccessRequestReviewEvent): ConnectAccessRequestReviewEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConnectAccessRequestReviewEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConnectAccessRequestReviewEvent;
    static deserializeBinaryFromReader(message: ConnectAccessRequestReviewEvent, reader: jspb.BinaryReader): ConnectAccessRequestReviewEvent;
}

export namespace ConnectAccessRequestReviewEvent {
    export type AsObject = {
        clusterName: string,
        userName: string,
    }
}

export class ConnectAccessRequestAssumeRoleEvent extends jspb.Message { 
    getClusterName(): string;
    setClusterName(value: string): ConnectAccessRequestAssumeRoleEvent;

    getUserName(): string;
    setUserName(value: string): ConnectAccessRequestAssumeRoleEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConnectAccessRequestAssumeRoleEvent.AsObject;
    static toObject(includeInstance: boolean, msg: ConnectAccessRequestAssumeRoleEvent): ConnectAccessRequestAssumeRoleEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConnectAccessRequestAssumeRoleEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConnectAccessRequestAssumeRoleEvent;
    static deserializeBinaryFromReader(message: ConnectAccessRequestAssumeRoleEvent, reader: jspb.BinaryReader): ConnectAccessRequestAssumeRoleEvent;
}

export namespace ConnectAccessRequestAssumeRoleEvent {
    export type AsObject = {
        clusterName: string,
        userName: string,
    }
}

export class ConnectFileTransferRunEvent extends jspb.Message { 
    getClusterName(): string;
    setClusterName(value: string): ConnectFileTransferRunEvent;

    getUserName(): string;
    setUserName(value: string): ConnectFileTransferRunEvent;

    getDirection(): string;
    setDirection(value: string): ConnectFileTransferRunEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConnectFileTransferRunEvent.AsObject;
    static toObject(includeInstance: boolean, msg: ConnectFileTransferRunEvent): ConnectFileTransferRunEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConnectFileTransferRunEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConnectFileTransferRunEvent;
    static deserializeBinaryFromReader(message: ConnectFileTransferRunEvent, reader: jspb.BinaryReader): ConnectFileTransferRunEvent;
}

export namespace ConnectFileTransferRunEvent {
    export type AsObject = {
        clusterName: string,
        userName: string,
        direction: string,
    }
}

export class ConnectUserJobRoleUpdateEvent extends jspb.Message { 
    getJobRole(): string;
    setJobRole(value: string): ConnectUserJobRoleUpdateEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConnectUserJobRoleUpdateEvent.AsObject;
    static toObject(includeInstance: boolean, msg: ConnectUserJobRoleUpdateEvent): ConnectUserJobRoleUpdateEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConnectUserJobRoleUpdateEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConnectUserJobRoleUpdateEvent;
    static deserializeBinaryFromReader(message: ConnectUserJobRoleUpdateEvent, reader: jspb.BinaryReader): ConnectUserJobRoleUpdateEvent;
}

export namespace ConnectUserJobRoleUpdateEvent {
    export type AsObject = {
        jobRole: string,
    }
}

export class SubmitConnectEventRequest extends jspb.Message { 
    getDistinctId(): string;
    setDistinctId(value: string): SubmitConnectEventRequest;


    hasTimestamp(): boolean;
    clearTimestamp(): void;
    getTimestamp(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setTimestamp(value?: google_protobuf_timestamp_pb.Timestamp): SubmitConnectEventRequest;


    hasUserLogin(): boolean;
    clearUserLogin(): void;
    getUserLogin(): ConnectUserLoginEvent | undefined;
    setUserLogin(value?: ConnectUserLoginEvent): SubmitConnectEventRequest;


    hasProtocolUse(): boolean;
    clearProtocolUse(): void;
    getProtocolUse(): ConnectProtocolUseEvent | undefined;
    setProtocolUse(value?: ConnectProtocolUseEvent): SubmitConnectEventRequest;


    hasAccessRequestCreate(): boolean;
    clearAccessRequestCreate(): void;
    getAccessRequestCreate(): ConnectAccessRequestCreateEvent | undefined;
    setAccessRequestCreate(value?: ConnectAccessRequestCreateEvent): SubmitConnectEventRequest;


    hasAccessRequestReview(): boolean;
    clearAccessRequestReview(): void;
    getAccessRequestReview(): ConnectAccessRequestReviewEvent | undefined;
    setAccessRequestReview(value?: ConnectAccessRequestReviewEvent): SubmitConnectEventRequest;


    hasAccessRequestAssumeRole(): boolean;
    clearAccessRequestAssumeRole(): void;
    getAccessRequestAssumeRole(): ConnectAccessRequestAssumeRoleEvent | undefined;
    setAccessRequestAssumeRole(value?: ConnectAccessRequestAssumeRoleEvent): SubmitConnectEventRequest;


    hasFileTransferRun(): boolean;
    clearFileTransferRun(): void;
    getFileTransferRun(): ConnectFileTransferRunEvent | undefined;
    setFileTransferRun(value?: ConnectFileTransferRunEvent): SubmitConnectEventRequest;


    hasUserJobRoleUpdate(): boolean;
    clearUserJobRoleUpdate(): void;
    getUserJobRoleUpdate(): ConnectUserJobRoleUpdateEvent | undefined;
    setUserJobRoleUpdate(value?: ConnectUserJobRoleUpdateEvent): SubmitConnectEventRequest;


    getEventCase(): SubmitConnectEventRequest.EventCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SubmitConnectEventRequest.AsObject;
    static toObject(includeInstance: boolean, msg: SubmitConnectEventRequest): SubmitConnectEventRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SubmitConnectEventRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SubmitConnectEventRequest;
    static deserializeBinaryFromReader(message: SubmitConnectEventRequest, reader: jspb.BinaryReader): SubmitConnectEventRequest;
}

export namespace SubmitConnectEventRequest {
    export type AsObject = {
        distinctId: string,
        timestamp?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        userLogin?: ConnectUserLoginEvent.AsObject,
        protocolUse?: ConnectProtocolUseEvent.AsObject,
        accessRequestCreate?: ConnectAccessRequestCreateEvent.AsObject,
        accessRequestReview?: ConnectAccessRequestReviewEvent.AsObject,
        accessRequestAssumeRole?: ConnectAccessRequestAssumeRoleEvent.AsObject,
        fileTransferRun?: ConnectFileTransferRunEvent.AsObject,
        userJobRoleUpdate?: ConnectUserJobRoleUpdateEvent.AsObject,
    }

    export enum EventCase {
        EVENT_NOT_SET = 0,
    
    USER_LOGIN = 3,

    PROTOCOL_USE = 4,

    ACCESS_REQUEST_CREATE = 5,

    ACCESS_REQUEST_REVIEW = 6,

    ACCESS_REQUEST_ASSUME_ROLE = 7,

    FILE_TRANSFER_RUN = 8,

    USER_JOB_ROLE_UPDATE = 9,

    }

}

export class SubmitConnectEventResponse extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SubmitConnectEventResponse.AsObject;
    static toObject(includeInstance: boolean, msg: SubmitConnectEventResponse): SubmitConnectEventResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SubmitConnectEventResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SubmitConnectEventResponse;
    static deserializeBinaryFromReader(message: SubmitConnectEventResponse, reader: jspb.BinaryReader): SubmitConnectEventResponse;
}

export namespace SubmitConnectEventResponse {
    export type AsObject = {
    }
}
