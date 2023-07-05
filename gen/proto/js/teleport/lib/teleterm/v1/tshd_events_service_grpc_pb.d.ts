// package: teleport.lib.teleterm.v1
// file: teleport/lib/teleterm/v1/tshd_events_service.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as teleport_lib_teleterm_v1_tshd_events_service_pb from "../../../../teleport/lib/teleterm/v1/tshd_events_service_pb";

interface ITshdEventsServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    relogin: ITshdEventsServiceService_IRelogin;
    sendNotification: ITshdEventsServiceService_ISendNotification;
    headlessAuthentication: ITshdEventsServiceService_IHeadlessAuthentication;
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
interface ITshdEventsServiceService_IHeadlessAuthentication extends grpc.MethodDefinition<teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationRequest, teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationResponse> {
    path: "/teleport.lib.teleterm.v1.TshdEventsService/HeadlessAuthentication";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationResponse>;
}

export const TshdEventsServiceService: ITshdEventsServiceService;

export interface ITshdEventsServiceServer {
    relogin: grpc.handleUnaryCall<teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest, teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse>;
    sendNotification: grpc.handleUnaryCall<teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest, teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse>;
    headlessAuthentication: grpc.handleUnaryCall<teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationRequest, teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationResponse>;
}

export interface ITshdEventsServiceClient {
    relogin(request: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse) => void): grpc.ClientUnaryCall;
    relogin(request: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse) => void): grpc.ClientUnaryCall;
    relogin(request: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse) => void): grpc.ClientUnaryCall;
    sendNotification(request: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse) => void): grpc.ClientUnaryCall;
    sendNotification(request: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse) => void): grpc.ClientUnaryCall;
    sendNotification(request: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse) => void): grpc.ClientUnaryCall;
    headlessAuthentication(request: teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationResponse) => void): grpc.ClientUnaryCall;
    headlessAuthentication(request: teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationResponse) => void): grpc.ClientUnaryCall;
    headlessAuthentication(request: teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationResponse) => void): grpc.ClientUnaryCall;
}

export class TshdEventsServiceClient extends grpc.Client implements ITshdEventsServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public relogin(request: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse) => void): grpc.ClientUnaryCall;
    public relogin(request: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse) => void): grpc.ClientUnaryCall;
    public relogin(request: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.ReloginResponse) => void): grpc.ClientUnaryCall;
    public sendNotification(request: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse) => void): grpc.ClientUnaryCall;
    public sendNotification(request: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse) => void): grpc.ClientUnaryCall;
    public sendNotification(request: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.SendNotificationResponse) => void): grpc.ClientUnaryCall;
    public headlessAuthentication(request: teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationResponse) => void): grpc.ClientUnaryCall;
    public headlessAuthentication(request: teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationResponse) => void): grpc.ClientUnaryCall;
    public headlessAuthentication(request: teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_tshd_events_service_pb.HeadlessAuthenticationResponse) => void): grpc.ClientUnaryCall;
}
