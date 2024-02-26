// package: teleport.devicetrust.v1
// file: teleport/devicetrust/v1/devicetrust_service.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as google_protobuf_field_mask_pb from "google-protobuf/google/protobuf/field_mask_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";
import * as google_rpc_status_pb from "../../../google/rpc/status_pb";
import * as teleport_devicetrust_v1_device_pb from "../../../teleport/devicetrust/v1/device_pb";
import * as teleport_devicetrust_v1_device_collected_data_pb from "../../../teleport/devicetrust/v1/device_collected_data_pb";
import * as teleport_devicetrust_v1_device_enroll_token_pb from "../../../teleport/devicetrust/v1/device_enroll_token_pb";
import * as teleport_devicetrust_v1_device_source_pb from "../../../teleport/devicetrust/v1/device_source_pb";
import * as teleport_devicetrust_v1_device_web_token_pb from "../../../teleport/devicetrust/v1/device_web_token_pb";
import * as teleport_devicetrust_v1_tpm_pb from "../../../teleport/devicetrust/v1/tpm_pb";
import * as teleport_devicetrust_v1_usage_pb from "../../../teleport/devicetrust/v1/usage_pb";
import * as teleport_devicetrust_v1_user_certificates_pb from "../../../teleport/devicetrust/v1/user_certificates_pb";

export class CreateDeviceRequest extends jspb.Message { 

    hasDevice(): boolean;
    clearDevice(): void;
    getDevice(): teleport_devicetrust_v1_device_pb.Device | undefined;
    setDevice(value?: teleport_devicetrust_v1_device_pb.Device): CreateDeviceRequest;
    getCreateEnrollToken(): boolean;
    setCreateEnrollToken(value: boolean): CreateDeviceRequest;
    getCreateAsResource(): boolean;
    setCreateAsResource(value: boolean): CreateDeviceRequest;

    hasEnrollTokenExpireTime(): boolean;
    clearEnrollTokenExpireTime(): void;
    getEnrollTokenExpireTime(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setEnrollTokenExpireTime(value?: google_protobuf_timestamp_pb.Timestamp): CreateDeviceRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CreateDeviceRequest.AsObject;
    static toObject(includeInstance: boolean, msg: CreateDeviceRequest): CreateDeviceRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CreateDeviceRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CreateDeviceRequest;
    static deserializeBinaryFromReader(message: CreateDeviceRequest, reader: jspb.BinaryReader): CreateDeviceRequest;
}

export namespace CreateDeviceRequest {
    export type AsObject = {
        device?: teleport_devicetrust_v1_device_pb.Device.AsObject,
        createEnrollToken: boolean,
        createAsResource: boolean,
        enrollTokenExpireTime?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    }
}

export class UpdateDeviceRequest extends jspb.Message { 

    hasDevice(): boolean;
    clearDevice(): void;
    getDevice(): teleport_devicetrust_v1_device_pb.Device | undefined;
    setDevice(value?: teleport_devicetrust_v1_device_pb.Device): UpdateDeviceRequest;

    hasUpdateMask(): boolean;
    clearUpdateMask(): void;
    getUpdateMask(): google_protobuf_field_mask_pb.FieldMask | undefined;
    setUpdateMask(value?: google_protobuf_field_mask_pb.FieldMask): UpdateDeviceRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UpdateDeviceRequest.AsObject;
    static toObject(includeInstance: boolean, msg: UpdateDeviceRequest): UpdateDeviceRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UpdateDeviceRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UpdateDeviceRequest;
    static deserializeBinaryFromReader(message: UpdateDeviceRequest, reader: jspb.BinaryReader): UpdateDeviceRequest;
}

export namespace UpdateDeviceRequest {
    export type AsObject = {
        device?: teleport_devicetrust_v1_device_pb.Device.AsObject,
        updateMask?: google_protobuf_field_mask_pb.FieldMask.AsObject,
    }
}

export class UpsertDeviceRequest extends jspb.Message { 

    hasDevice(): boolean;
    clearDevice(): void;
    getDevice(): teleport_devicetrust_v1_device_pb.Device | undefined;
    setDevice(value?: teleport_devicetrust_v1_device_pb.Device): UpsertDeviceRequest;
    getCreateAsResource(): boolean;
    setCreateAsResource(value: boolean): UpsertDeviceRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UpsertDeviceRequest.AsObject;
    static toObject(includeInstance: boolean, msg: UpsertDeviceRequest): UpsertDeviceRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UpsertDeviceRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UpsertDeviceRequest;
    static deserializeBinaryFromReader(message: UpsertDeviceRequest, reader: jspb.BinaryReader): UpsertDeviceRequest;
}

export namespace UpsertDeviceRequest {
    export type AsObject = {
        device?: teleport_devicetrust_v1_device_pb.Device.AsObject,
        createAsResource: boolean,
    }
}

export class DeleteDeviceRequest extends jspb.Message { 
    getDeviceId(): string;
    setDeviceId(value: string): DeleteDeviceRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DeleteDeviceRequest.AsObject;
    static toObject(includeInstance: boolean, msg: DeleteDeviceRequest): DeleteDeviceRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DeleteDeviceRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DeleteDeviceRequest;
    static deserializeBinaryFromReader(message: DeleteDeviceRequest, reader: jspb.BinaryReader): DeleteDeviceRequest;
}

export namespace DeleteDeviceRequest {
    export type AsObject = {
        deviceId: string,
    }
}

export class FindDevicesRequest extends jspb.Message { 
    getIdOrTag(): string;
    setIdOrTag(value: string): FindDevicesRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): FindDevicesRequest.AsObject;
    static toObject(includeInstance: boolean, msg: FindDevicesRequest): FindDevicesRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: FindDevicesRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): FindDevicesRequest;
    static deserializeBinaryFromReader(message: FindDevicesRequest, reader: jspb.BinaryReader): FindDevicesRequest;
}

