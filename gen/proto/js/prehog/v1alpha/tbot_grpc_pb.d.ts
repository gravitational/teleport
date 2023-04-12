// package: prehog.v1alpha
// file: prehog/v1alpha/tbot.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as prehog_v1alpha_tbot_pb from "../../prehog/v1alpha/tbot_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

interface ITbotReportingServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    submitTbotEvent: ITbotReportingServiceService_ISubmitTbotEvent;
}

interface ITbotReportingServiceService_ISubmitTbotEvent extends grpc.MethodDefinition<prehog_v1alpha_tbot_pb.SubmitTbotEventRequest, prehog_v1alpha_tbot_pb.SubmitTbotEventResponse> {
    path: "/prehog.v1alpha.TbotReportingService/SubmitTbotEvent";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<prehog_v1alpha_tbot_pb.SubmitTbotEventRequest>;
    requestDeserialize: grpc.deserialize<prehog_v1alpha_tbot_pb.SubmitTbotEventRequest>;
    responseSerialize: grpc.serialize<prehog_v1alpha_tbot_pb.SubmitTbotEventResponse>;
    responseDeserialize: grpc.deserialize<prehog_v1alpha_tbot_pb.SubmitTbotEventResponse>;
}

export const TbotReportingServiceService: ITbotReportingServiceService;

export interface ITbotReportingServiceServer {
    submitTbotEvent: grpc.handleUnaryCall<prehog_v1alpha_tbot_pb.SubmitTbotEventRequest, prehog_v1alpha_tbot_pb.SubmitTbotEventResponse>;
}

export interface ITbotReportingServiceClient {
    submitTbotEvent(request: prehog_v1alpha_tbot_pb.SubmitTbotEventRequest, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_tbot_pb.SubmitTbotEventResponse) => void): grpc.ClientUnaryCall;
    submitTbotEvent(request: prehog_v1alpha_tbot_pb.SubmitTbotEventRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_tbot_pb.SubmitTbotEventResponse) => void): grpc.ClientUnaryCall;
    submitTbotEvent(request: prehog_v1alpha_tbot_pb.SubmitTbotEventRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_tbot_pb.SubmitTbotEventResponse) => void): grpc.ClientUnaryCall;
}

export class TbotReportingServiceClient extends grpc.Client implements ITbotReportingServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public submitTbotEvent(request: prehog_v1alpha_tbot_pb.SubmitTbotEventRequest, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_tbot_pb.SubmitTbotEventResponse) => void): grpc.ClientUnaryCall;
    public submitTbotEvent(request: prehog_v1alpha_tbot_pb.SubmitTbotEventRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_tbot_pb.SubmitTbotEventResponse) => void): grpc.ClientUnaryCall;
    public submitTbotEvent(request: prehog_v1alpha_tbot_pb.SubmitTbotEventRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: prehog_v1alpha_tbot_pb.SubmitTbotEventResponse) => void): grpc.ClientUnaryCall;
}
