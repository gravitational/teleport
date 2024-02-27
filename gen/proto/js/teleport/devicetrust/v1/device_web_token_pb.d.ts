// package: teleport.devicetrust.v1
// file: teleport/devicetrust/v1/device_web_token.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class DeviceWebToken extends jspb.Message { 
    getId(): string;
    setId(value: string): DeviceWebToken;
    getToken(): string;
    setToken(value: string): DeviceWebToken;
    getWebSessionId(): string;
    setWebSessionId(value: string): DeviceWebToken;
    getBrowserUserAgent(): string;
    setBrowserUserAgent(value: string): DeviceWebToken;
    getBrowserIp(): string;
    setBrowserIp(value: string): DeviceWebToken;
    getUser(): string;
    setUser(value: string): DeviceWebToken;
    clearExpectedDeviceIdsList(): void;
    getExpectedDeviceIdsList(): Array<string>;
    setExpectedDeviceIdsList(value: Array<string>): DeviceWebToken;
    addExpectedDeviceIds(value: string, index?: number): string;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): DeviceWebToken.AsObject;
    static toObject(includeInstance: boolean, msg: DeviceWebToken): DeviceWebToken.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: DeviceWebToken, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): DeviceWebToken;
    static deserializeBinaryFromReader(message: DeviceWebToken, reader: jspb.BinaryReader): DeviceWebToken;
}

export namespace DeviceWebToken {
    export type AsObject = {
        id: string,
        token: string,
        webSessionId: string,
        browserUserAgent: string,
        browserIp: string,
        user: string,
        expectedDeviceIdsList: Array<string>,
    }
}
