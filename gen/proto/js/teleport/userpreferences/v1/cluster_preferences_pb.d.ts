// package: teleport.userpreferences.v1
// file: teleport/userpreferences/v1/cluster_preferences.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class PinnedResourcesUserPreferences extends jspb.Message { 
    clearResourceIdsList(): void;
    getResourceIdsList(): Array<string>;
    setResourceIdsList(value: Array<string>): PinnedResourcesUserPreferences;
    addResourceIds(value: string, index?: number): string;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): PinnedResourcesUserPreferences.AsObject;
    static toObject(includeInstance: boolean, msg: PinnedResourcesUserPreferences): PinnedResourcesUserPreferences.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: PinnedResourcesUserPreferences, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): PinnedResourcesUserPreferences;
    static deserializeBinaryFromReader(message: PinnedResourcesUserPreferences, reader: jspb.BinaryReader): PinnedResourcesUserPreferences;
}

export namespace PinnedResourcesUserPreferences {
    export type AsObject = {
        resourceIdsList: Array<string>,
    }
}

export class ClusterUserPreferences extends jspb.Message { 

    hasPinnedResources(): boolean;
    clearPinnedResources(): void;
    getPinnedResources(): PinnedResourcesUserPreferences | undefined;
    setPinnedResources(value?: PinnedResourcesUserPreferences): ClusterUserPreferences;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ClusterUserPreferences.AsObject;
    static toObject(includeInstance: boolean, msg: ClusterUserPreferences): ClusterUserPreferences.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ClusterUserPreferences, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ClusterUserPreferences;
    static deserializeBinaryFromReader(message: ClusterUserPreferences, reader: jspb.BinaryReader): ClusterUserPreferences;
}

export namespace ClusterUserPreferences {
    export type AsObject = {
        pinnedResources?: PinnedResourcesUserPreferences.AsObject,
    }
}
