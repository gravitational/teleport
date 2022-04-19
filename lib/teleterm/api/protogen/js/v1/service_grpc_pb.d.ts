// package: teleport.terminal.v1
// file: v1/service.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as v1_service_pb from "../v1/service_pb";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as v1_cluster_pb from "../v1/cluster_pb";
import * as v1_database_pb from "../v1/database_pb";
import * as v1_gateway_pb from "../v1/gateway_pb";
import * as v1_kube_pb from "../v1/kube_pb";
import * as v1_app_pb from "../v1/app_pb";
import * as v1_server_pb from "../v1/server_pb";
import * as v1_auth_settings_pb from "../v1/auth_settings_pb";

interface ITerminalServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    listRootClusters: ITerminalServiceService_IListRootClusters;
    listLeafClusters: ITerminalServiceService_IListLeafClusters;
    listDatabases: ITerminalServiceService_IListDatabases;
    listGateways: ITerminalServiceService_IListGateways;
    listServers: ITerminalServiceService_IListServers;
    listKubes: ITerminalServiceService_IListKubes;
    listApps: ITerminalServiceService_IListApps;
    createGateway: ITerminalServiceService_ICreateGateway;
    addCluster: ITerminalServiceService_IAddCluster;
    removeCluster: ITerminalServiceService_IRemoveCluster;
    removeGateway: ITerminalServiceService_IRemoveGateway;
    getAuthSettings: ITerminalServiceService_IGetAuthSettings;
    getCluster: ITerminalServiceService_IGetCluster;
    login: ITerminalServiceService_ILogin;
    logout: ITerminalServiceService_ILogout;
}

