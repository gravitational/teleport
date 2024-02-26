// package: teleport.userpreferences.v1
// file: teleport/userpreferences/v1/assist.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";

export class AssistUserPreferences extends jspb.Message { 
    clearPreferredLoginsList(): void;
    getPreferredLoginsList(): Array<string>;
    setPreferredLoginsList(value: Array<string>): AssistUserPreferences;
    addPreferredLogins(value: string, index?: number): string;
    getViewMode(): AssistViewMode;
    setViewMode(value: AssistViewMode): AssistUserPreferences;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AssistUserPreferences.AsObject;
    static toObject(includeInstance: boolean, msg: AssistUserPreferences): AssistUserPreferences.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AssistUserPreferences, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AssistUserPreferences;
    static deserializeBinaryFromReader(message: AssistUserPreferences, reader: jspb.BinaryReader): AssistUserPreferences;
}

export namespace AssistUserPreferences {
    export type AsObject = {
        preferredLoginsList: Array<string>,
        viewMode: AssistViewMode,
    }
}

export enum AssistViewMode {
    ASSIST_VIEW_MODE_UNSPECIFIED = 0,
    ASSIST_VIEW_MODE_DOCKED = 1,
    ASSIST_VIEW_MODE_POPUP = 2,
    ASSIST_VIEW_MODE_POPUP_EXPANDED = 3,
    ASSIST_VIEW_MODE_POPUP_EXPANDED_SIDEBAR_VISIBLE = 4,
}
