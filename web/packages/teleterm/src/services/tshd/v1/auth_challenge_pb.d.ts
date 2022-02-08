// package: teleport.terminal.v1
// file: v1/auth_challenge.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class AuthChallengeU2F extends jspb.Message { 
    getKeyHandle(): string;
    setKeyHandle(value: string): AuthChallengeU2F;

    getChallenge(): string;
    setChallenge(value: string): AuthChallengeU2F;

    getAppId(): string;
    setAppId(value: string): AuthChallengeU2F;

    getVersion(): string;
    setVersion(value: string): AuthChallengeU2F;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AuthChallengeU2F.AsObject;
    static toObject(includeInstance: boolean, msg: AuthChallengeU2F): AuthChallengeU2F.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AuthChallengeU2F, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AuthChallengeU2F;
    static deserializeBinaryFromReader(message: AuthChallengeU2F, reader: jspb.BinaryReader): AuthChallengeU2F;
}

export namespace AuthChallengeU2F {
    export type AsObject = {
        keyHandle: string,
        challenge: string,
        appId: string,
        version: string,
    }
}

export class ChallengeU2F extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ChallengeU2F.AsObject;
    static toObject(includeInstance: boolean, msg: ChallengeU2F): ChallengeU2F.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ChallengeU2F, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ChallengeU2F;
    static deserializeBinaryFromReader(message: ChallengeU2F, reader: jspb.BinaryReader): ChallengeU2F;
}

export namespace ChallengeU2F {
    export type AsObject = {
    }
}

export class ChallengeTOTP extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ChallengeTOTP.AsObject;
    static toObject(includeInstance: boolean, msg: ChallengeTOTP): ChallengeTOTP.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ChallengeTOTP, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ChallengeTOTP;
    static deserializeBinaryFromReader(message: ChallengeTOTP, reader: jspb.BinaryReader): ChallengeTOTP;
}

export namespace ChallengeTOTP {
    export type AsObject = {
    }
}

export class SolvedChallengeU2F extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SolvedChallengeU2F.AsObject;
    static toObject(includeInstance: boolean, msg: SolvedChallengeU2F): SolvedChallengeU2F.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SolvedChallengeU2F, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SolvedChallengeU2F;
    static deserializeBinaryFromReader(message: SolvedChallengeU2F, reader: jspb.BinaryReader): SolvedChallengeU2F;
}

export namespace SolvedChallengeU2F {
    export type AsObject = {
    }
}

export class SolvedChallengeTOTP extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SolvedChallengeTOTP.AsObject;
    static toObject(includeInstance: boolean, msg: SolvedChallengeTOTP): SolvedChallengeTOTP.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SolvedChallengeTOTP, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SolvedChallengeTOTP;
    static deserializeBinaryFromReader(message: SolvedChallengeTOTP, reader: jspb.BinaryReader): SolvedChallengeTOTP;
}

export namespace SolvedChallengeTOTP {
    export type AsObject = {
    }
}
