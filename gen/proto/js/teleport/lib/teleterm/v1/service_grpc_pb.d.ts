// package: teleport.lib.teleterm.v1
// file: teleport/lib/teleterm/v1/service.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as teleport_lib_teleterm_v1_service_pb from "../../../../teleport/lib/teleterm/v1/service_pb";
import * as teleport_lib_teleterm_v1_app_pb from "../../../../teleport/lib/teleterm/v1/app_pb";
import * as teleport_lib_teleterm_v1_auth_settings_pb from "../../../../teleport/lib/teleterm/v1/auth_settings_pb";
import * as teleport_lib_teleterm_v1_cluster_pb from "../../../../teleport/lib/teleterm/v1/cluster_pb";
import * as teleport_lib_teleterm_v1_database_pb from "../../../../teleport/lib/teleterm/v1/database_pb";
import * as teleport_lib_teleterm_v1_gateway_pb from "../../../../teleport/lib/teleterm/v1/gateway_pb";
import * as teleport_lib_teleterm_v1_kube_pb from "../../../../teleport/lib/teleterm/v1/kube_pb";
import * as teleport_lib_teleterm_v1_server_pb from "../../../../teleport/lib/teleterm/v1/server_pb";

interface ITerminalServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    listRootClusters: ITerminalServiceService_IListRootClusters;
    listLeafClusters: ITerminalServiceService_IListLeafClusters;
    listDatabases: ITerminalServiceService_IListDatabases;
    listDatabaseUsers: ITerminalServiceService_IListDatabaseUsers;
    listServers: ITerminalServiceService_IListServers;
    listKubes: ITerminalServiceService_IListKubes;
    listApps: ITerminalServiceService_IListApps;
    addCluster: ITerminalServiceService_IAddCluster;
    removeCluster: ITerminalServiceService_IRemoveCluster;
    listGateways: ITerminalServiceService_IListGateways;
    createGateway: ITerminalServiceService_ICreateGateway;
    removeGateway: ITerminalServiceService_IRemoveGateway;
    restartGateway: ITerminalServiceService_IRestartGateway;
    setGatewayTargetSubresourceName: ITerminalServiceService_ISetGatewayTargetSubresourceName;
    setGatewayLocalPort: ITerminalServiceService_ISetGatewayLocalPort;
    getAuthSettings: ITerminalServiceService_IGetAuthSettings;
    getCluster: ITerminalServiceService_IGetCluster;
    login: ITerminalServiceService_ILogin;
    loginPasswordless: ITerminalServiceService_ILoginPasswordless;
    logout: ITerminalServiceService_ILogout;
}

