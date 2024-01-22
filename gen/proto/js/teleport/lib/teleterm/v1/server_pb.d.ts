// package: teleport.lib.teleterm.v1
// file: teleport/lib/teleterm/v1/server.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as teleport_lib_teleterm_v1_label_pb from "../../../../teleport/lib/teleterm/v1/label_pb";

export class Server extends jspb.Message { 
    getUri(): string;
    setUri(value: string): Server;

    getTunnel(): boolean;
    setTunnel(value: boolean): Server;

    getName(): string;
    setName(value: string): Server;

    getHostname(): string;
    setHostname(value: string): Server;

    getAddr(): string;
    setAddr(value: string): Server;

    clearLabelsList(): void;
    getLabelsList(): Array<teleport_lib_teleterm_v1_label_pb.Label>;
    setLabelsList(value: Array<teleport_lib_teleterm_v1_label_pb.Label>): Server;
    addLabels(value?: teleport_lib_teleterm_v1_label_pb.Label, index?: number): teleport_lib_teleterm_v1_label_pb.Label;

    getSubKind(): string;
    setSubKind(value: string): Server;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Server.AsObject;
    static toObject(includeInstance: boolean, msg: Server): Server.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Server, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Server;
    static deserializeBinaryFromReader(message: Server, reader: jspb.BinaryReader): Server;
}

export namespace Server {
    export type AsObject = {
        uri: string,
        tunnel: boolean,
        name: string,
        hostname: string,
        addr: string,
        labelsList: Array<teleport_lib_teleterm_v1_label_pb.Label.AsObject>,
        subKind: string,
    }
}
