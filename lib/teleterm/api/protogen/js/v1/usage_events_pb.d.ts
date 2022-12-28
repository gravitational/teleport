// package: teleport.terminal.v1
// file: v1/usage_events.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";
import * as prehog_v1alpha_connect_pb from "../prehog/v1alpha/connect_pb";

export class ReportEventRequest extends jspb.Message { 
    getDistinctId(): string;
    setDistinctId(value: string): ReportEventRequest;


    hasTimestamp(): boolean;
    clearTimestamp(): void;
    getTimestamp(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setTimestamp(value?: google_protobuf_timestamp_pb.Timestamp): ReportEventRequest;

    getAuthClusterId(): string;
    setAuthClusterId(value: string): ReportEventRequest;


    hasConnectLogin(): boolean;
    clearConnectLogin(): void;
    getConnectLogin(): prehog_v1alpha_connect_pb.ConnectLoginEvent | undefined;
    setConnectLogin(value?: prehog_v1alpha_connect_pb.ConnectLoginEvent): ReportEventRequest;


    hasConnectProtocolRun(): boolean;
    clearConnectProtocolRun(): void;
    getConnectProtocolRun(): prehog_v1alpha_connect_pb.ConnectProtocolRunEvent | undefined;
    setConnectProtocolRun(value?: prehog_v1alpha_connect_pb.ConnectProtocolRunEvent): ReportEventRequest;


    hasConnectAccessRequestCreate(): boolean;
    clearConnectAccessRequestCreate(): void;
    getConnectAccessRequestCreate(): prehog_v1alpha_connect_pb.ConnectAccessRequestCreateEvent | undefined;
    setConnectAccessRequestCreate(value?: prehog_v1alpha_connect_pb.ConnectAccessRequestCreateEvent): ReportEventRequest;


    hasConnectAccessRequestReview(): boolean;
    clearConnectAccessRequestReview(): void;
    getConnectAccessRequestReview(): prehog_v1alpha_connect_pb.ConnectAccessRequestReviewEvent | undefined;
    setConnectAccessRequestReview(value?: prehog_v1alpha_connect_pb.ConnectAccessRequestReviewEvent): ReportEventRequest;


    hasConnectAccessRequestAssumeRole(): boolean;
    clearConnectAccessRequestAssumeRole(): void;
    getConnectAccessRequestAssumeRole(): prehog_v1alpha_connect_pb.ConnectAccessRequestAssumeRoleEvent | undefined;
    setConnectAccessRequestAssumeRole(value?: prehog_v1alpha_connect_pb.ConnectAccessRequestAssumeRoleEvent): ReportEventRequest;


    hasConnectFileTransferRunEvent(): boolean;
    clearConnectFileTransferRunEvent(): void;
    getConnectFileTransferRunEvent(): prehog_v1alpha_connect_pb.ConnectFileTransferRunEvent | undefined;
    setConnectFileTransferRunEvent(value?: prehog_v1alpha_connect_pb.ConnectFileTransferRunEvent): ReportEventRequest;


    hasConnectUserJobRoleUpdateEvent(): boolean;
    clearConnectUserJobRoleUpdateEvent(): void;
    getConnectUserJobRoleUpdateEvent(): prehog_v1alpha_connect_pb.ConnectUserJobRoleUpdateEvent | undefined;
    setConnectUserJobRoleUpdateEvent(value?: prehog_v1alpha_connect_pb.ConnectUserJobRoleUpdateEvent): ReportEventRequest;


    getEventCase(): ReportEventRequest.EventCase;

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
        authClusterId: string,
        connectLogin?: prehog_v1alpha_connect_pb.ConnectLoginEvent.AsObject,
        connectProtocolRun?: prehog_v1alpha_connect_pb.ConnectProtocolRunEvent.AsObject,
        connectAccessRequestCreate?: prehog_v1alpha_connect_pb.ConnectAccessRequestCreateEvent.AsObject,
        connectAccessRequestReview?: prehog_v1alpha_connect_pb.ConnectAccessRequestReviewEvent.AsObject,
        connectAccessRequestAssumeRole?: prehog_v1alpha_connect_pb.ConnectAccessRequestAssumeRoleEvent.AsObject,
        connectFileTransferRunEvent?: prehog_v1alpha_connect_pb.ConnectFileTransferRunEvent.AsObject,
        connectUserJobRoleUpdateEvent?: prehog_v1alpha_connect_pb.ConnectUserJobRoleUpdateEvent.AsObject,
    }

    export enum EventCase {
        EVENT_NOT_SET = 0,
    
    CONNECT_LOGIN = 4,

    CONNECT_PROTOCOL_RUN = 5,

    CONNECT_ACCESS_REQUEST_CREATE = 6,

    CONNECT_ACCESS_REQUEST_REVIEW = 7,

    CONNECT_ACCESS_REQUEST_ASSUME_ROLE = 8,

    CONNECT_FILE_TRANSFER_RUN_EVENT = 9,

    CONNECT_USER_JOB_ROLE_UPDATE_EVENT = 10,

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
