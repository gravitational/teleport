// package: teleport.devicetrust.v1
// file: teleport/devicetrust/v1/device_enroll_token.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class DeviceEnrollToken extends jspb.Message { 
    getToken(): string;
    setToken(value: string): DeviceEnrollToken;

    hasExpireTime(): boolean;
    clearExpireTime(): void;
    getExpireTime(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setExpireTime(value?: google_protobuf_timestamp_pb.Timestamp): DeviceEnrollToken;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DeviceEnrollToken.AsObject;
    static toObject(includeInstance: boolean, msg: DeviceEnrollToken): DeviceEnrollToken.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DeviceEnrollToken, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DeviceEnrollToken;
    static deserializeBinaryFromReader(message: DeviceEnrollToken, reader: jspb.BinaryReader): DeviceEnrollToken;
}

export namespace DeviceEnrollToken {
    export type AsObject = {
        token: string,
        expireTime?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    }
}
