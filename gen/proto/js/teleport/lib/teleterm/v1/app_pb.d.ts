// package: teleport.lib.teleterm.v1
// file: teleport/lib/teleterm/v1/app.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as teleport_lib_teleterm_v1_label_pb from "../../../../teleport/lib/teleterm/v1/label_pb";

export class App extends jspb.Message { 
    getUri(): string;
    setUri(value: string): App;

    getName(): string;
    setName(value: string): App;

    getEndpointUri(): string;
    setEndpointUri(value: string): App;

    getDesc(): string;
    setDesc(value: string): App;

    getAwsConsole(): boolean;
    setAwsConsole(value: boolean): App;

    getPublicAddr(): string;
    setPublicAddr(value: string): App;

    getFriendlyName(): string;
    setFriendlyName(value: string): App;

    getSamlApp(): boolean;
    setSamlApp(value: boolean): App;

    clearLabelsList(): void;
    getLabelsList(): Array<teleport_lib_teleterm_v1_label_pb.Label>;
    setLabelsList(value: Array<teleport_lib_teleterm_v1_label_pb.Label>): App;
    addLabels(value?: teleport_lib_teleterm_v1_label_pb.Label, index?: number): teleport_lib_teleterm_v1_label_pb.Label;


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
        endpointUri: string,
        desc: string,
        awsConsole: boolean,
        publicAddr: string,
        friendlyName: string,
        samlApp: boolean,
        labelsList: Array<teleport_lib_teleterm_v1_label_pb.Label.AsObject>,
    }
}
