// package: prehog.v1alpha
// file: teleport/lib/prehog/v1alpha/connect.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as teleport_lib_prehog_v1alpha_connect_pb from "../../../../teleport/lib/prehog/v1alpha/connect_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

interface IConnectReportingServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    submitConnectEvent: IConnectReportingServiceService_ISubmitConnectEvent;
}

interface IConnectReportingServiceService_ISubmitConnectEvent extends grpc.MethodDefinition<teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventRequest, teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventResponse> {
    path: "/prehog.v1alpha.ConnectReportingService/SubmitConnectEvent";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventRequest>;
    responseSerialize: grpc.serialize<teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventResponse>;
}

export const ConnectReportingServiceService: IConnectReportingServiceService;

export interface IConnectReportingServiceServer {
    submitConnectEvent: grpc.handleUnaryCall<teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventRequest, teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventResponse>;
}

export interface IConnectReportingServiceClient {
    submitConnectEvent(request: teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventResponse) => void): grpc.ClientUnaryCall;
    submitConnectEvent(request: teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventResponse) => void): grpc.ClientUnaryCall;
    submitConnectEvent(request: teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventResponse) => void): grpc.ClientUnaryCall;
}

export class ConnectReportingServiceClient extends grpc.Client implements IConnectReportingServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public submitConnectEvent(request: teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventResponse) => void): grpc.ClientUnaryCall;
    public submitConnectEvent(request: teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventResponse) => void): grpc.ClientUnaryCall;
    public submitConnectEvent(request: teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_prehog_v1alpha_connect_pb.SubmitConnectEventResponse) => void): grpc.ClientUnaryCall;
}
