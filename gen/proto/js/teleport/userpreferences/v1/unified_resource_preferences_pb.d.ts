// package: teleport.userpreferences.v1
// file: teleport/userpreferences/v1/unified_resource_preferences.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class UnifiedResourcePreferences extends jspb.Message { 
    getDefaultTab(): DefaultTab;
    setDefaultTab(value: DefaultTab): UnifiedResourcePreferences;

    getViewMode(): ViewMode;
    setViewMode(value: ViewMode): UnifiedResourcePreferences;

    getLabelsViewMode(): LabelsViewMode;
    setLabelsViewMode(value: LabelsViewMode): UnifiedResourcePreferences;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UnifiedResourcePreferences.AsObject;
    static toObject(includeInstance: boolean, msg: UnifiedResourcePreferences): UnifiedResourcePreferences.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UnifiedResourcePreferences, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UnifiedResourcePreferences;
    static deserializeBinaryFromReader(message: UnifiedResourcePreferences, reader: jspb.BinaryReader): UnifiedResourcePreferences;
}

export namespace UnifiedResourcePreferences {
    export type AsObject = {
        defaultTab: DefaultTab,
        viewMode: ViewMode,
        labelsViewMode: LabelsViewMode,
    }
}

export enum DefaultTab {
    DEFAULT_TAB_UNSPECIFIED = 0,
    DEFAULT_TAB_ALL = 1,
    DEFAULT_TAB_PINNED = 2,
}

export enum ViewMode {
    VIEW_MODE_UNSPECIFIED = 0,
    VIEW_MODE_CARD = 1,
    VIEW_MODE_LIST = 2,
}

export enum LabelsViewMode {
    LABELS_VIEW_MODE_UNSPECIFIED = 0,
    LABELS_VIEW_MODE_EXPANDED = 1,
    LABELS_VIEW_MODE_COLLAPSED = 2,
}
