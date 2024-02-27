// package: teleport.devicetrust.v1
// file: teleport/devicetrust/v1/user_certificates.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class UserCertificates extends jspb.Message { 
    getX509Der(): Uint8Array | string;
    getX509Der_asU8(): Uint8Array;
    getX509Der_asB64(): string;
    setX509Der(value: Uint8Array | string): UserCertificates;

    getSshAuthorizedKey(): Uint8Array | string;
    getSshAuthorizedKey_asU8(): Uint8Array;
    getSshAuthorizedKey_asB64(): string;
    setSshAuthorizedKey(value: Uint8Array | string): UserCertificates;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UserCertificates.AsObject;
    static toObject(includeInstance: boolean, msg: UserCertificates): UserCertificates.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UserCertificates, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UserCertificates;
    static deserializeBinaryFromReader(message: UserCertificates, reader: jspb.BinaryReader): UserCertificates;
}

export namespace UserCertificates {
    export type AsObject = {
        x509Der: Uint8Array | string,
        sshAuthorizedKey: Uint8Array | string,
    }
}
