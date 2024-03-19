// package: teleport.trait.v1
// file: teleport/trait/v1/trait.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class Trait extends jspb.Message { 
    getKey(): string;
    setKey(value: string): Trait;
    clearValuesList(): void;
    getValuesList(): Array<string>;
    setValuesList(value: Array<string>): Trait;
    addValues(value: string, index?: number): string;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Trait.AsObject;
    static toObject(includeInstance: boolean, msg: Trait): Trait.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Trait, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Trait;
    static deserializeBinaryFromReader(message: Trait, reader: jspb.BinaryReader): Trait;
}

export namespace Trait {
    export type AsObject = {
        key: string,
        valuesList: Array<string>,
    }
}
