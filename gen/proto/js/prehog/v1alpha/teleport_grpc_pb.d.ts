// package: prehog.v1alpha
// file: prehog/v1alpha/teleport.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as prehog_v1alpha_teleport_pb from "../../prehog/v1alpha/teleport_pb";
import * as google_protobuf_duration_pb from "google-protobuf/google/protobuf/duration_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

interface ITeleportReportingServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    submitEvent: ITeleportReportingServiceService_ISubmitEvent;
    submitEvents: ITeleportReportingServiceService_ISubmitEvents;
    helloTeleport: ITeleportReportingServiceService_IHelloTeleport;
}

interface ITeleportReportingServiceService_ISubmitEvent extends grpc.MethodDefinition<prehog_v1alpha_teleport_pb.SubmitEventRequest, prehog_v1alpha_teleport_pb.SubmitEventResponse> {
    path: "/prehog.v1alpha.TeleportReportingService/SubmitEvent";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<prehog_v1alpha_teleport_pb.SubmitEventRequest>;
    requestDeserialize: grpc.deserialize<prehog_v1alpha_teleport_pb.SubmitEventRequest>;
    responseSerialize: grpc.serialize<prehog_v1alpha_teleport_pb.SubmitEventResponse>;
    responseDeserialize: grpc.deserialize<prehog_v1alpha_teleport_pb.SubmitEventResponse>;
}
interface ITeleportReportingServiceService_ISubmitEvents extends grpc.MethodDefinition<prehog_v1alpha_teleport_pb.SubmitEventsRequest, prehog_v1alpha_teleport_pb.SubmitEventsResponse> {
    path: "/prehog.v1alpha.TeleportReportingService/SubmitEvents";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<prehog_v1alpha_teleport_pb.SubmitEventsRequest>;
    requestDeserialize: grpc.deserialize<prehog_v1alpha_teleport_pb.SubmitEventsRequest>;
    responseSerialize: grpc.serialize<prehog_v1alpha_teleport_pb.SubmitEventsResponse>;
    responseDeserialize: grpc.deserialize<prehog_v1alpha_teleport_pb.SubmitEventsResponse>;
}
interface ITeleportReportingServiceService_IHelloTeleport extends grpc.MethodDefinition<prehog_v1alpha_teleport_pb.HelloTeleportRequest, prehog_v1alpha_teleport_pb.HelloTeleportResponse> {
    path: "/prehog.v1alpha.TeleportReportingService/HelloTeleport";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<prehog_v1alpha_teleport_pb.HelloTeleportRequest>;
    requestDeserialize: grpc.deserialize<prehog_v1alpha_teleport_pb.HelloTeleportRequest>;
    responseSerialize: grpc.serialize<prehog_v1alpha_teleport_pb.HelloTeleportResponse>;
    responseDeserialize: grpc.deserialize<prehog_v1alpha_teleport_pb.HelloTeleportResponse>;
}

export const TeleportReportingServiceService: ITeleportReportingServiceService;

export interface ITeleportReportingServiceServer {
    submitEvent: grpc.handleUnaryCall<prehog_v1alpha_teleport_pb.SubmitEventRequest, prehog_v1alpha_teleport_pb.SubmitEventResponse>;
    submitEvents: grpc.handleUnaryCall<prehog_v1alpha_teleport_pb.SubmitEventsRequest, prehog_v1alpha_teleport_pb.SubmitEventsResponse>;
    helloTeleport: grpc.handleUnaryCall<prehog_v1alpha_teleport_pb.HelloTeleportRequest, prehog_v1alpha_teleport_pb.HelloTeleportResponse>;
}

export interface ITeleportReportingServiceClient {
    submitEvent(request: prehog_v1alpha_teleport_pb.SubmitEventRequest, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.SubmitEventResponse) => void): grpc.ClientUnaryCall;
    submitEvent(request: prehog_v1alpha_teleport_pb.SubmitEventRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.SubmitEventResponse) => void): grpc.ClientUnaryCall;
    submitEvent(request: prehog_v1alpha_teleport_pb.SubmitEventRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.SubmitEventResponse) => void): grpc.ClientUnaryCall;
    submitEvents(request: prehog_v1alpha_teleport_pb.SubmitEventsRequest, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.SubmitEventsResponse) => void): grpc.ClientUnaryCall;
    submitEvents(request: prehog_v1alpha_teleport_pb.SubmitEventsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.SubmitEventsResponse) => void): grpc.ClientUnaryCall;
    submitEvents(request: prehog_v1alpha_teleport_pb.SubmitEventsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.SubmitEventsResponse) => void): grpc.ClientUnaryCall;
    helloTeleport(request: prehog_v1alpha_teleport_pb.HelloTeleportRequest, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.HelloTeleportResponse) => void): grpc.ClientUnaryCall;
    helloTeleport(request: prehog_v1alpha_teleport_pb.HelloTeleportRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.HelloTeleportResponse) => void): grpc.ClientUnaryCall;
    helloTeleport(request: prehog_v1alpha_teleport_pb.HelloTeleportRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.HelloTeleportResponse) => void): grpc.ClientUnaryCall;
}

export class TeleportReportingServiceClient extends grpc.Client implements ITeleportReportingServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public submitEvent(request: prehog_v1alpha_teleport_pb.SubmitEventRequest, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.SubmitEventResponse) => void): grpc.ClientUnaryCall;
    public submitEvent(request: prehog_v1alpha_teleport_pb.SubmitEventRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.SubmitEventResponse) => void): grpc.ClientUnaryCall;
    public submitEvent(request: prehog_v1alpha_teleport_pb.SubmitEventRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.SubmitEventResponse) => void): grpc.ClientUnaryCall;
    public submitEvents(request: prehog_v1alpha_teleport_pb.SubmitEventsRequest, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.SubmitEventsResponse) => void): grpc.ClientUnaryCall;
    public submitEvents(request: prehog_v1alpha_teleport_pb.SubmitEventsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.SubmitEventsResponse) => void): grpc.ClientUnaryCall;
    public submitEvents(request: prehog_v1alpha_teleport_pb.SubmitEventsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.SubmitEventsResponse) => void): grpc.ClientUnaryCall;
    public helloTeleport(request: prehog_v1alpha_teleport_pb.HelloTeleportRequest, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.HelloTeleportResponse) => void): grpc.ClientUnaryCall;
    public helloTeleport(request: prehog_v1alpha_teleport_pb.HelloTeleportRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.HelloTeleportResponse) => void): grpc.ClientUnaryCall;
    public helloTeleport(request: prehog_v1alpha_teleport_pb.HelloTeleportRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_teleport_pb.HelloTeleportResponse) => void): grpc.ClientUnaryCall;
}
