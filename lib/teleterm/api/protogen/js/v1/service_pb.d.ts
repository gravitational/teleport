// package: teleport.terminal.v1
// file: v1/service.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as v1_cluster_pb from "../v1/cluster_pb";
import * as v1_database_pb from "../v1/database_pb";
import * as v1_gateway_pb from "../v1/gateway_pb";
import * as v1_kube_pb from "../v1/kube_pb";
import * as v1_app_pb from "../v1/app_pb";
import * as v1_server_pb from "../v1/server_pb";
import * as v1_auth_settings_pb from "../v1/auth_settings_pb";

export class RemoveClusterRequest extends jspb.Message { 
    getClusterUri(): string;
    setClusterUri(value: string): RemoveClusterRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RemoveClusterRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RemoveClusterRequest): RemoveClusterRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RemoveClusterRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RemoveClusterRequest;
    static deserializeBinaryFromReader(message: RemoveClusterRequest, reader: jspb.BinaryReader): RemoveClusterRequest;
}

export namespace RemoveClusterRequest {
    export type AsObject = {
        clusterUri: string,
    }
}

export class GetClusterRequest extends jspb.Message { 
    getClusterUri(): string;
    setClusterUri(value: string): GetClusterRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetClusterRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetClusterRequest): GetClusterRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetClusterRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetClusterRequest;
    static deserializeBinaryFromReader(message: GetClusterRequest, reader: jspb.BinaryReader): GetClusterRequest;
}

export namespace GetClusterRequest {
    export type AsObject = {
        clusterUri: string,
    }
}

export class LogoutRequest extends jspb.Message { 
    getClusterUri(): string;
    setClusterUri(value: string): LogoutRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): LogoutRequest.AsObject;
    static toObject(includeInstance: boolean, msg: LogoutRequest): LogoutRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: LogoutRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): LogoutRequest;
    static deserializeBinaryFromReader(message: LogoutRequest, reader: jspb.BinaryReader): LogoutRequest;
}

export namespace LogoutRequest {
    export type AsObject = {
        clusterUri: string,
    }
}

export class LoginRequest extends jspb.Message { 
    getClusterUri(): string;
    setClusterUri(value: string): LoginRequest;


    hasLocal(): boolean;
    clearLocal(): void;
    getLocal(): LoginRequest.LocalParams | undefined;
    setLocal(value?: LoginRequest.LocalParams): LoginRequest;


    hasSso(): boolean;
    clearSso(): void;
    getSso(): LoginRequest.SsoParams | undefined;
    setSso(value?: LoginRequest.SsoParams): LoginRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): LoginRequest.AsObject;
    static toObject(includeInstance: boolean, msg: LoginRequest): LoginRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: LoginRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): LoginRequest;
    static deserializeBinaryFromReader(message: LoginRequest, reader: jspb.BinaryReader): LoginRequest;
}

export namespace LoginRequest {
    export type AsObject = {
        clusterUri: string,
        local?: LoginRequest.LocalParams.AsObject,
        sso?: LoginRequest.SsoParams.AsObject,
    }


    export class LocalParams extends jspb.Message { 
        getUser(): string;
        setUser(value: string): LocalParams;

        getPassword(): string;
        setPassword(value: string): LocalParams;

        getToken(): string;
        setToken(value: string): LocalParams;


        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): LocalParams.AsObject;
        static toObject(includeInstance: boolean, msg: LocalParams): LocalParams.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: LocalParams, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): LocalParams;
        static deserializeBinaryFromReader(message: LocalParams, reader: jspb.BinaryReader): LocalParams;
    }

    export namespace LocalParams {
        export type AsObject = {
            user: string,
            password: string,
            token: string,
        }
    }

    export class SsoParams extends jspb.Message { 
        getProviderType(): string;
        setProviderType(value: string): SsoParams;

        getProviderName(): string;
        setProviderName(value: string): SsoParams;


        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): SsoParams.AsObject;
        static toObject(includeInstance: boolean, msg: SsoParams): SsoParams.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: SsoParams, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): SsoParams;
        static deserializeBinaryFromReader(message: SsoParams, reader: jspb.BinaryReader): SsoParams;
    }

    export namespace SsoParams {
        export type AsObject = {
            providerType: string,
            providerName: string,
        }
    }

}

export class AddClusterRequest extends jspb.Message { 
    getName(): string;
    setName(value: string): AddClusterRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AddClusterRequest.AsObject;
    static toObject(includeInstance: boolean, msg: AddClusterRequest): AddClusterRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AddClusterRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AddClusterRequest;
    static deserializeBinaryFromReader(message: AddClusterRequest, reader: jspb.BinaryReader): AddClusterRequest;
}

export namespace AddClusterRequest {
    export type AsObject = {
        name: string,
    }
}

export class ListKubesRequest extends jspb.Message { 
    getClusterUri(): string;
    setClusterUri(value: string): ListKubesRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListKubesRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ListKubesRequest): ListKubesRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListKubesRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListKubesRequest;
    static deserializeBinaryFromReader(message: ListKubesRequest, reader: jspb.BinaryReader): ListKubesRequest;
}

