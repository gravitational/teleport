// package: 
// file: ptyHostService.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as ptyHostService_pb from "./ptyHostService_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

interface IPtyHostService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    createPtyProcess: IPtyHostService_ICreatePtyProcess;
    exchangeEvents: IPtyHostService_IExchangeEvents;
    getCwd: IPtyHostService_IGetCwd;
}

interface IPtyHostService_ICreatePtyProcess extends grpc.MethodDefinition<ptyHostService_pb.PtyCreate, ptyHostService_pb.PtyId> {
    path: "/PtyHost/CreatePtyProcess";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<ptyHostService_pb.PtyCreate>;
    requestDeserialize: grpc.deserialize<ptyHostService_pb.PtyCreate>;
    responseSerialize: grpc.serialize<ptyHostService_pb.PtyId>;
    responseDeserialize: grpc.deserialize<ptyHostService_pb.PtyId>;
}
interface IPtyHostService_IExchangeEvents extends grpc.MethodDefinition<ptyHostService_pb.PtyClientEvent, ptyHostService_pb.PtyServerEvent> {
    path: "/PtyHost/ExchangeEvents";
    requestStream: true;
    responseStream: true;
    requestSerialize: grpc.serialize<ptyHostService_pb.PtyClientEvent>;
    requestDeserialize: grpc.deserialize<ptyHostService_pb.PtyClientEvent>;
    responseSerialize: grpc.serialize<ptyHostService_pb.PtyServerEvent>;
    responseDeserialize: grpc.deserialize<ptyHostService_pb.PtyServerEvent>;
}
interface IPtyHostService_IGetCwd extends grpc.MethodDefinition<ptyHostService_pb.PtyId, ptyHostService_pb.PtyCwd> {
    path: "/PtyHost/GetCwd";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<ptyHostService_pb.PtyId>;
    requestDeserialize: grpc.deserialize<ptyHostService_pb.PtyId>;
    responseSerialize: grpc.serialize<ptyHostService_pb.PtyCwd>;
    responseDeserialize: grpc.deserialize<ptyHostService_pb.PtyCwd>;
}

export const PtyHostService: IPtyHostService;

export interface IPtyHostServer {
    createPtyProcess: grpc.handleUnaryCall<ptyHostService_pb.PtyCreate, ptyHostService_pb.PtyId>;
    exchangeEvents: grpc.handleBidiStreamingCall<ptyHostService_pb.PtyClientEvent, ptyHostService_pb.PtyServerEvent>;
    getCwd: grpc.handleUnaryCall<ptyHostService_pb.PtyId, ptyHostService_pb.PtyCwd>;
}

export interface IPtyHostClient {
    createPtyProcess(request: ptyHostService_pb.PtyCreate, callback: (error: grpc.ServiceError | null, response: ptyHostService_pb.PtyId) => void): grpc.ClientUnaryCall;
    createPtyProcess(request: ptyHostService_pb.PtyCreate, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: ptyHostService_pb.PtyId) => void): grpc.ClientUnaryCall;
    createPtyProcess(request: ptyHostService_pb.PtyCreate, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: ptyHostService_pb.PtyId) => void): grpc.ClientUnaryCall;
    exchangeEvents(): grpc.ClientDuplexStream<ptyHostService_pb.PtyClientEvent, ptyHostService_pb.PtyServerEvent>;
    exchangeEvents(options: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<ptyHostService_pb.PtyClientEvent, ptyHostService_pb.PtyServerEvent>;
    exchangeEvents(metadata: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<ptyHostService_pb.PtyClientEvent, ptyHostService_pb.PtyServerEvent>;
    getCwd(request: ptyHostService_pb.PtyId, callback: (error: grpc.ServiceError | null, response: ptyHostService_pb.PtyCwd) => void): grpc.ClientUnaryCall;
    getCwd(request: ptyHostService_pb.PtyId, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: ptyHostService_pb.PtyCwd) => void): grpc.ClientUnaryCall;
    getCwd(request: ptyHostService_pb.PtyId, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: ptyHostService_pb.PtyCwd) => void): grpc.ClientUnaryCall;
}

export class PtyHostClient extends grpc.Client implements IPtyHostClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public createPtyProcess(request: ptyHostService_pb.PtyCreate, callback: (error: grpc.ServiceError | null, response: ptyHostService_pb.PtyId) => void): grpc.ClientUnaryCall;
    public createPtyProcess(request: ptyHostService_pb.PtyCreate, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: ptyHostService_pb.PtyId) => void): grpc.ClientUnaryCall;
    public createPtyProcess(request: ptyHostService_pb.PtyCreate, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: ptyHostService_pb.PtyId) => void): grpc.ClientUnaryCall;
    public exchangeEvents(options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<ptyHostService_pb.PtyClientEvent, ptyHostService_pb.PtyServerEvent>;
    public exchangeEvents(metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<ptyHostService_pb.PtyClientEvent, ptyHostService_pb.PtyServerEvent>;
    public getCwd(request: ptyHostService_pb.PtyId, callback: (error: grpc.ServiceError | null, response: ptyHostService_pb.PtyCwd) => void): grpc.ClientUnaryCall;
    public getCwd(request: ptyHostService_pb.PtyId, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: ptyHostService_pb.PtyCwd) => void): grpc.ClientUnaryCall;
    public getCwd(request: ptyHostService_pb.PtyId, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: ptyHostService_pb.PtyCwd) => void): grpc.ClientUnaryCall;
}
