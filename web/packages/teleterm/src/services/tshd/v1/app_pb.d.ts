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
// file: v1/app.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as v1_label_pb from "../v1/label_pb";

export class App extends jspb.Message { 
    getUri(): string;
    setUri(value: string): App;

    getName(): string;
    setName(value: string): App;

    getDescription(): string;
    setDescription(value: string): App;

    getAppUri(): string;
    setAppUri(value: string): App;

    getPublicAddr(): string;
    setPublicAddr(value: string): App;

    getFqdn(): string;
    setFqdn(value: string): App;

    clearLabelsList(): void;
    getLabelsList(): Array<v1_label_pb.Label>;
    setLabelsList(value: Array<v1_label_pb.Label>): App;
    addLabels(value?: v1_label_pb.Label, index?: number): v1_label_pb.Label;

    getAwsConsole(): boolean;
    setAwsConsole(value: boolean): App;

    clearAwsRolesList(): void;
    getAwsRolesList(): Array<App.AWSRole>;
    setAwsRolesList(value: Array<App.AWSRole>): App;
    addAwsRoles(value?: App.AWSRole, index?: number): App.AWSRole;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): App.AsObject;
    static toObject(includeInstance: boolean, msg: App): App.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: App, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): App;
    static deserializeBinaryFromReader(message: App, reader: jspb.BinaryReader): App;
}

export namespace App {
    export type AsObject = {
        uri: string,
        name: string,
        description: string,
        appUri: string,
        publicAddr: string,
        fqdn: string,
        labelsList: Array<v1_label_pb.Label.AsObject>,
        awsConsole: boolean,
        awsRolesList: Array<App.AWSRole.AsObject>,
    }


    export class AWSRole extends jspb.Message { 
        getDisplay(): string;
        setDisplay(value: string): AWSRole;

        getArn(): string;
        setArn(value: string): AWSRole;


        serializeBinary(): Uint8Array;
        toObject(includeInstance?: boolean): AWSRole.AsObject;
        static toObject(includeInstance: boolean, msg: AWSRole): AWSRole.AsObject;
        static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
        static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
        static serializeBinaryToWriter(message: AWSRole, writer: jspb.BinaryWriter): void;
        static deserializeBinary(bytes: Uint8Array): AWSRole;
        static deserializeBinaryFromReader(message: AWSRole, reader: jspb.BinaryReader): AWSRole;
    }

    export namespace AWSRole {
        export type AsObject = {
            display: string,
            arn: string,
        }
    }

}
