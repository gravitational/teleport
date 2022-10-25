// package: teleport.terminal.v1
// file: v1/service.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as v1_service_pb from "../v1/service_pb";
import * as v1_access_request_pb from "../v1/access_request_pb";
import * as v1_app_pb from "../v1/app_pb";
import * as v1_auth_settings_pb from "../v1/auth_settings_pb";
import * as v1_cluster_pb from "../v1/cluster_pb";
import * as v1_database_pb from "../v1/database_pb";
import * as v1_gateway_pb from "../v1/gateway_pb";
import * as v1_kube_pb from "../v1/kube_pb";
import * as v1_server_pb from "../v1/server_pb";

interface ITerminalServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    updateTshdEventsServerAddress: ITerminalServiceService_IUpdateTshdEventsServerAddress;
    listRootClusters: ITerminalServiceService_IListRootClusters;
    listLeafClusters: ITerminalServiceService_IListLeafClusters;
    getAllDatabases: ITerminalServiceService_IGetAllDatabases;
    getDatabases: ITerminalServiceService_IGetDatabases;
    listDatabaseUsers: ITerminalServiceService_IListDatabaseUsers;
    getAllServers: ITerminalServiceService_IGetAllServers;
    getServers: ITerminalServiceService_IGetServers;
    getAccessRequests: ITerminalServiceService_IGetAccessRequests;
    getAccessRequest: ITerminalServiceService_IGetAccessRequest;
    deleteAccessRequest: ITerminalServiceService_IDeleteAccessRequest;
    createAccessRequest: ITerminalServiceService_ICreateAccessRequest;
    reviewAccessRequest: ITerminalServiceService_IReviewAccessRequest;
    getRequestableRoles: ITerminalServiceService_IGetRequestableRoles;
    assumeRole: ITerminalServiceService_IAssumeRole;
    getAllKubes: ITerminalServiceService_IGetAllKubes;
    getKubes: ITerminalServiceService_IGetKubes;
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
    transferFile: ITerminalServiceService_ITransferFile;
}

