// package: teleport.devicetrust.v1
// file: teleport/devicetrust/v1/device_collected_data.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";
import * as teleport_devicetrust_v1_os_type_pb from "../../../teleport/devicetrust/v1/os_type_pb";
import * as teleport_devicetrust_v1_tpm_pb from "../../../teleport/devicetrust/v1/tpm_pb";

export class DeviceCollectedData extends jspb.Message { 

    hasCollectTime(): boolean;
    clearCollectTime(): void;
    getCollectTime(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setCollectTime(value?: google_protobuf_timestamp_pb.Timestamp): DeviceCollectedData;

    hasRecordTime(): boolean;
    clearRecordTime(): void;
    getRecordTime(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setRecordTime(value?: google_protobuf_timestamp_pb.Timestamp): DeviceCollectedData;
    getOsType(): teleport_devicetrust_v1_os_type_pb.OSType;
    setOsType(value: teleport_devicetrust_v1_os_type_pb.OSType): DeviceCollectedData;
    getSerialNumber(): string;
    setSerialNumber(value: string): DeviceCollectedData;
    getModelIdentifier(): string;
    setModelIdentifier(value: string): DeviceCollectedData;
    getOsVersion(): string;
    setOsVersion(value: string): DeviceCollectedData;
    getOsBuild(): string;
    setOsBuild(value: string): DeviceCollectedData;
    getOsUsername(): string;
    setOsUsername(value: string): DeviceCollectedData;
    getJamfBinaryVersion(): string;
    setJamfBinaryVersion(value: string): DeviceCollectedData;
    getMacosEnrollmentProfiles(): string;
    setMacosEnrollmentProfiles(value: string): DeviceCollectedData;
    getReportedAssetTag(): string;
    setReportedAssetTag(value: string): DeviceCollectedData;
    getSystemSerialNumber(): string;
    setSystemSerialNumber(value: string): DeviceCollectedData;
    getBaseBoardSerialNumber(): string;
    setBaseBoardSerialNumber(value: string): DeviceCollectedData;

    hasTpmPlatformAttestation(): boolean;
    clearTpmPlatformAttestation(): void;
    getTpmPlatformAttestation(): teleport_devicetrust_v1_tpm_pb.TPMPlatformAttestation | undefined;
    setTpmPlatformAttestation(value?: teleport_devicetrust_v1_tpm_pb.TPMPlatformAttestation): DeviceCollectedData;
    getOsId(): string;
    setOsId(value: string): DeviceCollectedData;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DeviceCollectedData.AsObject;
    static toObject(includeInstance: boolean, msg: DeviceCollectedData): DeviceCollectedData.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DeviceCollectedData, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DeviceCollectedData;
    static deserializeBinaryFromReader(message: DeviceCollectedData, reader: jspb.BinaryReader): DeviceCollectedData;
}

export namespace DeviceCollectedData {
    export type AsObject = {
        collectTime?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        recordTime?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        osType: teleport_devicetrust_v1_os_type_pb.OSType,
        serialNumber: string,
        modelIdentifier: string,
        osVersion: string,
        osBuild: string,
        osUsername: string,
        jamfBinaryVersion: string,
        macosEnrollmentProfiles: string,
        reportedAssetTag: string,
        systemSerialNumber: string,
        baseBoardSerialNumber: string,
        tpmPlatformAttestation?: teleport_devicetrust_v1_tpm_pb.TPMPlatformAttestation.AsObject,
        osId: string,
    }
}
