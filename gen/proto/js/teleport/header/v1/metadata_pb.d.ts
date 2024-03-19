// package: teleport.header.v1
// file: teleport/header/v1/metadata.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class Metadata extends jspb.Message { 
    getName(): string;
    setName(value: string): Metadata;
    getNamespace(): string;
    setNamespace(value: string): Metadata;
    getDescription(): string;
    setDescription(value: string): Metadata;

    getLabelsMap(): jspb.Map<string, string>;
    clearLabelsMap(): void;

    hasExpires(): boolean;
    clearExpires(): void;
    getExpires(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setExpires(value?: google_protobuf_timestamp_pb.Timestamp): Metadata;
    getId(): number;
    setId(value: number): Metadata;
    getRevision(): string;
    setRevision(value: string): Metadata;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Metadata.AsObject;
    static toObject(includeInstance: boolean, msg: Metadata): Metadata.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Metadata, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Metadata;
    static deserializeBinaryFromReader(message: Metadata, reader: jspb.BinaryReader): Metadata;
}

export namespace Metadata {
    export type AsObject = {
        name: string,
        namespace: string,
        description: string,

        labelsMap: Array<[string, string]>,
        expires?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        id: number,
        revision: string,
    }
}
