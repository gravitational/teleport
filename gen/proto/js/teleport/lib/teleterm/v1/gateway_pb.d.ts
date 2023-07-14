// package: teleport.lib.teleterm.v1
// file: teleport/lib/teleterm/v1/gateway.proto

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

    getTargetSubresourceName(): string;
    setTargetSubresourceName(value: string): Gateway;


    hasGatewayCliCommand(): boolean;
    clearGatewayCliCommand(): void;
    getGatewayCliCommand(): GatewayCLICommand | undefined;
    setGatewayCliCommand(value?: GatewayCLICommand): Gateway;


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
        targetSubresourceName: string,
        gatewayCliCommand?: GatewayCLICommand.AsObject,
    }
}

export class GatewayCLICommand extends jspb.Message { 
    getPath(): string;
    setPath(value: string): GatewayCLICommand;

    clearArgsList(): void;
    getArgsList(): Array<string>;
    setArgsList(value: Array<string>): GatewayCLICommand;
    addArgs(value: string, index?: number): string;

    clearEnvList(): void;
    getEnvList(): Array<string>;
    setEnvList(value: Array<string>): GatewayCLICommand;
    addEnv(value: string, index?: number): string;

    getPreview(): string;
    setPreview(value: string): GatewayCLICommand;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GatewayCLICommand.AsObject;
    static toObject(includeInstance: boolean, msg: GatewayCLICommand): GatewayCLICommand.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GatewayCLICommand, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GatewayCLICommand;
    static deserializeBinaryFromReader(message: GatewayCLICommand, reader: jspb.BinaryReader): GatewayCLICommand;
}

export namespace GatewayCLICommand {
    export type AsObject = {
        path: string,
        argsList: Array<string>,
        envList: Array<string>,
        preview: string,
    }
}