export namespace FindDevicesRequest {
    export type AsObject = {
        idOrTag: string,
    }
}

export class FindDevicesResponse extends jspb.Message { 
    clearDevicesList(): void;
    getDevicesList(): Array<teleport_devicetrust_v1_device_pb.Device>;
    setDevicesList(value: Array<teleport_devicetrust_v1_device_pb.Device>): FindDevicesResponse;
    addDevices(value?: teleport_devicetrust_v1_device_pb.Device, index?: number): teleport_devicetrust_v1_device_pb.Device;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): FindDevicesResponse.AsObject;
    static toObject(includeInstance: boolean, msg: FindDevicesResponse): FindDevicesResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: FindDevicesResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): FindDevicesResponse;
    static deserializeBinaryFromReader(message: FindDevicesResponse, reader: jspb.BinaryReader): FindDevicesResponse;
}

export namespace FindDevicesResponse {
    export type AsObject = {
        devicesList: Array<teleport_devicetrust_v1_device_pb.Device.AsObject>,
    }
}

export class GetDeviceRequest extends jspb.Message { 
    getDeviceId(): string;
    setDeviceId(value: string): GetDeviceRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetDeviceRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetDeviceRequest): GetDeviceRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetDeviceRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetDeviceRequest;
    static deserializeBinaryFromReader(message: GetDeviceRequest, reader: jspb.BinaryReader): GetDeviceRequest;
}

export namespace GetDeviceRequest {
    export type AsObject = {
        deviceId: string,
    }
}

export class ListDevicesRequest extends jspb.Message { 
    getPageSize(): number;
    setPageSize(value: number): ListDevicesRequest;
    getPageToken(): string;
    setPageToken(value: string): ListDevicesRequest;
    getView(): DeviceView;
    setView(value: DeviceView): ListDevicesRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListDevicesRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ListDevicesRequest): ListDevicesRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListDevicesRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListDevicesRequest;
    static deserializeBinaryFromReader(message: ListDevicesRequest, reader: jspb.BinaryReader): ListDevicesRequest;
}

export namespace ListDevicesRequest {
    export type AsObject = {
        pageSize: number,
        pageToken: string,
        view: DeviceView,
    }
}

export class ListDevicesResponse extends jspb.Message { 
    clearDevicesList(): void;
    getDevicesList(): Array<teleport_devicetrust_v1_device_pb.Device>;
    setDevicesList(value: Array<teleport_devicetrust_v1_device_pb.Device>): ListDevicesResponse;
    addDevices(value?: teleport_devicetrust_v1_device_pb.Device, index?: number): teleport_devicetrust_v1_device_pb.Device;
    getNextPageToken(): string;
    setNextPageToken(value: string): ListDevicesResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListDevicesResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ListDevicesResponse): ListDevicesResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListDevicesResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListDevicesResponse;
    static deserializeBinaryFromReader(message: ListDevicesResponse, reader: jspb.BinaryReader): ListDevicesResponse;
}

export namespace ListDevicesResponse {
    export type AsObject = {
        devicesList: Array<teleport_devicetrust_v1_device_pb.Device.AsObject>,
        nextPageToken: string,
    }
}

export class BulkCreateDevicesRequest extends jspb.Message { 
    clearDevicesList(): void;
    getDevicesList(): Array<teleport_devicetrust_v1_device_pb.Device>;
    setDevicesList(value: Array<teleport_devicetrust_v1_device_pb.Device>): BulkCreateDevicesRequest;
    addDevices(value?: teleport_devicetrust_v1_device_pb.Device, index?: number): teleport_devicetrust_v1_device_pb.Device;
    getCreateAsResource(): boolean;
    setCreateAsResource(value: boolean): BulkCreateDevicesRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): BulkCreateDevicesRequest.AsObject;
    static toObject(includeInstance: boolean, msg: BulkCreateDevicesRequest): BulkCreateDevicesRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: BulkCreateDevicesRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): BulkCreateDevicesRequest;
    static deserializeBinaryFromReader(message: BulkCreateDevicesRequest, reader: jspb.BinaryReader): BulkCreateDevicesRequest;
}

export namespace BulkCreateDevicesRequest {
    export type AsObject = {
        devicesList: Array<teleport_devicetrust_v1_device_pb.Device.AsObject>,
        createAsResource: boolean,
    }
}

export class BulkCreateDevicesResponse extends jspb.Message { 
    clearDevicesList(): void;
    getDevicesList(): Array<DeviceOrStatus>;
    setDevicesList(value: Array<DeviceOrStatus>): BulkCreateDevicesResponse;
    addDevices(value?: DeviceOrStatus, index?: number): DeviceOrStatus;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): BulkCreateDevicesResponse.AsObject;
    static toObject(includeInstance: boolean, msg: BulkCreateDevicesResponse): BulkCreateDevicesResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: BulkCreateDevicesResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): BulkCreateDevicesResponse;
    static deserializeBinaryFromReader(message: BulkCreateDevicesResponse, reader: jspb.BinaryReader): BulkCreateDevicesResponse;
}

export namespace BulkCreateDevicesResponse {
    export type AsObject = {
        devicesList: Array<DeviceOrStatus.AsObject>,
    }
}

export class DeviceOrStatus extends jspb.Message { 

    hasStatus(): boolean;
    clearStatus(): void;
    getStatus(): google_rpc_status_pb.Status | undefined;
    setStatus(value?: google_rpc_status_pb.Status): DeviceOrStatus;
    getId(): string;
    setId(value: string): DeviceOrStatus;
    getDeleted(): boolean;
    setDeleted(value: boolean): DeviceOrStatus;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DeviceOrStatus.AsObject;
    static toObject(includeInstance: boolean, msg: DeviceOrStatus): DeviceOrStatus.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DeviceOrStatus, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DeviceOrStatus;
    static deserializeBinaryFromReader(message: DeviceOrStatus, reader: jspb.BinaryReader): DeviceOrStatus;
}

