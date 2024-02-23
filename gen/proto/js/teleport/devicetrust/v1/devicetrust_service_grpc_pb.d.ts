// package: teleport.devicetrust.v1
// file: teleport/devicetrust/v1/devicetrust_service.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as teleport_devicetrust_v1_devicetrust_service_pb from "../../../teleport/devicetrust/v1/devicetrust_service_pb";
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

interface IDeviceTrustServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    createDevice: IDeviceTrustServiceService_ICreateDevice;
    updateDevice: IDeviceTrustServiceService_IUpdateDevice;
    upsertDevice: IDeviceTrustServiceService_IUpsertDevice;
    deleteDevice: IDeviceTrustServiceService_IDeleteDevice;
    findDevices: IDeviceTrustServiceService_IFindDevices;
    getDevice: IDeviceTrustServiceService_IGetDevice;
    listDevices: IDeviceTrustServiceService_IListDevices;
    bulkCreateDevices: IDeviceTrustServiceService_IBulkCreateDevices;
    createDeviceEnrollToken: IDeviceTrustServiceService_ICreateDeviceEnrollToken;
    enrollDevice: IDeviceTrustServiceService_IEnrollDevice;
    authenticateDevice: IDeviceTrustServiceService_IAuthenticateDevice;
    syncInventory: IDeviceTrustServiceService_ISyncInventory;
    getDevicesUsage: IDeviceTrustServiceService_IGetDevicesUsage;
}

interface IDeviceTrustServiceService_ICreateDevice extends grpc.MethodDefinition<teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceRequest, teleport_devicetrust_v1_device_pb.Device> {
    path: "/teleport.devicetrust.v1.DeviceTrustService/CreateDevice";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceRequest>;
    requestDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceRequest>;
    responseSerialize: grpc.serialize<teleport_devicetrust_v1_device_pb.Device>;
    responseDeserialize: grpc.deserialize<teleport_devicetrust_v1_device_pb.Device>;
}
interface IDeviceTrustServiceService_IUpdateDevice extends grpc.MethodDefinition<teleport_devicetrust_v1_devicetrust_service_pb.UpdateDeviceRequest, teleport_devicetrust_v1_device_pb.Device> {
    path: "/teleport.devicetrust.v1.DeviceTrustService/UpdateDevice";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.UpdateDeviceRequest>;
    requestDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.UpdateDeviceRequest>;
    responseSerialize: grpc.serialize<teleport_devicetrust_v1_device_pb.Device>;
    responseDeserialize: grpc.deserialize<teleport_devicetrust_v1_device_pb.Device>;
}
interface IDeviceTrustServiceService_IUpsertDevice extends grpc.MethodDefinition<teleport_devicetrust_v1_devicetrust_service_pb.UpsertDeviceRequest, teleport_devicetrust_v1_device_pb.Device> {
    path: "/teleport.devicetrust.v1.DeviceTrustService/UpsertDevice";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.UpsertDeviceRequest>;
    requestDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.UpsertDeviceRequest>;
    responseSerialize: grpc.serialize<teleport_devicetrust_v1_device_pb.Device>;
    responseDeserialize: grpc.deserialize<teleport_devicetrust_v1_device_pb.Device>;
}
interface IDeviceTrustServiceService_IDeleteDevice extends grpc.MethodDefinition<teleport_devicetrust_v1_devicetrust_service_pb.DeleteDeviceRequest, google_protobuf_empty_pb.Empty> {
    path: "/teleport.devicetrust.v1.DeviceTrustService/DeleteDevice";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.DeleteDeviceRequest>;
    requestDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.DeleteDeviceRequest>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}
