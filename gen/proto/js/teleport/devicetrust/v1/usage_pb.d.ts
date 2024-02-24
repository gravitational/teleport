// package: teleport.devicetrust.v1
// file: teleport/devicetrust/v1/usage.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class DevicesUsage extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DevicesUsage.AsObject;
    static toObject(includeInstance: boolean, msg: DevicesUsage): DevicesUsage.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DevicesUsage, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DevicesUsage;
    static deserializeBinaryFromReader(message: DevicesUsage, reader: jspb.BinaryReader): DevicesUsage;
}

export namespace DevicesUsage {
    export type AsObject = {
    }
}

export enum AccountUsageType {
    ACCOUNT_USAGE_TYPE_UNSPECIFIED = 0,
    ACCOUNT_USAGE_TYPE_UNLIMITED = 1,
    ACCOUNT_USAGE_TYPE_USAGE_BASED = 2,
}
