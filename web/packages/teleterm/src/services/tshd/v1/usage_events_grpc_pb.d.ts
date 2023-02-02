// package: teleport.terminal.v1
// file: v1/usage_events.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as v1_usage_events_pb from "../v1/usage_events_pb";

interface IUsageEventsServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    reportEvent: IUsageEventsServiceService_IReportEvent;
}

interface IUsageEventsServiceService_IReportEvent extends grpc.MethodDefinition<v1_usage_events_pb.ReportEventRequest, v1_usage_events_pb.EventReportedResponse> {
    path: "/teleport.terminal.v1.UsageEventsService/ReportEvent";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_usage_events_pb.ReportEventRequest>;
    requestDeserialize: grpc.deserialize<v1_usage_events_pb.ReportEventRequest>;
    responseSerialize: grpc.serialize<v1_usage_events_pb.EventReportedResponse>;
    responseDeserialize: grpc.deserialize<v1_usage_events_pb.EventReportedResponse>;
}

export const UsageEventsServiceService: IUsageEventsServiceService;

export interface IUsageEventsServiceServer {
    reportEvent: grpc.handleUnaryCall<v1_usage_events_pb.ReportEventRequest, v1_usage_events_pb.EventReportedResponse>;
}

export interface IUsageEventsServiceClient {
    reportEvent(request: v1_usage_events_pb.ReportEventRequest, callback: (error: grpc.ServiceError | null, response: v1_usage_events_pb.EventReportedResponse) => void): grpc.ClientUnaryCall;
    reportEvent(request: v1_usage_events_pb.ReportEventRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_usage_events_pb.EventReportedResponse) => void): grpc.ClientUnaryCall;
    reportEvent(request: v1_usage_events_pb.ReportEventRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_usage_events_pb.EventReportedResponse) => void): grpc.ClientUnaryCall;
}

export class UsageEventsServiceClient extends grpc.Client implements IUsageEventsServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public reportEvent(request: v1_usage_events_pb.ReportEventRequest, callback: (error: grpc.ServiceError | null, response: v1_usage_events_pb.EventReportedResponse) => void): grpc.ClientUnaryCall;
    public reportEvent(request: v1_usage_events_pb.ReportEventRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_usage_events_pb.EventReportedResponse) => void): grpc.ClientUnaryCall;
    public reportEvent(request: v1_usage_events_pb.ReportEventRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_usage_events_pb.EventReportedResponse) => void): grpc.ClientUnaryCall;
}