interface ITerminalServiceService_IListRootClusters extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.ListClustersRequest, teleport_lib_teleterm_v1_service_pb.ListClustersResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/ListRootClusters";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListClustersRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListClustersRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListClustersResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListClustersResponse>;
}
interface ITerminalServiceService_IListLeafClusters extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest, teleport_lib_teleterm_v1_service_pb.ListClustersResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/ListLeafClusters";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListClustersResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListClustersResponse>;
}
interface ITerminalServiceService_IListDatabases extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.ListDatabasesRequest, teleport_lib_teleterm_v1_service_pb.ListDatabasesResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/ListDatabases";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListDatabasesRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListDatabasesRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListDatabasesResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListDatabasesResponse>;
}
interface ITerminalServiceService_IListDatabaseUsers extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest, teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/ListDatabaseUsers";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse>;
}
interface ITerminalServiceService_IListServers extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.ListServersRequest, teleport_lib_teleterm_v1_service_pb.ListServersResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/ListServers";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListServersRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListServersRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListServersResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListServersResponse>;
}
interface ITerminalServiceService_IListKubes extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.ListKubesRequest, teleport_lib_teleterm_v1_service_pb.ListKubesResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/ListKubes";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListKubesRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListKubesRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListKubesResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListKubesResponse>;
}
interface ITerminalServiceService_IListApps extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.ListAppsRequest, teleport_lib_teleterm_v1_service_pb.ListAppsResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/ListApps";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListAppsRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListAppsRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListAppsResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListAppsResponse>;
}
interface ITerminalServiceService_IAddCluster extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.AddClusterRequest, teleport_lib_teleterm_v1_cluster_pb.Cluster> {
    path: "/teleport.lib.teleterm.v1.TerminalService/AddCluster";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.AddClusterRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.AddClusterRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_cluster_pb.Cluster>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_cluster_pb.Cluster>;
}
interface ITerminalServiceService_IRemoveCluster extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.RemoveClusterRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/RemoveCluster";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.RemoveClusterRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.RemoveClusterRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_IListGateways extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.ListGatewaysRequest, teleport_lib_teleterm_v1_service_pb.ListGatewaysResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/ListGateways";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListGatewaysRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListGatewaysRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListGatewaysResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListGatewaysResponse>;
}
interface ITerminalServiceService_ICreateGateway extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.CreateGatewayRequest, teleport_lib_teleterm_v1_gateway_pb.Gateway> {
    path: "/teleport.lib.teleterm.v1.TerminalService/CreateGateway";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.CreateGatewayRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.CreateGatewayRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_gateway_pb.Gateway>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_gateway_pb.Gateway>;
}
interface ITerminalServiceService_IRemoveGateway extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.RemoveGatewayRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/RemoveGateway";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.RemoveGatewayRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.RemoveGatewayRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_IRestartGateway extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.RestartGatewayRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/RestartGateway";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.RestartGatewayRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.RestartGatewayRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_ISetGatewayTargetSubresourceName extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest, teleport_lib_teleterm_v1_gateway_pb.Gateway> {
    path: "/teleport.lib.teleterm.v1.TerminalService/SetGatewayTargetSubresourceName";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_gateway_pb.Gateway>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_gateway_pb.Gateway>;
}
interface ITerminalServiceService_ISetGatewayLocalPort extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.SetGatewayLocalPortRequest, teleport_lib_teleterm_v1_gateway_pb.Gateway> {
    path: "/teleport.lib.teleterm.v1.TerminalService/SetGatewayLocalPort";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.SetGatewayLocalPortRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.SetGatewayLocalPortRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_gateway_pb.Gateway>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_gateway_pb.Gateway>;
}
interface ITerminalServiceService_IGetAuthSettings extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.GetAuthSettingsRequest, teleport_lib_teleterm_v1_auth_settings_pb.AuthSettings> {
    path: "/teleport.lib.teleterm.v1.TerminalService/GetAuthSettings";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetAuthSettingsRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetAuthSettingsRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_auth_settings_pb.AuthSettings>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_auth_settings_pb.AuthSettings>;
}
interface ITerminalServiceService_IGetCluster extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.GetClusterRequest, teleport_lib_teleterm_v1_cluster_pb.Cluster> {
    path: "/teleport.lib.teleterm.v1.TerminalService/GetCluster";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetClusterRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetClusterRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_cluster_pb.Cluster>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_cluster_pb.Cluster>;
}
interface ITerminalServiceService_ILogin extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.LoginRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/Login";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.LoginRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.LoginRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_ILoginPasswordless extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.LoginPasswordlessRequest, teleport_lib_teleterm_v1_service_pb.LoginPasswordlessResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/LoginPasswordless";
    requestStream: true;
    responseStream: true;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.LoginPasswordlessRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.LoginPasswordlessRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.LoginPasswordlessResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.LoginPasswordlessResponse>;
}
interface ITerminalServiceService_ILogout extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.LogoutRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/Logout";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.LogoutRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.LogoutRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
}

export const TerminalServiceService: ITerminalServiceService;