export namespace DeviceOrStatus {
    export type AsObject = {
        status?: google_rpc_status_pb.Status.AsObject,
        id: string,
        deleted: boolean,
    }
}

export class CreateDeviceEnrollTokenRequest extends jspb.Message { 
    getDeviceId(): string;
    setDeviceId(value: string): CreateDeviceEnrollTokenRequest;

    hasDeviceData(): boolean;
    clearDeviceData(): void;
    getDeviceData(): teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData | undefined;
    setDeviceData(value?: teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData): CreateDeviceEnrollTokenRequest;

    hasExpireTime(): boolean;
    clearExpireTime(): void;
    getExpireTime(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setExpireTime(value?: google_protobuf_timestamp_pb.Timestamp): CreateDeviceEnrollTokenRequest;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CreateDeviceEnrollTokenRequest.AsObject;
    static toObject(includeInstance: boolean, msg: CreateDeviceEnrollTokenRequest): CreateDeviceEnrollTokenRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CreateDeviceEnrollTokenRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CreateDeviceEnrollTokenRequest;
    static deserializeBinaryFromReader(message: CreateDeviceEnrollTokenRequest, reader: jspb.BinaryReader): CreateDeviceEnrollTokenRequest;
}

export namespace CreateDeviceEnrollTokenRequest {
    export type AsObject = {
        deviceId: string,
        deviceData?: teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData.AsObject,
        expireTime?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    }
}

export class EnrollDeviceRequest extends jspb.Message { 

    hasInit(): boolean;
    clearInit(): void;
    getInit(): EnrollDeviceInit | undefined;
    setInit(value?: EnrollDeviceInit): EnrollDeviceRequest;

    hasMacosChallengeResponse(): boolean;
    clearMacosChallengeResponse(): void;
    getMacosChallengeResponse(): MacOSEnrollChallengeResponse | undefined;
    setMacosChallengeResponse(value?: MacOSEnrollChallengeResponse): EnrollDeviceRequest;

    hasTpmChallengeResponse(): boolean;
    clearTpmChallengeResponse(): void;
    getTpmChallengeResponse(): TPMEnrollChallengeResponse | undefined;
    setTpmChallengeResponse(value?: TPMEnrollChallengeResponse): EnrollDeviceRequest;

    getPayloadCase(): EnrollDeviceRequest.PayloadCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): EnrollDeviceRequest.AsObject;
    static toObject(includeInstance: boolean, msg: EnrollDeviceRequest): EnrollDeviceRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: EnrollDeviceRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): EnrollDeviceRequest;
    static deserializeBinaryFromReader(message: EnrollDeviceRequest, reader: jspb.BinaryReader): EnrollDeviceRequest;
}

export namespace EnrollDeviceRequest {
    export type AsObject = {
        init?: EnrollDeviceInit.AsObject,
        macosChallengeResponse?: MacOSEnrollChallengeResponse.AsObject,
        tpmChallengeResponse?: TPMEnrollChallengeResponse.AsObject,
    }

    export enum PayloadCase {
        PAYLOAD_NOT_SET = 0,
        INIT = 1,
        MACOS_CHALLENGE_RESPONSE = 2,
        TPM_CHALLENGE_RESPONSE = 3,
    }

}

export class EnrollDeviceResponse extends jspb.Message { 

    hasSuccess(): boolean;
    clearSuccess(): void;
    getSuccess(): EnrollDeviceSuccess | undefined;
    setSuccess(value?: EnrollDeviceSuccess): EnrollDeviceResponse;

    hasMacosChallenge(): boolean;
    clearMacosChallenge(): void;
    getMacosChallenge(): MacOSEnrollChallenge | undefined;
    setMacosChallenge(value?: MacOSEnrollChallenge): EnrollDeviceResponse;

    hasTpmChallenge(): boolean;
    clearTpmChallenge(): void;
    getTpmChallenge(): TPMEnrollChallenge | undefined;
    setTpmChallenge(value?: TPMEnrollChallenge): EnrollDeviceResponse;

    getPayloadCase(): EnrollDeviceResponse.PayloadCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): EnrollDeviceResponse.AsObject;
    static toObject(includeInstance: boolean, msg: EnrollDeviceResponse): EnrollDeviceResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: EnrollDeviceResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): EnrollDeviceResponse;
    static deserializeBinaryFromReader(message: EnrollDeviceResponse, reader: jspb.BinaryReader): EnrollDeviceResponse;
}

export namespace EnrollDeviceResponse {
    export type AsObject = {
        success?: EnrollDeviceSuccess.AsObject,
        macosChallenge?: MacOSEnrollChallenge.AsObject,
        tpmChallenge?: TPMEnrollChallenge.AsObject,
    }

    export enum PayloadCase {
        PAYLOAD_NOT_SET = 0,
        SUCCESS = 1,
        MACOS_CHALLENGE = 2,
        TPM_CHALLENGE = 3,
    }

}

export class EnrollDeviceInit extends jspb.Message { 
    getToken(): string;
    setToken(value: string): EnrollDeviceInit;
    getCredentialId(): string;
    setCredentialId(value: string): EnrollDeviceInit;

    hasDeviceData(): boolean;
    clearDeviceData(): void;
    getDeviceData(): teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData | undefined;
    setDeviceData(value?: teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData): EnrollDeviceInit;

    hasMacos(): boolean;
    clearMacos(): void;
    getMacos(): MacOSEnrollPayload | undefined;
    setMacos(value?: MacOSEnrollPayload): EnrollDeviceInit;

