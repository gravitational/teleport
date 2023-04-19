// package: prehog.v1
// file: prehog/v1/teleport.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as prehog_v1_teleport_pb from "../../prehog/v1/teleport_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

interface ITeleportReportingServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    submitUsageReports: ITeleportReportingServiceService_ISubmitUsageReports;
}

interface ITeleportReportingServiceService_ISubmitUsageReports extends grpc.MethodDefinition<prehog_v1_teleport_pb.SubmitUsageReportsRequest, prehog_v1_teleport_pb.SubmitUsageReportsResponse> {
    path: "/prehog.v1.TeleportReportingService/SubmitUsageReports";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<prehog_v1_teleport_pb.SubmitUsageReportsRequest>;
    requestDeserialize: grpc.deserialize<prehog_v1_teleport_pb.SubmitUsageReportsRequest>;
    responseSerialize: grpc.serialize<prehog_v1_teleport_pb.SubmitUsageReportsResponse>;
    responseDeserialize: grpc.deserialize<prehog_v1_teleport_pb.SubmitUsageReportsResponse>;
}

export const TeleportReportingServiceService: ITeleportReportingServiceService;

export interface ITeleportReportingServiceServer {
    submitUsageReports: grpc.handleUnaryCall<prehog_v1_teleport_pb.SubmitUsageReportsRequest, prehog_v1_teleport_pb.SubmitUsageReportsResponse>;
}

export interface ITeleportReportingServiceClient {
    submitUsageReports(request: prehog_v1_teleport_pb.SubmitUsageReportsRequest, callback: (error: grpc.ServiceError | null, response: prehog_v1_teleport_pb.SubmitUsageReportsResponse) => void): grpc.ClientUnaryCall;
    submitUsageReports(request: prehog_v1_teleport_pb.SubmitUsageReportsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: prehog_v1_teleport_pb.SubmitUsageReportsResponse) => void): grpc.ClientUnaryCall;
    submitUsageReports(request: prehog_v1_teleport_pb.SubmitUsageReportsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: prehog_v1_teleport_pb.SubmitUsageReportsResponse) => void): grpc.ClientUnaryCall;
}

export class TeleportReportingServiceClient extends grpc.Client implements ITeleportReportingServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public submitUsageReports(request: prehog_v1_teleport_pb.SubmitUsageReportsRequest, callback: (error: grpc.ServiceError | null, response: prehog_v1_teleport_pb.SubmitUsageReportsResponse) => void): grpc.ClientUnaryCall;
    public submitUsageReports(request: prehog_v1_teleport_pb.SubmitUsageReportsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: prehog_v1_teleport_pb.SubmitUsageReportsResponse) => void): grpc.ClientUnaryCall;
    public submitUsageReports(request: prehog_v1_teleport_pb.SubmitUsageReportsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: prehog_v1_teleport_pb.SubmitUsageReportsResponse) => void): grpc.ClientUnaryCall;
}
