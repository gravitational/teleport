// package: teleport.lib.teleterm.v1
// file: teleport/lib/teleterm/v1/fileserver.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class GetFileServerConfigRequest extends jspb.Message { 
    getClusterUri(): string;
    setClusterUri(value: string): GetFileServerConfigRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetFileServerConfigRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetFileServerConfigRequest): GetFileServerConfigRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetFileServerConfigRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetFileServerConfigRequest;
    static deserializeBinaryFromReader(message: GetFileServerConfigRequest, reader: jspb.BinaryReader): GetFileServerConfigRequest;
}

export namespace GetFileServerConfigRequest {
    export type AsObject = {
        clusterUri: string,
    }
}

export class GetFileServerConfigResponse extends jspb.Message { 

    hasConfig(): boolean;
    clearConfig(): void;
    getConfig(): FileServerConfig | undefined;
    setConfig(value?: FileServerConfig): GetFileServerConfigResponse;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetFileServerConfigResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetFileServerConfigResponse): GetFileServerConfigResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetFileServerConfigResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetFileServerConfigResponse;
    static deserializeBinaryFromReader(message: GetFileServerConfigResponse, reader: jspb.BinaryReader): GetFileServerConfigResponse;
}

export namespace GetFileServerConfigResponse {
    export type AsObject = {
        config?: FileServerConfig.AsObject,
    }
}

export class SetFileServerConfigRequest extends jspb.Message { 
    getClusterUri(): string;
    setClusterUri(value: string): SetFileServerConfigRequest;


    hasConfig(): boolean;
    clearConfig(): void;
    getConfig(): FileServerConfig | undefined;
    setConfig(value?: FileServerConfig): SetFileServerConfigRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SetFileServerConfigRequest.AsObject;
    static toObject(includeInstance: boolean, msg: SetFileServerConfigRequest): SetFileServerConfigRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SetFileServerConfigRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SetFileServerConfigRequest;
    static deserializeBinaryFromReader(message: SetFileServerConfigRequest, reader: jspb.BinaryReader): SetFileServerConfigRequest;
}

export namespace SetFileServerConfigRequest {
    export type AsObject = {
        clusterUri: string,
        config?: FileServerConfig.AsObject,
    }
}

export class SetFileServerConfigResponse extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SetFileServerConfigResponse.AsObject;
    static toObject(includeInstance: boolean, msg: SetFileServerConfigResponse): SetFileServerConfigResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SetFileServerConfigResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SetFileServerConfigResponse;
    static deserializeBinaryFromReader(message: SetFileServerConfigResponse, reader: jspb.BinaryReader): SetFileServerConfigResponse;
}

export namespace SetFileServerConfigResponse {
    export type AsObject = {
    }
}

export class FileServerConfig extends jspb.Message { 
    clearSharesList(): void;
    getSharesList(): Array<FileServerShare>;
    setSharesList(value: Array<FileServerShare>): FileServerConfig;
    addShares(value?: FileServerShare, index?: number): FileServerShare;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): FileServerConfig.AsObject;
    static toObject(includeInstance: boolean, msg: FileServerConfig): FileServerConfig.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: FileServerConfig, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): FileServerConfig;
    static deserializeBinaryFromReader(message: FileServerConfig, reader: jspb.BinaryReader): FileServerConfig;
}

export namespace FileServerConfig {
    export type AsObject = {
        sharesList: Array<FileServerShare.AsObject>,
    }
}

export class FileServerShare extends jspb.Message { 
    getName(): string;
    setName(value: string): FileServerShare;

    getPath(): string;
    setPath(value: string): FileServerShare;

    getAllowAnyone(): boolean;
    setAllowAnyone(value: boolean): FileServerShare;

    clearAllowedUsersList(): void;
    getAllowedUsersList(): Array<string>;
    setAllowedUsersList(value: Array<string>): FileServerShare;
    addAllowedUsers(value: string, index?: number): string;

    clearAllowedRolesList(): void;
    getAllowedRolesList(): Array<string>;
    setAllowedRolesList(value: Array<string>): FileServerShare;
    addAllowedRoles(value: string, index?: number): string;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): FileServerShare.AsObject;
    static toObject(includeInstance: boolean, msg: FileServerShare): FileServerShare.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: FileServerShare, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): FileServerShare;
    static deserializeBinaryFromReader(message: FileServerShare, reader: jspb.BinaryReader): FileServerShare;
}

export namespace FileServerShare {
    export type AsObject = {
        name: string,
        path: string,
        allowAnyone: boolean,
        allowedUsersList: Array<string>,
        allowedRolesList: Array<string>,
    }
}
