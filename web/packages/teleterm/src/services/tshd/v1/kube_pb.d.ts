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
// file: v1/kube.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as v1_label_pb from "../v1/label_pb";

export class Kube extends jspb.Message { 
    getUri(): string;
    setUri(value: string): Kube;

    getName(): string;
    setName(value: string): Kube;

    clearLabelsList(): void;
    getLabelsList(): Array<v1_label_pb.Label>;
    setLabelsList(value: Array<v1_label_pb.Label>): Kube;
    addLabels(value?: v1_label_pb.Label, index?: number): v1_label_pb.Label;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Kube.AsObject;
    static toObject(includeInstance: boolean, msg: Kube): Kube.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Kube, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Kube;
    static deserializeBinaryFromReader(message: Kube, reader: jspb.BinaryReader): Kube;
}

export namespace Kube {
    export type AsObject = {
        uri: string,
        name: string,
        labelsList: Array<v1_label_pb.Label.AsObject>,
    }
}
