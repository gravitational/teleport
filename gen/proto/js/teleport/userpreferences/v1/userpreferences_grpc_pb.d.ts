// package: teleport.userpreferences.v1
// file: teleport/userpreferences/v1/userpreferences.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as teleport_userpreferences_v1_userpreferences_pb from "../../../teleport/userpreferences/v1/userpreferences_pb";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as teleport_userpreferences_v1_assist_pb from "../../../teleport/userpreferences/v1/assist_pb";
import * as teleport_userpreferences_v1_cluster_preferences_pb from "../../../teleport/userpreferences/v1/cluster_preferences_pb";
import * as teleport_userpreferences_v1_onboard_pb from "../../../teleport/userpreferences/v1/onboard_pb";
import * as teleport_userpreferences_v1_theme_pb from "../../../teleport/userpreferences/v1/theme_pb";
import * as teleport_userpreferences_v1_unified_resource_preferences_pb from "../../../teleport/userpreferences/v1/unified_resource_preferences_pb";

interface IUserPreferencesServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    getUserPreferences: IUserPreferencesServiceService_IGetUserPreferences;
    upsertUserPreferences: IUserPreferencesServiceService_IUpsertUserPreferences;
}

interface IUserPreferencesServiceService_IGetUserPreferences extends grpc.MethodDefinition<teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesRequest, teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesResponse> {
    path: "/teleport.userpreferences.v1.UserPreferencesService/GetUserPreferences";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesRequest>;
    requestDeserialize: grpc.deserialize<teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesRequest>;
    responseSerialize: grpc.serialize<teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesResponse>;
    responseDeserialize: grpc.deserialize<teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesResponse>;
}
interface IUserPreferencesServiceService_IUpsertUserPreferences extends grpc.MethodDefinition<teleport_userpreferences_v1_userpreferences_pb.UpsertUserPreferencesRequest, google_protobuf_empty_pb.Empty> {
    path: "/teleport.userpreferences.v1.UserPreferencesService/UpsertUserPreferences";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_userpreferences_v1_userpreferences_pb.UpsertUserPreferencesRequest>;
    requestDeserialize: grpc.deserialize<teleport_userpreferences_v1_userpreferences_pb.UpsertUserPreferencesRequest>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}

export const UserPreferencesServiceService: IUserPreferencesServiceService;

export interface IUserPreferencesServiceServer {
    getUserPreferences: grpc.handleUnaryCall<teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesRequest, teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesResponse>;
    upsertUserPreferences: grpc.handleUnaryCall<teleport_userpreferences_v1_userpreferences_pb.UpsertUserPreferencesRequest, google_protobuf_empty_pb.Empty>;
}

export interface IUserPreferencesServiceClient {
    getUserPreferences(request: teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesRequest, callback: (error: grpc.ServiceError | null, response: teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesResponse) => void): grpc.ClientUnaryCall;
    getUserPreferences(request: teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesResponse) => void): grpc.ClientUnaryCall;
    getUserPreferences(request: teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesResponse) => void): grpc.ClientUnaryCall;
    upsertUserPreferences(request: teleport_userpreferences_v1_userpreferences_pb.UpsertUserPreferencesRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    upsertUserPreferences(request: teleport_userpreferences_v1_userpreferences_pb.UpsertUserPreferencesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    upsertUserPreferences(request: teleport_userpreferences_v1_userpreferences_pb.UpsertUserPreferencesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
}

export class UserPreferencesServiceClient extends grpc.Client implements IUserPreferencesServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public getUserPreferences(request: teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesRequest, callback: (error: grpc.ServiceError | null, response: teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesResponse) => void): grpc.ClientUnaryCall;
    public getUserPreferences(request: teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesResponse) => void): grpc.ClientUnaryCall;
    public getUserPreferences(request: teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_userpreferences_v1_userpreferences_pb.GetUserPreferencesResponse) => void): grpc.ClientUnaryCall;
    public upsertUserPreferences(request: teleport_userpreferences_v1_userpreferences_pb.UpsertUserPreferencesRequest, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public upsertUserPreferences(request: teleport_userpreferences_v1_userpreferences_pb.UpsertUserPreferencesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
    public upsertUserPreferences(request: teleport_userpreferences_v1_userpreferences_pb.UpsertUserPreferencesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientUnaryCall;
}