interface IDeviceTrustServiceService_IFindDevices extends grpc.MethodDefinition<teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesRequest, teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesResponse> {
    path: "/teleport.devicetrust.v1.DeviceTrustService/FindDevices";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesRequest>;
    requestDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesRequest>;
    responseSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesResponse>;
    responseDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesResponse>;
}
interface IDeviceTrustServiceService_IGetDevice extends grpc.MethodDefinition<teleport_devicetrust_v1_devicetrust_service_pb.GetDeviceRequest, teleport_devicetrust_v1_device_pb.Device> {
    path: "/teleport.devicetrust.v1.DeviceTrustService/GetDevice";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.GetDeviceRequest>;
    requestDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.GetDeviceRequest>;
    responseSerialize: grpc.serialize<teleport_devicetrust_v1_device_pb.Device>;
    responseDeserialize: grpc.deserialize<teleport_devicetrust_v1_device_pb.Device>;
}
interface IDeviceTrustServiceService_IListDevices extends grpc.MethodDefinition<teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesRequest, teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesResponse> {
    path: "/teleport.devicetrust.v1.DeviceTrustService/ListDevices";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesRequest>;
    requestDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesRequest>;
    responseSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesResponse>;
    responseDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesResponse>;
}
interface IDeviceTrustServiceService_IBulkCreateDevices extends grpc.MethodDefinition<teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesRequest, teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesResponse> {
    path: "/teleport.devicetrust.v1.DeviceTrustService/BulkCreateDevices";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesRequest>;
    requestDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesRequest>;
    responseSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesResponse>;
    responseDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesResponse>;
}
interface IDeviceTrustServiceService_ICreateDeviceEnrollToken extends grpc.MethodDefinition<teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceEnrollTokenRequest, teleport_devicetrust_v1_device_enroll_token_pb.DeviceEnrollToken> {
    path: "/teleport.devicetrust.v1.DeviceTrustService/CreateDeviceEnrollToken";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceEnrollTokenRequest>;
    requestDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceEnrollTokenRequest>;
    responseSerialize: grpc.serialize<teleport_devicetrust_v1_device_enroll_token_pb.DeviceEnrollToken>;
    responseDeserialize: grpc.deserialize<teleport_devicetrust_v1_device_enroll_token_pb.DeviceEnrollToken>;
}
interface IDeviceTrustServiceService_IEnrollDevice extends grpc.MethodDefinition<teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceRequest, teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceResponse> {
    path: "/teleport.devicetrust.v1.DeviceTrustService/EnrollDevice";
    requestStream: true;
    responseStream: true;
    requestSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceRequest>;
    requestDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceRequest>;
    responseSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceResponse>;
    responseDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceResponse>;
}
interface IDeviceTrustServiceService_IAuthenticateDevice extends grpc.MethodDefinition<teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceRequest, teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceResponse> {
    path: "/teleport.devicetrust.v1.DeviceTrustService/AuthenticateDevice";
    requestStream: true;
    responseStream: true;
    requestSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceRequest>;
    requestDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceRequest>;
    responseSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceResponse>;
    responseDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceResponse>;
}
interface IDeviceTrustServiceService_ISyncInventory extends grpc.MethodDefinition<teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryRequest, teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryResponse> {
    path: "/teleport.devicetrust.v1.DeviceTrustService/SyncInventory";
    requestStream: true;
    responseStream: true;
    requestSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryRequest>;
    requestDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryRequest>;
    responseSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryResponse>;
    responseDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryResponse>;
}
interface IDeviceTrustServiceService_IGetDevicesUsage extends grpc.MethodDefinition<teleport_devicetrust_v1_devicetrust_service_pb.GetDevicesUsageRequest, teleport_devicetrust_v1_usage_pb.DevicesUsage> {
    path: "/teleport.devicetrust.v1.DeviceTrustService/GetDevicesUsage";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_devicetrust_v1_devicetrust_service_pb.GetDevicesUsageRequest>;
    requestDeserialize: grpc.deserialize<teleport_devicetrust_v1_devicetrust_service_pb.GetDevicesUsageRequest>;
    responseSerialize: grpc.serialize<teleport_devicetrust_v1_usage_pb.DevicesUsage>;
    responseDeserialize: grpc.deserialize<teleport_devicetrust_v1_usage_pb.DevicesUsage>;
}

export const DeviceTrustServiceService: IDeviceTrustServiceService;

