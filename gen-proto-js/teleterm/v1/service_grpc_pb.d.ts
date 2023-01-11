// package: teleterm.v1
// file: teleterm/v1/service.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as teleterm_v1_service_pb from "../../teleterm/v1/service_pb";
import * as teleterm_v1_access_request_pb from "../../teleterm/v1/access_request_pb";
import * as teleterm_v1_app_pb from "../../teleterm/v1/app_pb";
import * as teleterm_v1_auth_settings_pb from "../../teleterm/v1/auth_settings_pb";
import * as teleterm_v1_cluster_pb from "../../teleterm/v1/cluster_pb";
import * as teleterm_v1_database_pb from "../../teleterm/v1/database_pb";
import * as teleterm_v1_gateway_pb from "../../teleterm/v1/gateway_pb";
import * as teleterm_v1_kube_pb from "../../teleterm/v1/kube_pb";
import * as teleterm_v1_server_pb from "../../teleterm/v1/server_pb";
import * as teleterm_v1_usage_events_pb from "../../teleterm/v1/usage_events_pb";

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
    reportUsageEvent: ITerminalServiceService_IReportUsageEvent;
}

interface ITerminalServiceService_IUpdateTshdEventsServerAddress extends grpc.MethodDefinition<teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest, teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse> {
    path: "/teleterm.v1.TerminalService/UpdateTshdEventsServerAddress";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse>;
}
interface ITerminalServiceService_IListRootClusters extends grpc.MethodDefinition<teleterm_v1_service_pb.ListClustersRequest, teleterm_v1_service_pb.ListClustersResponse> {
    path: "/teleterm.v1.TerminalService/ListRootClusters";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.ListClustersRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.ListClustersRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.ListClustersResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.ListClustersResponse>;
}
interface ITerminalServiceService_IListLeafClusters extends grpc.MethodDefinition<teleterm_v1_service_pb.ListLeafClustersRequest, teleterm_v1_service_pb.ListClustersResponse> {
    path: "/teleterm.v1.TerminalService/ListLeafClusters";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.ListLeafClustersRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.ListLeafClustersRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.ListClustersResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.ListClustersResponse>;
}
interface ITerminalServiceService_IGetAllDatabases extends grpc.MethodDefinition<teleterm_v1_service_pb.GetAllDatabasesRequest, teleterm_v1_service_pb.GetAllDatabasesResponse> {
    path: "/teleterm.v1.TerminalService/GetAllDatabases";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.GetAllDatabasesRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetAllDatabasesRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.GetAllDatabasesResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetAllDatabasesResponse>;
}
interface ITerminalServiceService_IGetDatabases extends grpc.MethodDefinition<teleterm_v1_service_pb.GetDatabasesRequest, teleterm_v1_service_pb.GetDatabasesResponse> {
    path: "/teleterm.v1.TerminalService/GetDatabases";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.GetDatabasesRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetDatabasesRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.GetDatabasesResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetDatabasesResponse>;
}
interface ITerminalServiceService_IListDatabaseUsers extends grpc.MethodDefinition<teleterm_v1_service_pb.ListDatabaseUsersRequest, teleterm_v1_service_pb.ListDatabaseUsersResponse> {
    path: "/teleterm.v1.TerminalService/ListDatabaseUsers";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.ListDatabaseUsersRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.ListDatabaseUsersRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.ListDatabaseUsersResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.ListDatabaseUsersResponse>;
}
interface ITerminalServiceService_IGetAllServers extends grpc.MethodDefinition<teleterm_v1_service_pb.GetAllServersRequest, teleterm_v1_service_pb.GetAllServersResponse> {
    path: "/teleterm.v1.TerminalService/GetAllServers";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.GetAllServersRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetAllServersRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.GetAllServersResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetAllServersResponse>;
}
interface ITerminalServiceService_IGetServers extends grpc.MethodDefinition<teleterm_v1_service_pb.GetServersRequest, teleterm_v1_service_pb.GetServersResponse> {
    path: "/teleterm.v1.TerminalService/GetServers";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.GetServersRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetServersRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.GetServersResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetServersResponse>;
}
interface ITerminalServiceService_IGetAccessRequests extends grpc.MethodDefinition<teleterm_v1_service_pb.GetAccessRequestsRequest, teleterm_v1_service_pb.GetAccessRequestsResponse> {
    path: "/teleterm.v1.TerminalService/GetAccessRequests";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.GetAccessRequestsRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetAccessRequestsRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.GetAccessRequestsResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetAccessRequestsResponse>;
}
interface ITerminalServiceService_IGetAccessRequest extends grpc.MethodDefinition<teleterm_v1_service_pb.GetAccessRequestRequest, teleterm_v1_service_pb.GetAccessRequestResponse> {
    path: "/teleterm.v1.TerminalService/GetAccessRequest";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.GetAccessRequestRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetAccessRequestRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.GetAccessRequestResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetAccessRequestResponse>;
}
interface ITerminalServiceService_IDeleteAccessRequest extends grpc.MethodDefinition<teleterm_v1_service_pb.DeleteAccessRequestRequest, teleterm_v1_service_pb.EmptyResponse> {
    path: "/teleterm.v1.TerminalService/DeleteAccessRequest";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.DeleteAccessRequestRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.DeleteAccessRequestRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_ICreateAccessRequest extends grpc.MethodDefinition<teleterm_v1_service_pb.CreateAccessRequestRequest, teleterm_v1_service_pb.CreateAccessRequestResponse> {
    path: "/teleterm.v1.TerminalService/CreateAccessRequest";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.CreateAccessRequestRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.CreateAccessRequestRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.CreateAccessRequestResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.CreateAccessRequestResponse>;
}
interface ITerminalServiceService_IReviewAccessRequest extends grpc.MethodDefinition<teleterm_v1_service_pb.ReviewAccessRequestRequest, teleterm_v1_service_pb.ReviewAccessRequestResponse> {
    path: "/teleterm.v1.TerminalService/ReviewAccessRequest";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.ReviewAccessRequestRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.ReviewAccessRequestRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.ReviewAccessRequestResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.ReviewAccessRequestResponse>;
}
interface ITerminalServiceService_IGetRequestableRoles extends grpc.MethodDefinition<teleterm_v1_service_pb.GetRequestableRolesRequest, teleterm_v1_service_pb.GetRequestableRolesResponse> {
    path: "/teleterm.v1.TerminalService/GetRequestableRoles";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.GetRequestableRolesRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetRequestableRolesRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.GetRequestableRolesResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetRequestableRolesResponse>;
}
interface ITerminalServiceService_IAssumeRole extends grpc.MethodDefinition<teleterm_v1_service_pb.AssumeRoleRequest, teleterm_v1_service_pb.EmptyResponse> {
    path: "/teleterm.v1.TerminalService/AssumeRole";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.AssumeRoleRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.AssumeRoleRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_IGetAllKubes extends grpc.MethodDefinition<teleterm_v1_service_pb.GetAllKubesRequest, teleterm_v1_service_pb.GetAllKubesResponse> {
    path: "/teleterm.v1.TerminalService/GetAllKubes";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.GetAllKubesRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetAllKubesRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.GetAllKubesResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetAllKubesResponse>;
}
interface ITerminalServiceService_IGetKubes extends grpc.MethodDefinition<teleterm_v1_service_pb.GetKubesRequest, teleterm_v1_service_pb.GetKubesResponse> {
    path: "/teleterm.v1.TerminalService/GetKubes";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.GetKubesRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetKubesRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.GetKubesResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetKubesResponse>;
}
interface ITerminalServiceService_IListApps extends grpc.MethodDefinition<teleterm_v1_service_pb.ListAppsRequest, teleterm_v1_service_pb.ListAppsResponse> {
    path: "/teleterm.v1.TerminalService/ListApps";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.ListAppsRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.ListAppsRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.ListAppsResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.ListAppsResponse>;
}
interface ITerminalServiceService_IAddCluster extends grpc.MethodDefinition<teleterm_v1_service_pb.AddClusterRequest, teleterm_v1_cluster_pb.Cluster> {
    path: "/teleterm.v1.TerminalService/AddCluster";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.AddClusterRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.AddClusterRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_cluster_pb.Cluster>;
    responseDeserialize: grpc.deserialize<teleterm_v1_cluster_pb.Cluster>;
}
interface ITerminalServiceService_IRemoveCluster extends grpc.MethodDefinition<teleterm_v1_service_pb.RemoveClusterRequest, teleterm_v1_service_pb.EmptyResponse> {
    path: "/teleterm.v1.TerminalService/RemoveCluster";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.RemoveClusterRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.RemoveClusterRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_IListGateways extends grpc.MethodDefinition<teleterm_v1_service_pb.ListGatewaysRequest, teleterm_v1_service_pb.ListGatewaysResponse> {
    path: "/teleterm.v1.TerminalService/ListGateways";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.ListGatewaysRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.ListGatewaysRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.ListGatewaysResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.ListGatewaysResponse>;
}
interface ITerminalServiceService_ICreateGateway extends grpc.MethodDefinition<teleterm_v1_service_pb.CreateGatewayRequest, teleterm_v1_gateway_pb.Gateway> {
    path: "/teleterm.v1.TerminalService/CreateGateway";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.CreateGatewayRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.CreateGatewayRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_gateway_pb.Gateway>;
    responseDeserialize: grpc.deserialize<teleterm_v1_gateway_pb.Gateway>;
}
interface ITerminalServiceService_IRemoveGateway extends grpc.MethodDefinition<teleterm_v1_service_pb.RemoveGatewayRequest, teleterm_v1_service_pb.EmptyResponse> {
    path: "/teleterm.v1.TerminalService/RemoveGateway";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.RemoveGatewayRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.RemoveGatewayRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_IRestartGateway extends grpc.MethodDefinition<teleterm_v1_service_pb.RestartGatewayRequest, teleterm_v1_service_pb.EmptyResponse> {
    path: "/teleterm.v1.TerminalService/RestartGateway";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.RestartGatewayRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.RestartGatewayRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_ISetGatewayTargetSubresourceName extends grpc.MethodDefinition<teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest, teleterm_v1_gateway_pb.Gateway> {
    path: "/teleterm.v1.TerminalService/SetGatewayTargetSubresourceName";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_gateway_pb.Gateway>;
    responseDeserialize: grpc.deserialize<teleterm_v1_gateway_pb.Gateway>;
}
interface ITerminalServiceService_ISetGatewayLocalPort extends grpc.MethodDefinition<teleterm_v1_service_pb.SetGatewayLocalPortRequest, teleterm_v1_gateway_pb.Gateway> {
    path: "/teleterm.v1.TerminalService/SetGatewayLocalPort";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.SetGatewayLocalPortRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.SetGatewayLocalPortRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_gateway_pb.Gateway>;
    responseDeserialize: grpc.deserialize<teleterm_v1_gateway_pb.Gateway>;
}
interface ITerminalServiceService_IGetAuthSettings extends grpc.MethodDefinition<teleterm_v1_service_pb.GetAuthSettingsRequest, teleterm_v1_auth_settings_pb.AuthSettings> {
    path: "/teleterm.v1.TerminalService/GetAuthSettings";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.GetAuthSettingsRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetAuthSettingsRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_auth_settings_pb.AuthSettings>;
    responseDeserialize: grpc.deserialize<teleterm_v1_auth_settings_pb.AuthSettings>;
}
interface ITerminalServiceService_IGetCluster extends grpc.MethodDefinition<teleterm_v1_service_pb.GetClusterRequest, teleterm_v1_cluster_pb.Cluster> {
    path: "/teleterm.v1.TerminalService/GetCluster";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.GetClusterRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.GetClusterRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_cluster_pb.Cluster>;
    responseDeserialize: grpc.deserialize<teleterm_v1_cluster_pb.Cluster>;
}
interface ITerminalServiceService_ILogin extends grpc.MethodDefinition<teleterm_v1_service_pb.LoginRequest, teleterm_v1_service_pb.EmptyResponse> {
    path: "/teleterm.v1.TerminalService/Login";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.LoginRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.LoginRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_ILoginPasswordless extends grpc.MethodDefinition<teleterm_v1_service_pb.LoginPasswordlessRequest, teleterm_v1_service_pb.LoginPasswordlessResponse> {
    path: "/teleterm.v1.TerminalService/LoginPasswordless";
    requestStream: true;
    responseStream: true;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.LoginPasswordlessRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.LoginPasswordlessRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.LoginPasswordlessResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.LoginPasswordlessResponse>;
}
interface ITerminalServiceService_ILogout extends grpc.MethodDefinition<teleterm_v1_service_pb.LogoutRequest, teleterm_v1_service_pb.EmptyResponse> {
    path: "/teleterm.v1.TerminalService/Logout";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.LogoutRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.LogoutRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_ITransferFile extends grpc.MethodDefinition<teleterm_v1_service_pb.FileTransferRequest, teleterm_v1_service_pb.FileTransferProgress> {
    path: "/teleterm.v1.TerminalService/TransferFile";
    requestStream: false;
    responseStream: true;
    requestSerialize: grpc.serialize<teleterm_v1_service_pb.FileTransferRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_service_pb.FileTransferRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.FileTransferProgress>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.FileTransferProgress>;
}
interface ITerminalServiceService_IReportUsageEvent extends grpc.MethodDefinition<teleterm_v1_usage_events_pb.ReportUsageEventRequest, teleterm_v1_service_pb.EmptyResponse> {
    path: "/teleterm.v1.TerminalService/ReportUsageEvent";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleterm_v1_usage_events_pb.ReportUsageEventRequest>;
    requestDeserialize: grpc.deserialize<teleterm_v1_usage_events_pb.ReportUsageEventRequest>;
    responseSerialize: grpc.serialize<teleterm_v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<teleterm_v1_service_pb.EmptyResponse>;
}

export const TerminalServiceService: ITerminalServiceService;

export interface ITerminalServiceServer {
    updateTshdEventsServerAddress: grpc.handleUnaryCall<teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest, teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse>;
    listRootClusters: grpc.handleUnaryCall<teleterm_v1_service_pb.ListClustersRequest, teleterm_v1_service_pb.ListClustersResponse>;
    listLeafClusters: grpc.handleUnaryCall<teleterm_v1_service_pb.ListLeafClustersRequest, teleterm_v1_service_pb.ListClustersResponse>;
    getAllDatabases: grpc.handleUnaryCall<teleterm_v1_service_pb.GetAllDatabasesRequest, teleterm_v1_service_pb.GetAllDatabasesResponse>;
    getDatabases: grpc.handleUnaryCall<teleterm_v1_service_pb.GetDatabasesRequest, teleterm_v1_service_pb.GetDatabasesResponse>;
    listDatabaseUsers: grpc.handleUnaryCall<teleterm_v1_service_pb.ListDatabaseUsersRequest, teleterm_v1_service_pb.ListDatabaseUsersResponse>;
    getAllServers: grpc.handleUnaryCall<teleterm_v1_service_pb.GetAllServersRequest, teleterm_v1_service_pb.GetAllServersResponse>;
    getServers: grpc.handleUnaryCall<teleterm_v1_service_pb.GetServersRequest, teleterm_v1_service_pb.GetServersResponse>;
    getAccessRequests: grpc.handleUnaryCall<teleterm_v1_service_pb.GetAccessRequestsRequest, teleterm_v1_service_pb.GetAccessRequestsResponse>;
    getAccessRequest: grpc.handleUnaryCall<teleterm_v1_service_pb.GetAccessRequestRequest, teleterm_v1_service_pb.GetAccessRequestResponse>;
    deleteAccessRequest: grpc.handleUnaryCall<teleterm_v1_service_pb.DeleteAccessRequestRequest, teleterm_v1_service_pb.EmptyResponse>;
    createAccessRequest: grpc.handleUnaryCall<teleterm_v1_service_pb.CreateAccessRequestRequest, teleterm_v1_service_pb.CreateAccessRequestResponse>;
    reviewAccessRequest: grpc.handleUnaryCall<teleterm_v1_service_pb.ReviewAccessRequestRequest, teleterm_v1_service_pb.ReviewAccessRequestResponse>;
    getRequestableRoles: grpc.handleUnaryCall<teleterm_v1_service_pb.GetRequestableRolesRequest, teleterm_v1_service_pb.GetRequestableRolesResponse>;
    assumeRole: grpc.handleUnaryCall<teleterm_v1_service_pb.AssumeRoleRequest, teleterm_v1_service_pb.EmptyResponse>;
    getAllKubes: grpc.handleUnaryCall<teleterm_v1_service_pb.GetAllKubesRequest, teleterm_v1_service_pb.GetAllKubesResponse>;
    getKubes: grpc.handleUnaryCall<teleterm_v1_service_pb.GetKubesRequest, teleterm_v1_service_pb.GetKubesResponse>;
    listApps: grpc.handleUnaryCall<teleterm_v1_service_pb.ListAppsRequest, teleterm_v1_service_pb.ListAppsResponse>;
    addCluster: grpc.handleUnaryCall<teleterm_v1_service_pb.AddClusterRequest, teleterm_v1_cluster_pb.Cluster>;
    removeCluster: grpc.handleUnaryCall<teleterm_v1_service_pb.RemoveClusterRequest, teleterm_v1_service_pb.EmptyResponse>;
    listGateways: grpc.handleUnaryCall<teleterm_v1_service_pb.ListGatewaysRequest, teleterm_v1_service_pb.ListGatewaysResponse>;
    createGateway: grpc.handleUnaryCall<teleterm_v1_service_pb.CreateGatewayRequest, teleterm_v1_gateway_pb.Gateway>;
    removeGateway: grpc.handleUnaryCall<teleterm_v1_service_pb.RemoveGatewayRequest, teleterm_v1_service_pb.EmptyResponse>;
    restartGateway: grpc.handleUnaryCall<teleterm_v1_service_pb.RestartGatewayRequest, teleterm_v1_service_pb.EmptyResponse>;
    setGatewayTargetSubresourceName: grpc.handleUnaryCall<teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest, teleterm_v1_gateway_pb.Gateway>;
    setGatewayLocalPort: grpc.handleUnaryCall<teleterm_v1_service_pb.SetGatewayLocalPortRequest, teleterm_v1_gateway_pb.Gateway>;
    getAuthSettings: grpc.handleUnaryCall<teleterm_v1_service_pb.GetAuthSettingsRequest, teleterm_v1_auth_settings_pb.AuthSettings>;
    getCluster: grpc.handleUnaryCall<teleterm_v1_service_pb.GetClusterRequest, teleterm_v1_cluster_pb.Cluster>;
    login: grpc.handleUnaryCall<teleterm_v1_service_pb.LoginRequest, teleterm_v1_service_pb.EmptyResponse>;
    loginPasswordless: grpc.handleBidiStreamingCall<teleterm_v1_service_pb.LoginPasswordlessRequest, teleterm_v1_service_pb.LoginPasswordlessResponse>;
    logout: grpc.handleUnaryCall<teleterm_v1_service_pb.LogoutRequest, teleterm_v1_service_pb.EmptyResponse>;
    transferFile: grpc.handleServerStreamingCall<teleterm_v1_service_pb.FileTransferRequest, teleterm_v1_service_pb.FileTransferProgress>;
    reportUsageEvent: grpc.handleUnaryCall<teleterm_v1_usage_events_pb.ReportUsageEventRequest, teleterm_v1_service_pb.EmptyResponse>;
}

export interface ITerminalServiceClient {
    updateTshdEventsServerAddress(request: teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    updateTshdEventsServerAddress(request: teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    updateTshdEventsServerAddress(request: teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    listRootClusters(request: teleterm_v1_service_pb.ListClustersRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listRootClusters(request: teleterm_v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listRootClusters(request: teleterm_v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listLeafClusters(request: teleterm_v1_service_pb.ListLeafClustersRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listLeafClusters(request: teleterm_v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listLeafClusters(request: teleterm_v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    getAllDatabases(request: teleterm_v1_service_pb.GetAllDatabasesRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllDatabasesResponse) => void): grpc.ClientUnaryCall;
    getAllDatabases(request: teleterm_v1_service_pb.GetAllDatabasesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllDatabasesResponse) => void): grpc.ClientUnaryCall;
    getAllDatabases(request: teleterm_v1_service_pb.GetAllDatabasesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllDatabasesResponse) => void): grpc.ClientUnaryCall;
    getDatabases(request: teleterm_v1_service_pb.GetDatabasesRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    getDatabases(request: teleterm_v1_service_pb.GetDatabasesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    getDatabases(request: teleterm_v1_service_pb.GetDatabasesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    listDatabaseUsers(request: teleterm_v1_service_pb.ListDatabaseUsersRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    listDatabaseUsers(request: teleterm_v1_service_pb.ListDatabaseUsersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    listDatabaseUsers(request: teleterm_v1_service_pb.ListDatabaseUsersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    getAllServers(request: teleterm_v1_service_pb.GetAllServersRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllServersResponse) => void): grpc.ClientUnaryCall;
    getAllServers(request: teleterm_v1_service_pb.GetAllServersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllServersResponse) => void): grpc.ClientUnaryCall;
    getAllServers(request: teleterm_v1_service_pb.GetAllServersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllServersResponse) => void): grpc.ClientUnaryCall;
    getServers(request: teleterm_v1_service_pb.GetServersRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    getServers(request: teleterm_v1_service_pb.GetServersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    getServers(request: teleterm_v1_service_pb.GetServersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    getAccessRequests(request: teleterm_v1_service_pb.GetAccessRequestsRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    getAccessRequests(request: teleterm_v1_service_pb.GetAccessRequestsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    getAccessRequests(request: teleterm_v1_service_pb.GetAccessRequestsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    getAccessRequest(request: teleterm_v1_service_pb.GetAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    getAccessRequest(request: teleterm_v1_service_pb.GetAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    getAccessRequest(request: teleterm_v1_service_pb.GetAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    deleteAccessRequest(request: teleterm_v1_service_pb.DeleteAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    deleteAccessRequest(request: teleterm_v1_service_pb.DeleteAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    deleteAccessRequest(request: teleterm_v1_service_pb.DeleteAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    createAccessRequest(request: teleterm_v1_service_pb.CreateAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    createAccessRequest(request: teleterm_v1_service_pb.CreateAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    createAccessRequest(request: teleterm_v1_service_pb.CreateAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    reviewAccessRequest(request: teleterm_v1_service_pb.ReviewAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    reviewAccessRequest(request: teleterm_v1_service_pb.ReviewAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    reviewAccessRequest(request: teleterm_v1_service_pb.ReviewAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    getRequestableRoles(request: teleterm_v1_service_pb.GetRequestableRolesRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    getRequestableRoles(request: teleterm_v1_service_pb.GetRequestableRolesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    getRequestableRoles(request: teleterm_v1_service_pb.GetRequestableRolesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    assumeRole(request: teleterm_v1_service_pb.AssumeRoleRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    assumeRole(request: teleterm_v1_service_pb.AssumeRoleRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    assumeRole(request: teleterm_v1_service_pb.AssumeRoleRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    getAllKubes(request: teleterm_v1_service_pb.GetAllKubesRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllKubesResponse) => void): grpc.ClientUnaryCall;
    getAllKubes(request: teleterm_v1_service_pb.GetAllKubesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllKubesResponse) => void): grpc.ClientUnaryCall;
    getAllKubes(request: teleterm_v1_service_pb.GetAllKubesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllKubesResponse) => void): grpc.ClientUnaryCall;
    getKubes(request: teleterm_v1_service_pb.GetKubesRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    getKubes(request: teleterm_v1_service_pb.GetKubesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    getKubes(request: teleterm_v1_service_pb.GetKubesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    listApps(request: teleterm_v1_service_pb.ListAppsRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    listApps(request: teleterm_v1_service_pb.ListAppsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    listApps(request: teleterm_v1_service_pb.ListAppsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    addCluster(request: teleterm_v1_service_pb.AddClusterRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    addCluster(request: teleterm_v1_service_pb.AddClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    addCluster(request: teleterm_v1_service_pb.AddClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    removeCluster(request: teleterm_v1_service_pb.RemoveClusterRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    removeCluster(request: teleterm_v1_service_pb.RemoveClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    removeCluster(request: teleterm_v1_service_pb.RemoveClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    listGateways(request: teleterm_v1_service_pb.ListGatewaysRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    listGateways(request: teleterm_v1_service_pb.ListGatewaysRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    listGateways(request: teleterm_v1_service_pb.ListGatewaysRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    createGateway(request: teleterm_v1_service_pb.CreateGatewayRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    createGateway(request: teleterm_v1_service_pb.CreateGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    createGateway(request: teleterm_v1_service_pb.CreateGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    removeGateway(request: teleterm_v1_service_pb.RemoveGatewayRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    removeGateway(request: teleterm_v1_service_pb.RemoveGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    removeGateway(request: teleterm_v1_service_pb.RemoveGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    restartGateway(request: teleterm_v1_service_pb.RestartGatewayRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    restartGateway(request: teleterm_v1_service_pb.RestartGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    restartGateway(request: teleterm_v1_service_pb.RestartGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    setGatewayTargetSubresourceName(request: teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    setGatewayTargetSubresourceName(request: teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    setGatewayTargetSubresourceName(request: teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    setGatewayLocalPort(request: teleterm_v1_service_pb.SetGatewayLocalPortRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    setGatewayLocalPort(request: teleterm_v1_service_pb.SetGatewayLocalPortRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    setGatewayLocalPort(request: teleterm_v1_service_pb.SetGatewayLocalPortRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    getAuthSettings(request: teleterm_v1_service_pb.GetAuthSettingsRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    getAuthSettings(request: teleterm_v1_service_pb.GetAuthSettingsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    getAuthSettings(request: teleterm_v1_service_pb.GetAuthSettingsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    getCluster(request: teleterm_v1_service_pb.GetClusterRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    getCluster(request: teleterm_v1_service_pb.GetClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    getCluster(request: teleterm_v1_service_pb.GetClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    login(request: teleterm_v1_service_pb.LoginRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    login(request: teleterm_v1_service_pb.LoginRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    login(request: teleterm_v1_service_pb.LoginRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    loginPasswordless(): grpc.ClientDuplexStream<teleterm_v1_service_pb.LoginPasswordlessRequest, teleterm_v1_service_pb.LoginPasswordlessResponse>;
    loginPasswordless(options: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleterm_v1_service_pb.LoginPasswordlessRequest, teleterm_v1_service_pb.LoginPasswordlessResponse>;
    loginPasswordless(metadata: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleterm_v1_service_pb.LoginPasswordlessRequest, teleterm_v1_service_pb.LoginPasswordlessResponse>;
    logout(request: teleterm_v1_service_pb.LogoutRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    logout(request: teleterm_v1_service_pb.LogoutRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    logout(request: teleterm_v1_service_pb.LogoutRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    transferFile(request: teleterm_v1_service_pb.FileTransferRequest, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<teleterm_v1_service_pb.FileTransferProgress>;
    transferFile(request: teleterm_v1_service_pb.FileTransferRequest, metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<teleterm_v1_service_pb.FileTransferProgress>;
    reportUsageEvent(request: teleterm_v1_usage_events_pb.ReportUsageEventRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    reportUsageEvent(request: teleterm_v1_usage_events_pb.ReportUsageEventRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    reportUsageEvent(request: teleterm_v1_usage_events_pb.ReportUsageEventRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
}

export class TerminalServiceClient extends grpc.Client implements ITerminalServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public updateTshdEventsServerAddress(request: teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    public updateTshdEventsServerAddress(request: teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    public updateTshdEventsServerAddress(request: teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    public listRootClusters(request: teleterm_v1_service_pb.ListClustersRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listRootClusters(request: teleterm_v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listRootClusters(request: teleterm_v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listLeafClusters(request: teleterm_v1_service_pb.ListLeafClustersRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listLeafClusters(request: teleterm_v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listLeafClusters(request: teleterm_v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public getAllDatabases(request: teleterm_v1_service_pb.GetAllDatabasesRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllDatabasesResponse) => void): grpc.ClientUnaryCall;
    public getAllDatabases(request: teleterm_v1_service_pb.GetAllDatabasesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllDatabasesResponse) => void): grpc.ClientUnaryCall;
    public getAllDatabases(request: teleterm_v1_service_pb.GetAllDatabasesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllDatabasesResponse) => void): grpc.ClientUnaryCall;
    public getDatabases(request: teleterm_v1_service_pb.GetDatabasesRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    public getDatabases(request: teleterm_v1_service_pb.GetDatabasesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    public getDatabases(request: teleterm_v1_service_pb.GetDatabasesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    public listDatabaseUsers(request: teleterm_v1_service_pb.ListDatabaseUsersRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    public listDatabaseUsers(request: teleterm_v1_service_pb.ListDatabaseUsersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    public listDatabaseUsers(request: teleterm_v1_service_pb.ListDatabaseUsersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    public getAllServers(request: teleterm_v1_service_pb.GetAllServersRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllServersResponse) => void): grpc.ClientUnaryCall;
    public getAllServers(request: teleterm_v1_service_pb.GetAllServersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllServersResponse) => void): grpc.ClientUnaryCall;
    public getAllServers(request: teleterm_v1_service_pb.GetAllServersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllServersResponse) => void): grpc.ClientUnaryCall;
    public getServers(request: teleterm_v1_service_pb.GetServersRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    public getServers(request: teleterm_v1_service_pb.GetServersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    public getServers(request: teleterm_v1_service_pb.GetServersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequests(request: teleterm_v1_service_pb.GetAccessRequestsRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequests(request: teleterm_v1_service_pb.GetAccessRequestsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequests(request: teleterm_v1_service_pb.GetAccessRequestsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequest(request: teleterm_v1_service_pb.GetAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequest(request: teleterm_v1_service_pb.GetAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequest(request: teleterm_v1_service_pb.GetAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public deleteAccessRequest(request: teleterm_v1_service_pb.DeleteAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public deleteAccessRequest(request: teleterm_v1_service_pb.DeleteAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public deleteAccessRequest(request: teleterm_v1_service_pb.DeleteAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public createAccessRequest(request: teleterm_v1_service_pb.CreateAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public createAccessRequest(request: teleterm_v1_service_pb.CreateAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public createAccessRequest(request: teleterm_v1_service_pb.CreateAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public reviewAccessRequest(request: teleterm_v1_service_pb.ReviewAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public reviewAccessRequest(request: teleterm_v1_service_pb.ReviewAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public reviewAccessRequest(request: teleterm_v1_service_pb.ReviewAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public getRequestableRoles(request: teleterm_v1_service_pb.GetRequestableRolesRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    public getRequestableRoles(request: teleterm_v1_service_pb.GetRequestableRolesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    public getRequestableRoles(request: teleterm_v1_service_pb.GetRequestableRolesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    public assumeRole(request: teleterm_v1_service_pb.AssumeRoleRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public assumeRole(request: teleterm_v1_service_pb.AssumeRoleRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public assumeRole(request: teleterm_v1_service_pb.AssumeRoleRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public getAllKubes(request: teleterm_v1_service_pb.GetAllKubesRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllKubesResponse) => void): grpc.ClientUnaryCall;
    public getAllKubes(request: teleterm_v1_service_pb.GetAllKubesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllKubesResponse) => void): grpc.ClientUnaryCall;
    public getAllKubes(request: teleterm_v1_service_pb.GetAllKubesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetAllKubesResponse) => void): grpc.ClientUnaryCall;
    public getKubes(request: teleterm_v1_service_pb.GetKubesRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    public getKubes(request: teleterm_v1_service_pb.GetKubesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    public getKubes(request: teleterm_v1_service_pb.GetKubesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    public listApps(request: teleterm_v1_service_pb.ListAppsRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    public listApps(request: teleterm_v1_service_pb.ListAppsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    public listApps(request: teleterm_v1_service_pb.ListAppsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListAppsResponse) => void): grpc.ClientUnaryCall;
    public addCluster(request: teleterm_v1_service_pb.AddClusterRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public addCluster(request: teleterm_v1_service_pb.AddClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public addCluster(request: teleterm_v1_service_pb.AddClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public removeCluster(request: teleterm_v1_service_pb.RemoveClusterRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public removeCluster(request: teleterm_v1_service_pb.RemoveClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public removeCluster(request: teleterm_v1_service_pb.RemoveClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public listGateways(request: teleterm_v1_service_pb.ListGatewaysRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    public listGateways(request: teleterm_v1_service_pb.ListGatewaysRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    public listGateways(request: teleterm_v1_service_pb.ListGatewaysRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.ListGatewaysResponse) => void): grpc.ClientUnaryCall;
    public createGateway(request: teleterm_v1_service_pb.CreateGatewayRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public createGateway(request: teleterm_v1_service_pb.CreateGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public createGateway(request: teleterm_v1_service_pb.CreateGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public removeGateway(request: teleterm_v1_service_pb.RemoveGatewayRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public removeGateway(request: teleterm_v1_service_pb.RemoveGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public removeGateway(request: teleterm_v1_service_pb.RemoveGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public restartGateway(request: teleterm_v1_service_pb.RestartGatewayRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public restartGateway(request: teleterm_v1_service_pb.RestartGatewayRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public restartGateway(request: teleterm_v1_service_pb.RestartGatewayRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public setGatewayTargetSubresourceName(request: teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public setGatewayTargetSubresourceName(request: teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public setGatewayTargetSubresourceName(request: teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public setGatewayLocalPort(request: teleterm_v1_service_pb.SetGatewayLocalPortRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public setGatewayLocalPort(request: teleterm_v1_service_pb.SetGatewayLocalPortRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public setGatewayLocalPort(request: teleterm_v1_service_pb.SetGatewayLocalPortRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_gateway_pb.Gateway) => void): grpc.ClientUnaryCall;
    public getAuthSettings(request: teleterm_v1_service_pb.GetAuthSettingsRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    public getAuthSettings(request: teleterm_v1_service_pb.GetAuthSettingsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    public getAuthSettings(request: teleterm_v1_service_pb.GetAuthSettingsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_auth_settings_pb.AuthSettings) => void): grpc.ClientUnaryCall;
    public getCluster(request: teleterm_v1_service_pb.GetClusterRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public getCluster(request: teleterm_v1_service_pb.GetClusterRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public getCluster(request: teleterm_v1_service_pb.GetClusterRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_cluster_pb.Cluster) => void): grpc.ClientUnaryCall;
    public login(request: teleterm_v1_service_pb.LoginRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public login(request: teleterm_v1_service_pb.LoginRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public login(request: teleterm_v1_service_pb.LoginRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public loginPasswordless(options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleterm_v1_service_pb.LoginPasswordlessRequest, teleterm_v1_service_pb.LoginPasswordlessResponse>;
    public loginPasswordless(metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientDuplexStream<teleterm_v1_service_pb.LoginPasswordlessRequest, teleterm_v1_service_pb.LoginPasswordlessResponse>;
    public logout(request: teleterm_v1_service_pb.LogoutRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public logout(request: teleterm_v1_service_pb.LogoutRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public logout(request: teleterm_v1_service_pb.LogoutRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public transferFile(request: teleterm_v1_service_pb.FileTransferRequest, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<teleterm_v1_service_pb.FileTransferProgress>;
    public transferFile(request: teleterm_v1_service_pb.FileTransferRequest, metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<teleterm_v1_service_pb.FileTransferProgress>;
    public reportUsageEvent(request: teleterm_v1_usage_events_pb.ReportUsageEventRequest, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public reportUsageEvent(request: teleterm_v1_usage_events_pb.ReportUsageEventRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public reportUsageEvent(request: teleterm_v1_usage_events_pb.ReportUsageEventRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
}
