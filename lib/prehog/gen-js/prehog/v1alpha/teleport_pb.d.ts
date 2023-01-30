// package: prehog.v1alpha
// file: prehog/v1alpha/teleport.proto

/* tslint:disable */
/* eslint-disable */

import * as jspb from "google-protobuf";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class UserLoginEvent extends jspb.Message { 
    getUserName(): string;
    setUserName(value: string): UserLoginEvent;

    getConnectorType(): string;
    setConnectorType(value: string): UserLoginEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UserLoginEvent.AsObject;
    static toObject(includeInstance: boolean, msg: UserLoginEvent): UserLoginEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UserLoginEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UserLoginEvent;
    static deserializeBinaryFromReader(message: UserLoginEvent, reader: jspb.BinaryReader): UserLoginEvent;
}

export namespace UserLoginEvent {
    export type AsObject = {
        userName: string,
        connectorType: string,
    }
}

export class SSOCreateEvent extends jspb.Message { 
    getConnectorType(): string;
    setConnectorType(value: string): SSOCreateEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SSOCreateEvent.AsObject;
    static toObject(includeInstance: boolean, msg: SSOCreateEvent): SSOCreateEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SSOCreateEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SSOCreateEvent;
    static deserializeBinaryFromReader(message: SSOCreateEvent, reader: jspb.BinaryReader): SSOCreateEvent;
}

export namespace SSOCreateEvent {
    export type AsObject = {
        connectorType: string,
    }
}

export class ResourceCreateEvent extends jspb.Message { 
    getResourceType(): string;
    setResourceType(value: string): ResourceCreateEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ResourceCreateEvent.AsObject;
    static toObject(includeInstance: boolean, msg: ResourceCreateEvent): ResourceCreateEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ResourceCreateEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ResourceCreateEvent;
    static deserializeBinaryFromReader(message: ResourceCreateEvent, reader: jspb.BinaryReader): ResourceCreateEvent;
}

export namespace ResourceCreateEvent {
    export type AsObject = {
        resourceType: string,
    }
}

export class SessionStartEvent extends jspb.Message { 
    getUserName(): string;
    setUserName(value: string): SessionStartEvent;

    getSessionType(): string;
    setSessionType(value: string): SessionStartEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SessionStartEvent.AsObject;
    static toObject(includeInstance: boolean, msg: SessionStartEvent): SessionStartEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SessionStartEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SessionStartEvent;
    static deserializeBinaryFromReader(message: SessionStartEvent, reader: jspb.BinaryReader): SessionStartEvent;
}

export namespace SessionStartEvent {
    export type AsObject = {
        userName: string,
        sessionType: string,
    }
}

export class UIBannerClickEvent extends jspb.Message { 
    getUserName(): string;
    setUserName(value: string): UIBannerClickEvent;

    getAlert(): string;
    setAlert(value: string): UIBannerClickEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UIBannerClickEvent.AsObject;
    static toObject(includeInstance: boolean, msg: UIBannerClickEvent): UIBannerClickEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UIBannerClickEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UIBannerClickEvent;
    static deserializeBinaryFromReader(message: UIBannerClickEvent, reader: jspb.BinaryReader): UIBannerClickEvent;
}

export namespace UIBannerClickEvent {
    export type AsObject = {
        userName: string,
        alert: string,
    }
}

export class UIOnboardCompleteGoToDashboardClickEvent extends jspb.Message { 
    getUserName(): string;
    setUserName(value: string): UIOnboardCompleteGoToDashboardClickEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UIOnboardCompleteGoToDashboardClickEvent.AsObject;
    static toObject(includeInstance: boolean, msg: UIOnboardCompleteGoToDashboardClickEvent): UIOnboardCompleteGoToDashboardClickEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UIOnboardCompleteGoToDashboardClickEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UIOnboardCompleteGoToDashboardClickEvent;
    static deserializeBinaryFromReader(message: UIOnboardCompleteGoToDashboardClickEvent, reader: jspb.BinaryReader): UIOnboardCompleteGoToDashboardClickEvent;
}

export namespace UIOnboardCompleteGoToDashboardClickEvent {
    export type AsObject = {
        userName: string,
    }
}

export class UIOnboardAddFirstResourceClickEvent extends jspb.Message { 
    getUserName(): string;
    setUserName(value: string): UIOnboardAddFirstResourceClickEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UIOnboardAddFirstResourceClickEvent.AsObject;
    static toObject(includeInstance: boolean, msg: UIOnboardAddFirstResourceClickEvent): UIOnboardAddFirstResourceClickEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UIOnboardAddFirstResourceClickEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UIOnboardAddFirstResourceClickEvent;
    static deserializeBinaryFromReader(message: UIOnboardAddFirstResourceClickEvent, reader: jspb.BinaryReader): UIOnboardAddFirstResourceClickEvent;
}

