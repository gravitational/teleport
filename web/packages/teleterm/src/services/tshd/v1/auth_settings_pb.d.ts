/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// package: teleport.terminal.v1
// file: v1/auth_settings.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class AuthSettings extends jspb.Message { 
    getLocalAuthEnabled(): boolean;
    setLocalAuthEnabled(value: boolean): AuthSettings;

    getSecondFactor(): string;
    setSecondFactor(value: string): AuthSettings;

    getPreferredMfa(): string;
    setPreferredMfa(value: string): AuthSettings;

    clearAuthProvidersList(): void;
    getAuthProvidersList(): Array<AuthProvider>;
    setAuthProvidersList(value: Array<AuthProvider>): AuthSettings;
    addAuthProviders(value?: AuthProvider, index?: number): AuthProvider;

    getHasMessageOfTheDay(): boolean;
    setHasMessageOfTheDay(value: boolean): AuthSettings;

    getAuthType(): string;
    setAuthType(value: string): AuthSettings;

    getAllowPasswordless(): boolean;
    setAllowPasswordless(value: boolean): AuthSettings;

    getLocalConnectorName(): string;
    setLocalConnectorName(value: string): AuthSettings;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AuthSettings.AsObject;
    static toObject(includeInstance: boolean, msg: AuthSettings): AuthSettings.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AuthSettings, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AuthSettings;
    static deserializeBinaryFromReader(message: AuthSettings, reader: jspb.BinaryReader): AuthSettings;
}

export namespace AuthSettings {
    export type AsObject = {
        localAuthEnabled: boolean,
        secondFactor: string,
        preferredMfa: string,
        authProvidersList: Array<AuthProvider.AsObject>,
        hasMessageOfTheDay: boolean,
        authType: string,
        allowPasswordless: boolean,
        localConnectorName: string,
    }
}

export class AuthProvider extends jspb.Message { 
    getType(): string;
    setType(value: string): AuthProvider;

    getName(): string;
    setName(value: string): AuthProvider;

    getDisplayName(): string;
    setDisplayName(value: string): AuthProvider;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AuthProvider.AsObject;
    static toObject(includeInstance: boolean, msg: AuthProvider): AuthProvider.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AuthProvider, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AuthProvider;
    static deserializeBinaryFromReader(message: AuthProvider, reader: jspb.BinaryReader): AuthProvider;
}

export namespace AuthProvider {
    export type AsObject = {
        type: string,
        name: string,
        displayName: string,
    }
}
