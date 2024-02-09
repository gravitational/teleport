// package: teleport.lib.teleterm.vnet.v1
// file: teleport/lib/teleterm/vnet/v1/vnet_service.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class StartRequest extends jspb.Message { 
    getRootClusterUri(): string;
    setRootClusterUri(value: string): StartRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): StartRequest.AsObject;
    static toObject(includeInstance: boolean, msg: StartRequest): StartRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: StartRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): StartRequest;
    static deserializeBinaryFromReader(message: StartRequest, reader: jspb.BinaryReader): StartRequest;
}

export namespace StartRequest {
    export type AsObject = {
        rootClusterUri: string,
    }
}

export class StartResponse extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): StartResponse.AsObject;
    static toObject(includeInstance: boolean, msg: StartResponse): StartResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: StartResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): StartResponse;
    static deserializeBinaryFromReader(message: StartResponse, reader: jspb.BinaryReader): StartResponse;
}

export namespace StartResponse {
    export type AsObject = {
    }
}

export class StopRequest extends jspb.Message { 
    getRootClusterUri(): string;
    setRootClusterUri(value: string): StopRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): StopRequest.AsObject;
    static toObject(includeInstance: boolean, msg: StopRequest): StopRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: StopRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): StopRequest;
    static deserializeBinaryFromReader(message: StopRequest, reader: jspb.BinaryReader): StopRequest;
}

export namespace StopRequest {
    export type AsObject = {
        rootClusterUri: string,
    }
}

export class StopResponse extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): StopResponse.AsObject;
    static toObject(includeInstance: boolean, msg: StopResponse): StopResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: StopResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): StopResponse;
    static deserializeBinaryFromReader(message: StopResponse, reader: jspb.BinaryReader): StopResponse;
}

export namespace StopResponse {
    export type AsObject = {
    }
}
