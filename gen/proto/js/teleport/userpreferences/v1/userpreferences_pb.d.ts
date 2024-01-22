// package: teleport.userpreferences.v1
// file: teleport/userpreferences/v1/userpreferences.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as teleport_userpreferences_v1_assist_pb from "../../../teleport/userpreferences/v1/assist_pb";
import * as teleport_userpreferences_v1_cluster_preferences_pb from "../../../teleport/userpreferences/v1/cluster_preferences_pb";
import * as teleport_userpreferences_v1_onboard_pb from "../../../teleport/userpreferences/v1/onboard_pb";
import * as teleport_userpreferences_v1_theme_pb from "../../../teleport/userpreferences/v1/theme_pb";
import * as teleport_userpreferences_v1_unified_resource_preferences_pb from "../../../teleport/userpreferences/v1/unified_resource_preferences_pb";

export class UserPreferences extends jspb.Message { 

    hasAssist(): boolean;
    clearAssist(): void;
    getAssist(): teleport_userpreferences_v1_assist_pb.AssistUserPreferences | undefined;
    setAssist(value?: teleport_userpreferences_v1_assist_pb.AssistUserPreferences): UserPreferences;

    getTheme(): teleport_userpreferences_v1_theme_pb.Theme;
    setTheme(value: teleport_userpreferences_v1_theme_pb.Theme): UserPreferences;


    hasOnboard(): boolean;
    clearOnboard(): void;
    getOnboard(): teleport_userpreferences_v1_onboard_pb.OnboardUserPreferences | undefined;
    setOnboard(value?: teleport_userpreferences_v1_onboard_pb.OnboardUserPreferences): UserPreferences;


    hasClusterPreferences(): boolean;
    clearClusterPreferences(): void;
    getClusterPreferences(): teleport_userpreferences_v1_cluster_preferences_pb.ClusterUserPreferences | undefined;
    setClusterPreferences(value?: teleport_userpreferences_v1_cluster_preferences_pb.ClusterUserPreferences): UserPreferences;


    hasUnifiedResourcePreferences(): boolean;
    clearUnifiedResourcePreferences(): void;
    getUnifiedResourcePreferences(): teleport_userpreferences_v1_unified_resource_preferences_pb.UnifiedResourcePreferences | undefined;
    setUnifiedResourcePreferences(value?: teleport_userpreferences_v1_unified_resource_preferences_pb.UnifiedResourcePreferences): UserPreferences;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UserPreferences.AsObject;
    static toObject(includeInstance: boolean, msg: UserPreferences): UserPreferences.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UserPreferences, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UserPreferences;
    static deserializeBinaryFromReader(message: UserPreferences, reader: jspb.BinaryReader): UserPreferences;
}

export namespace UserPreferences {
    export type AsObject = {
        assist?: teleport_userpreferences_v1_assist_pb.AssistUserPreferences.AsObject,
        theme: teleport_userpreferences_v1_theme_pb.Theme,
        onboard?: teleport_userpreferences_v1_onboard_pb.OnboardUserPreferences.AsObject,
        clusterPreferences?: teleport_userpreferences_v1_cluster_preferences_pb.ClusterUserPreferences.AsObject,
        unifiedResourcePreferences?: teleport_userpreferences_v1_unified_resource_preferences_pb.UnifiedResourcePreferences.AsObject,
    }
}

export class GetUserPreferencesRequest extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetUserPreferencesRequest.AsObject;
    static toObject(includeInstance: boolean, msg: GetUserPreferencesRequest): GetUserPreferencesRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetUserPreferencesRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetUserPreferencesRequest;
    static deserializeBinaryFromReader(message: GetUserPreferencesRequest, reader: jspb.BinaryReader): GetUserPreferencesRequest;
}

export namespace GetUserPreferencesRequest {
    export type AsObject = {
    }
}

export class GetUserPreferencesResponse extends jspb.Message { 

    hasPreferences(): boolean;
    clearPreferences(): void;
    getPreferences(): UserPreferences | undefined;
    setPreferences(value?: UserPreferences): GetUserPreferencesResponse;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): GetUserPreferencesResponse.AsObject;
    static toObject(includeInstance: boolean, msg: GetUserPreferencesResponse): GetUserPreferencesResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: GetUserPreferencesResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): GetUserPreferencesResponse;
    static deserializeBinaryFromReader(message: GetUserPreferencesResponse, reader: jspb.BinaryReader): GetUserPreferencesResponse;
}

export namespace GetUserPreferencesResponse {
    export type AsObject = {
        preferences?: UserPreferences.AsObject,
    }
}

export class UpsertUserPreferencesRequest extends jspb.Message { 

    hasPreferences(): boolean;
    clearPreferences(): void;
    getPreferences(): UserPreferences | undefined;
    setPreferences(value?: UserPreferences): UpsertUserPreferencesRequest;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UpsertUserPreferencesRequest.AsObject;
    static toObject(includeInstance: boolean, msg: UpsertUserPreferencesRequest): UpsertUserPreferencesRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UpsertUserPreferencesRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UpsertUserPreferencesRequest;
    static deserializeBinaryFromReader(message: UpsertUserPreferencesRequest, reader: jspb.BinaryReader): UpsertUserPreferencesRequest;
}

export namespace UpsertUserPreferencesRequest {
    export type AsObject = {
        preferences?: UserPreferences.AsObject,
    }
}