    hasTpm(): boolean;
    clearTpm(): void;
    getTpm(): TPMEnrollPayload | undefined;
    setTpm(value?: TPMEnrollPayload): EnrollDeviceInit;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): EnrollDeviceInit.AsObject;
    static toObject(includeInstance: boolean, msg: EnrollDeviceInit): EnrollDeviceInit.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: EnrollDeviceInit, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): EnrollDeviceInit;
    static deserializeBinaryFromReader(message: EnrollDeviceInit, reader: jspb.BinaryReader): EnrollDeviceInit;
}

export namespace EnrollDeviceInit {
    export type AsObject = {
        token: string,
        credentialId: string,
        deviceData?: teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData.AsObject,
        macos?: MacOSEnrollPayload.AsObject,
        tpm?: TPMEnrollPayload.AsObject,
    }
}

export class EnrollDeviceSuccess extends jspb.Message { 

    hasDevice(): boolean;
    clearDevice(): void;
    getDevice(): teleport_devicetrust_v1_device_pb.Device | undefined;
    setDevice(value?: teleport_devicetrust_v1_device_pb.Device): EnrollDeviceSuccess;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): EnrollDeviceSuccess.AsObject;
    static toObject(includeInstance: boolean, msg: EnrollDeviceSuccess): EnrollDeviceSuccess.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: EnrollDeviceSuccess, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): EnrollDeviceSuccess;
    static deserializeBinaryFromReader(message: EnrollDeviceSuccess, reader: jspb.BinaryReader): EnrollDeviceSuccess;
}

export namespace EnrollDeviceSuccess {
    export type AsObject = {
        device?: teleport_devicetrust_v1_device_pb.Device.AsObject,
    }
}

export class MacOSEnrollPayload extends jspb.Message { 
    getPublicKeyDer(): Uint8Array | string;
    getPublicKeyDer_asU8(): Uint8Array;
    getPublicKeyDer_asB64(): string;
    setPublicKeyDer(value: Uint8Array | string): MacOSEnrollPayload;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): MacOSEnrollPayload.AsObject;
    static toObject(includeInstance: boolean, msg: MacOSEnrollPayload): MacOSEnrollPayload.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: MacOSEnrollPayload, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): MacOSEnrollPayload;
    static deserializeBinaryFromReader(message: MacOSEnrollPayload, reader: jspb.BinaryReader): MacOSEnrollPayload;
}

export namespace MacOSEnrollPayload {
    export type AsObject = {
        publicKeyDer: Uint8Array | string,
    }
}

export class MacOSEnrollChallenge extends jspb.Message { 
    getChallenge(): Uint8Array | string;
    getChallenge_asU8(): Uint8Array;
    getChallenge_asB64(): string;
    setChallenge(value: Uint8Array | string): MacOSEnrollChallenge;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): MacOSEnrollChallenge.AsObject;
    static toObject(includeInstance: boolean, msg: MacOSEnrollChallenge): MacOSEnrollChallenge.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: MacOSEnrollChallenge, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): MacOSEnrollChallenge;
    static deserializeBinaryFromReader(message: MacOSEnrollChallenge, reader: jspb.BinaryReader): MacOSEnrollChallenge;
}

export namespace MacOSEnrollChallenge {
    export type AsObject = {
        challenge: Uint8Array | string,
    }
}

export class MacOSEnrollChallengeResponse extends jspb.Message { 
    getSignature(): Uint8Array | string;
    getSignature_asU8(): Uint8Array;
    getSignature_asB64(): string;
    setSignature(value: Uint8Array | string): MacOSEnrollChallengeResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): MacOSEnrollChallengeResponse.AsObject;
    static toObject(includeInstance: boolean, msg: MacOSEnrollChallengeResponse): MacOSEnrollChallengeResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: MacOSEnrollChallengeResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): MacOSEnrollChallengeResponse;
    static deserializeBinaryFromReader(message: MacOSEnrollChallengeResponse, reader: jspb.BinaryReader): MacOSEnrollChallengeResponse;
}

export namespace MacOSEnrollChallengeResponse {
    export type AsObject = {
        signature: Uint8Array | string,
    }
}

export class TPMEnrollPayload extends jspb.Message { 

    hasEkCert(): boolean;
    clearEkCert(): void;
    getEkCert(): Uint8Array | string;
    getEkCert_asU8(): Uint8Array;
    getEkCert_asB64(): string;
    setEkCert(value: Uint8Array | string): TPMEnrollPayload;

    hasEkKey(): boolean;
    clearEkKey(): void;
    getEkKey(): Uint8Array | string;
    getEkKey_asU8(): Uint8Array;
    getEkKey_asB64(): string;
    setEkKey(value: Uint8Array | string): TPMEnrollPayload;

    hasAttestationParameters(): boolean;
    clearAttestationParameters(): void;
    getAttestationParameters(): TPMAttestationParameters | undefined;
    setAttestationParameters(value?: TPMAttestationParameters): TPMEnrollPayload;

    getEkCase(): TPMEnrollPayload.EkCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TPMEnrollPayload.AsObject;
    static toObject(includeInstance: boolean, msg: TPMEnrollPayload): TPMEnrollPayload.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TPMEnrollPayload, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TPMEnrollPayload;
    static deserializeBinaryFromReader(message: TPMEnrollPayload, reader: jspb.BinaryReader): TPMEnrollPayload;
}

export namespace TPMEnrollPayload {
    export type AsObject = {
        ekCert: Uint8Array | string,
        ekKey: Uint8Array | string,
        attestationParameters?: TPMAttestationParameters.AsObject,
    }

    export enum EkCase {
        EK_NOT_SET = 0,
        EK_CERT = 1,
        EK_KEY = 2,
    }

}