export namespace ListKubesRequest {
    export type AsObject = {
        clusterUri: string,
    }
}

export class ListAppsRequest extends jspb.Message { 
    getClusterUri(): string;
    setClusterUri(value: string): ListAppsRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListAppsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ListAppsRequest): ListAppsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListAppsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListAppsRequest;
    static deserializeBinaryFromReader(message: ListAppsRequest, reader: jspb.BinaryReader): ListAppsRequest;
}

export namespace ListAppsRequest {
    export type AsObject = {
        clusterUri: string,
    }
}

export class ListClustersRequest extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListClustersRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ListClustersRequest): ListClustersRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListClustersRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListClustersRequest;
    static deserializeBinaryFromReader(message: ListClustersRequest, reader: jspb.BinaryReader): ListClustersRequest;
}

export namespace ListClustersRequest {
    export type AsObject = {
    }
}

export class ListClustersResponse extends jspb.Message { 
    clearClustersList(): void;
    getClustersList(): Array<v1_cluster_pb.Cluster>;
    setClustersList(value: Array<v1_cluster_pb.Cluster>): ListClustersResponse;
    addClusters(value?: v1_cluster_pb.Cluster, index?: number): v1_cluster_pb.Cluster;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListClustersResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ListClustersResponse): ListClustersResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListClustersResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListClustersResponse;
    static deserializeBinaryFromReader(message: ListClustersResponse, reader: jspb.BinaryReader): ListClustersResponse;
}

export namespace ListClustersResponse {
    export type AsObject = {
        clustersList: Array<v1_cluster_pb.Cluster.AsObject>,
    }
}

export class ListDatabasesRequest extends jspb.Message { 
    getClusterUri(): string;
    setClusterUri(value: string): ListDatabasesRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListDatabasesRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ListDatabasesRequest): ListDatabasesRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListDatabasesRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListDatabasesRequest;
    static deserializeBinaryFromReader(message: ListDatabasesRequest, reader: jspb.BinaryReader): ListDatabasesRequest;
}

export namespace ListDatabasesRequest {
    export type AsObject = {
        clusterUri: string,
    }
}

export class ListLeafClustersRequest extends jspb.Message { 
    getClusterUri(): string;
    setClusterUri(value: string): ListLeafClustersRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListLeafClustersRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ListLeafClustersRequest): ListLeafClustersRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListLeafClustersRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListLeafClustersRequest;
    static deserializeBinaryFromReader(message: ListLeafClustersRequest, reader: jspb.BinaryReader): ListLeafClustersRequest;
}

export namespace ListLeafClustersRequest {
    export type AsObject = {
        clusterUri: string,
    }
}

export class ListDatabasesResponse extends jspb.Message { 
    clearDatabasesList(): void;
    getDatabasesList(): Array<v1_database_pb.Database>;
    setDatabasesList(value: Array<v1_database_pb.Database>): ListDatabasesResponse;
    addDatabases(value?: v1_database_pb.Database, index?: number): v1_database_pb.Database;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListDatabasesResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ListDatabasesResponse): ListDatabasesResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListDatabasesResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListDatabasesResponse;
    static deserializeBinaryFromReader(message: ListDatabasesResponse, reader: jspb.BinaryReader): ListDatabasesResponse;
}

export namespace ListDatabasesResponse {
    export type AsObject = {
        databasesList: Array<v1_database_pb.Database.AsObject>,
    }
}

export class CreateGatewayRequest extends jspb.Message { 
    getTargetUri(): string;
    setTargetUri(value: string): CreateGatewayRequest;

    getTargetUser(): string;
    setTargetUser(value: string): CreateGatewayRequest;

    getLocalPort(): string;
    setLocalPort(value: string): CreateGatewayRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CreateGatewayRequest.AsObject;
    static toObject(includeInstance: boolean, msg: CreateGatewayRequest): CreateGatewayRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CreateGatewayRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CreateGatewayRequest;
    static deserializeBinaryFromReader(message: CreateGatewayRequest, reader: jspb.BinaryReader): CreateGatewayRequest;
}

export namespace CreateGatewayRequest {
    export type AsObject = {
        targetUri: string,
        targetUser: string,
        localPort: string,
    }
}

export class ListGatewaysRequest extends jspb.Message { 
    clearClusterIdsList(): void;
    getClusterIdsList(): Array<string>;
    setClusterIdsList(value: Array<string>): ListGatewaysRequest;
    addClusterIds(value: string, index?: number): string;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListGatewaysRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ListGatewaysRequest): ListGatewaysRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListGatewaysRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListGatewaysRequest;
    static deserializeBinaryFromReader(message: ListGatewaysRequest, reader: jspb.BinaryReader): ListGatewaysRequest;
}

