// package: teleport.terminal.v1
// file: v1/usage_events.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class ReportEventRequest extends jspb.Message { 
    getDistinctId(): string;
    setDistinctId(value: string): ReportEventRequest;


    hasTimestamp(): boolean;
    clearTimestamp(): void;
    getTimestamp(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setTimestamp(value?: google_protobuf_timestamp_pb.Timestamp): ReportEventRequest;


    hasEvent(): boolean;
    clearEvent(): void;
    getEvent(): ConnectUsageEventOneOf | undefined;
    setEvent(value?: ConnectUsageEventOneOf): ReportEventRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ReportEventRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ReportEventRequest): ReportEventRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ReportEventRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ReportEventRequest;
    static deserializeBinaryFromReader(message: ReportEventRequest, reader: jspb.BinaryReader): ReportEventRequest;
}

export namespace ReportEventRequest {
    export type AsObject = {
        distinctId: string,
        timestamp?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        event?: ConnectUsageEventOneOf.AsObject,
    }
}

export class EventReportedResponse extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): EventReportedResponse.AsObject;
    static toObject(includeInstance: boolean, msg: EventReportedResponse): EventReportedResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: EventReportedResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): EventReportedResponse;
    static deserializeBinaryFromReader(message: EventReportedResponse, reader: jspb.BinaryReader): EventReportedResponse;
}

export namespace EventReportedResponse {
    export type AsObject = {
    }
}

export class ClusterProperties extends jspb.Message { 
    getAuthClusterId(): string;
    setAuthClusterId(value: string): ClusterProperties;

    getClusterName(): string;
    setClusterName(value: string): ClusterProperties;

    getUserName(): string;
    setUserName(value: string): ClusterProperties;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ClusterProperties.AsObject;
    static toObject(includeInstance: boolean, msg: ClusterProperties): ClusterProperties.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ClusterProperties, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ClusterProperties;
    static deserializeBinaryFromReader(message: ClusterProperties, reader: jspb.BinaryReader): ClusterProperties;
}

export namespace ClusterProperties {
    export type AsObject = {
        authClusterId: string,
        clusterName: string,
        userName: string,
    }
}

export class LoginEvent extends jspb.Message { 

    hasClusterProperties(): boolean;
    clearClusterProperties(): void;
    getClusterProperties(): ClusterProperties | undefined;
    setClusterProperties(value?: ClusterProperties): LoginEvent;

    getArch(): string;
    setArch(value: string): LoginEvent;

    getOs(): string;
    setOs(value: string): LoginEvent;

    getOsVersion(): string;
    setOsVersion(value: string): LoginEvent;

    getConnectVersion(): string;
    setConnectVersion(value: string): LoginEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): LoginEvent.AsObject;
    static toObject(includeInstance: boolean, msg: LoginEvent): LoginEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: LoginEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): LoginEvent;
    static deserializeBinaryFromReader(message: LoginEvent, reader: jspb.BinaryReader): LoginEvent;
}

export namespace LoginEvent {
    export type AsObject = {
        clusterProperties?: ClusterProperties.AsObject,
        arch: string,
        os: string,
        osVersion: string,
        connectVersion: string,
    }
}

export class ProtocolRunEvent extends jspb.Message { 

    hasClusterProperties(): boolean;
    clearClusterProperties(): void;
    getClusterProperties(): ClusterProperties | undefined;
    setClusterProperties(value?: ClusterProperties): ProtocolRunEvent;

    getProtocol(): string;
    setProtocol(value: string): ProtocolRunEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ProtocolRunEvent.AsObject;
    static toObject(includeInstance: boolean, msg: ProtocolRunEvent): ProtocolRunEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ProtocolRunEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ProtocolRunEvent;
    static deserializeBinaryFromReader(message: ProtocolRunEvent, reader: jspb.BinaryReader): ProtocolRunEvent;
}

export namespace ProtocolRunEvent {
    export type AsObject = {
        clusterProperties?: ClusterProperties.AsObject,
        protocol: string,
    }
}

