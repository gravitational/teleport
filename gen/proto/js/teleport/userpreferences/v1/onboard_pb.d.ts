// package: teleport.userpreferences.v1
// file: teleport/userpreferences/v1/onboard.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class MarketingParams extends jspb.Message { 
    getCampaign(): string;
    setCampaign(value: string): MarketingParams;

    getSource(): string;
    setSource(value: string): MarketingParams;

    getMedium(): string;
    setMedium(value: string): MarketingParams;

    getIntent(): string;
    setIntent(value: string): MarketingParams;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): MarketingParams.AsObject;
    static toObject(includeInstance: boolean, msg: MarketingParams): MarketingParams.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: MarketingParams, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): MarketingParams;
    static deserializeBinaryFromReader(message: MarketingParams, reader: jspb.BinaryReader): MarketingParams;
}

export namespace MarketingParams {
    export type AsObject = {
        campaign: string,
        source: string,
        medium: string,
        intent: string,
    }
}

export class OnboardUserPreferences extends jspb.Message { 
    clearPreferredResourcesList(): void;
    getPreferredResourcesList(): Array<Resource>;
    setPreferredResourcesList(value: Array<Resource>): OnboardUserPreferences;
    addPreferredResources(value: Resource, index?: number): Resource;


    hasMarketingParams(): boolean;
    clearMarketingParams(): void;
    getMarketingParams(): MarketingParams | undefined;
    setMarketingParams(value?: MarketingParams): OnboardUserPreferences;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): OnboardUserPreferences.AsObject;
    static toObject(includeInstance: boolean, msg: OnboardUserPreferences): OnboardUserPreferences.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: OnboardUserPreferences, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): OnboardUserPreferences;
    static deserializeBinaryFromReader(message: OnboardUserPreferences, reader: jspb.BinaryReader): OnboardUserPreferences;
}

export namespace OnboardUserPreferences {
    export type AsObject = {
        preferredResourcesList: Array<Resource>,
        marketingParams?: MarketingParams.AsObject,
    }
}

export enum Resource {
    RESOURCE_UNSPECIFIED = 0,
    RESOURCE_WINDOWS_DESKTOPS = 1,
    RESOURCE_SERVER_SSH = 2,
    RESOURCE_DATABASES = 3,
    RESOURCE_KUBERNETES = 4,
    RESOURCE_WEB_APPLICATIONS = 5,
}
