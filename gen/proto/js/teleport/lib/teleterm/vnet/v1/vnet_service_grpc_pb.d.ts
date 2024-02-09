// package: teleport.lib.teleterm.vnet.v1
// file: teleport/lib/teleterm/vnet/v1/vnet_service.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as teleport_lib_teleterm_vnet_v1_vnet_service_pb from "../../../../../teleport/lib/teleterm/vnet/v1/vnet_service_pb";

interface IVnetServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    start: IVnetServiceService_IStart;
    stop: IVnetServiceService_IStop;
}

interface IVnetServiceService_IStart extends grpc.MethodDefinition<teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartRequest, teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartResponse> {
    path: "/teleport.lib.teleterm.vnet.v1.VnetService/Start";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartResponse>;
}
interface IVnetServiceService_IStop extends grpc.MethodDefinition<teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopRequest, teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopResponse> {
    path: "/teleport.lib.teleterm.vnet.v1.VnetService/Stop";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopResponse>;
}

export const VnetServiceService: IVnetServiceService;

export interface IVnetServiceServer {
    start: grpc.handleUnaryCall<teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartRequest, teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartResponse>;
    stop: grpc.handleUnaryCall<teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopRequest, teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopResponse>;
}

export interface IVnetServiceClient {
    start(request: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartResponse) => void): grpc.ClientUnaryCall;
    start(request: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartResponse) => void): grpc.ClientUnaryCall;
    start(request: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartResponse) => void): grpc.ClientUnaryCall;
    stop(request: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopResponse) => void): grpc.ClientUnaryCall;
    stop(request: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopResponse) => void): grpc.ClientUnaryCall;
    stop(request: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopResponse) => void): grpc.ClientUnaryCall;
}

export class VnetServiceClient extends grpc.Client implements IVnetServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public start(request: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartResponse) => void): grpc.ClientUnaryCall;
    public start(request: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartResponse) => void): grpc.ClientUnaryCall;
    public start(request: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StartResponse) => void): grpc.ClientUnaryCall;
    public stop(request: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopResponse) => void): grpc.ClientUnaryCall;
    public stop(request: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopResponse) => void): grpc.ClientUnaryCall;
    public stop(request: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_vnet_v1_vnet_service_pb.StopResponse) => void): grpc.ClientUnaryCall;
}
