// package: teleterm.v1
// file: teleterm/v1/usage_events.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as prehog_v1alpha_connect_pb from "../../prehog/v1alpha/connect_pb";

export class ReportUsageEventRequest extends jspb.Message { 
    getAuthClusterId(): string;
    setAuthClusterId(value: string): ReportUsageEventRequest;


    hasPrehogReq(): boolean;
    clearPrehogReq(): void;
    getPrehogReq(): prehog_v1alpha_connect_pb.SubmitConnectEventRequest | undefined;
    setPrehogReq(value?: prehog_v1alpha_connect_pb.SubmitConnectEventRequest): ReportUsageEventRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ReportUsageEventRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ReportUsageEventRequest): ReportUsageEventRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ReportUsageEventRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ReportUsageEventRequest;
    static deserializeBinaryFromReader(message: ReportUsageEventRequest, reader: jspb.BinaryReader): ReportUsageEventRequest;
}

export namespace ReportUsageEventRequest {
    export type AsObject = {
        authClusterId: string,
        prehogReq?: prehog_v1alpha_connect_pb.SubmitConnectEventRequest.AsObject,
    }
}