export class TPMAttestationParameters extends jspb.Message { 
    getPublic(): Uint8Array | string;
    getPublic_asU8(): Uint8Array;
    getPublic_asB64(): string;
    setPublic(value: Uint8Array | string): TPMAttestationParameters;
    getCreateData(): Uint8Array | string;
    getCreateData_asU8(): Uint8Array;
    getCreateData_asB64(): string;
    setCreateData(value: Uint8Array | string): TPMAttestationParameters;
    getCreateAttestation(): Uint8Array | string;
    getCreateAttestation_asU8(): Uint8Array;
    getCreateAttestation_asB64(): string;
    setCreateAttestation(value: Uint8Array | string): TPMAttestationParameters;
    getCreateSignature(): Uint8Array | string;
    getCreateSignature_asU8(): Uint8Array;
    getCreateSignature_asB64(): string;
    setCreateSignature(value: Uint8Array | string): TPMAttestationParameters;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TPMAttestationParameters.AsObject;
    static toObject(includeInstance: boolean, msg: TPMAttestationParameters): TPMAttestationParameters.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TPMAttestationParameters, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TPMAttestationParameters;
    static deserializeBinaryFromReader(message: TPMAttestationParameters, reader: jspb.BinaryReader): TPMAttestationParameters;
}

export namespace TPMAttestationParameters {
    export type AsObject = {
        pb_public: Uint8Array | string,
        createData: Uint8Array | string,
        createAttestation: Uint8Array | string,
        createSignature: Uint8Array | string,
    }
}

export class TPMEnrollChallenge extends jspb.Message { 

    hasEncryptedCredential(): boolean;
    clearEncryptedCredential(): void;
    getEncryptedCredential(): TPMEncryptedCredential | undefined;
    setEncryptedCredential(value?: TPMEncryptedCredential): TPMEnrollChallenge;
    getAttestationNonce(): Uint8Array | string;
    getAttestationNonce_asU8(): Uint8Array;
    getAttestationNonce_asB64(): string;
    setAttestationNonce(value: Uint8Array | string): TPMEnrollChallenge;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TPMEnrollChallenge.AsObject;
    static toObject(includeInstance: boolean, msg: TPMEnrollChallenge): TPMEnrollChallenge.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TPMEnrollChallenge, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TPMEnrollChallenge;
    static deserializeBinaryFromReader(message: TPMEnrollChallenge, reader: jspb.BinaryReader): TPMEnrollChallenge;
}

export namespace TPMEnrollChallenge {
    export type AsObject = {
        encryptedCredential?: TPMEncryptedCredential.AsObject,
        attestationNonce: Uint8Array | string,
    }
}

export class TPMEncryptedCredential extends jspb.Message { 
    getCredentialBlob(): Uint8Array | string;
    getCredentialBlob_asU8(): Uint8Array;
    getCredentialBlob_asB64(): string;
    setCredentialBlob(value: Uint8Array | string): TPMEncryptedCredential;
    getSecret(): Uint8Array | string;
    getSecret_asU8(): Uint8Array;
    getSecret_asB64(): string;
    setSecret(value: Uint8Array | string): TPMEncryptedCredential;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TPMEncryptedCredential.AsObject;
    static toObject(includeInstance: boolean, msg: TPMEncryptedCredential): TPMEncryptedCredential.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TPMEncryptedCredential, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TPMEncryptedCredential;
    static deserializeBinaryFromReader(message: TPMEncryptedCredential, reader: jspb.BinaryReader): TPMEncryptedCredential;
}

export namespace TPMEncryptedCredential {
    export type AsObject = {
        credentialBlob: Uint8Array | string,
        secret: Uint8Array | string,
    }
}

export class TPMEnrollChallengeResponse extends jspb.Message { 
    getSolution(): Uint8Array | string;
    getSolution_asU8(): Uint8Array;
    getSolution_asB64(): string;
    setSolution(value: Uint8Array | string): TPMEnrollChallengeResponse;

    hasPlatformParameters(): boolean;
    clearPlatformParameters(): void;
    getPlatformParameters(): teleport_devicetrust_v1_tpm_pb.TPMPlatformParameters | undefined;
    setPlatformParameters(value?: teleport_devicetrust_v1_tpm_pb.TPMPlatformParameters): TPMEnrollChallengeResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TPMEnrollChallengeResponse.AsObject;
    static toObject(includeInstance: boolean, msg: TPMEnrollChallengeResponse): TPMEnrollChallengeResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TPMEnrollChallengeResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TPMEnrollChallengeResponse;
    static deserializeBinaryFromReader(message: TPMEnrollChallengeResponse, reader: jspb.BinaryReader): TPMEnrollChallengeResponse;
}

export namespace TPMEnrollChallengeResponse {
    export type AsObject = {
        solution: Uint8Array | string,
        platformParameters?: teleport_devicetrust_v1_tpm_pb.TPMPlatformParameters.AsObject,
    }
}

export class AuthenticateDeviceRequest extends jspb.Message { 

    hasInit(): boolean;
    clearInit(): void;
    getInit(): AuthenticateDeviceInit | undefined;
    setInit(value?: AuthenticateDeviceInit): AuthenticateDeviceRequest;

    hasChallengeResponse(): boolean;
    clearChallengeResponse(): void;
    getChallengeResponse(): AuthenticateDeviceChallengeResponse | undefined;
    setChallengeResponse(value?: AuthenticateDeviceChallengeResponse): AuthenticateDeviceRequest;

    hasTpmChallengeResponse(): boolean;
    clearTpmChallengeResponse(): void;
    getTpmChallengeResponse(): TPMAuthenticateDeviceChallengeResponse | undefined;
    setTpmChallengeResponse(value?: TPMAuthenticateDeviceChallengeResponse): AuthenticateDeviceRequest;