export interface IDeviceTrustServiceServer {
    createDevice: grpc.handleUnaryCall<teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceRequest, teleport_devicetrust_v1_device_pb.Device>;
    updateDevice: grpc.handleUnaryCall<teleport_devicetrust_v1_devicetrust_service_pb.UpdateDeviceRequest, teleport_devicetrust_v1_device_pb.Device>;
    upsertDevice: grpc.handleUnaryCall<teleport_devicetrust_v1_devicetrust_service_pb.UpsertDeviceRequest, teleport_devicetrust_v1_device_pb.Device>;
    deleteDevice: grpc.handleUnaryCall<teleport_devicetrust_v1_devicetrust_service_pb.DeleteDeviceRequest, google_protobuf_empty_pb.Empty>;
    findDevices: grpc.handleUnaryCall<teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesRequest, teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesResponse>;
    getDevice: grpc.handleUnaryCall<teleport_devicetrust_v1_devicetrust_service_pb.GetDeviceRequest, teleport_devicetrust_v1_device_pb.Device>;
    listDevices: grpc.handleUnaryCall<teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesRequest, teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesResponse>;
    bulkCreateDevices: grpc.handleUnaryCall<teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesRequest, teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesResponse>;
    createDeviceEnrollToken: grpc.handleUnaryCall<teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceEnrollTokenRequest, teleport_devicetrust_v1_device_enroll_token_pb.DeviceEnrollToken>;
    enrollDevice: grpc.handleBidiStreamingCall<teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceRequest, teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceResponse>;
    authenticateDevice: grpc.handleBidiStreamingCall<teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceRequest, teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceResponse>;
    syncInventory: grpc.handleBidiStreamingCall<teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryRequest, teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryResponse>;
    getDevicesUsage: grpc.handleUnaryCall<teleport_devicetrust_v1_devicetrust_service_pb.GetDevicesUsageRequest, teleport_devicetrust_v1_usage_pb.DevicesUsage>;
}

