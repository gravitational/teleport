// package: teleport.terminal.v1
// file: v1/gateway.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class Gateway extends jspb.Message { 
    getUri(): string;
    setUri(value: string): Gateway;

    getTargetName(): string;
    setTargetName(value: string): Gateway;

    getTargetUri(): string;
    setTargetUri(value: string): Gateway;

    getTargetUser(): string;
    setTargetUser(value: string): Gateway;

    getLocalAddress(): string;
    setLocalAddress(value: string): Gateway;

    getLocalPort(): string;
    setLocalPort(value: string): Gateway;

    getProtocol(): string;
    setProtocol(value: string): Gateway;

    getInsecure(): boolean;
    setInsecure(value: boolean): Gateway;

    getCaCertPath(): string;
    setCaCertPath(value: string): Gateway;

    getCertPath(): string;
    setCertPath(value: string): Gateway;

    getKeyPath(): string;
    setKeyPath(value: string): Gateway;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Gateway.AsObject;
    static toObject(includeInstance: boolean, msg: Gateway): Gateway.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Gateway, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Gateway;
    static deserializeBinaryFromReader(message: Gateway, reader: jspb.BinaryReader): Gateway;
}

export namespace Gateway {
    export type AsObject = {
        uri: string,
        targetName: string,
        targetUri: string,
        targetUser: string,
        localAddress: string,
        localPort: string,
        protocol: string,
        insecure: boolean,
        caCertPath: string,
        certPath: string,
        keyPath: string,
    }
}