interface ITerminalServiceService_IUpdateTshdEventsServerAddress extends grpc.MethodDefinition<v1_service_pb.UpdateTshdEventsServerAddressRequest, v1_service_pb.UpdateTshdEventsServerAddressResponse> {
    path: "/teleport.terminal.v1.TerminalService/UpdateTshdEventsServerAddress";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.UpdateTshdEventsServerAddressRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.UpdateTshdEventsServerAddressRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.UpdateTshdEventsServerAddressResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.UpdateTshdEventsServerAddressResponse>;
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
interface ITerminalServiceService_IGetAllDatabases extends grpc.MethodDefinition<v1_service_pb.GetAllDatabasesRequest, v1_service_pb.GetAllDatabasesResponse> {
    path: "/teleport.terminal.v1.TerminalService/GetAllDatabases";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.GetAllDatabasesRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.GetAllDatabasesRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.GetAllDatabasesResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.GetAllDatabasesResponse>;
}
interface ITerminalServiceService_IGetDatabases extends grpc.MethodDefinition<v1_service_pb.GetDatabasesRequest, v1_service_pb.GetDatabasesResponse> {
    path: "/teleport.terminal.v1.TerminalService/GetDatabases";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.GetDatabasesRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.GetDatabasesRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.GetDatabasesResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.GetDatabasesResponse>;
}
interface ITerminalServiceService_IListDatabaseUsers extends grpc.MethodDefinition<v1_service_pb.ListDatabaseUsersRequest, v1_service_pb.ListDatabaseUsersResponse> {
    path: "/teleport.terminal.v1.TerminalService/ListDatabaseUsers";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.ListDatabaseUsersRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.ListDatabaseUsersRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.ListDatabaseUsersResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.ListDatabaseUsersResponse>;
}
interface ITerminalServiceService_IGetAllServers extends grpc.MethodDefinition<v1_service_pb.GetAllServersRequest, v1_service_pb.GetAllServersResponse> {
    path: "/teleport.terminal.v1.TerminalService/GetAllServers";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.GetAllServersRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.GetAllServersRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.GetAllServersResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.GetAllServersResponse>;
}
interface ITerminalServiceService_IGetServers extends grpc.MethodDefinition<v1_service_pb.GetServersRequest, v1_service_pb.GetServersResponse> {
    path: "/teleport.terminal.v1.TerminalService/GetServers";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.GetServersRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.GetServersRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.GetServersResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.GetServersResponse>;
}
interface ITerminalServiceService_IGetAccessRequests extends grpc.MethodDefinition<v1_service_pb.GetAccessRequestsRequest, v1_service_pb.GetAccessRequestsResponse> {
    path: "/teleport.terminal.v1.TerminalService/GetAccessRequests";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.GetAccessRequestsRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.GetAccessRequestsRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.GetAccessRequestsResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.GetAccessRequestsResponse>;
}
interface ITerminalServiceService_IGetAccessRequest extends grpc.MethodDefinition<v1_service_pb.GetAccessRequestRequest, v1_service_pb.GetAccessRequestResponse> {
    path: "/teleport.terminal.v1.TerminalService/GetAccessRequest";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.GetAccessRequestRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.GetAccessRequestRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.GetAccessRequestResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.GetAccessRequestResponse>;
}
interface ITerminalServiceService_IDeleteAccessRequest extends grpc.MethodDefinition<v1_service_pb.DeleteAccessRequestRequest, v1_service_pb.EmptyResponse> {
    path: "/teleport.terminal.v1.TerminalService/DeleteAccessRequest";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.DeleteAccessRequestRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.DeleteAccessRequestRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_ICreateAccessRequest extends grpc.MethodDefinition<v1_service_pb.CreateAccessRequestRequest, v1_service_pb.CreateAccessRequestResponse> {
    path: "/teleport.terminal.v1.TerminalService/CreateAccessRequest";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.CreateAccessRequestRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.CreateAccessRequestRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.CreateAccessRequestResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.CreateAccessRequestResponse>;
}
interface ITerminalServiceService_IReviewAccessRequest extends grpc.MethodDefinition<v1_service_pb.ReviewAccessRequestRequest, v1_service_pb.ReviewAccessRequestResponse> {
    path: "/teleport.terminal.v1.TerminalService/ReviewAccessRequest";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.ReviewAccessRequestRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.ReviewAccessRequestRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.ReviewAccessRequestResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.ReviewAccessRequestResponse>;
}
interface ITerminalServiceService_IGetRequestableRoles extends grpc.MethodDefinition<v1_service_pb.GetRequestableRolesRequest, v1_service_pb.GetRequestableRolesResponse> {
    path: "/teleport.terminal.v1.TerminalService/GetRequestableRoles";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.GetRequestableRolesRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.GetRequestableRolesRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.GetRequestableRolesResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.GetRequestableRolesResponse>;
}
interface ITerminalServiceService_IAssumeRole extends grpc.MethodDefinition<v1_service_pb.AssumeRoleRequest, v1_service_pb.EmptyResponse> {
    path: "/teleport.terminal.v1.TerminalService/AssumeRole";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.AssumeRoleRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.AssumeRoleRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_IGetAllKubes extends grpc.MethodDefinition<v1_service_pb.GetAllKubesRequest, v1_service_pb.GetAllKubesResponse> {
    path: "/teleport.terminal.v1.TerminalService/GetAllKubes";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.GetAllKubesRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.GetAllKubesRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.GetAllKubesResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.GetAllKubesResponse>;
}
interface ITerminalServiceService_IGetKubes extends grpc.MethodDefinition<v1_service_pb.GetKubesRequest, v1_service_pb.GetKubesResponse> {
    path: "/teleport.terminal.v1.TerminalService/GetKubes";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.GetKubesRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.GetKubesRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.GetKubesResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.GetKubesResponse>;
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
interface ITerminalServiceService_IListGateways extends grpc.MethodDefinition<v1_service_pb.ListGatewaysRequest, v1_service_pb.ListGatewaysResponse> {
    path: "/teleport.terminal.v1.TerminalService/ListGateways";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.ListGatewaysRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.ListGatewaysRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.ListGatewaysResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.ListGatewaysResponse>;
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
interface ITerminalServiceService_IRemoveGateway extends grpc.MethodDefinition<v1_service_pb.RemoveGatewayRequest, v1_service_pb.EmptyResponse> {
    path: "/teleport.terminal.v1.TerminalService/RemoveGateway";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.RemoveGatewayRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.RemoveGatewayRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_IRestartGateway extends grpc.MethodDefinition<v1_service_pb.RestartGatewayRequest, v1_service_pb.EmptyResponse> {
    path: "/teleport.terminal.v1.TerminalService/RestartGateway";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.RestartGatewayRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.RestartGatewayRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_ISetGatewayTargetSubresourceName extends grpc.MethodDefinition<v1_service_pb.SetGatewayTargetSubresourceNameRequest, v1_gateway_pb.Gateway> {
    path: "/teleport.terminal.v1.TerminalService/SetGatewayTargetSubresourceName";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.SetGatewayTargetSubresourceNameRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.SetGatewayTargetSubresourceNameRequest>;
    responseSerialize: grpc.serialize<v1_gateway_pb.Gateway>;
    responseDeserialize: grpc.deserialize<v1_gateway_pb.Gateway>;
}
interface ITerminalServiceService_ISetGatewayLocalPort extends grpc.MethodDefinition<v1_service_pb.SetGatewayLocalPortRequest, v1_gateway_pb.Gateway> {
    path: "/teleport.terminal.v1.TerminalService/SetGatewayLocalPort";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<v1_service_pb.SetGatewayLocalPortRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.SetGatewayLocalPortRequest>;
    responseSerialize: grpc.serialize<v1_gateway_pb.Gateway>;
    responseDeserialize: grpc.deserialize<v1_gateway_pb.Gateway>;
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
interface ITerminalServiceService_ILoginPasswordless extends grpc.MethodDefinition<v1_service_pb.LoginPasswordlessRequest, v1_service_pb.LoginPasswordlessResponse> {
    path: "/teleport.terminal.v1.TerminalService/LoginPasswordless";
    requestStream: true;
    responseStream: true;
    requestSerialize: grpc.serialize<v1_service_pb.LoginPasswordlessRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.LoginPasswordlessRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.LoginPasswordlessResponse>;
    responseDeserialize: grpc.deserialize<v1_service_pb.LoginPasswordlessResponse>;
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
interface ITerminalServiceService_ITransferFile extends grpc.MethodDefinition<v1_service_pb.FileTransferRequest, v1_service_pb.FileTransferProgress> {
    path: "/teleport.terminal.v1.TerminalService/TransferFile";
    requestStream: false;
    responseStream: true;
    requestSerialize: grpc.serialize<v1_service_pb.FileTransferRequest>;
    requestDeserialize: grpc.deserialize<v1_service_pb.FileTransferRequest>;
    responseSerialize: grpc.serialize<v1_service_pb.FileTransferProgress>;
    responseDeserialize: grpc.deserialize<v1_service_pb.FileTransferProgress>;
}

export const TerminalServiceService: ITerminalServiceService;

export interface ITerminalServiceServer {
    updateTshdEventsServerAddress: grpc.handleUnaryCall<v1_service_pb.UpdateTshdEventsServerAddressRequest, v1_service_pb.UpdateTshdEventsServerAddressResponse>;
    listRootClusters: grpc.handleUnaryCall<v1_service_pb.ListClustersRequest, v1_service_pb.ListClustersResponse>;
    listLeafClusters: grpc.handleUnaryCall<v1_service_pb.ListLeafClustersRequest, v1_service_pb.ListClustersResponse>;
    getAllDatabases: grpc.handleUnaryCall<v1_service_pb.GetAllDatabasesRequest, v1_service_pb.GetAllDatabasesResponse>;
    getDatabases: grpc.handleUnaryCall<v1_service_pb.GetDatabasesRequest, v1_service_pb.GetDatabasesResponse>;
    listDatabaseUsers: grpc.handleUnaryCall<v1_service_pb.ListDatabaseUsersRequest, v1_service_pb.ListDatabaseUsersResponse>;
    getAllServers: grpc.handleUnaryCall<v1_service_pb.GetAllServersRequest, v1_service_pb.GetAllServersResponse>;
    getServers: grpc.handleUnaryCall<v1_service_pb.GetServersRequest, v1_service_pb.GetServersResponse>;
    getAccessRequests: grpc.handleUnaryCall<v1_service_pb.GetAccessRequestsRequest, v1_service_pb.GetAccessRequestsResponse>;
    getAccessRequest: grpc.handleUnaryCall<v1_service_pb.GetAccessRequestRequest, v1_service_pb.GetAccessRequestResponse>;
    deleteAccessRequest: grpc.handleUnaryCall<v1_service_pb.DeleteAccessRequestRequest, v1_service_pb.EmptyResponse>;
    createAccessRequest: grpc.handleUnaryCall<v1_service_pb.CreateAccessRequestRequest, v1_service_pb.CreateAccessRequestResponse>;
    reviewAccessRequest: grpc.handleUnaryCall<v1_service_pb.ReviewAccessRequestRequest, v1_service_pb.ReviewAccessRequestResponse>;
    getRequestableRoles: grpc.handleUnaryCall<v1_service_pb.GetRequestableRolesRequest, v1_service_pb.GetRequestableRolesResponse>;
    assumeRole: grpc.handleUnaryCall<v1_service_pb.AssumeRoleRequest, v1_service_pb.EmptyResponse>;
    getAllKubes: grpc.handleUnaryCall<v1_service_pb.GetAllKubesRequest, v1_service_pb.GetAllKubesResponse>;
    getKubes: grpc.handleUnaryCall<v1_service_pb.GetKubesRequest, v1_service_pb.GetKubesResponse>;
    listApps: grpc.handleUnaryCall<v1_service_pb.ListAppsRequest, v1_service_pb.ListAppsResponse>;
    addCluster: grpc.handleUnaryCall<v1_service_pb.AddClusterRequest, v1_cluster_pb.Cluster>;
    removeCluster: grpc.handleUnaryCall<v1_service_pb.RemoveClusterRequest, v1_service_pb.EmptyResponse>;
    listGateways: grpc.handleUnaryCall<v1_service_pb.ListGatewaysRequest, v1_service_pb.ListGatewaysResponse>;
    createGateway: grpc.handleUnaryCall<v1_service_pb.CreateGatewayRequest, v1_gateway_pb.Gateway>;
    removeGateway: grpc.handleUnaryCall<v1_service_pb.RemoveGatewayRequest, v1_service_pb.EmptyResponse>;
    restartGateway: grpc.handleUnaryCall<v1_service_pb.RestartGatewayRequest, v1_service_pb.EmptyResponse>;
    setGatewayTargetSubresourceName: grpc.handleUnaryCall<v1_service_pb.SetGatewayTargetSubresourceNameRequest, v1_gateway_pb.Gateway>;
    setGatewayLocalPort: grpc.handleUnaryCall<v1_service_pb.SetGatewayLocalPortRequest, v1_gateway_pb.Gateway>;
    getAuthSettings: grpc.handleUnaryCall<v1_service_pb.GetAuthSettingsRequest, v1_auth_settings_pb.AuthSettings>;
    getCluster: grpc.handleUnaryCall<v1_service_pb.GetClusterRequest, v1_cluster_pb.Cluster>;
    login: grpc.handleUnaryCall<v1_service_pb.LoginRequest, v1_service_pb.EmptyResponse>;
    loginPasswordless: grpc.handleBidiStreamingCall<v1_service_pb.LoginPasswordlessRequest, v1_service_pb.LoginPasswordlessResponse>;
    logout: grpc.handleUnaryCall<v1_service_pb.LogoutRequest, v1_service_pb.EmptyResponse>;
    transferFile: grpc.handleServerStreamingCall<v1_service_pb.FileTransferRequest, v1_service_pb.FileTransferProgress>;
}

export interface ITerminalServiceClient {
    updateTshdEventsServerAddress(request: v1_service_pb.UpdateTshdEventsServerAddressRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    updateTshdEventsServerAddress(request: v1_service_pb.UpdateTshdEventsServerAddressRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    updateTshdEventsServerAddress(request: v1_service_pb.UpdateTshdEventsServerAddressRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    listRootClusters(request: v1_service_pb.ListClustersRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listRootClusters(request: v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listRootClusters(request: v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listLeafClusters(request: v1_service_pb.ListLeafClustersRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listLeafClusters(request: v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listLeafClusters(request: v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    getAllDatabases(request: v1_service_pb.GetAllDatabasesRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllDatabasesResponse) => void): grpc.ClientUnaryCall;
    getAllDatabases(request: v1_service_pb.GetAllDatabasesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllDatabasesResponse) => void): grpc.ClientUnaryCall;
    getAllDatabases(request: v1_service_pb.GetAllDatabasesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllDatabasesResponse) => void): grpc.ClientUnaryCall;
    getDatabases(request: v1_service_pb.GetDatabasesRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    getDatabases(request: v1_service_pb.GetDatabasesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    getDatabases(request: v1_service_pb.GetDatabasesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    listDatabaseUsers(request: v1_service_pb.ListDatabaseUsersRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    listDatabaseUsers(request: v1_service_pb.ListDatabaseUsersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    listDatabaseUsers(request: v1_service_pb.ListDatabaseUsersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    getAllServers(request: v1_service_pb.GetAllServersRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllServersResponse) => void): grpc.ClientUnaryCall;
    getAllServers(request: v1_service_pb.GetAllServersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllServersResponse) => void): grpc.ClientUnaryCall;
    getAllServers(request: v1_service_pb.GetAllServersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllServersResponse) => void): grpc.ClientUnaryCall;
    getServers(request: v1_service_pb.GetServersRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    getServers(request: v1_service_pb.GetServersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    getServers(request: v1_service_pb.GetServersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    getAccessRequests(request: v1_service_pb.GetAccessRequestsRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    getAccessRequests(request: v1_service_pb.GetAccessRequestsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    getAccessRequests(request: v1_service_pb.GetAccessRequestsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    getAccessRequest(request: v1_service_pb.GetAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    getAccessRequest(request: v1_service_pb.GetAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    getAccessRequest(request: v1_service_pb.GetAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    deleteAccessRequest(request: v1_service_pb.DeleteAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    deleteAccessRequest(request: v1_service_pb.DeleteAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    deleteAccessRequest(request: v1_service_pb.DeleteAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    createAccessRequest(request: v1_service_pb.CreateAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    createAccessRequest(request: v1_service_pb.CreateAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    createAccessRequest(request: v1_service_pb.CreateAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    reviewAccessRequest(request: v1_service_pb.ReviewAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    reviewAccessRequest(request: v1_service_pb.ReviewAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    reviewAccessRequest(request: v1_service_pb.ReviewAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    getRequestableRoles(request: v1_service_pb.GetRequestableRolesRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    getRequestableRoles(request: v1_service_pb.GetRequestableRolesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    getRequestableRoles(request: v1_service_pb.GetRequestableRolesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    assumeRole(request: v1_service_pb.AssumeRoleRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    assumeRole(request: v1_service_pb.AssumeRoleRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    assumeRole(request: v1_service_pb.AssumeRoleRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    getAllKubes(request: v1_service_pb.GetAllKubesRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllKubesResponse) => void): grpc.ClientUnaryCall;
    getAllKubes(request: v1_service_pb.GetAllKubesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllKubesResponse) => void): grpc.ClientUnaryCall;
    getAllKubes(request: v1_service_pb.GetAllKubesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllKubesResponse) => void): grpc.ClientUnaryCall;
    getKubes(request: v1_service_pb.GetKubesRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    getKubes(request: v1_service_pb.GetKubesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    getKubes(request: v1_service_pb.GetKubesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    listApps(request: v1_service_pb.ListAppsRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    listApps(request: v1_service_pb.ListAppsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    listApps(request: v1_service_pb.ListAppsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    addCluster(request: v1_service_pb.AddClusterRequest, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    addCluster(request: v1_service_pb.AddClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    addCluster(request: v1_service_pb.AddClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    removeCluster(request: v1_service_pb.RemoveClusterRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    removeCluster(request: v1_service_pb.RemoveClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    removeCluster(request: v1_service_pb.RemoveClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    listGateways(request: v1_service_pb.ListGatewaysRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    listGateways(request: v1_service_pb.ListGatewaysRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    listGateways(request: v1_service_pb.ListGatewaysRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    createGateway(request: v1_service_pb.CreateGatewayRequest, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    createGateway(request: v1_service_pb.CreateGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    createGateway(request: v1_service_pb.CreateGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    removeGateway(request: v1_service_pb.RemoveGatewayRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    removeGateway(request: v1_service_pb.RemoveGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    removeGateway(request: v1_service_pb.RemoveGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    restartGateway(request: v1_service_pb.RestartGatewayRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    restartGateway(request: v1_service_pb.RestartGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    restartGateway(request: v1_service_pb.RestartGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    setGatewayTargetSubresourceName(request: v1_service_pb.SetGatewayTargetSubresourceNameRequest, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    setGatewayTargetSubresourceName(request: v1_service_pb.SetGatewayTargetSubresourceNameRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    setGatewayTargetSubresourceName(request: v1_service_pb.SetGatewayTargetSubresourceNameRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    setGatewayLocalPort(request: v1_service_pb.SetGatewayLocalPortRequest, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    setGatewayLocalPort(request: v1_service_pb.SetGatewayLocalPortRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    setGatewayLocalPort(request: v1_service_pb.SetGatewayLocalPortRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    getAuthSettings(request: v1_service_pb.GetAuthSettingsRequest, callback: (error: grpc.ServiceError | null, response: v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    getAuthSettings(request: v1_service_pb.GetAuthSettingsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    getAuthSettings(request: v1_service_pb.GetAuthSettingsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    getCluster(request: v1_service_pb.GetClusterRequest, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    getCluster(request: v1_service_pb.GetClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    getCluster(request: v1_service_pb.GetClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    login(request: v1_service_pb.LoginRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    login(request: v1_service_pb.LoginRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    login(request: v1_service_pb.LoginRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    loginPasswordless(): grpc.ClientDuplexStream<v1_service_pb.LoginPasswordlessRequest, v1_service_pb.LoginPasswordlessResponse>;
    loginPasswordless(options: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<v1_service_pb.LoginPasswordlessRequest, v1_service_pb.LoginPasswordlessResponse>;
    loginPasswordless(metadata: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<v1_service_pb.LoginPasswordlessRequest, v1_service_pb.LoginPasswordlessResponse>;
    logout(request: v1_service_pb.LogoutRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    logout(request: v1_service_pb.LogoutRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    logout(request: v1_service_pb.LogoutRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    transferFile(request: v1_service_pb.FileTransferRequest, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<v1_service_pb.FileTransferProgress>;
    transferFile(request: v1_service_pb.FileTransferRequest, metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<v1_service_pb.FileTransferProgress>;
}

export class TerminalServiceClient extends grpc.Client implements ITerminalServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public updateTshdEventsServerAddress(request: v1_service_pb.UpdateTshdEventsServerAddressRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    public updateTshdEventsServerAddress(request: v1_service_pb.UpdateTshdEventsServerAddressRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    public updateTshdEventsServerAddress(request: v1_service_pb.UpdateTshdEventsServerAddressRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    public listRootClusters(request: v1_service_pb.ListClustersRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listRootClusters(request: v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listRootClusters(request: v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listLeafClusters(request: v1_service_pb.ListLeafClustersRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listLeafClusters(request: v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listLeafClusters(request: v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public getAllDatabases(request: v1_service_pb.GetAllDatabasesRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllDatabasesResponse) => void): grpc.ClientUnaryCall;
    public getAllDatabases(request: v1_service_pb.GetAllDatabasesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllDatabasesResponse) => void): grpc.ClientUnaryCall;
    public getAllDatabases(request: v1_service_pb.GetAllDatabasesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllDatabasesResponse) => void): grpc.ClientUnaryCall;
    public getDatabases(request: v1_service_pb.GetDatabasesRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    public getDatabases(request: v1_service_pb.GetDatabasesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    public getDatabases(request: v1_service_pb.GetDatabasesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    public listDatabaseUsers(request: v1_service_pb.ListDatabaseUsersRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    public listDatabaseUsers(request: v1_service_pb.ListDatabaseUsersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    public listDatabaseUsers(request: v1_service_pb.ListDatabaseUsersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    public getAllServers(request: v1_service_pb.GetAllServersRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllServersResponse) => void): grpc.ClientUnaryCall;
    public getAllServers(request: v1_service_pb.GetAllServersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllServersResponse) => void): grpc.ClientUnaryCall;
    public getAllServers(request: v1_service_pb.GetAllServersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllServersResponse) => void): grpc.ClientUnaryCall;
    public getServers(request: v1_service_pb.GetServersRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    public getServers(request: v1_service_pb.GetServersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    public getServers(request: v1_service_pb.GetServersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequests(request: v1_service_pb.GetAccessRequestsRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequests(request: v1_service_pb.GetAccessRequestsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequests(request: v1_service_pb.GetAccessRequestsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequest(request: v1_service_pb.GetAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequest(request: v1_service_pb.GetAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequest(request: v1_service_pb.GetAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public deleteAccessRequest(request: v1_service_pb.DeleteAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public deleteAccessRequest(request: v1_service_pb.DeleteAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public deleteAccessRequest(request: v1_service_pb.DeleteAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public createAccessRequest(request: v1_service_pb.CreateAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public createAccessRequest(request: v1_service_pb.CreateAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public createAccessRequest(request: v1_service_pb.CreateAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public reviewAccessRequest(request: v1_service_pb.ReviewAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public reviewAccessRequest(request: v1_service_pb.ReviewAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public reviewAccessRequest(request: v1_service_pb.ReviewAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public getRequestableRoles(request: v1_service_pb.GetRequestableRolesRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    public getRequestableRoles(request: v1_service_pb.GetRequestableRolesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    public getRequestableRoles(request: v1_service_pb.GetRequestableRolesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    public assumeRole(request: v1_service_pb.AssumeRoleRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public assumeRole(request: v1_service_pb.AssumeRoleRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public assumeRole(request: v1_service_pb.AssumeRoleRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public getAllKubes(request: v1_service_pb.GetAllKubesRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllKubesResponse) => void): grpc.ClientUnaryCall;
    public getAllKubes(request: v1_service_pb.GetAllKubesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllKubesResponse) => void): grpc.ClientUnaryCall;
    public getAllKubes(request: v1_service_pb.GetAllKubesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetAllKubesResponse) => void): grpc.ClientUnaryCall;
    public getKubes(request: v1_service_pb.GetKubesRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    public getKubes(request: v1_service_pb.GetKubesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    public getKubes(request: v1_service_pb.GetKubesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    public listApps(request: v1_service_pb.ListAppsRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    public listApps(request: v1_service_pb.ListAppsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    public listApps(request: v1_service_pb.ListAppsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    public addCluster(request: v1_service_pb.AddClusterRequest, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public addCluster(request: v1_service_pb.AddClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public addCluster(request: v1_service_pb.AddClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public removeCluster(request: v1_service_pb.RemoveClusterRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public removeCluster(request: v1_service_pb.RemoveClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public removeCluster(request: v1_service_pb.RemoveClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public listGateways(request: v1_service_pb.ListGatewaysRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    public listGateways(request: v1_service_pb.ListGatewaysRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    public listGateways(request: v1_service_pb.ListGatewaysRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    public createGateway(request: v1_service_pb.CreateGatewayRequest, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public createGateway(request: v1_service_pb.CreateGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public createGateway(request: v1_service_pb.CreateGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public removeGateway(request: v1_service_pb.RemoveGatewayRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public removeGateway(request: v1_service_pb.RemoveGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public removeGateway(request: v1_service_pb.RemoveGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public restartGateway(request: v1_service_pb.RestartGatewayRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public restartGateway(request: v1_service_pb.RestartGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public restartGateway(request: v1_service_pb.RestartGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public setGatewayTargetSubresourceName(request: v1_service_pb.SetGatewayTargetSubresourceNameRequest, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public setGatewayTargetSubresourceName(request: v1_service_pb.SetGatewayTargetSubresourceNameRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public setGatewayTargetSubresourceName(request: v1_service_pb.SetGatewayTargetSubresourceNameRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public setGatewayLocalPort(request: v1_service_pb.SetGatewayLocalPortRequest, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public setGatewayLocalPort(request: v1_service_pb.SetGatewayLocalPortRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public setGatewayLocalPort(request: v1_service_pb.SetGatewayLocalPortRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public getAuthSettings(request: v1_service_pb.GetAuthSettingsRequest, callback: (error: grpc.ServiceError | null, response: v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    public getAuthSettings(request: v1_service_pb.GetAuthSettingsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    public getAuthSettings(request: v1_service_pb.GetAuthSettingsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    public getCluster(request: v1_service_pb.GetClusterRequest, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public getCluster(request: v1_service_pb.GetClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public getCluster(request: v1_service_pb.GetClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public login(request: v1_service_pb.LoginRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public login(request: v1_service_pb.LoginRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public login(request: v1_service_pb.LoginRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public loginPasswordless(options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<v1_service_pb.LoginPasswordlessRequest, v1_service_pb.LoginPasswordlessResponse>;
    public loginPasswordless(metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<v1_service_pb.LoginPasswordlessRequest, v1_service_pb.LoginPasswordlessResponse>;
    public logout(request: v1_service_pb.LogoutRequest, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public logout(request: v1_service_pb.LogoutRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public logout(request: v1_service_pb.LogoutRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public transferFile(request: v1_service_pb.FileTransferRequest, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<v1_service_pb.FileTransferProgress>;
    public transferFile(request: v1_service_pb.FileTransferRequest, metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<v1_service_pb.FileTransferProgress>;
}
