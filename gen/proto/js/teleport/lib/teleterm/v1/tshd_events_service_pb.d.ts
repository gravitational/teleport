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


    hasCannotPromptMfa(): boolean;
    clearCannotPromptMfa(): void;
    getCannotPromptMfa(): CannotPromptMFA | undefined;
    setCannotPromptMfa(value?: CannotPromptMFA): SendNotificationRequest;


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
        cannotPromptMfa?: CannotPromptMFA.AsObject,
    }

    export enum SubjectCase {
        SUBJECT_NOT_SET = 0,
    
    CANNOT_PROXY_GATEWAY_CONNECTION = 1,

    CANNOT_PROMPT_MFA = 2,

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

export class CannotPromptMFA extends jspb.Message { 
    getTargetUri(): string;
    setTargetUri(value: string): CannotPromptMFA;

    getError(): string;
    setError(value: string): CannotPromptMFA;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CannotPromptMFA.AsObject;
    static toObject(includeInstance: boolean, msg: CannotPromptMFA): CannotPromptMFA.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CannotPromptMFA, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CannotPromptMFA;
    static deserializeBinaryFromReader(message: CannotPromptMFA, reader: jspb.BinaryReader): CannotPromptMFA;
}

export namespace CannotPromptMFA {
    export type AsObject = {
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

export class PromptMFARequest extends jspb.Message { 
    getRootClusterUri(): string;
    setRootClusterUri(value: string): PromptMFARequest;


    hasHeadlessRequest(): boolean;
    clearHeadlessRequest(): void;
    getHeadlessRequest(): HeadlessRequest | undefined;
    setHeadlessRequest(value?: HeadlessRequest): PromptMFARequest;


    getRequestCase(): PromptMFARequest.RequestCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PromptMFARequest.AsObject;
    static toObject(includeInstance: boolean, msg: PromptMFARequest): PromptMFARequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PromptMFARequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PromptMFARequest;
    static deserializeBinaryFromReader(message: PromptMFARequest, reader: jspb.BinaryReader): PromptMFARequest;
}

export namespace PromptMFARequest {
    export type AsObject = {
        rootClusterUri: string,
        headlessRequest?: HeadlessRequest.AsObject,
    }

    export enum RequestCase {
        REQUEST_NOT_SET = 0,
    
    HEADLESS_REQUEST = 2,

    }

}

export class HeadlessRequest extends jspb.Message { 

    hasHeadlessAuthentication(): boolean;
    clearHeadlessAuthentication(): void;
    getHeadlessAuthentication(): HeadlessAuthentication | undefined;
    setHeadlessAuthentication(value?: HeadlessAuthentication): HeadlessRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): HeadlessRequest.AsObject;
    static toObject(includeInstance: boolean, msg: HeadlessRequest): HeadlessRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: HeadlessRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): HeadlessRequest;
    static deserializeBinaryFromReader(message: HeadlessRequest, reader: jspb.BinaryReader): HeadlessRequest;
}

export namespace HeadlessRequest {
    export type AsObject = {
        headlessAuthentication?: HeadlessAuthentication.AsObject,
    }
}

export class HeadlessAuthentication extends jspb.Message { 
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
        user: string,
        clientIpAddress: string,
    }
}

export class PromptMFAResponse extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PromptMFAResponse.AsObject;
    static toObject(includeInstance: boolean, msg: PromptMFAResponse): PromptMFAResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PromptMFAResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PromptMFAResponse;
    static deserializeBinaryFromReader(message: PromptMFAResponse, reader: jspb.BinaryReader): PromptMFAResponse;
}

export namespace PromptMFAResponse {
    export type AsObject = {
    }
}
