// package: prehog.v1alpha
// file: prehog/v1alpha/connect.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class ConnectLoginEvent extends jspb.Message { 
    getClusterName(): string;
    setClusterName(value: string): ConnectLoginEvent;

    getUserName(): string;
    setUserName(value: string): ConnectLoginEvent;

    getArch(): string;
    setArch(value: string): ConnectLoginEvent;

    getOs(): string;
    setOs(value: string): ConnectLoginEvent;

    getOsVersion(): string;
    setOsVersion(value: string): ConnectLoginEvent;

    getConnectVersion(): string;
    setConnectVersion(value: string): ConnectLoginEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConnectLoginEvent.AsObject;
    static toObject(includeInstance: boolean, msg: ConnectLoginEvent): ConnectLoginEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConnectLoginEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConnectLoginEvent;
    static deserializeBinaryFromReader(message: ConnectLoginEvent, reader: jspb.BinaryReader): ConnectLoginEvent;
}

export namespace ConnectLoginEvent {
    export type AsObject = {
        clusterName: string,
        userName: string,
        arch: string,
        os: string,
        osVersion: string,
        connectVersion: string,
    }
}

export class ConnectProtocolRunEvent extends jspb.Message { 
    getClusterName(): string;
    setClusterName(value: string): ConnectProtocolRunEvent;

    getUserName(): string;
    setUserName(value: string): ConnectProtocolRunEvent;

    getProtocol(): string;
    setProtocol(value: string): ConnectProtocolRunEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConnectProtocolRunEvent.AsObject;
    static toObject(includeInstance: boolean, msg: ConnectProtocolRunEvent): ConnectProtocolRunEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConnectProtocolRunEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConnectProtocolRunEvent;
    static deserializeBinaryFromReader(message: ConnectProtocolRunEvent, reader: jspb.BinaryReader): ConnectProtocolRunEvent;
}

export namespace ConnectProtocolRunEvent {
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


    hasConnectLogin(): boolean;
    clearConnectLogin(): void;
    getConnectLogin(): ConnectLoginEvent | undefined;
    setConnectLogin(value?: ConnectLoginEvent): SubmitConnectEventRequest;


    hasConnectProtocolRun(): boolean;
    clearConnectProtocolRun(): void;
    getConnectProtocolRun(): ConnectProtocolRunEvent | undefined;
    setConnectProtocolRun(value?: ConnectProtocolRunEvent): SubmitConnectEventRequest;


    hasConnectAccessRequestCreate(): boolean;
    clearConnectAccessRequestCreate(): void;
    getConnectAccessRequestCreate(): ConnectAccessRequestCreateEvent | undefined;
    setConnectAccessRequestCreate(value?: ConnectAccessRequestCreateEvent): SubmitConnectEventRequest;


    hasConnectAccessRequestReview(): boolean;
    clearConnectAccessRequestReview(): void;
    getConnectAccessRequestReview(): ConnectAccessRequestReviewEvent | undefined;
    setConnectAccessRequestReview(value?: ConnectAccessRequestReviewEvent): SubmitConnectEventRequest;


    hasConnectAccessRequestAssumeRole(): boolean;
    clearConnectAccessRequestAssumeRole(): void;
    getConnectAccessRequestAssumeRole(): ConnectAccessRequestAssumeRoleEvent | undefined;
    setConnectAccessRequestAssumeRole(value?: ConnectAccessRequestAssumeRoleEvent): SubmitConnectEventRequest;


    hasConnectFileTransferRunEvent(): boolean;
    clearConnectFileTransferRunEvent(): void;
    getConnectFileTransferRunEvent(): ConnectFileTransferRunEvent | undefined;
    setConnectFileTransferRunEvent(value?: ConnectFileTransferRunEvent): SubmitConnectEventRequest;


    hasConnectUserJobRoleUpdateEvent(): boolean;
    clearConnectUserJobRoleUpdateEvent(): void;
    getConnectUserJobRoleUpdateEvent(): ConnectUserJobRoleUpdateEvent | undefined;
    setConnectUserJobRoleUpdateEvent(value?: ConnectUserJobRoleUpdateEvent): SubmitConnectEventRequest;


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
        connectLogin?: ConnectLoginEvent.AsObject,
        connectProtocolRun?: ConnectProtocolRunEvent.AsObject,
        connectAccessRequestCreate?: ConnectAccessRequestCreateEvent.AsObject,
        connectAccessRequestReview?: ConnectAccessRequestReviewEvent.AsObject,
        connectAccessRequestAssumeRole?: ConnectAccessRequestAssumeRoleEvent.AsObject,
        connectFileTransferRunEvent?: ConnectFileTransferRunEvent.AsObject,
        connectUserJobRoleUpdateEvent?: ConnectUserJobRoleUpdateEvent.AsObject,
    }

    export enum EventCase {
        EVENT_NOT_SET = 0,
    
    CONNECT_LOGIN = 3,

    CONNECT_PROTOCOL_RUN = 4,

    CONNECT_ACCESS_REQUEST_CREATE = 5,

    CONNECT_ACCESS_REQUEST_REVIEW = 6,

    CONNECT_ACCESS_REQUEST_ASSUME_ROLE = 7,

    CONNECT_FILE_TRANSFER_RUN_EVENT = 8,

    CONNECT_USER_JOB_ROLE_UPDATE_EVENT = 9,

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