interface ITerminalServiceService_IListRootClusters extends grpc.MethodDefinition<v1_service_pb.ListClustersRequest, v1_service_pb.ListClustersResponse> {
    path: "/teleport.terminal.v1.TerminalService/ListRootClusters";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.ListClustersRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.ListClustersRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.ListClustersResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.ListClustersResponse>;
}
interface ITerminalServiceService_IListLeafClusters extends grpc.MethodDefinition<v1_service_pb.ListLeafClustersRequest, v1_service_pb.ListClustersResponse> {
    path: "/teleport.terminal.v1.TerminalService/ListLeafClusters";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.ListLeafClustersRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.ListLeafClustersRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.ListClustersResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.ListClustersResponse>;
}
interface ITerminalServiceService_IListDatabases extends grpc.MethodDefinition<v1_service_pb.ListDatabasesRequest, v1_service_pb.ListDatabasesResponse> {
    path: "/teleport.terminal.v1.TerminalService/ListDatabases";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.ListDatabasesRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.ListDatabasesRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.ListDatabasesResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.ListDatabasesResponse>;
}
interface ITerminalServiceService_IListGateways extends grpc.MethodDefinition<v1_service_pb.ListGatewaysRequest, v1_service_pb.ListGatewaysResponse> {
    path: "/teleport.terminal.v1.TerminalService/ListGateways";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.ListGatewaysRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.ListGatewaysRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.ListGatewaysResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.ListGatewaysResponse>;
}
interface ITerminalServiceService_IListServers extends grpc.MethodDefinition<v1_service_pb.ListServersRequest, v1_service_pb.ListServersResponse> {
    path: "/teleport.terminal.v1.TerminalService/ListServers";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.ListServersRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.ListServersRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.ListServersResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.ListServersResponse>;
}
interface ITerminalServiceService_IListKubes extends grpc.MethodDefinition<v1_service_pb.ListKubesRequest, v1_service_pb.ListKubesResponse> {
    path: "/teleport.terminal.v1.TerminalService/ListKubes";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.ListKubesRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.ListKubesRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.ListKubesResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.ListKubesResponse>;
}
interface ITerminalServiceService_IListApps extends grpc.MethodDefinition<v1_service_pb.ListAppsRequest, v1_service_pb.ListAppsResponse> {
    path: "/teleport.terminal.v1.TerminalService/ListApps";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.ListAppsRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.ListAppsRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.ListAppsResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.ListAppsResponse>;
}
interface ITerminalServiceService_ICreateGateway extends grpc.MethodDefinition<v1_service_pb.CreateGatewayRequest, v1_gateway_pb.Gateway> {
    path: "/teleport.terminal.v1.TerminalService/CreateGateway";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.CreateGatewayRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.CreateGatewayRequest>;
    responseSerialize: grpc.serialize<v1_gateway_pb.Gateway>;
    responseDeserialize: grpc.deserialize<v1_gateway_pb.Gateway>;
}
interface ITerminalServiceService_IAddCluster extends grpc.MethodDefinition<v1_service_pb.AddClusterRequest, v1_cluster_pb.Cluster> {
    path: "/teleport.terminal.v1.TerminalService/AddCluster";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.AddClusterRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.AddClusterRequest>;
    responseSerialize: grpc.serialize<v1_cluster_pb.Cluster>;
    responseDeserialize: grpc.deserialize<v1_cluster_pb.Cluster>;
}
interface ITerminalServiceService_IRemoveCluster extends grpc.MethodDefinition<v1_service_pb.RemoveClusterRequest, v1_service_pb.EmptyResponse> {
    path: "/teleport.terminal.v1.TerminalService/RemoveCluster";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.RemoveClusterRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.RemoveClusterRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_IRemoveGateway extends grpc.MethodDefinition<v1_service_pb.RemoveGatewayRequest, v1_service_pb.EmptyResponse> {
    path: "/teleport.terminal.v1.TerminalService/RemoveGateway";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.RemoveGatewayRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.RemoveGatewayRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_IGetAuthSettings extends grpc.MethodDefinition<v1_service_pb.GetAuthSettingsRequest, v1_auth_settings_pb.AuthSettings> {
    path: "/teleport.terminal.v1.TerminalService/GetAuthSettings";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.GetAuthSettingsRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.GetAuthSettingsRequest>;
    responseSerialize: grpc.serialize<v1_auth_settings_pb.AuthSettings>;
    responseDeserialize: grpc.deserialize<v1_auth_settings_pb.AuthSettings>;
}
interface ITerminalServiceService_IGetCluster extends grpc.MethodDefinition<v1_service_pb.GetClusterRequest, v1_cluster_pb.Cluster> {
    path: "/teleport.terminal.v1.TerminalService/GetCluster";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.GetClusterRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.GetClusterRequest>;
    responseSerialize: grpc.serialize<v1_cluster_pb.Cluster>;
    responseDeserialize: grpc.deserialize<v1_cluster_pb.Cluster>;
}
interface ITerminalServiceService_ILogin extends grpc.MethodDefinition<v1_service_pb.LoginRequest, v1_service_pb.EmptyResponse> {
    path: "/teleport.terminal.v1.TerminalService/Login";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.LoginRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.LoginRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_ILogout extends grpc.MethodDefinition<v1_service_pb.LogoutRequest, v1_service_pb.EmptyResponse> {
    path: "/teleport.terminal.v1.TerminalService/Logout";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.LogoutRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.LogoutRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.EmptyResponse>;
}

export const TerminalServiceService: ITerminalServiceService;

export interface ITerminalServiceServer {
    listRootClusters: grpc.handleUnaryCall<v1_service_pb.ListClustersRequest, v1_service_pb.ListClustersResponse>;
    listLeafClusters: grpc.handleUnaryCall<v1_service_pb.ListLeafClustersRequest, v1_service_pb.ListClustersResponse>;
    listDatabases: grpc.handleUnaryCall<v1_service_pb.ListDatabasesRequest, v1_service_pb.ListDatabasesResponse>;
    listGateways: grpc.handleUnaryCall<v1_service_pb.ListGatewaysRequest, v1_service_pb.ListGatewaysResponse>;
    listServers: grpc.handleUnaryCall<v1_service_pb.ListServersRequest, v1_service_pb.ListServersResponse>;
    listKubes: grpc.handleUnaryCall<v1_service_pb.ListKubesRequest, v1_service_pb.ListKubesResponse>;
    listApps: grpc.handleUnaryCall<v1_service_pb.ListAppsRequest, v1_service_pb.ListAppsResponse>;
    createGateway: grpc.handleUnaryCall<v1_service_pb.CreateGatewayRequest, v1_gateway_pb.Gateway>;
    addCluster: grpc.handleUnaryCall<v1_service_pb.AddClusterRequest, v1_cluster_pb.Cluster>;
    removeCluster: grpc.handleUnaryCall<v1_service_pb.RemoveClusterRequest, v1_service_pb.EmptyResponse>;
    removeGateway: grpc.handleUnaryCall<v1_service_pb.RemoveGatewayRequest, v1_service_pb.EmptyResponse>;
    getAuthSettings: grpc.handleUnaryCall<v1_service_pb.GetAuthSettingsRequest, v1_auth_settings_pb.AuthSettings>;
    getCluster: grpc.handleUnaryCall<v1_service_pb.GetClusterRequest, v1_cluster_pb.Cluster>;
    login: grpc.handleUnaryCall<v1_service_pb.LoginRequest, v1_service_pb.EmptyResponse>;
    logout: grpc.handleUnaryCall<v1_service_pb.LogoutRequest, v1_service_pb.EmptyResponse>;
}

export interface ITerminalServiceClient {
    listRootClusters(request: v1_service_pb.ListClustersRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listRootClusters(request: v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listRootClusters(request: v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listLeafClusters(request: v1_service_pb.ListLeafClustersRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listLeafClusters(request: v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listLeafClusters(request: v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listDatabases(request: v1_service_pb.ListDatabasesRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListDatabasesResponse) => void): grpc.ClientUnaryCall;
    listDatabases(request: v1_service_pb.ListDatabasesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListDatabasesResponse) => void): grpc.ClientUnaryCall;
    listDatabases(request: v1_service_pb.ListDatabasesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListDatabasesResponse) => void): grpc.ClientUnaryCall;
    listGateways(request: v1_service_pb.ListGatewaysRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    listGateways(request: v1_service_pb.ListGatewaysRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    listGateways(request: v1_service_pb.ListGatewaysRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    listServers(request: v1_service_pb.ListServersRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListServersResponse) => void): grpc.ClientUnaryCall;
    listServers(request: v1_service_pb.ListServersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListServersResponse) => void): grpc.ClientUnaryCall;
    listServers(request: v1_service_pb.ListServersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListServersResponse) => void): grpc.ClientUnaryCall;
    listKubes(request: v1_service_pb.ListKubesRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListKubesResponse) => void): grpc.ClientUnaryCall;
    listKubes(request: v1_service_pb.ListKubesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListKubesResponse) => void): grpc.ClientUnaryCall;
    listKubes(request: v1_service_pb.ListKubesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListKubesResponse) => void): grpc.ClientUnaryCall;
    listApps(request: v1_service_pb.ListAppsRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    listApps(request: v1_service_pb.ListAppsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    listApps(request: v1_service_pb.ListAppsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    createGateway(request: v1_service_pb.CreateGatewayRequest, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    createGateway(request: v1_service_pb.CreateGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    createGateway(request: v1_service_pb.CreateGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    addCluster(request: v1_service_pb.AddClusterRequest, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    addCluster(request: v1_service_pb.AddClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    addCluster(request: v1_service_pb.AddClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    removeCluster(request: v1_service_pb.RemoveClusterRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    removeCluster(request: v1_service_pb.RemoveClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    removeCluster(request: v1_service_pb.RemoveClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    removeGateway(request: v1_service_pb.RemoveGatewayRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    removeGateway(request: v1_service_pb.RemoveGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    removeGateway(request: v1_service_pb.RemoveGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    getAuthSettings(request: v1_service_pb.GetAuthSettingsRequest, callback: (error: grpc.ServiceError | null, response: v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    getAuthSettings(request: v1_service_pb.GetAuthSettingsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    getAuthSettings(request: v1_service_pb.GetAuthSettingsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    getCluster(request: v1_service_pb.GetClusterRequest, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    getCluster(request: v1_service_pb.GetClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    getCluster(request: v1_service_pb.GetClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    login(request: v1_service_pb.LoginRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    login(request: v1_service_pb.LoginRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    login(request: v1_service_pb.LoginRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    logout(request: v1_service_pb.LogoutRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    logout(request: v1_service_pb.LogoutRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    logout(request: v1_service_pb.LogoutRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
}

export class TerminalServiceClient extends grpc.Client implements ITerminalServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public listRootClusters(request: v1_service_pb.ListClustersRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listRootClusters(request: v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listRootClusters(request: v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listLeafClusters(request: v1_service_pb.ListLeafClustersRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listLeafClusters(request: v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listLeafClusters(request: v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listDatabases(request: v1_service_pb.ListDatabasesRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListDatabasesResponse) => void): grpc.ClientUnaryCall;
    public listDatabases(request: v1_service_pb.ListDatabasesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListDatabasesResponse) => void): grpc.ClientUnaryCall;
    public listDatabases(request: v1_service_pb.ListDatabasesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListDatabasesResponse) => void): grpc.ClientUnaryCall;
    public listGateways(request: v1_service_pb.ListGatewaysRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    public listGateways(request: v1_service_pb.ListGatewaysRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    public listGateways(request: v1_service_pb.ListGatewaysRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    public listServers(request: v1_service_pb.ListServersRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListServersResponse) => void): grpc.ClientUnaryCall;
    public listServers(request: v1_service_pb.ListServersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListServersResponse) => void): grpc.ClientUnaryCall;
    public listServers(request: v1_service_pb.ListServersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListServersResponse) => void): grpc.ClientUnaryCall;
    public listKubes(request: v1_service_pb.ListKubesRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListKubesResponse) => void): grpc.ClientUnaryCall;
    public listKubes(request: v1_service_pb.ListKubesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListKubesResponse) => void): grpc.ClientUnaryCall;
    public listKubes(request: v1_service_pb.ListKubesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListKubesResponse) => void): grpc.ClientUnaryCall;
    public listApps(request: v1_service_pb.ListAppsRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    public listApps(request: v1_service_pb.ListAppsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    public listApps(request: v1_service_pb.ListAppsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    public createGateway(request: v1_service_pb.CreateGatewayRequest, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public createGateway(request: v1_service_pb.CreateGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public createGateway(request: v1_service_pb.CreateGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public addCluster(request: v1_service_pb.AddClusterRequest, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public addCluster(request: v1_service_pb.AddClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public addCluster(request: v1_service_pb.AddClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public removeCluster(request: v1_service_pb.RemoveClusterRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public removeCluster(request: v1_service_pb.RemoveClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public removeCluster(request: v1_service_pb.RemoveClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public removeGateway(request: v1_service_pb.RemoveGatewayRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public removeGateway(request: v1_service_pb.RemoveGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public removeGateway(request: v1_service_pb.RemoveGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public getAuthSettings(request: v1_service_pb.GetAuthSettingsRequest, callback: (error: grpc.ServiceError | null, response: v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    public getAuthSettings(request: v1_service_pb.GetAuthSettingsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    public getAuthSettings(request: v1_service_pb.GetAuthSettingsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    public getCluster(request: v1_service_pb.GetClusterRequest, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public getCluster(request: v1_service_pb.GetClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public getCluster(request: v1_service_pb.GetClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public login(request: v1_service_pb.LoginRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public login(request: v1_service_pb.LoginRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public login(request: v1_service_pb.LoginRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public logout(request: v1_service_pb.LogoutRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public logout(request: v1_service_pb.LogoutRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public logout(request: v1_service_pb.LogoutRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
}
