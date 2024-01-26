// package: teleport.lib.teleterm.v1
// file: teleport/lib/teleterm/v1/service.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "grpc";
import * as teleport_lib_teleterm_v1_service_pb from "../../../../teleport/lib/teleterm/v1/service_pb";
import * as teleport_accesslist_v1_accesslist_pb from "../../../../teleport/accesslist/v1/accesslist_pb";
import * as teleport_lib_teleterm_v1_access_request_pb from "../../../../teleport/lib/teleterm/v1/access_request_pb";
import * as teleport_lib_teleterm_v1_app_pb from "../../../../teleport/lib/teleterm/v1/app_pb";
import * as teleport_lib_teleterm_v1_auth_settings_pb from "../../../../teleport/lib/teleterm/v1/auth_settings_pb";
import * as teleport_lib_teleterm_v1_cluster_pb from "../../../../teleport/lib/teleterm/v1/cluster_pb";
import * as teleport_lib_teleterm_v1_database_pb from "../../../../teleport/lib/teleterm/v1/database_pb";
import * as teleport_lib_teleterm_v1_gateway_pb from "../../../../teleport/lib/teleterm/v1/gateway_pb";
import * as teleport_lib_teleterm_v1_kube_pb from "../../../../teleport/lib/teleterm/v1/kube_pb";
import * as teleport_lib_teleterm_v1_server_pb from "../../../../teleport/lib/teleterm/v1/server_pb";
import * as teleport_lib_teleterm_v1_usage_events_pb from "../../../../teleport/lib/teleterm/v1/usage_events_pb";
import * as teleport_userpreferences_v1_cluster_preferences_pb from "../../../../teleport/userpreferences/v1/cluster_preferences_pb";
import * as teleport_userpreferences_v1_unified_resource_preferences_pb from "../../../../teleport/userpreferences/v1/unified_resource_preferences_pb";

interface ITerminalServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    updateTshdEventsServerAddress: ITerminalServiceService_IUpdateTshdEventsServerAddress;
    listRootClusters: ITerminalServiceService_IListRootClusters;
    listLeafClusters: ITerminalServiceService_IListLeafClusters;
    getDatabases: ITerminalServiceService_IGetDatabases;
    listDatabaseUsers: ITerminalServiceService_IListDatabaseUsers;
    getServers: ITerminalServiceService_IGetServers;
    getAccessRequests: ITerminalServiceService_IGetAccessRequests;
    getAccessRequest: ITerminalServiceService_IGetAccessRequest;
    deleteAccessRequest: ITerminalServiceService_IDeleteAccessRequest;
    createAccessRequest: ITerminalServiceService_ICreateAccessRequest;
    reviewAccessRequest: ITerminalServiceService_IReviewAccessRequest;
    getRequestableRoles: ITerminalServiceService_IGetRequestableRoles;
    assumeRole: ITerminalServiceService_IAssumeRole;
    promoteAccessRequest: ITerminalServiceService_IPromoteAccessRequest;
    getSuggestedAccessLists: ITerminalServiceService_IGetSuggestedAccessLists;
    getKubes: ITerminalServiceService_IGetKubes;
    getApps: ITerminalServiceService_IGetApps;
    addCluster: ITerminalServiceService_IAddCluster;
    removeCluster: ITerminalServiceService_IRemoveCluster;
    listGateways: ITerminalServiceService_IListGateways;
    createGateway: ITerminalServiceService_ICreateGateway;
    removeGateway: ITerminalServiceService_IRemoveGateway;
    setGatewayTargetSubresourceName: ITerminalServiceService_ISetGatewayTargetSubresourceName;
    setGatewayLocalPort: ITerminalServiceService_ISetGatewayLocalPort;
    getAuthSettings: ITerminalServiceService_IGetAuthSettings;
    getCluster: ITerminalServiceService_IGetCluster;
    login: ITerminalServiceService_ILogin;
    loginPasswordless: ITerminalServiceService_ILoginPasswordless;
    logout: ITerminalServiceService_ILogout;
    transferFile: ITerminalServiceService_ITransferFile;
    reportUsageEvent: ITerminalServiceService_IReportUsageEvent;
    updateHeadlessAuthenticationState: ITerminalServiceService_IUpdateHeadlessAuthenticationState;
    createConnectMyComputerRole: ITerminalServiceService_ICreateConnectMyComputerRole;
    createConnectMyComputerNodeToken: ITerminalServiceService_ICreateConnectMyComputerNodeToken;
    deleteConnectMyComputerToken: ITerminalServiceService_IDeleteConnectMyComputerToken;
    waitForConnectMyComputerNodeJoin: ITerminalServiceService_IWaitForConnectMyComputerNodeJoin;
    deleteConnectMyComputerNode: ITerminalServiceService_IDeleteConnectMyComputerNode;
    getConnectMyComputerNodeName: ITerminalServiceService_IGetConnectMyComputerNodeName;
    listUnifiedResources: ITerminalServiceService_IListUnifiedResources;
    getUserPreferences: ITerminalServiceService_IGetUserPreferences;
    updateUserPreferences: ITerminalServiceService_IUpdateUserPreferences;
}