    getPayloadCase(): AuthenticateDeviceRequest.PayloadCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AuthenticateDeviceRequest.AsObject;
    static toObject(includeInstance: boolean, msg: AuthenticateDeviceRequest): AuthenticateDeviceRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AuthenticateDeviceRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AuthenticateDeviceRequest;
    static deserializeBinaryFromReader(message: AuthenticateDeviceRequest, reader: jspb.BinaryReader): AuthenticateDeviceRequest;
}

export namespace AuthenticateDeviceRequest {
    export type AsObject = {
        init?: AuthenticateDeviceInit.AsObject,
        challengeResponse?: AuthenticateDeviceChallengeResponse.AsObject,
        tpmChallengeResponse?: TPMAuthenticateDeviceChallengeResponse.AsObject,
    }

    export enum PayloadCase {
        PAYLOAD_NOT_SET = 0,
        INIT = 1,
        CHALLENGE_RESPONSE = 2,
        TPM_CHALLENGE_RESPONSE = 3,
    }

}

export class AuthenticateDeviceResponse extends jspb.Message { 

    hasChallenge(): boolean;
    clearChallenge(): void;
    getChallenge(): AuthenticateDeviceChallenge | undefined;
    setChallenge(value?: AuthenticateDeviceChallenge): AuthenticateDeviceResponse;

    hasUserCertificates(): boolean;
    clearUserCertificates(): void;
    getUserCertificates(): teleport_devicetrust_v1_user_certificates_pb.UserCertificates | undefined;
    setUserCertificates(value?: teleport_devicetrust_v1_user_certificates_pb.UserCertificates): AuthenticateDeviceResponse;

    hasTpmChallenge(): boolean;
    clearTpmChallenge(): void;
    getTpmChallenge(): TPMAuthenticateDeviceChallenge | undefined;
    setTpmChallenge(value?: TPMAuthenticateDeviceChallenge): AuthenticateDeviceResponse;

    getPayloadCase(): AuthenticateDeviceResponse.PayloadCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AuthenticateDeviceResponse.AsObject;
    static toObject(includeInstance: boolean, msg: AuthenticateDeviceResponse): AuthenticateDeviceResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AuthenticateDeviceResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AuthenticateDeviceResponse;
    static deserializeBinaryFromReader(message: AuthenticateDeviceResponse, reader: jspb.BinaryReader): AuthenticateDeviceResponse;
}

export namespace AuthenticateDeviceResponse {
    export type AsObject = {
        challenge?: AuthenticateDeviceChallenge.AsObject,
        userCertificates?: teleport_devicetrust_v1_user_certificates_pb.UserCertificates.AsObject,
        tpmChallenge?: TPMAuthenticateDeviceChallenge.AsObject,
    }

    export enum PayloadCase {
        PAYLOAD_NOT_SET = 0,
        CHALLENGE = 1,
        USER_CERTIFICATES = 2,
        TPM_CHALLENGE = 3,
    }

}

export class AuthenticateDeviceInit extends jspb.Message { 

    hasUserCertificates(): boolean;
    clearUserCertificates(): void;
    getUserCertificates(): teleport_devicetrust_v1_user_certificates_pb.UserCertificates | undefined;
    setUserCertificates(value?: teleport_devicetrust_v1_user_certificates_pb.UserCertificates): AuthenticateDeviceInit;
    getCredentialId(): string;
    setCredentialId(value: string): AuthenticateDeviceInit;

    hasDeviceData(): boolean;
    clearDeviceData(): void;
    getDeviceData(): teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData | undefined;
    setDeviceData(value?: teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData): AuthenticateDeviceInit;

    hasDeviceWebToken(): boolean;
    clearDeviceWebToken(): void;
    getDeviceWebToken(): teleport_devicetrust_v1_device_web_token_pb.DeviceWebToken | undefined;
    setDeviceWebToken(value?: teleport_devicetrust_v1_device_web_token_pb.DeviceWebToken): AuthenticateDeviceInit;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AuthenticateDeviceInit.AsObject;
    static toObject(includeInstance: boolean, msg: AuthenticateDeviceInit): AuthenticateDeviceInit.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AuthenticateDeviceInit, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AuthenticateDeviceInit;
    static deserializeBinaryFromReader(message: AuthenticateDeviceInit, reader: jspb.BinaryReader): AuthenticateDeviceInit;
}

export namespace AuthenticateDeviceInit {
    export type AsObject = {
        userCertificates?: teleport_devicetrust_v1_user_certificates_pb.UserCertificates.AsObject,
        credentialId: string,
        deviceData?: teleport_devicetrust_v1_device_collected_data_pb.DeviceCollectedData.AsObject,
        deviceWebToken?: teleport_devicetrust_v1_device_web_token_pb.DeviceWebToken.AsObject,
    }
}

export class TPMAuthenticateDeviceChallenge extends jspb.Message { 
    getAttestationNonce(): Uint8Array | string;
    getAttestationNonce_asU8(): Uint8Array;
    getAttestationNonce_asB64(): string;
    setAttestationNonce(value: Uint8Array | string): TPMAuthenticateDeviceChallenge;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TPMAuthenticateDeviceChallenge.AsObject;
    static toObject(includeInstance: boolean, msg: TPMAuthenticateDeviceChallenge): TPMAuthenticateDeviceChallenge.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TPMAuthenticateDeviceChallenge, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TPMAuthenticateDeviceChallenge;
    static deserializeBinaryFromReader(message: TPMAuthenticateDeviceChallenge, reader: jspb.BinaryReader): TPMAuthenticateDeviceChallenge;
}

export namespace TPMAuthenticateDeviceChallenge {
    export type AsObject = {
        attestationNonce: Uint8Array | string,
    }
}