export class AccessRequestCreateEvent extends jspb.Message { 

    hasClusterProperties(): boolean;
    clearClusterProperties(): void;
    getClusterProperties(): ClusterProperties | undefined;
    setClusterProperties(value?: ClusterProperties): AccessRequestCreateEvent;

    getKind(): string;
    setKind(value: string): AccessRequestCreateEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AccessRequestCreateEvent.AsObject;
    static toObject(includeInstance: boolean, msg: AccessRequestCreateEvent): AccessRequestCreateEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AccessRequestCreateEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AccessRequestCreateEvent;
    static deserializeBinaryFromReader(message: AccessRequestCreateEvent, reader: jspb.BinaryReader): AccessRequestCreateEvent;
}

export namespace AccessRequestCreateEvent {
    export type AsObject = {
        clusterProperties?: ClusterProperties.AsObject,
        kind: string,
    }
}

export class AccessRequestReviewEvent extends jspb.Message { 

    hasClusterProperties(): boolean;
    clearClusterProperties(): void;
    getClusterProperties(): ClusterProperties | undefined;
    setClusterProperties(value?: ClusterProperties): AccessRequestReviewEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AccessRequestReviewEvent.AsObject;
    static toObject(includeInstance: boolean, msg: AccessRequestReviewEvent): AccessRequestReviewEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AccessRequestReviewEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AccessRequestReviewEvent;
    static deserializeBinaryFromReader(message: AccessRequestReviewEvent, reader: jspb.BinaryReader): AccessRequestReviewEvent;
}

export namespace AccessRequestReviewEvent {
    export type AsObject = {
        clusterProperties?: ClusterProperties.AsObject,
    }
}

export class AccessRequestAssumeRoleEvent extends jspb.Message { 

    hasClusterProperties(): boolean;
    clearClusterProperties(): void;
    getClusterProperties(): ClusterProperties | undefined;
    setClusterProperties(value?: ClusterProperties): AccessRequestAssumeRoleEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AccessRequestAssumeRoleEvent.AsObject;
    static toObject(includeInstance: boolean, msg: AccessRequestAssumeRoleEvent): AccessRequestAssumeRoleEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AccessRequestAssumeRoleEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AccessRequestAssumeRoleEvent;
    static deserializeBinaryFromReader(message: AccessRequestAssumeRoleEvent, reader: jspb.BinaryReader): AccessRequestAssumeRoleEvent;
}

export namespace AccessRequestAssumeRoleEvent {
    export type AsObject = {
        clusterProperties?: ClusterProperties.AsObject,
    }
}

export class FileTransferRunEvent extends jspb.Message { 

    hasClusterProperties(): boolean;
    clearClusterProperties(): void;
    getClusterProperties(): ClusterProperties | undefined;
    setClusterProperties(value?: ClusterProperties): FileTransferRunEvent;

    getDirection(): string;
    setDirection(value: string): FileTransferRunEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): FileTransferRunEvent.AsObject;
    static toObject(includeInstance: boolean, msg: FileTransferRunEvent): FileTransferRunEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: FileTransferRunEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): FileTransferRunEvent;
    static deserializeBinaryFromReader(message: FileTransferRunEvent, reader: jspb.BinaryReader): FileTransferRunEvent;
}

export namespace FileTransferRunEvent {
    export type AsObject = {
        clusterProperties?: ClusterProperties.AsObject,
        direction: string,
    }
}

export class UserJobRoleUpdateEvent extends jspb.Message { 
    getJobRole(): string;
    setJobRole(value: string): UserJobRoleUpdateEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UserJobRoleUpdateEvent.AsObject;
    static toObject(includeInstance: boolean, msg: UserJobRoleUpdateEvent): UserJobRoleUpdateEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UserJobRoleUpdateEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UserJobRoleUpdateEvent;
    static deserializeBinaryFromReader(message: UserJobRoleUpdateEvent, reader: jspb.BinaryReader): UserJobRoleUpdateEvent;
}