interface ITerminalServiceService_IUpdateTshdEventsServerAddress extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest, teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/UpdateTshdEventsServerAddress";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse>;
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
interface ITerminalServiceService_IGetDatabases extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.GetDatabasesRequest, teleport_lib_teleterm_v1_service_pb.GetDatabasesResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/GetDatabases";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetDatabasesRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetDatabasesRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetDatabasesResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetDatabasesResponse>;
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
interface ITerminalServiceService_IGetServers extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.GetServersRequest, teleport_lib_teleterm_v1_service_pb.GetServersResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/GetServers";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetServersRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetServersRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetServersResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetServersResponse>;
}
interface ITerminalServiceService_IGetAccessRequests extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.GetAccessRequestsRequest, teleport_lib_teleterm_v1_service_pb.GetAccessRequestsResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/GetAccessRequests";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetAccessRequestsRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetAccessRequestsRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetAccessRequestsResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetAccessRequestsResponse>;
}
interface ITerminalServiceService_IGetAccessRequest extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.GetAccessRequestRequest, teleport_lib_teleterm_v1_service_pb.GetAccessRequestResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/GetAccessRequest";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetAccessRequestRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetAccessRequestRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetAccessRequestResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetAccessRequestResponse>;
}
interface ITerminalServiceService_IDeleteAccessRequest extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.DeleteAccessRequestRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/DeleteAccessRequest";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.DeleteAccessRequestRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.DeleteAccessRequestRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_ICreateAccessRequest extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.CreateAccessRequestRequest, teleport_lib_teleterm_v1_service_pb.CreateAccessRequestResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/CreateAccessRequest";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.CreateAccessRequestRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.CreateAccessRequestRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.CreateAccessRequestResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.CreateAccessRequestResponse>;
}
interface ITerminalServiceService_IReviewAccessRequest extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestRequest, teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/ReviewAccessRequest";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestResponse>;
}
interface ITerminalServiceService_IGetRequestableRoles extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.GetRequestableRolesRequest, teleport_lib_teleterm_v1_service_pb.GetRequestableRolesResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/GetRequestableRoles";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetRequestableRolesRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetRequestableRolesRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetRequestableRolesResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetRequestableRolesResponse>;
}
interface ITerminalServiceService_IAssumeRole extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.AssumeRoleRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/AssumeRole";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.AssumeRoleRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.AssumeRoleRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_IPromoteAccessRequest extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestRequest, teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/PromoteAccessRequest";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestResponse>;
}
interface ITerminalServiceService_IGetSuggestedAccessLists extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsRequest, teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/GetSuggestedAccessLists";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsResponse>;
}
interface ITerminalServiceService_IGetKubes extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.GetKubesRequest, teleport_lib_teleterm_v1_service_pb.GetKubesResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/GetKubes";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetKubesRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetKubesRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetKubesResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetKubesResponse>;
}
interface ITerminalServiceService_IGetApps extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.GetAppsRequest, teleport_lib_teleterm_v1_service_pb.GetAppsResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/GetApps";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetAppsRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetAppsRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetAppsResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetAppsResponse>;
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
interface ITerminalServiceService_ITransferFile extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.FileTransferRequest, teleport_lib_teleterm_v1_service_pb.FileTransferProgress> {
    path: "/teleport.lib.teleterm.v1.TerminalService/TransferFile";
    requestStream: false;
    responseStream: true;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.FileTransferRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.FileTransferRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.FileTransferProgress>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.FileTransferProgress>;
}
interface ITerminalServiceService_IReportUsageEvent extends grpc.MethodDefinition<teleport_lib_teleterm_v1_usage_events_pb.ReportUsageEventRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/ReportUsageEvent";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_usage_events_pb.ReportUsageEventRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_usage_events_pb.ReportUsageEventRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
}
interface ITerminalServiceService_IUpdateHeadlessAuthenticationState extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateRequest, teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/UpdateHeadlessAuthenticationState";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateResponse>;
}
interface ITerminalServiceService_ICreateConnectMyComputerRole extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleRequest, teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/CreateConnectMyComputerRole";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleResponse>;
}
interface ITerminalServiceService_ICreateConnectMyComputerNodeToken extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenRequest, teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/CreateConnectMyComputerNodeToken";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenResponse>;
}
interface ITerminalServiceService_IDeleteConnectMyComputerToken extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenRequest, teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/DeleteConnectMyComputerToken";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenResponse>;
}
interface ITerminalServiceService_IWaitForConnectMyComputerNodeJoin extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinRequest, teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/WaitForConnectMyComputerNodeJoin";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinResponse>;
}
interface ITerminalServiceService_IDeleteConnectMyComputerNode extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeRequest, teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/DeleteConnectMyComputerNode";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeResponse>;
}
interface ITerminalServiceService_IGetConnectMyComputerNodeName extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameRequest, teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/GetConnectMyComputerNodeName";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameResponse>;
}
interface ITerminalServiceService_IListUnifiedResources extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesRequest, teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/ListUnifiedResources";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesResponse>;
}
interface ITerminalServiceService_IGetUserPreferences extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.GetUserPreferencesRequest, teleport_lib_teleterm_v1_service_pb.GetUserPreferencesResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/GetUserPreferences";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetUserPreferencesRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetUserPreferencesRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.GetUserPreferencesResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.GetUserPreferencesResponse>;
}
interface ITerminalServiceService_IUpdateUserPreferences extends grpc.MethodDefinition<teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesRequest, teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesResponse> {
    path: "/teleport.lib.teleterm.v1.TerminalService/UpdateUserPreferences";
    requestStream: false;
    responseStream: false;
    requestSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesRequest>;
    requestDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesRequest>;
    responseSerialize: grpc.serialize<teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesResponse>;
    responseDeserialize: grpc.deserialize<teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesResponse>;
}

