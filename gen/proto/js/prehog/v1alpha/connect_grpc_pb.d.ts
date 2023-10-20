// package: prehog.v1alpha
// file: prehog/v1alpha/connect.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as prehog_v1alpha_connect_pb from "../../prehog/v1alpha/connect_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

interface IConnectReportingServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    submitConnectEvent: IConnectReportingServiceService_ISubmitConnectEvent;
}

interface IConnectReportingServiceService_ISubmitConnectEvent extends grpc.MethodDefinition<prehog_v1alpha_connect_pb.SubmitConnectEventRequest, prehog_v1alpha_connect_pb.SubmitConnectEventResponse> {
    path: "/prehog.v1alpha.ConnectReportingService/SubmitConnectEvent";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<prehog_v1alpha_connect_pb.SubmitConnectEventRequest>;
    requestDeserialize: grpc.deserialize<prehog_v1alpha_connect_pb.SubmitConnectEventRequest>;
    responseSerialize: grpc.serialize<prehog_v1alpha_connect_pb.SubmitConnectEventResponse>;
    responseDeserialize: grpc.deserialize<prehog_v1alpha_connect_pb.SubmitConnectEventResponse>;
}

export const ConnectReportingServiceService: IConnectReportingServiceService;

export interface IConnectReportingServiceServer {
    submitConnectEvent: grpc.handleUnaryCall<prehog_v1alpha_connect_pb.SubmitConnectEventRequest, prehog_v1alpha_connect_pb.SubmitConnectEventResponse>;
}

export interface IConnectReportingServiceClient {
    submitConnectEvent(request: prehog_v1alpha_connect_pb.SubmitConnectEventRequest, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_connect_pb.SubmitConnectEventResponse) => void): grpc.ClientUnaryCall;
    submitConnectEvent(request: prehog_v1alpha_connect_pb.SubmitConnectEventRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_connect_pb.SubmitConnectEventResponse) => void): grpc.ClientUnaryCall;
    submitConnectEvent(request: prehog_v1alpha_connect_pb.SubmitConnectEventRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_connect_pb.SubmitConnectEventResponse) => void): grpc.ClientUnaryCall;
}

export class ConnectReportingServiceClient extends grpc.Client implements IConnectReportingServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public submitConnectEvent(request: prehog_v1alpha_connect_pb.SubmitConnectEventRequest, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_connect_pb.SubmitConnectEventResponse) => void): grpc.ClientUnaryCall;
    public submitConnectEvent(request: prehog_v1alpha_connect_pb.SubmitConnectEventRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_connect_pb.SubmitConnectEventResponse) => void): grpc.ClientUnaryCall;
    public submitConnectEvent(request: prehog_v1alpha_connect_pb.SubmitConnectEventRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_connect_pb.SubmitConnectEventResponse) => void): grpc.ClientUnaryCall;
}