export namespace UserJobRoleUpdateEvent {
    export type AsObject = {
        jobRole: string,
    }
}

export class ConnectUsageEventOneOf extends jspb.Message { 

    hasLoginEvent(): boolean;
    clearLoginEvent(): void;
    getLoginEvent(): LoginEvent | undefined;
    setLoginEvent(value?: LoginEvent): ConnectUsageEventOneOf;


    hasProtocolRunEvent(): boolean;
    clearProtocolRunEvent(): void;
    getProtocolRunEvent(): ProtocolRunEvent | undefined;
    setProtocolRunEvent(value?: ProtocolRunEvent): ConnectUsageEventOneOf;


    hasAccessRequestCreateEvent(): boolean;
    clearAccessRequestCreateEvent(): void;
    getAccessRequestCreateEvent(): AccessRequestCreateEvent | undefined;
    setAccessRequestCreateEvent(value?: AccessRequestCreateEvent): ConnectUsageEventOneOf;


    hasAccessRequestReviewEvent(): boolean;
    clearAccessRequestReviewEvent(): void;
    getAccessRequestReviewEvent(): AccessRequestReviewEvent | undefined;
    setAccessRequestReviewEvent(value?: AccessRequestReviewEvent): ConnectUsageEventOneOf;


    hasAccessRequestAssumeRoleEvent(): boolean;
    clearAccessRequestAssumeRoleEvent(): void;
    getAccessRequestAssumeRoleEvent(): AccessRequestAssumeRoleEvent | undefined;
    setAccessRequestAssumeRoleEvent(value?: AccessRequestAssumeRoleEvent): ConnectUsageEventOneOf;


    hasFileTransferRunEvent(): boolean;
    clearFileTransferRunEvent(): void;
    getFileTransferRunEvent(): FileTransferRunEvent | undefined;
    setFileTransferRunEvent(value?: FileTransferRunEvent): ConnectUsageEventOneOf;


    hasUserJobRoleUpdateEvent(): boolean;
    clearUserJobRoleUpdateEvent(): void;
    getUserJobRoleUpdateEvent(): UserJobRoleUpdateEvent | undefined;
    setUserJobRoleUpdateEvent(value?: UserJobRoleUpdateEvent): ConnectUsageEventOneOf;


    getEventCase(): ConnectUsageEventOneOf.EventCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConnectUsageEventOneOf.AsObject;
    static toObject(includeInstance: boolean, msg: ConnectUsageEventOneOf): ConnectUsageEventOneOf.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConnectUsageEventOneOf, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConnectUsageEventOneOf;
    static deserializeBinaryFromReader(message: ConnectUsageEventOneOf, reader: jspb.BinaryReader): ConnectUsageEventOneOf;
}

export namespace ConnectUsageEventOneOf {
    export type AsObject = {
        loginEvent?: LoginEvent.AsObject,
        protocolRunEvent?: ProtocolRunEvent.AsObject,
        accessRequestCreateEvent?: AccessRequestCreateEvent.AsObject,
        accessRequestReviewEvent?: AccessRequestReviewEvent.AsObject,
        accessRequestAssumeRoleEvent?: AccessRequestAssumeRoleEvent.AsObject,
        fileTransferRunEvent?: FileTransferRunEvent.AsObject,
        userJobRoleUpdateEvent?: UserJobRoleUpdateEvent.AsObject,
    }

    export enum EventCase {
        EVENT_NOT_SET = 0,
    
    LOGIN_EVENT = 1,

    PROTOCOL_RUN_EVENT = 2,

    ACCESS_REQUEST_CREATE_EVENT = 3,

    ACCESS_REQUEST_REVIEW_EVENT = 4,

    ACCESS_REQUEST_ASSUME_ROLE_EVENT = 5,

    FILE_TRANSFER_RUN_EVENT = 6,

    USER_JOB_ROLE_UPDATE_EVENT = 7,

    }

}
