// package: teleterm.v1
// file: teleterm/v1/database.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as teleterm_v1_label_pb from "../../teleterm/v1/label_pb";

export class Database extends jspb.Message { 
    getUri(): string;
    setUri(value: string): Database;

    getName(): string;
    setName(value: string): Database;

    getDesc(): string;
    setDesc(value: string): Database;

    getProtocol(): string;
    setProtocol(value: string): Database;

    getType(): string;
    setType(value: string): Database;

    getHostname(): string;
    setHostname(value: string): Database;

    getAddr(): string;
    setAddr(value: string): Database;

    clearLabelsList(): void;
    getLabelsList(): Array<teleterm_v1_label_pb.Label>;
    setLabelsList(value: Array<teleterm_v1_label_pb.Label>): Database;
    addLabels(value?: teleterm_v1_label_pb.Label, index?: number): teleterm_v1_label_pb.Label;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Database.AsObject;
    static toObject(includeInstance: boolean, msg: Database): Database.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Database, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Database;
    static deserializeBinaryFromReader(message: Database, reader: jspb.BinaryReader): Database;
}

export namespace Database {
    export type AsObject = {
        uri: string,
        name: string,
        desc: string,
        protocol: string,
        type: string,
        hostname: string,
        addr: string,
        labelsList: Array<teleterm_v1_label_pb.Label.AsObject>,
    }
}