export class TPMAuthenticateDeviceChallengeResponse extends jspb.Message { 

    hasPlatformParameters(): boolean;
    clearPlatformParameters(): void;
    getPlatformParameters(): teleport_devicetrust_v1_tpm_pb.TPMPlatformParameters | undefined;
    setPlatformParameters(value?: teleport_devicetrust_v1_tpm_pb.TPMPlatformParameters): TPMAuthenticateDeviceChallengeResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TPMAuthenticateDeviceChallengeResponse.AsObject;
    static toObject(includeInstance: boolean, msg: TPMAuthenticateDeviceChallengeResponse): TPMAuthenticateDeviceChallengeResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TPMAuthenticateDeviceChallengeResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TPMAuthenticateDeviceChallengeResponse;
    static deserializeBinaryFromReader(message: TPMAuthenticateDeviceChallengeResponse, reader: jspb.BinaryReader): TPMAuthenticateDeviceChallengeResponse;
}

export namespace TPMAuthenticateDeviceChallengeResponse {
    export type AsObject = {
        platformParameters?: teleport_devicetrust_v1_tpm_pb.TPMPlatformParameters.AsObject,
    }
}

export class AuthenticateDeviceChallenge extends jspb.Message { 
    getChallenge(): Uint8Array | string;
    getChallenge_asU8(): Uint8Array;
    getChallenge_asB64(): string;
    setChallenge(value: Uint8Array | string): AuthenticateDeviceChallenge;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AuthenticateDeviceChallenge.AsObject;
    static toObject(includeInstance: boolean, msg: AuthenticateDeviceChallenge): AuthenticateDeviceChallenge.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AuthenticateDeviceChallenge, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AuthenticateDeviceChallenge;
    static deserializeBinaryFromReader(message: AuthenticateDeviceChallenge, reader: jspb.BinaryReader): AuthenticateDeviceChallenge;
}

export namespace AuthenticateDeviceChallenge {
    export type AsObject = {
        challenge: Uint8Array | string,
    }
}

export class AuthenticateDeviceChallengeResponse extends jspb.Message { 
    getSignature(): Uint8Array | string;
    getSignature_asU8(): Uint8Array;
    getSignature_asB64(): string;
    setSignature(value: Uint8Array | string): AuthenticateDeviceChallengeResponse;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AuthenticateDeviceChallengeResponse.AsObject;
    static toObject(includeInstance: boolean, msg: AuthenticateDeviceChallengeResponse): AuthenticateDeviceChallengeResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AuthenticateDeviceChallengeResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AuthenticateDeviceChallengeResponse;
    static deserializeBinaryFromReader(message: AuthenticateDeviceChallengeResponse, reader: jspb.BinaryReader): AuthenticateDeviceChallengeResponse;
}

export namespace AuthenticateDeviceChallengeResponse {
    export type AsObject = {
        signature: Uint8Array | string,
    }
}

export class SyncInventoryRequest extends jspb.Message { 

    hasStart(): boolean;
    clearStart(): void;
    getStart(): SyncInventoryStart | undefined;
    setStart(value?: SyncInventoryStart): SyncInventoryRequest;

    hasEnd(): boolean;
    clearEnd(): void;
    getEnd(): SyncInventoryEnd | undefined;
    setEnd(value?: SyncInventoryEnd): SyncInventoryRequest;

    hasDevicesToUpsert(): boolean;
    clearDevicesToUpsert(): void;
    getDevicesToUpsert(): SyncInventoryDevices | undefined;
    setDevicesToUpsert(value?: SyncInventoryDevices): SyncInventoryRequest;

    hasDevicesToRemove(): boolean;
    clearDevicesToRemove(): void;
    getDevicesToRemove(): SyncInventoryDevices | undefined;
    setDevicesToRemove(value?: SyncInventoryDevices): SyncInventoryRequest;

    getPayloadCase(): SyncInventoryRequest.PayloadCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SyncInventoryRequest.AsObject;
    static toObject(includeInstance: boolean, msg: SyncInventoryRequest): SyncInventoryRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SyncInventoryRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SyncInventoryRequest;
    static deserializeBinaryFromReader(message: SyncInventoryRequest, reader: jspb.BinaryReader): SyncInventoryRequest;
}

export namespace SyncInventoryRequest {
    export type AsObject = {
        start?: SyncInventoryStart.AsObject,
        end?: SyncInventoryEnd.AsObject,
        devicesToUpsert?: SyncInventoryDevices.AsObject,
        devicesToRemove?: SyncInventoryDevices.AsObject,
    }

    export enum PayloadCase {
        PAYLOAD_NOT_SET = 0,
        START = 1,
        END = 2,
        DEVICES_TO_UPSERT = 3,
        DEVICES_TO_REMOVE = 4,
    }

}

export class SyncInventoryResponse extends jspb.Message { 

    hasAck(): boolean;
    clearAck(): void;
    getAck(): SyncInventoryAck | undefined;
    setAck(value?: SyncInventoryAck): SyncInventoryResponse;

    hasResult(): boolean;
    clearResult(): void;
    getResult(): SyncInventoryResult | undefined;
    setResult(value?: SyncInventoryResult): SyncInventoryResponse;

    hasMissingDevices(): boolean;
    clearMissingDevices(): void;
    getMissingDevices(): SyncInventoryMissingDevices | undefined;
    setMissingDevices(value?: SyncInventoryMissingDevices): SyncInventoryResponse;

    getPayloadCase(): SyncInventoryResponse.PayloadCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SyncInventoryResponse.AsObject;
    static toObject(includeInstance: boolean, msg: SyncInventoryResponse): SyncInventoryResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SyncInventoryResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SyncInventoryResponse;
    static deserializeBinaryFromReader(message: SyncInventoryResponse, reader: jspb.BinaryReader): SyncInventoryResponse;
}

