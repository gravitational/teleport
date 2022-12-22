// package: teleport.terminal.v1
// file: v1/label.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class Label extends jspb.Message { 
    getName(): string;
    setName(value: string): Label;

    getValue(): string;
    setValue(value: string): Label;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Label.AsObject;
    static toObject(includeInstance: boolean, msg: Label): Label.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Label, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Label;
    static deserializeBinaryFromReader(message: Label, reader: jspb.BinaryReader): Label;
}

export namespace Label {
    export type AsObject = {
        name: string,
        value: string,
    }
}