export const TerminalServiceService: ITerminalServiceService;

export interface ITerminalServiceServer {
    updateTshdEventsServerAddress: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest, teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse>;
    listRootClusters: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.ListClustersRequest, teleport_lib_teleterm_v1_service_pb.ListClustersResponse>;
    listLeafClusters: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest, teleport_lib_teleterm_v1_service_pb.ListClustersResponse>;
    getDatabases: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.GetDatabasesRequest, teleport_lib_teleterm_v1_service_pb.GetDatabasesResponse>;
    listDatabaseUsers: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest, teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse>;
    getServers: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.GetServersRequest, teleport_lib_teleterm_v1_service_pb.GetServersResponse>;
    getAccessRequests: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.GetAccessRequestsRequest, teleport_lib_teleterm_v1_service_pb.GetAccessRequestsResponse>;
    getAccessRequest: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.GetAccessRequestRequest, teleport_lib_teleterm_v1_service_pb.GetAccessRequestResponse>;
    deleteAccessRequest: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.DeleteAccessRequestRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    createAccessRequest: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.CreateAccessRequestRequest, teleport_lib_teleterm_v1_service_pb.CreateAccessRequestResponse>;
    reviewAccessRequest: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestRequest, teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestResponse>;
    getRequestableRoles: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.GetRequestableRolesRequest, teleport_lib_teleterm_v1_service_pb.GetRequestableRolesResponse>;
    assumeRole: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.AssumeRoleRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    promoteAccessRequest: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestRequest, teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestResponse>;
    getSuggestedAccessLists: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsRequest, teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsResponse>;
    getKubes: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.GetKubesRequest, teleport_lib_teleterm_v1_service_pb.GetKubesResponse>;
    getApps: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.GetAppsRequest, teleport_lib_teleterm_v1_service_pb.GetAppsResponse>;
    addCluster: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.AddClusterRequest, teleport_lib_teleterm_v1_cluster_pb.Cluster>;
    removeCluster: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.RemoveClusterRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    listGateways: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.ListGatewaysRequest, teleport_lib_teleterm_v1_service_pb.ListGatewaysResponse>;
    createGateway: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.CreateGatewayRequest, teleport_lib_teleterm_v1_gateway_pb.Gateway>;
    removeGateway: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.RemoveGatewayRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    setGatewayTargetSubresourceName: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.SetGatewayTargetSubresourceNameRequest, teleport_lib_teleterm_v1_gateway_pb.Gateway>;
    setGatewayLocalPort: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.SetGatewayLocalPortRequest, teleport_lib_teleterm_v1_gateway_pb.Gateway>;
    getAuthSettings: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.GetAuthSettingsRequest, teleport_lib_teleterm_v1_auth_settings_pb.AuthSettings>;
    getCluster: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.GetClusterRequest, teleport_lib_teleterm_v1_cluster_pb.Cluster>;
    login: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.LoginRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    loginPasswordless: grpc.handleBidiStreamingCall<teleport_lib_teleterm_v1_service_pb.LoginPasswordlessRequest, teleport_lib_teleterm_v1_service_pb.LoginPasswordlessResponse>;
    logout: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.LogoutRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    transferFile: grpc.handleServerStreamingCall<teleport_lib_teleterm_v1_service_pb.FileTransferRequest, teleport_lib_teleterm_v1_service_pb.FileTransferProgress>;
    reportUsageEvent: grpc.handleUnaryCall<teleport_lib_teleterm_v1_usage_events_pb.ReportUsageEventRequest, teleport_lib_teleterm_v1_service_pb.EmptyResponse>;
    updateHeadlessAuthenticationState: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateRequest, teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateResponse>;
    createConnectMyComputerRole: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleRequest, teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleResponse>;
    createConnectMyComputerNodeToken: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenRequest, teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenResponse>;
    deleteConnectMyComputerToken: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenRequest, teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenResponse>;
    waitForConnectMyComputerNodeJoin: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinRequest, teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinResponse>;
    deleteConnectMyComputerNode: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeRequest, teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeResponse>;
    getConnectMyComputerNodeName: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameRequest, teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameResponse>;
    listUnifiedResources: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesRequest, teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesResponse>;
    getUserPreferences: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.GetUserPreferencesRequest, teleport_lib_teleterm_v1_service_pb.GetUserPreferencesResponse>;
    updateUserPreferences: grpc.handleUnaryCall<teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesRequest, teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesResponse>;
}