export namespace UIOnboardAddFirstResourceClickEvent {
    export type AsObject = {
        userName: string,
    }
}

export class UIOnboardAddFirstResourceLaterClickEvent extends jspb.Message { 
    getUserName(): string;
    setUserName(value: string): UIOnboardAddFirstResourceLaterClickEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UIOnboardAddFirstResourceLaterClickEvent.AsObject;
    static toObject(includeInstance: boolean, msg: UIOnboardAddFirstResourceLaterClickEvent): UIOnboardAddFirstResourceLaterClickEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UIOnboardAddFirstResourceLaterClickEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UIOnboardAddFirstResourceLaterClickEvent;
    static deserializeBinaryFromReader(message: UIOnboardAddFirstResourceLaterClickEvent, reader: jspb.BinaryReader): UIOnboardAddFirstResourceLaterClickEvent;
}

export namespace UIOnboardAddFirstResourceLaterClickEvent {
    export type AsObject = {
        userName: string,
    }
}

export class UIOnboardSetCredentialSubmitEvent extends jspb.Message { 
    getUserName(): string;
    setUserName(value: string): UIOnboardSetCredentialSubmitEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UIOnboardSetCredentialSubmitEvent.AsObject;
    static toObject(includeInstance: boolean, msg: UIOnboardSetCredentialSubmitEvent): UIOnboardSetCredentialSubmitEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UIOnboardSetCredentialSubmitEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UIOnboardSetCredentialSubmitEvent;
    static deserializeBinaryFromReader(message: UIOnboardSetCredentialSubmitEvent, reader: jspb.BinaryReader): UIOnboardSetCredentialSubmitEvent;
}

export namespace UIOnboardSetCredentialSubmitEvent {
    export type AsObject = {
        userName: string,
    }
}

export class UIOnboardRegisterChallengeSubmitEvent extends jspb.Message { 
    getUserName(): string;
    setUserName(value: string): UIOnboardRegisterChallengeSubmitEvent;

    getMfaType(): string;
    setMfaType(value: string): UIOnboardRegisterChallengeSubmitEvent;

    getLoginFlow(): string;
    setLoginFlow(value: string): UIOnboardRegisterChallengeSubmitEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UIOnboardRegisterChallengeSubmitEvent.AsObject;
    static toObject(includeInstance: boolean, msg: UIOnboardRegisterChallengeSubmitEvent): UIOnboardRegisterChallengeSubmitEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UIOnboardRegisterChallengeSubmitEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UIOnboardRegisterChallengeSubmitEvent;
    static deserializeBinaryFromReader(message: UIOnboardRegisterChallengeSubmitEvent, reader: jspb.BinaryReader): UIOnboardRegisterChallengeSubmitEvent;
}

export namespace UIOnboardRegisterChallengeSubmitEvent {
    export type AsObject = {
        userName: string,
        mfaType: string,
        loginFlow: string,
    }
}

export class UIRecoveryCodesContinueClickEvent extends jspb.Message { 
    getUserName(): string;
    setUserName(value: string): UIRecoveryCodesContinueClickEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UIRecoveryCodesContinueClickEvent.AsObject;
    static toObject(includeInstance: boolean, msg: UIRecoveryCodesContinueClickEvent): UIRecoveryCodesContinueClickEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UIRecoveryCodesContinueClickEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UIRecoveryCodesContinueClickEvent;
    static deserializeBinaryFromReader(message: UIRecoveryCodesContinueClickEvent, reader: jspb.BinaryReader): UIRecoveryCodesContinueClickEvent;
}

export namespace UIRecoveryCodesContinueClickEvent {
    export type AsObject = {
        userName: string,
    }
}

export class UIRecoveryCodesCopyClickEvent extends jspb.Message { 
    getUserName(): string;
    setUserName(value: string): UIRecoveryCodesCopyClickEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UIRecoveryCodesCopyClickEvent.AsObject;
    static toObject(includeInstance: boolean, msg: UIRecoveryCodesCopyClickEvent): UIRecoveryCodesCopyClickEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UIRecoveryCodesCopyClickEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UIRecoveryCodesCopyClickEvent;
    static deserializeBinaryFromReader(message: UIRecoveryCodesCopyClickEvent, reader: jspb.BinaryReader): UIRecoveryCodesCopyClickEvent;
}

export namespace UIRecoveryCodesCopyClickEvent {
    export type AsObject = {
        userName: string,
    }
}