export interface ITerminalServiceServer {
    listRootClusters: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.ListClustersRequest, teleport_lib_teleterm_v1_service_pb.ListClustersResponse>;
    listLeafClusters: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest, teleport_lib_teleterm_v1_service_pb.ListClustersResponse>;
    listDatabases: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.ListDatabasesRequest, teleport_lib_teleterm_v1_service_pb.ListDatabasesResponse>;
    listDatabaseUsers: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest, teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse>;
    listServers: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.ListServersRequest, teleport_lib_teleterm_v1_service_pb.ListServersResponse>;
    listKubes: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.ListKubesRequest, teleport_lib_teleterm_v1_service_pb.ListKubesResponse>;
    listApps: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.ListAppsRequest, teleport_lib_teleterm_v1_service_pb.ListAppsResponse>;
    addCluster: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.AddClusterRequest, teleport_lib_teleterm_v1_cluster_pb.Cluster>;
    removeCluster: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.RemoveClusterRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    listGateways: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.ListGatewaysRequest, teleport_lib_teleterm_v1_service_pb.ListGatewaysResponse>;
    createGateway: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.CreateGatewayRequest, teleport_lib_teleterm_v1_gateway_pb.Gateway>;
    removeGateway: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.RemoveGatewayRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    restartGateway: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.RestartGatewayRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    setGatewayTargetSubresourceName: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest, teleport_lib_teleterm_v1_gateway_pb.Gateway>;
    setGatewayLocalPort: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.SetGatewayLocalPortRequest, teleport_lib_teleterm_v1_gateway_pb.Gateway>;
    getAuthSettings: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.GetAuthSettingsRequest, teleport_lib_teleterm_v1_auth_settings_pb.AuthSettings>;
    getCluster: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.GetClusterRequest, teleport_lib_teleterm_v1_cluster_pb.Cluster>;
    login: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.LoginRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    loginPasswordless: grpc.handleBidiStreamingCall<teleport_lib_teleterm_v1_service_pb.LoginPasswordlessRequest, teleport_lib_teleterm_v1_service_pb.LoginPasswordlessResponse>;
    logout: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.LogoutRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
}

