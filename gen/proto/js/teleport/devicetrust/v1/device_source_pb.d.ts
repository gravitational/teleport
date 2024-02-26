// package: teleport.devicetrust.v1
// file: teleport/devicetrust/v1/device_source.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class DeviceSource extends jspb.Message { 
    getName(): string;
    setName(value: string): DeviceSource;
    getOrigin(): DeviceOrigin;
    setOrigin(value: DeviceOrigin): DeviceSource;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DeviceSource.AsObject;
    static toObject(includeInstance: boolean, msg: DeviceSource): DeviceSource.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DeviceSource, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DeviceSource;
    static deserializeBinaryFromReader(message: DeviceSource, reader: jspb.BinaryReader): DeviceSource;
}

export namespace DeviceSource {
    export type AsObject = {
        name: string,
        origin: DeviceOrigin,
    }
}

export enum DeviceOrigin {
    DEVICE_ORIGIN_UNSPECIFIED = 0,
    DEVICE_ORIGIN_API = 1,
    DEVICE_ORIGIN_JAMF = 2,
    DEVICE_ORIGIN_INTUNE = 3,
}
