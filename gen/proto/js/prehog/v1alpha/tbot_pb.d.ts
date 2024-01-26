// package: prehog.v1alpha
// file: prehog/v1alpha/tbot.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class TbotStartEvent extends jspb.Message { 
    getRunMode(): TbotStartEvent.RunMode;
    setRunMode(value: TbotStartEvent.RunMode): TbotStartEvent;

    getVersion(): string;
    setVersion(value: string): TbotStartEvent;

    getJoinType(): string;
    setJoinType(value: string): TbotStartEvent;

    getHelper(): string;
    setHelper(value: string): TbotStartEvent;

    getHelperVersion(): string;
    setHelperVersion(value: string): TbotStartEvent;

    getDestinationsOther(): number;
    setDestinationsOther(value: number): TbotStartEvent;

    getDestinationsDatabase(): number;
    setDestinationsDatabase(value: number): TbotStartEvent;

    getDestinationsKubernetes(): number;
    setDestinationsKubernetes(value: number): TbotStartEvent;

    getDestinationsApplication(): number;
    setDestinationsApplication(value: number): TbotStartEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TbotStartEvent.AsObject;
    static toObject(includeInstance: boolean, msg: TbotStartEvent): TbotStartEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TbotStartEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TbotStartEvent;
    static deserializeBinaryFromReader(message: TbotStartEvent, reader: jspb.BinaryReader): TbotStartEvent;
}

export namespace TbotStartEvent {
    export type AsObject = {
        runMode: TbotStartEvent.RunMode,
        version: string,
        joinType: string,
        helper: string,
        helperVersion: string,
        destinationsOther: number,
        destinationsDatabase: number,
        destinationsKubernetes: number,
        destinationsApplication: number,
    }

    export enum RunMode {
    RUN_MODE_UNSPECIFIED = 0,
    RUN_MODE_ONE_SHOT = 1,
    RUN_MODE_DAEMON = 2,
    }

}

export class SubmitTbotEventRequest extends jspb.Message { 
    getDistinctId(): string;
    setDistinctId(value: string): SubmitTbotEventRequest;


    hasTimestamp(): boolean;
    clearTimestamp(): void;
    getTimestamp(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setTimestamp(value?: google_protobuf_timestamp_pb.Timestamp): SubmitTbotEventRequest;


    hasStart(): boolean;
    clearStart(): void;
    getStart(): TbotStartEvent | undefined;
    setStart(value?: TbotStartEvent): SubmitTbotEventRequest;


    getEventCase(): SubmitTbotEventRequest.EventCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SubmitTbotEventRequest.AsObject;
    static toObject(includeInstance: boolean, msg: SubmitTbotEventRequest): SubmitTbotEventRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SubmitTbotEventRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SubmitTbotEventRequest;
    static deserializeBinaryFromReader(message: SubmitTbotEventRequest, reader: jspb.BinaryReader): SubmitTbotEventRequest;
}

export namespace SubmitTbotEventRequest {
    export type AsObject = {
        distinctId: string,
        timestamp?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        start?: TbotStartEvent.AsObject,
    }

    export enum EventCase {
        EVENT_NOT_SET = 0,
    
    START = 3,

    }

}

export class SubmitTbotEventResponse extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SubmitTbotEventResponse.AsObject;
    static toObject(includeInstance: boolean, msg: SubmitTbotEventResponse): SubmitTbotEventResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SubmitTbotEventResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SubmitTbotEventResponse;
    static deserializeBinaryFromReader(message: SubmitTbotEventResponse, reader: jspb.BinaryReader): SubmitTbotEventResponse;
}

export namespace SubmitTbotEventResponse {
    export type AsObject = {
    }
}