export interface ITerminalServiceClient {
    listRootClusters(request: teleport_lib_teleterm_v1_service_pb.ListClustersRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listRootClusters(request: teleport_lib_teleterm_v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listRootClusters(request: teleport_lib_teleterm_v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listLeafClusters(request: teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listLeafClusters(request: teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listLeafClusters(request: teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listDatabases(request: teleport_lib_teleterm_v1_service_pb.ListDatabasesRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabasesResponse) => void): grpc.ClientUnaryCall;
    listDatabases(request: teleport_lib_teleterm_v1_service_pb.ListDatabasesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabasesResponse) => void): grpc.ClientUnaryCall;
    listDatabases(request: teleport_lib_teleterm_v1_service_pb.ListDatabasesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabasesResponse) => void): grpc.ClientUnaryCall;
    listDatabaseUsers(request: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    listDatabaseUsers(request: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    listDatabaseUsers(request: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    listServers(request: teleport_lib_teleterm_v1_service_pb.ListServersRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListServersResponse) => void): grpc.ClientUnaryCall;
    listServers(request: teleport_lib_teleterm_v1_service_pb.ListServersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListServersResponse) => void): grpc.ClientUnaryCall;
    listServers(request: teleport_lib_teleterm_v1_service_pb.ListServersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListServersResponse) => void): grpc.ClientUnaryCall;
    listKubes(request: teleport_lib_teleterm_v1_service_pb.ListKubesRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListKubesResponse) => void): grpc.ClientUnaryCall;
    listKubes(request: teleport_lib_teleterm_v1_service_pb.ListKubesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListKubesResponse) => void): grpc.ClientUnaryCall;
    listKubes(request: teleport_lib_teleterm_v1_service_pb.ListKubesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListKubesResponse) => void): grpc.ClientUnaryCall;
    listApps(request: teleport_lib_teleterm_v1_service_pb.ListAppsRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    listApps(request: teleport_lib_teleterm_v1_service_pb.ListAppsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    listApps(request: teleport_lib_teleterm_v1_service_pb.ListAppsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    addCluster(request: teleport_lib_teleterm_v1_service_pb.AddClusterRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    addCluster(request: teleport_lib_teleterm_v1_service_pb.AddClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    addCluster(request: teleport_lib_teleterm_v1_service_pb.AddClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    removeCluster(request: teleport_lib_teleterm_v1_service_pb.RemoveClusterRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    removeCluster(request: teleport_lib_teleterm_v1_service_pb.RemoveClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    removeCluster(request: teleport_lib_teleterm_v1_service_pb.RemoveClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    listGateways(request: teleport_lib_teleterm_v1_service_pb.ListGatewaysRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    listGateways(request: teleport_lib_teleterm_v1_service_pb.ListGatewaysRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    listGateways(request: teleport_lib_teleterm_v1_service_pb.ListGatewaysRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    createGateway(request: teleport_lib_teleterm_v1_service_pb.CreateGatewayRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    createGateway(request: teleport_lib_teleterm_v1_service_pb.CreateGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    createGateway(request: teleport_lib_teleterm_v1_service_pb.CreateGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    removeGateway(request: teleport_lib_teleterm_v1_service_pb.RemoveGatewayRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    removeGateway(request: teleport_lib_teleterm_v1_service_pb.RemoveGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    removeGateway(request: teleport_lib_teleterm_v1_service_pb.RemoveGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    restartGateway(request: teleport_lib_teleterm_v1_service_pb.RestartGatewayRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    restartGateway(request: teleport_lib_teleterm_v1_service_pb.RestartGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    restartGateway(request: teleport_lib_teleterm_v1_service_pb.RestartGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    setGatewayTargetSubresourceName(request: teleport_lib_teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    setGatewayTargetSubresourceName(request: teleport_lib_teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    setGatewayTargetSubresourceName(request: teleport_lib_teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    setGatewayLocalPort(request: teleport_lib_teleterm_v1_service_pb.SetGatewayLocalPortRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    setGatewayLocalPort(request: teleport_lib_teleterm_v1_service_pb.SetGatewayLocalPortRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    setGatewayLocalPort(request: teleport_lib_teleterm_v1_service_pb.SetGatewayLocalPortRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    getAuthSettings(request: teleport_lib_teleterm_v1_service_pb.GetAuthSettingsRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    getAuthSettings(request: teleport_lib_teleterm_v1_service_pb.GetAuthSettingsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    getAuthSettings(request: teleport_lib_teleterm_v1_service_pb.GetAuthSettingsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    getCluster(request: teleport_lib_teleterm_v1_service_pb.GetClusterRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    getCluster(request: teleport_lib_teleterm_v1_service_pb.GetClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    getCluster(request: teleport_lib_teleterm_v1_service_pb.GetClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    login(request: teleport_lib_teleterm_v1_service_pb.LoginRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    login(request: teleport_lib_teleterm_v1_service_pb.LoginRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    login(request: teleport_lib_teleterm_v1_service_pb.LoginRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    loginPasswordless(): grpc.ClientDuplexStream<teleport_lib_teleterm_v1_service_pb.LoginPasswordlessRequest, teleport_lib_teleterm_v1_service_pb.LoginPasswordlessResponse>;
    loginPasswordless(options: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleport_lib_teleterm_v1_service_pb.LoginPasswordlessRequest, teleport_lib_teleterm_v1_service_pb.LoginPasswordlessResponse>;
    loginPasswordless(metadata: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleport_lib_teleterm_v1_service_pb.LoginPasswordlessRequest, teleport_lib_teleterm_v1_service_pb.LoginPasswordlessResponse>;
    logout(request: teleport_lib_teleterm_v1_service_pb.LogoutRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    logout(request: teleport_lib_teleterm_v1_service_pb.LogoutRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    logout(request: teleport_lib_teleterm_v1_service_pb.LogoutRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
}

export class TerminalServiceClient extends grpc.Client implements ITerminalServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public listRootClusters(request: teleport_lib_teleterm_v1_service_pb.ListClustersRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listRootClusters(request: teleport_lib_teleterm_v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listRootClusters(request: teleport_lib_teleterm_v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listLeafClusters(request: teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listLeafClusters(request: teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listLeafClusters(request: teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listDatabases(request: teleport_lib_teleterm_v1_service_pb.ListDatabasesRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabasesResponse) => void): grpc.ClientUnaryCall;
    public listDatabases(request: teleport_lib_teleterm_v1_service_pb.ListDatabasesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabasesResponse) => void): grpc.ClientUnaryCall;
    public listDatabases(request: teleport_lib_teleterm_v1_service_pb.ListDatabasesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabasesResponse) => void): grpc.ClientUnaryCall;
    public listDatabaseUsers(request: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    public listDatabaseUsers(request: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    public listDatabaseUsers(request: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    public listServers(request: teleport_lib_teleterm_v1_service_pb.ListServersRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListServersResponse) => void): grpc.ClientUnaryCall;
    public listServers(request: teleport_lib_teleterm_v1_service_pb.ListServersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListServersResponse) => void): grpc.ClientUnaryCall;
    public listServers(request: teleport_lib_teleterm_v1_service_pb.ListServersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListServersResponse) => void): grpc.ClientUnaryCall;
    public listKubes(request: teleport_lib_teleterm_v1_service_pb.ListKubesRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListKubesResponse) => void): grpc.ClientUnaryCall;
    public listKubes(request: teleport_lib_teleterm_v1_service_pb.ListKubesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListKubesResponse) => void): grpc.ClientUnaryCall;
    public listKubes(request: teleport_lib_teleterm_v1_service_pb.ListKubesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListKubesResponse) => void): grpc.ClientUnaryCall;
    public listApps(request: teleport_lib_teleterm_v1_service_pb.ListAppsRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    public listApps(request: teleport_lib_teleterm_v1_service_pb.ListAppsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    public listApps(request: teleport_lib_teleterm_v1_service_pb.ListAppsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    public addCluster(request: teleport_lib_teleterm_v1_service_pb.AddClusterRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public addCluster(request: teleport_lib_teleterm_v1_service_pb.AddClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public addCluster(request: teleport_lib_teleterm_v1_service_pb.AddClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public removeCluster(request: teleport_lib_teleterm_v1_service_pb.RemoveClusterRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public removeCluster(request: teleport_lib_teleterm_v1_service_pb.RemoveClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public removeCluster(request: teleport_lib_teleterm_v1_service_pb.RemoveClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public listGateways(request: teleport_lib_teleterm_v1_service_pb.ListGatewaysRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    public listGateways(request: teleport_lib_teleterm_v1_service_pb.ListGatewaysRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    public listGateways(request: teleport_lib_teleterm_v1_service_pb.ListGatewaysRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    public createGateway(request: teleport_lib_teleterm_v1_service_pb.CreateGatewayRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public createGateway(request: teleport_lib_teleterm_v1_service_pb.CreateGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public createGateway(request: teleport_lib_teleterm_v1_service_pb.CreateGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public removeGateway(request: teleport_lib_teleterm_v1_service_pb.RemoveGatewayRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public removeGateway(request: teleport_lib_teleterm_v1_service_pb.RemoveGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public removeGateway(request: teleport_lib_teleterm_v1_service_pb.RemoveGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public restartGateway(request: teleport_lib_teleterm_v1_service_pb.RestartGatewayRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public restartGateway(request: teleport_lib_teleterm_v1_service_pb.RestartGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public restartGateway(request: teleport_lib_teleterm_v1_service_pb.RestartGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public setGatewayTargetSubresourceName(request: teleport_lib_teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public setGatewayTargetSubresourceName(request: teleport_lib_teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public setGatewayTargetSubresourceName(request: teleport_lib_teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public setGatewayLocalPort(request: teleport_lib_teleterm_v1_service_pb.SetGatewayLocalPortRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public setGatewayLocalPort(request: teleport_lib_teleterm_v1_service_pb.SetGatewayLocalPortRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public setGatewayLocalPort(request: teleport_lib_teleterm_v1_service_pb.SetGatewayLocalPortRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public getAuthSettings(request: teleport_lib_teleterm_v1_service_pb.GetAuthSettingsRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    public getAuthSettings(request: teleport_lib_teleterm_v1_service_pb.GetAuthSettingsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    public getAuthSettings(request: teleport_lib_teleterm_v1_service_pb.GetAuthSettingsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    public getCluster(request: teleport_lib_teleterm_v1_service_pb.GetClusterRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public getCluster(request: teleport_lib_teleterm_v1_service_pb.GetClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public getCluster(request: teleport_lib_teleterm_v1_service_pb.GetClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public login(request: teleport_lib_teleterm_v1_service_pb.LoginRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public login(request: teleport_lib_teleterm_v1_service_pb.LoginRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public login(request: teleport_lib_teleterm_v1_service_pb.LoginRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public loginPasswordless(options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleport_lib_teleterm_v1_service_pb.LoginPasswordlessRequest, teleport_lib_teleterm_v1_service_pb.LoginPasswordlessResponse>;
    public loginPasswordless(metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleport_lib_teleterm_v1_service_pb.LoginPasswordlessRequest, teleport_lib_teleterm_v1_service_pb.LoginPasswordlessResponse>;
    public logout(request: teleport_lib_teleterm_v1_service_pb.LogoutRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public logout(request: teleport_lib_teleterm_v1_service_pb.LogoutRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public logout(request: teleport_lib_teleterm_v1_service_pb.LogoutRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
}
