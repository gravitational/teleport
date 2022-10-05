// package: teleport.terminal.v1
// file: v1/tshd_events_service.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as v1_tshd_events_service_pb from "../v1/tshd_events_service_pb";

interface ITshdEventsServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    test: ITshdEventsServiceService_ITest;
}

interface ITshdEventsServiceService_ITest extends grpc.MethodDefinition<v1_tshd_events_service_pb.TestRequest, v1_tshd_events_service_pb.TestResponse> {
    path: "/teleport.terminal.v1.TshdEventsService/Test";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_tshd_events_service_pb.TestRequest>;
    requestDeserialize: grpc.deserialize<v1_tshd_events_service_pb.TestRequest>;
    responseSerialize: grpc.serialize<v1_tshd_events_service_pb.TestResponse>;
    responseDeserialize: grpc.deserialize<v1_tshd_events_service_pb.TestResponse>;
}

export const TshdEventsServiceService: ITshdEventsServiceService;

export interface ITshdEventsServiceServer {
    test: grpc.handleUnaryCall<v1_tshd_events_service_pb.TestRequest, v1_tshd_events_service_pb.TestResponse>;
}

export interface ITshdEventsServiceClient {
    test(request: v1_tshd_events_service_pb.TestRequest, callback: (error: grpc.ServiceError | null, response: v1_tshd_events_service_pb.TestResponse) => void): grpc.ClientUnaryCall;
    test(request: v1_tshd_events_service_pb.TestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_tshd_events_service_pb.TestResponse) => void): grpc.ClientUnaryCall;
    test(request: v1_tshd_events_service_pb.TestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_tshd_events_service_pb.TestResponse) => void): grpc.ClientUnaryCall;
}

export class TshdEventsServiceClient extends grpc.Client implements ITshdEventsServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public test(request: v1_tshd_events_service_pb.TestRequest, callback: (error: grpc.ServiceError | null, response: v1_tshd_events_service_pb.TestResponse) => void): grpc.ClientUnaryCall;
    public test(request: v1_tshd_events_service_pb.TestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_tshd_events_service_pb.TestResponse) => void): grpc.ClientUnaryCall;
    public test(request: v1_tshd_events_service_pb.TestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_tshd_events_service_pb.TestResponse) => void): grpc.ClientUnaryCall;
}
