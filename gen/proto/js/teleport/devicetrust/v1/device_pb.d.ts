// package: teleport.devicetrust.v1
// file: teleport/devicetrust/v1/device.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";
import * as teleport_devicetrust_v1_device_collected_data_pb from "../../../teleport/devicetrust/v1/device_collected_data_pb";
import * as teleport_devicetrust_v1_device_enroll_token_pb from "../../../teleport/devicetrust/v1/device_enroll_token_pb";
import * as teleport_devicetrust_v1_device_profile_pb from "../../../teleport/devicetrust/v1/device_profile_pb";
import * as teleport_devicetrust_v1_device_source_pb from "../../../teleport/devicetrust/v1/device_source_pb";
import * as teleport_devicetrust_v1_os_type_pb from "../../../teleport/devicetrust/v1/os_type_pb";

export class Device extends jspb.Message { 
    getApiVersion(): string;
    setApiVersion(value: string): Device;
    getId(): string;
    setId(value: string): Device;
    getOsType(): teleport_devicetrust_v1_os_type_pb.OSType;
    setOsType(value: teleport_devicetrust_v1_os_type_pb.OSType): Device;
    getAssetTag(): string;
    setAssetTag(value: string): Device;

    hasCreateTime(): boolean;
    clearCreateTime(): void;
    getCreateTime(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setCreateTime(value?: google_protobuf_timestamp_pb.Timestamp): Device;

    hasUpdateTime(): boolean;
    clearUpdateTime(): void;
    getUpdateTime(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setUpdateTime(value?: google_protobuf_timestamp_pb.Timestamp): Device;

    hasEnrollToken(): boolean;
    clearEnrollToken(): void;
    getEnrollToken(): teleport_devicetrust_v1_device_enroll_token_pb.DeviceEnrollToken | undefined;
    setEnrollToken(value?: teleport_devicetrust_v1_device_enroll_token_pb.DeviceEnrollToken): Device;
    getEnrollStatus(): DeviceEnrollStatus;
    setEnrollStatus(value: DeviceEnrollStatus): Device;

    hasCredential(): boolean;
    clearCredential(): void;
    getCredential(): DeviceCredential | undefined;
    setCredential(value?: DeviceCredential): Device;
    clearCollectedDataList(): void;
    getCollectedDataList(): Array<teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData>;
    setCollectedDataList(value: Array<teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData>): Device;
    addCollectedData(value?: teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData, index?: number): teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData;

    hasSource(): boolean;
    clearSource(): void;
    getSource(): teleport_devicetrust_v1_device_source_pb.DeviceSource | undefined;
    setSource(value?: teleport_devicetrust_v1_device_source_pb.DeviceSource): Device;

    hasProfile(): boolean;
    clearProfile(): void;
    getProfile(): teleport_devicetrust_v1_device_profile_pb.DeviceProfile | undefined;
    setProfile(value?: teleport_devicetrust_v1_device_profile_pb.DeviceProfile): Device;
    getOwner(): string;
    setOwner(value: string): Device;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Device.AsObject;
    static toObject(includeInstance: boolean, msg: Device): Device.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Device, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Device;
    static deserializeBinaryFromReader(message: Device, reader: jspb.BinaryReader): Device;
}

export namespace Device {
    export type AsObject = {
        apiVersion: string,
        id: string,
        osType: teleport_devicetrust_v1_os_type_pb.OSType,
        assetTag: string,
        createTime?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        updateTime?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        enrollToken?: teleport_devicetrust_v1_device_enroll_token_pb.DeviceEnrollToken.AsObject,
        enrollStatus: DeviceEnrollStatus,
        credential?: DeviceCredential.AsObject,
        collectedDataList: Array<teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData.AsObject>,
        source?: teleport_devicetrust_v1_device_source_pb.DeviceSource.AsObject,
        profile?: teleport_devicetrust_v1_device_profile_pb.DeviceProfile.AsObject,
        owner: string,
    }
}

export class DeviceCredential extends jspb.Message { 
    getId(): string;
    setId(value: string): DeviceCredential;
    getPublicKeyDer(): Uint8Array | string;
    getPublicKeyDer_asU8(): Uint8Array;
    getPublicKeyDer_asB64(): string;
    setPublicKeyDer(value: Uint8Array | string): DeviceCredential;
    getDeviceAttestationType(): DeviceAttestationType;
    setDeviceAttestationType(value: DeviceAttestationType): DeviceCredential;
    getTpmEkcertSerial(): string;
    setTpmEkcertSerial(value: string): DeviceCredential;
    getTpmAkPublic(): Uint8Array | string;
    getTpmAkPublic_asU8(): Uint8Array;
    getTpmAkPublic_asB64(): string;
    setTpmAkPublic(value: Uint8Array | string): DeviceCredential;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DeviceCredential.AsObject;
    static toObject(includeInstance: boolean, msg: DeviceCredential): DeviceCredential.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DeviceCredential, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DeviceCredential;
    static deserializeBinaryFromReader(message: DeviceCredential, reader: jspb.BinaryReader): DeviceCredential;
}

export namespace DeviceCredential {
    export type AsObject = {
        id: string,
        publicKeyDer: Uint8Array | string,
        deviceAttestationType: DeviceAttestationType,
        tpmEkcertSerial: string,
        tpmAkPublic: Uint8Array | string,
    }
}

export enum DeviceAttestationType {
    DEVICE_ATTESTATION_TYPE_UNSPECIFIED = 0,
    DEVICE_ATTESTATION_TYPE_TPM_EKPUB = 1,
    DEVICE_ATTESTATION_TYPE_TPM_EKCERT = 2,
    DEVICE_ATTESTATION_TYPE_TPM_EKCERT_TRUSTED = 3,
}

export enum DeviceEnrollStatus {
    DEVICE_ENROLL_STATUS_UNSPECIFIED = 0,
    DEVICE_ENROLL_STATUS_NOT_ENROLLED = 1,
    DEVICE_ENROLL_STATUS_ENROLLED = 2,
}