export namespace SyncInventoryResponse {
    export type AsObject = {
        ack?: SyncInventoryAck.AsObject,
        result?: SyncInventoryResult.AsObject,
        missingDevices?: SyncInventoryMissingDevices.AsObject,
    }

    export enum PayloadCase {
        PAYLOAD_NOT_SET = 0,
        ACK = 1,
        RESULT = 2,
        MISSING_DEVICES = 3,
    }

}

export class SyncInventoryStart extends jspb.Message { 

    hasSource(): boolean;
    clearSource(): void;
    getSource(): teleport_devicetrust_v1_device_source_pb.DeviceSource | undefined;
    setSource(value?: teleport_devicetrust_v1_device_source_pb.DeviceSource): SyncInventoryStart;
    getTrackMissingDevices(): boolean;
    setTrackMissingDevices(value: boolean): SyncInventoryStart;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SyncInventoryStart.AsObject;
    static toObject(includeInstance: boolean, msg: SyncInventoryStart): SyncInventoryStart.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SyncInventoryStart, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SyncInventoryStart;
    static deserializeBinaryFromReader(message: SyncInventoryStart, reader: jspb.BinaryReader): SyncInventoryStart;
}

export namespace SyncInventoryStart {
    export type AsObject = {
        source?: teleport_devicetrust_v1_device_source_pb.DeviceSource.AsObject,
        trackMissingDevices: boolean,
    }
}

export class SyncInventoryEnd extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SyncInventoryEnd.AsObject;
    static toObject(includeInstance: boolean, msg: SyncInventoryEnd): SyncInventoryEnd.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SyncInventoryEnd, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SyncInventoryEnd;
    static deserializeBinaryFromReader(message: SyncInventoryEnd, reader: jspb.BinaryReader): SyncInventoryEnd;
}

export namespace SyncInventoryEnd {
    export type AsObject = {
    }
}

export class SyncInventoryDevices extends jspb.Message { 
    clearDevicesList(): void;
    getDevicesList(): Array<teleport_devicetrust_v1_device_pb.Device>;
    setDevicesList(value: Array<teleport_devicetrust_v1_device_pb.Device>): SyncInventoryDevices;
    addDevices(value?: teleport_devicetrust_v1_device_pb.Device, index?: number): teleport_devicetrust_v1_device_pb.Device;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SyncInventoryDevices.AsObject;
    static toObject(includeInstance: boolean, msg: SyncInventoryDevices): SyncInventoryDevices.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SyncInventoryDevices, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SyncInventoryDevices;
    static deserializeBinaryFromReader(message: SyncInventoryDevices, reader: jspb.BinaryReader): SyncInventoryDevices;
}

export namespace SyncInventoryDevices {
    export type AsObject = {
        devicesList: Array<teleport_devicetrust_v1_device_pb.Device.AsObject>,
    }
}

export class SyncInventoryAck extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SyncInventoryAck.AsObject;
    static toObject(includeInstance: boolean, msg: SyncInventoryAck): SyncInventoryAck.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SyncInventoryAck, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SyncInventoryAck;
    static deserializeBinaryFromReader(message: SyncInventoryAck, reader: jspb.BinaryReader): SyncInventoryAck;
}

export namespace SyncInventoryAck {
    export type AsObject = {
    }
}

export class SyncInventoryResult extends jspb.Message { 
    clearDevicesList(): void;
    getDevicesList(): Array<DeviceOrStatus>;
    setDevicesList(value: Array<DeviceOrStatus>): SyncInventoryResult;
    addDevices(value?: DeviceOrStatus, index?: number): DeviceOrStatus;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SyncInventoryResult.AsObject;
    static toObject(includeInstance: boolean, msg: SyncInventoryResult): SyncInventoryResult.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SyncInventoryResult, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SyncInventoryResult;
    static deserializeBinaryFromReader(message: SyncInventoryResult, reader: jspb.BinaryReader): SyncInventoryResult;
}

export namespace SyncInventoryResult {
    export type AsObject = {
        devicesList: Array<DeviceOrStatus.AsObject>,
    }
}

export class SyncInventoryMissingDevices extends jspb.Message { 
    clearDevicesList(): void;
    getDevicesList(): Array<teleport_devicetrust_v1_device_pb.Device>;
    setDevicesList(value: Array<teleport_devicetrust_v1_device_pb.Device>): SyncInventoryMissingDevices;
    addDevices(value?: teleport_devicetrust_v1_device_pb.Device, index?: number): teleport_devicetrust_v1_device_pb.Device;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SyncInventoryMissingDevices.AsObject;
    static toObject(includeInstance: boolean, msg: SyncInventoryMissingDevices): SyncInventoryMissingDevices.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SyncInventoryMissingDevices, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SyncInventoryMissingDevices;
    static deserializeBinaryFromReader(message: SyncInventoryMissingDevices, reader: jspb.BinaryReader): SyncInventoryMissingDevices;
}

export namespace SyncInventoryMissingDevices {
    export type AsObject = {
        devicesList: Array<teleport_devicetrust_v1_device_pb.Device.AsObject>,
    }
}

export class GetDevicesUsageRequest extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetDevicesUsageRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetDevicesUsageRequest): GetDevicesUsageRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetDevicesUsageRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetDevicesUsageRequest;
    static deserializeBinaryFromReader(message: GetDevicesUsageRequest, reader: jspb.BinaryReader): GetDevicesUsageRequest;
}

export namespace GetDevicesUsageRequest {
    export type AsObject = {
    }
}

export enum DeviceView {
    DEVICE_VIEW_UNSPECIFIED = 0,
    DEVICE_VIEW_LIST = 1,
    DEVICE_VIEW_RESOURCE = 2,
}
