// package: teleport.header.v1
// file: teleport/header/v1/resourceheader.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as teleport_header_v1_metadata_pb from "../../../teleport/header/v1/metadata_pb";

export class ResourceHeader extends jspb.Message { 
    getKind(): string;
    setKind(value: string): ResourceHeader;
    getSubKind(): string;
    setSubKind(value: string): ResourceHeader;
    getVersion(): string;
    setVersion(value: string): ResourceHeader;

    hasMetadata(): boolean;
    clearMetadata(): void;
    getMetadata(): teleport_header_v1_metadata_pb.Metadata | undefined;
    setMetadata(value?: teleport_header_v1_metadata_pb.Metadata): ResourceHeader;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ResourceHeader.AsObject;
    static toObject(includeInstance: boolean, msg: ResourceHeader): ResourceHeader.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ResourceHeader, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ResourceHeader;
    static deserializeBinaryFromReader(message: ResourceHeader, reader: jspb.BinaryReader): ResourceHeader;
}

export namespace ResourceHeader {
    export type AsObject = {
        kind: string,
        subKind: string,
        version: string,
        metadata?: teleport_header_v1_metadata_pb.Metadata.AsObject,
    }
}