export namespace ListGatewaysRequest {
    export type AsObject = {
        clusterIdsList: Array<string>,
    }
}

export class ListGatewaysResponse extends jspb.Message { 
    clearGatewaysList(): void;
    getGatewaysList(): Array<v1_gateway_pb.Gateway>;
    setGatewaysList(value: Array<v1_gateway_pb.Gateway>): ListGatewaysResponse;
    addGateways(value?: v1_gateway_pb.Gateway, index?: number): v1_gateway_pb.Gateway;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListGatewaysResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ListGatewaysResponse): ListGatewaysResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListGatewaysResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListGatewaysResponse;
    static deserializeBinaryFromReader(message: ListGatewaysResponse, reader: jspb.BinaryReader): ListGatewaysResponse;
}

export namespace ListGatewaysResponse {
    export type AsObject = {
        gatewaysList: Array<v1_gateway_pb.Gateway.AsObject>,
    }
}

export class RemoveGatewayRequest extends jspb.Message { 
    getGatewayUri(): string;
    setGatewayUri(value: string): RemoveGatewayRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): RemoveGatewayRequest.AsObject;
    static toObject(includeInstance: boolean, msg: RemoveGatewayRequest): RemoveGatewayRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: RemoveGatewayRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): RemoveGatewayRequest;
    static deserializeBinaryFromReader(message: RemoveGatewayRequest, reader: jspb.BinaryReader): RemoveGatewayRequest;
}

export namespace RemoveGatewayRequest {
    export type AsObject = {
        gatewayUri: string,
    }
}

export class ListServersRequest extends jspb.Message { 
    getClusterUri(): string;
    setClusterUri(value: string): ListServersRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListServersRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ListServersRequest): ListServersRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListServersRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListServersRequest;
    static deserializeBinaryFromReader(message: ListServersRequest, reader: jspb.BinaryReader): ListServersRequest;
}

export namespace ListServersRequest {
    export type AsObject = {
        clusterUri: string,
    }
}

export class ListServersResponse extends jspb.Message { 
    clearServersList(): void;
    getServersList(): Array<v1_server_pb.Server>;
    setServersList(value: Array<v1_server_pb.Server>): ListServersResponse;
    addServers(value?: v1_server_pb.Server, index?: number): v1_server_pb.Server;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListServersResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ListServersResponse): ListServersResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListServersResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListServersResponse;
    static deserializeBinaryFromReader(message: ListServersResponse, reader: jspb.BinaryReader): ListServersResponse;
}

export namespace ListServersResponse {
    export type AsObject = {
        serversList: Array<v1_server_pb.Server.AsObject>,
    }
}

export class ListKubesResponse extends jspb.Message { 
    clearKubesList(): void;
    getKubesList(): Array<v1_kube_pb.Kube>;
    setKubesList(value: Array<v1_kube_pb.Kube>): ListKubesResponse;
    addKubes(value?: v1_kube_pb.Kube, index?: number): v1_kube_pb.Kube;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListKubesResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ListKubesResponse): ListKubesResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListKubesResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListKubesResponse;
    static deserializeBinaryFromReader(message: ListKubesResponse, reader: jspb.BinaryReader): ListKubesResponse;
}

export namespace ListKubesResponse {
    export type AsObject = {
        kubesList: Array<v1_kube_pb.Kube.AsObject>,
    }
}

export class ListAppsResponse extends jspb.Message { 
    clearAppsList(): void;
    getAppsList(): Array<v1_app_pb.App>;
    setAppsList(value: Array<v1_app_pb.App>): ListAppsResponse;
    addApps(value?: v1_app_pb.App, index?: number): v1_app_pb.App;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListAppsResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ListAppsResponse): ListAppsResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListAppsResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListAppsResponse;
    static deserializeBinaryFromReader(message: ListAppsResponse, reader: jspb.BinaryReader): ListAppsResponse;
}

export namespace ListAppsResponse {
    export type AsObject = {
        appsList: Array<v1_app_pb.App.AsObject>,
    }
}

export class GetAuthSettingsRequest extends jspb.Message { 
    getClusterUri(): string;
    setClusterUri(value: string): GetAuthSettingsRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetAuthSettingsRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetAuthSettingsRequest): GetAuthSettingsRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetAuthSettingsRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetAuthSettingsRequest;
    static deserializeBinaryFromReader(message: GetAuthSettingsRequest, reader: jspb.BinaryReader): GetAuthSettingsRequest;
}

export namespace GetAuthSettingsRequest {
    export type AsObject = {
        clusterUri: string,
    }
}

export class EmptyResponse extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): EmptyResponse.AsObject;
    static toObject(includeInstance: boolean, msg: EmptyResponse): EmptyResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: EmptyResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): EmptyResponse;
    static deserializeBinaryFromReader(message: EmptyResponse, reader: jspb.BinaryReader): EmptyResponse;
}

export namespace EmptyResponse {
    export type AsObject = {
    }
}
