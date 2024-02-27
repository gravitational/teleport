// package: teleport.devicetrust.v1
// file: teleport/devicetrust/v1/tpm.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class TPMPCR extends jspb.Message { 
    getIndex(): number;
    setIndex(value: number): TPMPCR;
    getDigest(): Uint8Array | string;
    getDigest_asU8(): Uint8Array;
    getDigest_asB64(): string;
    setDigest(value: Uint8Array | string): TPMPCR;
    getDigestAlg(): number;
    setDigestAlg(value: number): TPMPCR;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TPMPCR.AsObject;
    static toObject(includeInstance: boolean, msg: TPMPCR): TPMPCR.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TPMPCR, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TPMPCR;
    static deserializeBinaryFromReader(message: TPMPCR, reader: jspb.BinaryReader): TPMPCR;
}

export namespace TPMPCR {
    export type AsObject = {
        index: number,
        digest: Uint8Array | string,
        digestAlg: number,
    }
}

export class TPMQuote extends jspb.Message { 
    getQuote(): Uint8Array | string;
    getQuote_asU8(): Uint8Array;
    getQuote_asB64(): string;
    setQuote(value: Uint8Array | string): TPMQuote;
    getSignature(): Uint8Array | string;
    getSignature_asU8(): Uint8Array;
    getSignature_asB64(): string;
    setSignature(value: Uint8Array | string): TPMQuote;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TPMQuote.AsObject;
    static toObject(includeInstance: boolean, msg: TPMQuote): TPMQuote.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TPMQuote, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TPMQuote;
    static deserializeBinaryFromReader(message: TPMQuote, reader: jspb.BinaryReader): TPMQuote;
}

export namespace TPMQuote {
    export type AsObject = {
        quote: Uint8Array | string,
        signature: Uint8Array | string,
    }
}

export class TPMPlatformParameters extends jspb.Message { 
    clearQuotesList(): void;
    getQuotesList(): Array<TPMQuote>;
    setQuotesList(value: Array<TPMQuote>): TPMPlatformParameters;
    addQuotes(value?: TPMQuote, index?: number): TPMQuote;
    clearPcrsList(): void;
    getPcrsList(): Array<TPMPCR>;
    setPcrsList(value: Array<TPMPCR>): TPMPlatformParameters;
    addPcrs(value?: TPMPCR, index?: number): TPMPCR;
    getEventLog(): Uint8Array | string;
    getEventLog_asU8(): Uint8Array;
    getEventLog_asB64(): string;
    setEventLog(value: Uint8Array | string): TPMPlatformParameters;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TPMPlatformParameters.AsObject;
    static toObject(includeInstance: boolean, msg: TPMPlatformParameters): TPMPlatformParameters.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TPMPlatformParameters, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TPMPlatformParameters;
    static deserializeBinaryFromReader(message: TPMPlatformParameters, reader: jspb.BinaryReader): TPMPlatformParameters;
}

export namespace TPMPlatformParameters {
    export type AsObject = {
        quotesList: Array<TPMQuote.AsObject>,
        pcrsList: Array<TPMPCR.AsObject>,
        eventLog: Uint8Array | string,
    }
}

export class TPMPlatformAttestation extends jspb.Message { 
    getNonce(): Uint8Array | string;
    getNonce_asU8(): Uint8Array;
    getNonce_asB64(): string;
    setNonce(value: Uint8Array | string): TPMPlatformAttestation;

    hasPlatformParameters(): boolean;
    clearPlatformParameters(): void;
    getPlatformParameters(): TPMPlatformParameters | undefined;
    setPlatformParameters(value?: TPMPlatformParameters): TPMPlatformAttestation;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): TPMPlatformAttestation.AsObject;
    static toObject(includeInstance: boolean, msg: TPMPlatformAttestation): TPMPlatformAttestation.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: TPMPlatformAttestation, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): TPMPlatformAttestation;
    static deserializeBinaryFromReader(message: TPMPlatformAttestation, reader: jspb.BinaryReader): TPMPlatformAttestation;
}

export namespace TPMPlatformAttestation {
    export type AsObject = {
        nonce: Uint8Array | string,
        platformParameters?: TPMPlatformParameters.AsObject,
    }
}
