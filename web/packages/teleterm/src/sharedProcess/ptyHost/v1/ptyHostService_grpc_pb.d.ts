// package:
// file: v1/ptyHostService.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as v1_ptyHostService_pb from "../v1/ptyHostService_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

interface IPtyHostService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    createPtyProcess: IPtyHostService_ICreatePtyProcess;
    exchangeEvents: IPtyHostService_IExchangeEvents;
    getCwd: IPtyHostService_IGetCwd;
}

interface IPtyHostService_ICreatePtyProcess extends grpc.MethodDefinition<v1_ptyHostService_pb.PtyCreate, v1_ptyHostService_pb.PtyId> {
    path: "/PtyHost/CreatePtyProcess";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_ptyHostService_pb.PtyCreate>;
    requestDeserialize: grpc.deserialize<v1_ptyHostService_pb.PtyCreate>;
    responseSerialize: grpc.serialize<v1_ptyHostService_pb.PtyId>;
    responseDeserialize: grpc.deserialize<v1_ptyHostService_pb.PtyId>;
}
interface IPtyHostService_IExchangeEvents extends grpc.MethodDefinition<v1_ptyHostService_pb.PtyClientEvent, v1_ptyHostService_pb.PtyServerEvent> {
    path: "/PtyHost/ExchangeEvents";
    requestStream: true;
    responseStream: true;
    requestSerialize: grpc.serialize<v1_ptyHostService_pb.PtyClientEvent>;
    requestDeserialize: grpc.deserialize<v1_ptyHostService_pb.PtyClientEvent>;
    responseSerialize: grpc.serialize<v1_ptyHostService_pb.PtyServerEvent>;
    responseDeserialize: grpc.deserialize<v1_ptyHostService_pb.PtyServerEvent>;
}
interface IPtyHostService_IGetCwd extends grpc.MethodDefinition<v1_ptyHostService_pb.PtyId, v1_ptyHostService_pb.PtyCwd> {
    path: "/PtyHost/GetCwd";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_ptyHostService_pb.PtyId>;
    requestDeserialize: grpc.deserialize<v1_ptyHostService_pb.PtyId>;
    responseSerialize: grpc.serialize<v1_ptyHostService_pb.PtyCwd>;
    responseDeserialize: grpc.deserialize<v1_ptyHostService_pb.PtyCwd>;
}

export const PtyHostService: IPtyHostService;

export interface IPtyHostServer {
    createPtyProcess: grpc.handleUnaryCall<v1_ptyHostService_pb.PtyCreate, v1_ptyHostService_pb.PtyId>;
    exchangeEvents: grpc.handleBidiStreamingCall<v1_ptyHostService_pb.PtyClientEvent, v1_ptyHostService_pb.PtyServerEvent>;
    getCwd: grpc.handleUnaryCall<v1_ptyHostService_pb.PtyId, v1_ptyHostService_pb.PtyCwd>;
}

export interface IPtyHostClient {
    createPtyProcess(request: v1_ptyHostService_pb.PtyCreate, callback: (error: grpc.ServiceError | null, response: v1_ptyHostService_pb.PtyId) => void): grpc.ClientUnaryCall;
    createPtyProcess(request: v1_ptyHostService_pb.PtyCreate, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_ptyHostService_pb.PtyId) => void): grpc.ClientUnaryCall;
    createPtyProcess(request: v1_ptyHostService_pb.PtyCreate, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_ptyHostService_pb.PtyId) => void): grpc.ClientUnaryCall;
    exchangeEvents(): grpc.ClientDuplexStream<v1_ptyHostService_pb.PtyClientEvent, v1_ptyHostService_pb.PtyServerEvent>;
    exchangeEvents(options: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<v1_ptyHostService_pb.PtyClientEvent, v1_ptyHostService_pb.PtyServerEvent>;
    exchangeEvents(metadata: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<v1_ptyHostService_pb.PtyClientEvent, v1_ptyHostService_pb.PtyServerEvent>;
    getCwd(request: v1_ptyHostService_pb.PtyId, callback: (error: grpc.ServiceError | null, response: v1_ptyHostService_pb.PtyCwd) => void): grpc.ClientUnaryCall;
    getCwd(request: v1_ptyHostService_pb.PtyId, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_ptyHostService_pb.PtyCwd) => void): grpc.ClientUnaryCall;
    getCwd(request: v1_ptyHostService_pb.PtyId, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_ptyHostService_pb.PtyCwd) => void): grpc.ClientUnaryCall;
}

export class PtyHostClient extends grpc.Client implements IPtyHostClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public createPtyProcess(request: v1_ptyHostService_pb.PtyCreate, callback: (error: grpc.ServiceError | null, response: v1_ptyHostService_pb.PtyId) => void): grpc.ClientUnaryCall;
    public createPtyProcess(request: v1_ptyHostService_pb.PtyCreate, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_ptyHostService_pb.PtyId) => void): grpc.ClientUnaryCall;
    public createPtyProcess(request: v1_ptyHostService_pb.PtyCreate, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_ptyHostService_pb.PtyId) => void): grpc.ClientUnaryCall;
    public exchangeEvents(options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<v1_ptyHostService_pb.PtyClientEvent, v1_ptyHostService_pb.PtyServerEvent>;
    public exchangeEvents(metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<v1_ptyHostService_pb.PtyClientEvent, v1_ptyHostService_pb.PtyServerEvent>;
    public getCwd(request: v1_ptyHostService_pb.PtyId, callback: (error: grpc.ServiceError | null, response: v1_ptyHostService_pb.PtyCwd) => void): grpc.ClientUnaryCall;
    public getCwd(request: v1_ptyHostService_pb.PtyId, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_ptyHostService_pb.PtyCwd) => void): grpc.ClientUnaryCall;
    public getCwd(request: v1_ptyHostService_pb.PtyId, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_ptyHostService_pb.PtyCwd) => void): grpc.ClientUnaryCall;
}