export class UIRecoveryCodesPrintClickEvent extends jspb.Message { 
    getUserName(): string;
    setUserName(value: string): UIRecoveryCodesPrintClickEvent;


    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): UIRecoveryCodesPrintClickEvent.AsObject;
    static toObject(includeInstance: boolean, msg: UIRecoveryCodesPrintClickEvent): UIRecoveryCodesPrintClickEvent.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: UIRecoveryCodesPrintClickEvent, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): UIRecoveryCodesPrintClickEvent;
    static deserializeBinaryFromReader(message: UIRecoveryCodesPrintClickEvent, reader: jspb.BinaryReader): UIRecoveryCodesPrintClickEvent;
}

export namespace UIRecoveryCodesPrintClickEvent {
    export type AsObject = {
        userName: string,
    }
}

export class SubmitEventRequest extends jspb.Message { 
    getClusterName(): string;
    setClusterName(value: string): SubmitEventRequest;


    hasTimestamp(): boolean;
    clearTimestamp(): void;
    getTimestamp(): google_protobuf_timestamp_pb.Timestamp | undefined;
    setTimestamp(value?: google_protobuf_timestamp_pb.Timestamp): SubmitEventRequest;


    hasUserLogin(): boolean;
    clearUserLogin(): void;
    getUserLogin(): UserLoginEvent | undefined;
    setUserLogin(value?: UserLoginEvent): SubmitEventRequest;


    hasSsoCreate(): boolean;
    clearSsoCreate(): void;
    getSsoCreate(): SSOCreateEvent | undefined;
    setSsoCreate(value?: SSOCreateEvent): SubmitEventRequest;


    hasResourceCreate(): boolean;
    clearResourceCreate(): void;
    getResourceCreate(): ResourceCreateEvent | undefined;
    setResourceCreate(value?: ResourceCreateEvent): SubmitEventRequest;


    hasSessionStart(): boolean;
    clearSessionStart(): void;
    getSessionStart(): SessionStartEvent | undefined;
    setSessionStart(value?: SessionStartEvent): SubmitEventRequest;


    hasUiBannerClick(): boolean;
    clearUiBannerClick(): void;
    getUiBannerClick(): UIBannerClickEvent | undefined;
    setUiBannerClick(value?: UIBannerClickEvent): SubmitEventRequest;


    hasUiOnboardCompleteGoToDashboardClick(): boolean;
    clearUiOnboardCompleteGoToDashboardClick(): void;
    getUiOnboardCompleteGoToDashboardClick(): UIOnboardCompleteGoToDashboardClickEvent | undefined;
    setUiOnboardCompleteGoToDashboardClick(value?: UIOnboardCompleteGoToDashboardClickEvent): SubmitEventRequest;


    hasUiOnboardAddFirstResourceClick(): boolean;
    clearUiOnboardAddFirstResourceClick(): void;
    getUiOnboardAddFirstResourceClick(): UIOnboardAddFirstResourceClickEvent | undefined;
    setUiOnboardAddFirstResourceClick(value?: UIOnboardAddFirstResourceClickEvent): SubmitEventRequest;


    hasUiOnboardAddFirstResourceLaterClick(): boolean;
    clearUiOnboardAddFirstResourceLaterClick(): void;
    getUiOnboardAddFirstResourceLaterClick(): UIOnboardAddFirstResourceLaterClickEvent | undefined;
    setUiOnboardAddFirstResourceLaterClick(value?: UIOnboardAddFirstResourceLaterClickEvent): SubmitEventRequest;


    hasUiOnboardSetCredentialSubmit(): boolean;
    clearUiOnboardSetCredentialSubmit(): void;
    getUiOnboardSetCredentialSubmit(): UIOnboardSetCredentialSubmitEvent | undefined;
    setUiOnboardSetCredentialSubmit(value?: UIOnboardSetCredentialSubmitEvent): SubmitEventRequest;


    hasUiOnboardRegisterChallengeSubmit(): boolean;
    clearUiOnboardRegisterChallengeSubmit(): void;
    getUiOnboardRegisterChallengeSubmit(): UIOnboardRegisterChallengeSubmitEvent | undefined;
    setUiOnboardRegisterChallengeSubmit(value?: UIOnboardRegisterChallengeSubmitEvent): SubmitEventRequest;


    hasUiRecoveryCodesContinueClick(): boolean;
    clearUiRecoveryCodesContinueClick(): void;
    getUiRecoveryCodesContinueClick(): UIRecoveryCodesContinueClickEvent | undefined;
    setUiRecoveryCodesContinueClick(value?: UIRecoveryCodesContinueClickEvent): SubmitEventRequest;


    hasUiRecoveryCodesCopyClick(): boolean;
    clearUiRecoveryCodesCopyClick(): void;
    getUiRecoveryCodesCopyClick(): UIRecoveryCodesCopyClickEvent | undefined;
    setUiRecoveryCodesCopyClick(value?: UIRecoveryCodesCopyClickEvent): SubmitEventRequest;


