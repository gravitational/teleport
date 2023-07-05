// package: teleport.lib.teleterm.v1
// file: teleport/lib/teleterm/v1/tshd_events_service.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class ReloginRequest extends jspb.Message { 
    getRootClusterUri(): string;
    setRootClusterUri(value: string): ReloginRequest;


    hasGatewayCertExpired(): boolean;
    clearGatewayCertExpired(): void;
    getGatewayCertExpired(): GatewayCertExpired | undefined;
    setGatewayCertExpired(value?: GatewayCertExpired): ReloginRequest;


    getReasonCase(): ReloginRequest.ReasonCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ReloginRequest.AsObject;
    static toObject(includeInstance: boolean, msg: ReloginRequest): ReloginRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ReloginRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ReloginRequest;
    static deserializeBinaryFromReader(message: ReloginRequest, reader: jspb.BinaryReader): ReloginRequest;
}

export namespace ReloginRequest {
    export type AsObject = {
        rootClusterUri: string,
        gatewayCertExpired?: GatewayCertExpired.AsObject,
    }

    export enum ReasonCase {
        REASON_NOT_SET = 0,
    
    GATEWAY_CERT_EXPIRED = 2,

    }

}

export class GatewayCertExpired extends jspb.Message { 
    getGatewayUri(): string;
    setGatewayUri(value: string): GatewayCertExpired;

    getTargetUri(): string;
    setTargetUri(value: string): GatewayCertExpired;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GatewayCertExpired.AsObject;
    static toObject(includeInstance: boolean, msg: GatewayCertExpired): GatewayCertExpired.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GatewayCertExpired, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GatewayCertExpired;
    static deserializeBinaryFromReader(message: GatewayCertExpired, reader: jspb.BinaryReader): GatewayCertExpired;
}

export namespace GatewayCertExpired {
    export type AsObject = {
        gatewayUri: string,
        targetUri: string,
    }
}

export class ReloginResponse extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ReloginResponse.AsObject;
    static toObject(includeInstance: boolean, msg: ReloginResponse): ReloginResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ReloginResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ReloginResponse;
    static deserializeBinaryFromReader(message: ReloginResponse, reader: jspb.BinaryReader): ReloginResponse;
}

export namespace ReloginResponse {
    export type AsObject = {
    }
}

export class SendNotificationRequest extends jspb.Message { 

    hasCannotProxyGatewayConnection(): boolean;
    clearCannotProxyGatewayConnection(): void;
    getCannotProxyGatewayConnection(): CannotProxyGatewayConnection | undefined;
    setCannotProxyGatewayConnection(value?: CannotProxyGatewayConnection): SendNotificationRequest;


    getSubjectCase(): SendNotificationRequest.SubjectCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SendNotificationRequest.AsObject;
    static toObject(includeInstance: boolean, msg: SendNotificationRequest): SendNotificationRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SendNotificationRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SendNotificationRequest;
    static deserializeBinaryFromReader(message: SendNotificationRequest, reader: jspb.BinaryReader): SendNotificationRequest;
}

export namespace SendNotificationRequest {
    export type AsObject = {
        cannotProxyGatewayConnection?: CannotProxyGatewayConnection.AsObject,
    }

    export enum SubjectCase {
        SUBJECT_NOT_SET = 0,
    
    CANNOT_PROXY_GATEWAY_CONNECTION = 1,

    }

}

export class CannotProxyGatewayConnection extends jspb.Message { 
    getGatewayUri(): string;
    setGatewayUri(value: string): CannotProxyGatewayConnection;

    getTargetUri(): string;
    setTargetUri(value: string): CannotProxyGatewayConnection;

    getError(): string;
    setError(value: string): CannotProxyGatewayConnection;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CannotProxyGatewayConnection.AsObject;
    static toObject(includeInstance: boolean, msg: CannotProxyGatewayConnection): CannotProxyGatewayConnection.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CannotProxyGatewayConnection, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CannotProxyGatewayConnection;
    static deserializeBinaryFromReader(message: CannotProxyGatewayConnection, reader: jspb.BinaryReader): CannotProxyGatewayConnection;
}

export namespace CannotProxyGatewayConnection {
    export type AsObject = {
        gatewayUri: string,
        targetUri: string,
        error: string,
    }
}

export class SendNotificationResponse extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SendNotificationResponse.AsObject;
    static toObject(includeInstance: boolean, msg: SendNotificationResponse): SendNotificationResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SendNotificationResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SendNotificationResponse;
    static deserializeBinaryFromReader(message: SendNotificationResponse, reader: jspb.BinaryReader): SendNotificationResponse;
}

export namespace SendNotificationResponse {
    export type AsObject = {
    }
}

export class HeadlessAuthenticationRequest extends jspb.Message { 
    getClusterUri(): string;
    setClusterUri(value: string): HeadlessAuthenticationRequest;


    hasHeadlessAuthentication(): boolean;
    clearHeadlessAuthentication(): void;
    getHeadlessAuthentication(): HeadlessAuthentication | undefined;
    setHeadlessAuthentication(value?: HeadlessAuthentication): HeadlessAuthenticationRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): HeadlessAuthenticationRequest.AsObject;
    static toObject(includeInstance: boolean, msg: HeadlessAuthenticationRequest): HeadlessAuthenticationRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: HeadlessAuthenticationRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): HeadlessAuthenticationRequest;
    static deserializeBinaryFromReader(message: HeadlessAuthenticationRequest, reader: jspb.BinaryReader): HeadlessAuthenticationRequest;
}

export namespace HeadlessAuthenticationRequest {
    export type AsObject = {
        clusterUri: string,
        headlessAuthentication?: HeadlessAuthentication.AsObject,
    }
}

export class HeadlessAuthentication extends jspb.Message { 
    getName(): string;
    setName(value: string): HeadlessAuthentication;

    getUser(): string;
    setUser(value: string): HeadlessAuthentication;

    getClientIpAddress(): string;
    setClientIpAddress(value: string): HeadlessAuthentication;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): HeadlessAuthentication.AsObject;
    static toObject(includeInstance: boolean, msg: HeadlessAuthentication): HeadlessAuthentication.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: HeadlessAuthentication, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): HeadlessAuthentication;
    static deserializeBinaryFromReader(message: HeadlessAuthentication, reader: jspb.BinaryReader): HeadlessAuthentication;
}

export namespace HeadlessAuthentication {
    export type AsObject = {
        name: string,
        user: string,
        clientIpAddress: string,
    }
}

export class HeadlessAuthenticationResponse extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): HeadlessAuthenticationResponse.AsObject;
    static toObject(includeInstance: boolean, msg: HeadlessAuthenticationResponse): HeadlessAuthenticationResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: HeadlessAuthenticationResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): HeadlessAuthenticationResponse;
    static deserializeBinaryFromReader(message: HeadlessAuthenticationResponse, reader: jspb.BinaryReader): HeadlessAuthenticationResponse;
}

export namespace HeadlessAuthenticationResponse {
    export type AsObject = {
    }
}