export interface ITerminalServiceClient {
    updateTshdEventsServerAddress(request: teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    updateTshdEventsServerAddress(request: teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    updateTshdEventsServerAddress(request: teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    listRootClusters(request: teleport_lib_teleterm_v1_service_pb.ListClustersRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listRootClusters(request: teleport_lib_teleterm_v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listRootClusters(request: teleport_lib_teleterm_v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listLeafClusters(request: teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listLeafClusters(request: teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    listLeafClusters(request: teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    getDatabases(request: teleport_lib_teleterm_v1_service_pb.GetDatabasesRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    getDatabases(request: teleport_lib_teleterm_v1_service_pb.GetDatabasesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    getDatabases(request: teleport_lib_teleterm_v1_service_pb.GetDatabasesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    listDatabaseUsers(request: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    listDatabaseUsers(request: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    listDatabaseUsers(request: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    getServers(request: teleport_lib_teleterm_v1_service_pb.GetServersRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    getServers(request: teleport_lib_teleterm_v1_service_pb.GetServersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    getServers(request: teleport_lib_teleterm_v1_service_pb.GetServersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    getAccessRequests(request: teleport_lib_teleterm_v1_service_pb.GetAccessRequestsRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    getAccessRequests(request: teleport_lib_teleterm_v1_service_pb.GetAccessRequestsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    getAccessRequests(request: teleport_lib_teleterm_v1_service_pb.GetAccessRequestsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    getAccessRequest(request: teleport_lib_teleterm_v1_service_pb.GetAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    getAccessRequest(request: teleport_lib_teleterm_v1_service_pb.GetAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    getAccessRequest(request: teleport_lib_teleterm_v1_service_pb.GetAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    deleteAccessRequest(request: teleport_lib_teleterm_v1_service_pb.DeleteAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    deleteAccessRequest(request: teleport_lib_teleterm_v1_service_pb.DeleteAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    deleteAccessRequest(request: teleport_lib_teleterm_v1_service_pb.DeleteAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    createAccessRequest(request: teleport_lib_teleterm_v1_service_pb.CreateAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    createAccessRequest(request: teleport_lib_teleterm_v1_service_pb.CreateAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    createAccessRequest(request: teleport_lib_teleterm_v1_service_pb.CreateAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    reviewAccessRequest(request: teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    reviewAccessRequest(request: teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    reviewAccessRequest(request: teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    getRequestableRoles(request: teleport_lib_teleterm_v1_service_pb.GetRequestableRolesRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    getRequestableRoles(request: teleport_lib_teleterm_v1_service_pb.GetRequestableRolesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    getRequestableRoles(request: teleport_lib_teleterm_v1_service_pb.GetRequestableRolesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    assumeRole(request: teleport_lib_teleterm_v1_service_pb.AssumeRoleRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    assumeRole(request: teleport_lib_teleterm_v1_service_pb.AssumeRoleRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    assumeRole(request: teleport_lib_teleterm_v1_service_pb.AssumeRoleRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    promoteAccessRequest(request: teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestResponse) => void): grpc.ClientUnaryCall;
    promoteAccessRequest(request: teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestResponse) => void): grpc.ClientUnaryCall;
    promoteAccessRequest(request: teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestResponse) => void): grpc.ClientUnaryCall;
    getSuggestedAccessLists(request: teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsResponse) => void): grpc.ClientUnaryCall;
    getSuggestedAccessLists(request: teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsResponse) => void): grpc.ClientUnaryCall;
    getSuggestedAccessLists(request: teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsResponse) => void): grpc.ClientUnaryCall;
    getKubes(request: teleport_lib_teleterm_v1_service_pb.GetKubesRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    getKubes(request: teleport_lib_teleterm_v1_service_pb.GetKubesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    getKubes(request: teleport_lib_teleterm_v1_service_pb.GetKubesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    getApps(request: teleport_lib_teleterm_v1_service_pb.GetAppsRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAppsResponse) => void): grpc.ClientUnaryCall;
    getApps(request: teleport_lib_teleterm_v1_service_pb.GetAppsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAppsResponse) => void): grpc.ClientUnaryCall;
    getApps(request: teleport_lib_teleterm_v1_service_pb.GetAppsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAppsResponse) => void): grpc.ClientUnaryCall;
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
    transferFile(request: teleport_lib_teleterm_v1_service_pb.FileTransferRequest, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<teleport_lib_teleterm_v1_service_pb.FileTransferProgress>;
    transferFile(request: teleport_lib_teleterm_v1_service_pb.FileTransferRequest, metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<teleport_lib_teleterm_v1_service_pb.FileTransferProgress>;
    reportUsageEvent(request: teleport_lib_teleterm_v1_usage_events_pb.ReportUsageEventRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    reportUsageEvent(request: teleport_lib_teleterm_v1_usage_events_pb.ReportUsageEventRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    reportUsageEvent(request: teleport_lib_teleterm_v1_usage_events_pb.ReportUsageEventRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    updateHeadlessAuthenticationState(request: teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateResponse) => void): grpc.ClientUnaryCall;
    updateHeadlessAuthenticationState(request: teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateResponse) => void): grpc.ClientUnaryCall;
    updateHeadlessAuthenticationState(request: teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateResponse) => void): grpc.ClientUnaryCall;
    createConnectMyComputerRole(request: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleResponse) => void): grpc.ClientUnaryCall;
    createConnectMyComputerRole(request: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleResponse) => void): grpc.ClientUnaryCall;
    createConnectMyComputerRole(request: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleResponse) => void): grpc.ClientUnaryCall;
    createConnectMyComputerNodeToken(request: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenResponse) => void): grpc.ClientUnaryCall;
    createConnectMyComputerNodeToken(request: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenResponse) => void): grpc.ClientUnaryCall;
    createConnectMyComputerNodeToken(request: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenResponse) => void): grpc.ClientUnaryCall;
    deleteConnectMyComputerToken(request: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenResponse) => void): grpc.ClientUnaryCall;
    deleteConnectMyComputerToken(request: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenResponse) => void): grpc.ClientUnaryCall;
    deleteConnectMyComputerToken(request: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenResponse) => void): grpc.ClientUnaryCall;
    waitForConnectMyComputerNodeJoin(request: teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinResponse) => void): grpc.ClientUnaryCall;
    waitForConnectMyComputerNodeJoin(request: teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinResponse) => void): grpc.ClientUnaryCall;
    waitForConnectMyComputerNodeJoin(request: teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinResponse) => void): grpc.ClientUnaryCall;
    deleteConnectMyComputerNode(request: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeResponse) => void): grpc.ClientUnaryCall;
    deleteConnectMyComputerNode(request: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeResponse) => void): grpc.ClientUnaryCall;
    deleteConnectMyComputerNode(request: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeResponse) => void): grpc.ClientUnaryCall;
    getConnectMyComputerNodeName(request: teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameResponse) => void): grpc.ClientUnaryCall;
    getConnectMyComputerNodeName(request: teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameResponse) => void): grpc.ClientUnaryCall;
    getConnectMyComputerNodeName(request: teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameResponse) => void): grpc.ClientUnaryCall;
    listUnifiedResources(request: teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesResponse) => void): grpc.ClientUnaryCall;
    listUnifiedResources(request: teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesResponse) => void): grpc.ClientUnaryCall;
    listUnifiedResources(request: teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesResponse) => void): grpc.ClientUnaryCall;
    getUserPreferences(request: teleport_lib_teleterm_v1_service_pb.GetUserPreferencesRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetUserPreferencesResponse) => void): grpc.ClientUnaryCall;
    getUserPreferences(request: teleport_lib_teleterm_v1_service_pb.GetUserPreferencesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetUserPreferencesResponse) => void): grpc.ClientUnaryCall;
    getUserPreferences(request: teleport_lib_teleterm_v1_service_pb.GetUserPreferencesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetUserPreferencesResponse) => void): grpc.ClientUnaryCall;
    updateUserPreferences(request: teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesResponse) => void): grpc.ClientUnaryCall;
    updateUserPreferences(request: teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesResponse) => void): grpc.ClientUnaryCall;
    updateUserPreferences(request: teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesResponse) => void): grpc.ClientUnaryCall;
}

export class TerminalServiceClient extends grpc.Client implements ITerminalServiceClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
    public updateTshdEventsServerAddress(request: teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    public updateTshdEventsServerAddress(request: teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    public updateTshdEventsServerAddress(request: teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateTshdEventsServerAddressResponse) => void): grpc.ClientUnaryCall;
    public listRootClusters(request: teleport_lib_teleterm_v1_service_pb.ListClustersRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listRootClusters(request: teleport_lib_teleterm_v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listRootClusters(request: teleport_lib_teleterm_v1_service_pb.ListClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listLeafClusters(request: teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listLeafClusters(request: teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public listLeafClusters(request: teleport_lib_teleterm_v1_service_pb.ListLeafClustersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListClustersResponse) => void): grpc.ClientUnaryCall;
    public getDatabases(request: teleport_lib_teleterm_v1_service_pb.GetDatabasesRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    public getDatabases(request: teleport_lib_teleterm_v1_service_pb.GetDatabasesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    public getDatabases(request: teleport_lib_teleterm_v1_service_pb.GetDatabasesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetDatabasesResponse) => void): grpc.ClientUnaryCall;
    public listDatabaseUsers(request: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    public listDatabaseUsers(request: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    public listDatabaseUsers(request: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListDatabaseUsersResponse) => void): grpc.ClientUnaryCall;
    public getServers(request: teleport_lib_teleterm_v1_service_pb.GetServersRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    public getServers(request: teleport_lib_teleterm_v1_service_pb.GetServersRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    public getServers(request: teleport_lib_teleterm_v1_service_pb.GetServersRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetServersResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequests(request: teleport_lib_teleterm_v1_service_pb.GetAccessRequestsRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequests(request: teleport_lib_teleterm_v1_service_pb.GetAccessRequestsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequests(request: teleport_lib_teleterm_v1_service_pb.GetAccessRequestsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAccessRequestsResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequest(request: teleport_lib_teleterm_v1_service_pb.GetAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequest(request: teleport_lib_teleterm_v1_service_pb.GetAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public getAccessRequest(request: teleport_lib_teleterm_v1_service_pb.GetAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public deleteAccessRequest(request: teleport_lib_teleterm_v1_service_pb.DeleteAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public deleteAccessRequest(request: teleport_lib_teleterm_v1_service_pb.DeleteAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public deleteAccessRequest(request: teleport_lib_teleterm_v1_service_pb.DeleteAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public createAccessRequest(request: teleport_lib_teleterm_v1_service_pb.CreateAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public createAccessRequest(request: teleport_lib_teleterm_v1_service_pb.CreateAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public createAccessRequest(request: teleport_lib_teleterm_v1_service_pb.CreateAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public reviewAccessRequest(request: teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public reviewAccessRequest(request: teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public reviewAccessRequest(request: teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ReviewAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public getRequestableRoles(request: teleport_lib_teleterm_v1_service_pb.GetRequestableRolesRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    public getRequestableRoles(request: teleport_lib_teleterm_v1_service_pb.GetRequestableRolesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    public getRequestableRoles(request: teleport_lib_teleterm_v1_service_pb.GetRequestableRolesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetRequestableRolesResponse) => void): grpc.ClientUnaryCall;
    public assumeRole(request: teleport_lib_teleterm_v1_service_pb.AssumeRoleRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public assumeRole(request: teleport_lib_teleterm_v1_service_pb.AssumeRoleRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public assumeRole(request: teleport_lib_teleterm_v1_service_pb.AssumeRoleRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public promoteAccessRequest(request: teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public promoteAccessRequest(request: teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public promoteAccessRequest(request: teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.PromoteAccessRequestResponse) => void): grpc.ClientUnaryCall;
    public getSuggestedAccessLists(request: teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsResponse) => void): grpc.ClientUnaryCall;
    public getSuggestedAccessLists(request: teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsResponse) => void): grpc.ClientUnaryCall;
    public getSuggestedAccessLists(request: teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetSuggestedAccessListsResponse) => void): grpc.ClientUnaryCall;
    public getKubes(request: teleport_lib_teleterm_v1_service_pb.GetKubesRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    public getKubes(request: teleport_lib_teleterm_v1_service_pb.GetKubesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    public getKubes(request: teleport_lib_teleterm_v1_service_pb.GetKubesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetKubesResponse) => void): grpc.ClientUnaryCall;
    public getApps(request: teleport_lib_teleterm_v1_service_pb.GetAppsRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAppsResponse) => void): grpc.ClientUnaryCall;
    public getApps(request: teleport_lib_teleterm_v1_service_pb.GetAppsRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAppsResponse) => void): grpc.ClientUnaryCall;
    public getApps(request: teleport_lib_teleterm_v1_service_pb.GetAppsRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetAppsResponse) => void): grpc.ClientUnaryCall;
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
    public transferFile(request: teleport_lib_teleterm_v1_service_pb.FileTransferRequest, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<teleport_lib_teleterm_v1_service_pb.FileTransferProgress>;
    public transferFile(request: teleport_lib_teleterm_v1_service_pb.FileTransferRequest, metadata?: grpc.Metadata, options?: Partial<grpc.CallOptions>): grpc.ClientReadableStream<teleport_lib_teleterm_v1_service_pb.FileTransferProgress>;
    public reportUsageEvent(request: teleport_lib_teleterm_v1_usage_events_pb.ReportUsageEventRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public reportUsageEvent(request: teleport_lib_teleterm_v1_usage_events_pb.ReportUsageEventRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public reportUsageEvent(request: teleport_lib_teleterm_v1_usage_events_pb.ReportUsageEventRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.EmptyResponse) => void): grpc.ClientUnaryCall;
    public updateHeadlessAuthenticationState(request: teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateResponse) => void): grpc.ClientUnaryCall;
    public updateHeadlessAuthenticationState(request: teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateResponse) => void): grpc.ClientUnaryCall;
    public updateHeadlessAuthenticationState(request: teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateHeadlessAuthenticationStateResponse) => void): grpc.ClientUnaryCall;
    public createConnectMyComputerRole(request: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleResponse) => void): grpc.ClientUnaryCall;
    public createConnectMyComputerRole(request: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleResponse) => void): grpc.ClientUnaryCall;
    public createConnectMyComputerRole(request: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerRoleResponse) => void): grpc.ClientUnaryCall;
    public createConnectMyComputerNodeToken(request: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenResponse) => void): grpc.ClientUnaryCall;
    public createConnectMyComputerNodeToken(request: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenResponse) => void): grpc.ClientUnaryCall;
    public createConnectMyComputerNodeToken(request: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.CreateConnectMyComputerNodeTokenResponse) => void): grpc.ClientUnaryCall;
    public deleteConnectMyComputerToken(request: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenResponse) => void): grpc.ClientUnaryCall;
    public deleteConnectMyComputerToken(request: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenResponse) => void): grpc.ClientUnaryCall;
    public deleteConnectMyComputerToken(request: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerTokenResponse) => void): grpc.ClientUnaryCall;
    public waitForConnectMyComputerNodeJoin(request: teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinResponse) => void): grpc.ClientUnaryCall;
    public waitForConnectMyComputerNodeJoin(request: teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinResponse) => void): grpc.ClientUnaryCall;
    public waitForConnectMyComputerNodeJoin(request: teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.WaitForConnectMyComputerNodeJoinResponse) => void): grpc.ClientUnaryCall;
    public deleteConnectMyComputerNode(request: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeResponse) => void): grpc.ClientUnaryCall;
    public deleteConnectMyComputerNode(request: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeResponse) => void): grpc.ClientUnaryCall;
    public deleteConnectMyComputerNode(request: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.DeleteConnectMyComputerNodeResponse) => void): grpc.ClientUnaryCall;
    public getConnectMyComputerNodeName(request: teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameResponse) => void): grpc.ClientUnaryCall;
    public getConnectMyComputerNodeName(request: teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameResponse) => void): grpc.ClientUnaryCall;
    public getConnectMyComputerNodeName(request: teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetConnectMyComputerNodeNameResponse) => void): grpc.ClientUnaryCall;
    public listUnifiedResources(request: teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesResponse) => void): grpc.ClientUnaryCall;
    public listUnifiedResources(request: teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesResponse) => void): grpc.ClientUnaryCall;
    public listUnifiedResources(request: teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.ListUnifiedResourcesResponse) => void): grpc.ClientUnaryCall;
    public getUserPreferences(request: teleport_lib_teleterm_v1_service_pb.GetUserPreferencesRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetUserPreferencesResponse) => void): grpc.ClientUnaryCall;
    public getUserPreferences(request: teleport_lib_teleterm_v1_service_pb.GetUserPreferencesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetUserPreferencesResponse) => void): grpc.ClientUnaryCall;
    public getUserPreferences(request: teleport_lib_teleterm_v1_service_pb.GetUserPreferencesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.GetUserPreferencesResponse) => void): grpc.ClientUnaryCall;
    public updateUserPreferences(request: teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesRequest, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesResponse) => void): grpc.ClientUnaryCall;
    public updateUserPreferences(request: teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesRequest, metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesResponse) => void): grpc.ClientUnaryCall;
    public updateUserPreferences(request: teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesRequest, metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: teleport_lib_teleterm_v1_service_pb.UpdateUserPreferencesResponse) => void): grpc.ClientUnaryCall;
}