export interface IDeviceTrustServiceClient {
    createDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    createDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    createDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    updateDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.UpdateDeviceRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    updateDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.UpdateDeviceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    updateDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.UpdateDeviceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    upsertDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.UpsertDeviceRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    upsertDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.UpsertDeviceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    upsertDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.UpsertDeviceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    deleteDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.DeleteDeviceRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    deleteDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.DeleteDeviceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    deleteDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.DeleteDeviceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    findDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesResponse) => void): grpc.ClientUnaryCall;
    findDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesResponse) => void): grpc.ClientUnaryCall;
    findDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesResponse) => void): grpc.ClientUnaryCall;
    getDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.GetDeviceRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    getDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.GetDeviceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    getDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.GetDeviceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    listDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesResponse) => void): grpc.ClientUnaryCall;
    listDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesResponse) => void): grpc.ClientUnaryCall;
    listDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesResponse) => void): grpc.ClientUnaryCall;
    bulkCreateDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesResponse) => void): grpc.ClientUnaryCall;
    bulkCreateDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesResponse) => void): grpc.ClientUnaryCall;
    bulkCreateDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesResponse) => void): grpc.ClientUnaryCall;
    createDeviceEnrollToken(request: teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceEnrollTokenRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_enroll_token_pb.DeviceEnrollToken) => void): grpc.ClientUnaryCall;
    createDeviceEnrollToken(request: teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceEnrollTokenRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_enroll_token_pb.DeviceEnrollToken) => void): grpc.ClientUnaryCall;
    createDeviceEnrollToken(request: teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceEnrollTokenRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_enroll_token_pb.DeviceEnrollToken) => void): grpc.ClientUnaryCall;
    enrollDevice(): grpc.ClientDuplexStream<teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceRequest, teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceResponse>;
    enrollDevice(options: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceRequest, teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceResponse>;
    enrollDevice(metadata: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceRequest, teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceResponse>;
    authenticateDevice(): grpc.ClientDuplexStream<teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceRequest, teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceResponse>;
    authenticateDevice(options: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceRequest, teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceResponse>;
    authenticateDevice(metadata: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceRequest, teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceResponse>;
    syncInventory(): grpc.ClientDuplexStream<teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryRequest, teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryResponse>;
    syncInventory(options: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryRequest, teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryResponse>;
    syncInventory(metadata: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryRequest, teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryResponse>;
    getDevicesUsage(request: teleport_devicetrust_v1_devicetrust_service_pb.GetDevicesUsageRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_usage_pb.DevicesUsage) => void): grpc.ClientUnaryCall;
    getDevicesUsage(request: teleport_devicetrust_v1_devicetrust_service_pb.GetDevicesUsageRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_usage_pb.DevicesUsage) => void): grpc.ClientUnaryCall;
    getDevicesUsage(request: teleport_devicetrust_v1_devicetrust_service_pb.GetDevicesUsageRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_usage_pb.DevicesUsage) => void): grpc.ClientUnaryCall;
}

export class DeviceTrustServiceClient extends grpc.Client implements IDeviceTrustServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public createDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    public createDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    public createDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    public updateDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.UpdateDeviceRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    public updateDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.UpdateDeviceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    public updateDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.UpdateDeviceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    public upsertDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.UpsertDeviceRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    public upsertDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.UpsertDeviceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    public upsertDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.UpsertDeviceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    public deleteDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.DeleteDeviceRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public deleteDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.DeleteDeviceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public deleteDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.DeleteDeviceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public findDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesResponse) => void): grpc.ClientUnaryCall;
    public findDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesResponse) => void): grpc.ClientUnaryCall;
    public findDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.FindDevicesResponse) => void): grpc.ClientUnaryCall;
    public getDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.GetDeviceRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    public getDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.GetDeviceRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    public getDevice(request: teleport_devicetrust_v1_devicetrust_service_pb.GetDeviceRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_pb.Device) => void): grpc.ClientUnaryCall;
    public listDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesResponse) => void): grpc.ClientUnaryCall;
    public listDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesResponse) => void): grpc.ClientUnaryCall;
    public listDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.ListDevicesResponse) => void): grpc.ClientUnaryCall;
    public bulkCreateDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesResponse) => void): grpc.ClientUnaryCall;
    public bulkCreateDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesResponse) => void): grpc.ClientUnaryCall;
    public bulkCreateDevices(request: teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_devicetrust_service_pb.BulkCreateDevicesResponse) => void): grpc.ClientUnaryCall;
    public createDeviceEnrollToken(request: teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceEnrollTokenRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_enroll_token_pb.DeviceEnrollToken) => void): grpc.ClientUnaryCall;
    public createDeviceEnrollToken(request: teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceEnrollTokenRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_enroll_token_pb.DeviceEnrollToken) => void): grpc.ClientUnaryCall;
    public createDeviceEnrollToken(request: teleport_devicetrust_v1_devicetrust_service_pb.CreateDeviceEnrollTokenRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_device_enroll_token_pb.DeviceEnrollToken) => void): grpc.ClientUnaryCall;
    public enrollDevice(options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceRequest, teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceResponse>;
    public enrollDevice(metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceRequest, teleport_devicetrust_v1_devicetrust_service_pb.EnrollDeviceResponse>;
    public authenticateDevice(options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceRequest, teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceResponse>;
    public authenticateDevice(metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceRequest, teleport_devicetrust_v1_devicetrust_service_pb.AuthenticateDeviceResponse>;
    public syncInventory(options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryRequest, teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryResponse>;
    public syncInventory(metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryRequest, teleport_devicetrust_v1_devicetrust_service_pb.SyncInventoryResponse>;
    public getDevicesUsage(request: teleport_devicetrust_v1_devicetrust_service_pb.GetDevicesUsageRequest, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_usage_pb.DevicesUsage) => void): grpc.ClientUnaryCall;
    public getDevicesUsage(request: teleport_devicetrust_v1_devicetrust_service_pb.GetDevicesUsageRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_usage_pb.DevicesUsage) => void): grpc.ClientUnaryCall;
    public getDevicesUsage(request: teleport_devicetrust_v1_devicetrust_service_pb.GetDevicesUsageRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_devicetrust_v1_usage_pb.DevicesUsage) => void): grpc.ClientUnaryCall;
}
