// package: teleterm.v1
// file: teleterm/v1/kube.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as teleterm_v1_label_pb from "../../teleterm/v1/label_pb";

export class Kube extends jspb.Message { 
    getUri(): string;
    setUri(value: string): Kube;

    getName(): string;
    setName(value: string): Kube;

    clearLabelsList(): void;
    getLabelsList(): Array<teleterm_v1_label_pb.Label>;
    setLabelsList(value: Array<teleterm_v1_label_pb.Label>): Kube;
    addLabels(value?: teleterm_v1_label_pb.Label, index?: number): teleterm_v1_label_pb.Label;


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
        labelsList: Array<teleterm_v1_label_pb.Label.AsObject>,
    }
}