    hasUiRecoveryCodesPrintClick(): boolean;
    clearUiRecoveryCodesPrintClick(): void;
    getUiRecoveryCodesPrintClick(): UIRecoveryCodesPrintClickEvent | undefined;
    setUiRecoveryCodesPrintClick(value?: UIRecoveryCodesPrintClickEvent): SubmitEventRequest;


    getEventCase(): SubmitEventRequest.EventCase;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SubmitEventRequest.AsObject;
    static toObject(includeInstance: boolean, msg: SubmitEventRequest): SubmitEventRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SubmitEventRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SubmitEventRequest;
    static deserializeBinaryFromReader(message: SubmitEventRequest, reader: jspb.BinaryReader): SubmitEventRequest;
}

export namespace SubmitEventRequest {
    export type AsObject = {
        clusterName: string,
        timestamp?: google_protobuf_timestamp_pb.Timestamp.AsObject,
        userLogin?: UserLoginEvent.AsObject,
        ssoCreate?: SSOCreateEvent.AsObject,
        resourceCreate?: ResourceCreateEvent.AsObject,
        sessionStart?: SessionStartEvent.AsObject,
        uiBannerClick?: UIBannerClickEvent.AsObject,
        uiOnboardCompleteGoToDashboardClick?: UIOnboardCompleteGoToDashboardClickEvent.AsObject,
        uiOnboardAddFirstResourceClick?: UIOnboardAddFirstResourceClickEvent.AsObject,
        uiOnboardAddFirstResourceLaterClick?: UIOnboardAddFirstResourceLaterClickEvent.AsObject,
        uiOnboardSetCredentialSubmit?: UIOnboardSetCredentialSubmitEvent.AsObject,
        uiOnboardRegisterChallengeSubmit?: UIOnboardRegisterChallengeSubmitEvent.AsObject,
        uiRecoveryCodesContinueClick?: UIRecoveryCodesContinueClickEvent.AsObject,
        uiRecoveryCodesCopyClick?: UIRecoveryCodesCopyClickEvent.AsObject,
        uiRecoveryCodesPrintClick?: UIRecoveryCodesPrintClickEvent.AsObject,
    }

    export enum EventCase {
        EVENT_NOT_SET = 0,
    
    USER_LOGIN = 3,

    SSO_CREATE = 4,

    RESOURCE_CREATE = 5,

    SESSION_START = 6,

    UI_BANNER_CLICK = 7,

    UI_ONBOARD_COMPLETE_GO_TO_DASHBOARD_CLICK = 9,

    UI_ONBOARD_ADD_FIRST_RESOURCE_CLICK = 10,

    UI_ONBOARD_ADD_FIRST_RESOURCE_LATER_CLICK = 11,

    UI_ONBOARD_SET_CREDENTIAL_SUBMIT = 12,

    UI_ONBOARD_REGISTER_CHALLENGE_SUBMIT = 13,

    UI_RECOVERY_CODES_CONTINUE_CLICK = 14,

    UI_RECOVERY_CODES_COPY_CLICK = 15,

    UI_RECOVERY_CODES_PRINT_CLICK = 16,

    }

}

export class SubmitEventResponse extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SubmitEventResponse.AsObject;
    static toObject(includeInstance: boolean, msg: SubmitEventResponse): SubmitEventResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SubmitEventResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SubmitEventResponse;
    static deserializeBinaryFromReader(message: SubmitEventResponse, reader: jspb.BinaryReader): SubmitEventResponse;
}

export namespace SubmitEventResponse {
    export type AsObject = {
    }
}

export class HelloTeleportRequest extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): HelloTeleportRequest.AsObject;
    static toObject(includeInstance: boolean, msg: HelloTeleportRequest): HelloTeleportRequest.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: HelloTeleportRequest, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): HelloTeleportRequest;
    static deserializeBinaryFromReader(message: HelloTeleportRequest, reader: jspb.BinaryReader): HelloTeleportRequest;
}

export namespace HelloTeleportRequest {
    export type AsObject = {
    }
}

export class HelloTeleportResponse extends jspb.Message { 

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): HelloTeleportResponse.AsObject;
    static toObject(includeInstance: boolean, msg: HelloTeleportResponse): HelloTeleportResponse.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: HelloTeleportResponse, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): HelloTeleportResponse;
    static deserializeBinaryFromReader(message: HelloTeleportResponse, reader: jspb.BinaryReader): HelloTeleportResponse;
}

export namespace HelloTeleportResponse {
    export type AsObject = {
    }
}
