// package: teleport.lib.teleterm.v1
// file: teleport/lib/teleterm/v1/tshd_events_service.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as teleport_lib_teleterm_v1_tshd_events_service_pb from "../../../../teleport/lib/teleterm/v1/tshd_events_service_pb";

interface ITshdEventsServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    relogin: ITshdEventsServiceService_IRelogin;
    sendNotification: ITshdEventsServiceService_ISendNotification;
    promptMFA: ITshdEventsServiceService_IPromptMFA;
}

interface ITshdEventsServiceService_IRelogin extends grpc.MethodDefinition<teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest, teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse> {
    path: "/teleport.lib.teleterm.v1.TshdEventsService/Relogin";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse>;
}
interface ITshdEventsServiceService_ISendNotification extends grpc.MethodDefinition<teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest, teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse> {
    path: "/teleport.lib.teleterm.v1.TshdEventsService/SendNotification";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse>;
}
interface ITshdEventsServiceService_IPromptMFA extends grpc.MethodDefinition<teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFARequest, teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFAResponse> {
    path: "/teleport.lib.teleterm.v1.TshdEventsService/PromptMFA";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFARequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFARequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFAResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFAResponse>;
}

export const TshdEventsServiceService: ITshdEventsServiceService;

export interface ITshdEventsServiceServer {
    relogin: grpc.handleUnaryCall<teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest, teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse>;
    sendNotification: grpc.handleUnaryCall<teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest, teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse>;
    promptMFA: grpc.handleUnaryCall<teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFARequest, teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFAResponse>;
}

export interface ITshdEventsServiceClient {
    relogin(request: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse) => void): grpc.ClientUnaryCall;
    relogin(request: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse) => void): grpc.ClientUnaryCall;
    relogin(request: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse) => void): grpc.ClientUnaryCall;
    sendNotification(request: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse) => void): grpc.ClientUnaryCall;
    sendNotification(request: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse) => void): grpc.ClientUnaryCall;
    sendNotification(request: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse) => void): grpc.ClientUnaryCall;
    promptMFA(request: teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFARequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFAResponse) => void): grpc.ClientUnaryCall;
    promptMFA(request: teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFARequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFAResponse) => void): grpc.ClientUnaryCall;
    promptMFA(request: teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFARequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFAResponse) => void): grpc.ClientUnaryCall;
}

export class TshdEventsServiceClient extends grpc.Client implements ITshdEventsServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public relogin(request: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse) => void): grpc.ClientUnaryCall;
    public relogin(request: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse) => void): grpc.ClientUnaryCall;
    public relogin(request: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse) => void): grpc.ClientUnaryCall;
    public sendNotification(request: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse) => void): grpc.ClientUnaryCall;
    public sendNotification(request: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse) => void): grpc.ClientUnaryCall;
    public sendNotification(request: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse) => void): grpc.ClientUnaryCall;
    public promptMFA(request: teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFARequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFAResponse) => void): grpc.ClientUnaryCall;
    public promptMFA(request: teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFARequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFAResponse) => void): grpc.ClientUnaryCall;
    public promptMFA(request: teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFARequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.PromptMFAResponse) => void): grpc.ClientUnaryCall;
}
