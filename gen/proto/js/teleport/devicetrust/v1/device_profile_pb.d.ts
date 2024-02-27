// package: teleport.devicetrust.v1
// file: teleport/devicetrust/v1/device_profile.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class DeviceProfile extends jspb.Message { 

    hasUpdateTime(): boolean;
    clearUpdateTime(): void;
    getUpdateTime(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setUpdateTime(value?: google_protobuf_timestamp_pb.Timestamp): DeviceProfile;

    getModelIdentifier(): string;
    setModelIdentifier(value: string): DeviceProfile;

    getOsVersion(): string;
    setOsVersion(value: string): DeviceProfile;

    getOsBuild(): string;
    setOsBuild(value: string): DeviceProfile;

    clearOsUsernamesList(): void;
    getOsUsernamesList(): Array<string>;
    setOsUsernamesList(value: Array<string>): DeviceProfile;
    addOsUsernames(value: string, index?: number): string;

    getJamfBinaryVersion(): string;
    setJamfBinaryVersion(value: string): DeviceProfile;

    getExternalId(): string;
    setExternalId(value: string): DeviceProfile;

    getOsBuildSupplemental(): string;
    setOsBuildSupplemental(value: string): DeviceProfile;

    getOsId(): string;
    setOsId(value: string): DeviceProfile;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DeviceProfile.AsObject;
    static toObject(includeInstance: boolean, msg: DeviceProfile): DeviceProfile.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DeviceProfile, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DeviceProfile;
    static deserializeBinaryFromReader(message: DeviceProfile, reader: jspb.BinaryReader): DeviceProfile;
}

export namespace DeviceProfile {
    export type AsObject = {
        updateTime?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        modelIdentifier: string,
        osVersion: string,
        osBuild: string,
        osUsernamesList: Array<string>,
        jamfBinaryVersion: string,
        externalId: string,
        osBuildSupplemental: string,
        osId: string,
    }
}
